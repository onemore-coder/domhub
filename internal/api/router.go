// Package router 路由注册与静态资源托管。
package api

import (
	"context"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/api/handler"
	"github.com/domhub-io/domhub/internal/api/middleware"
	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/config"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
)

// NewRouter 构建 gin 引擎。scheduleApplier 由 job.Scheduler 实现（可 nil），
// 用于设置页更新任务计划后热生效。
func NewRouter(db *gorm.DB, cfg *config.Config, staticFS fs.FS, scheduleApplier service.ScheduleApplier) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)

	r := gin.New()
	r.Use(gin.Recovery(), requestLogger())

	// API v1
	api := r.Group("/api/v1")

	authSvc := service.NewAuthService(
		repo.NewUserRepo(db), cfg.JWT.Secret, cfg.JWT.ExpireHours)
	authH := handler.NewAuthHandler(authSvc)

	// M1：云账号 / 域名台账 / 告警
	cipher, err := cryptox.New(cfg.Crypto.Key)
	if err != nil {
		panic("初始化凭证加密器失败: " + err.Error())
	}
	accountRepo := repo.NewCloudAccountRepo(db)
	domainRepo := repo.NewDomainRepo(db)
	alertRepo := repo.NewAlertRepo(db)
	taskRepo := repo.NewSyncTaskRepo(db)
	auditRepo := repo.NewAuditRepo(db)
	grantRepo := repo.NewGrantRepo(db)

	dashH := handler.NewDashboardHandler(accountRepo, domainRepo, auditRepo)

	accountSvc := service.NewCloudAccountService(accountRepo, domainRepo, taskRepo, cipher)
	alertSvc := service.NewAlertService(alertRepo, domainRepo)
	dnsSvc := service.NewDNSService(accountRepo, cipher, auditRepo, grantRepo)
	zoneSvc := service.NewZoneService(accountRepo, repo.NewZoneRepo(db), repo.NewDnsRecordRepo(db), grantRepo, dnsSvc)
	// 云端写成功后回源刷新该 Zone 的解析记录镜像（云端优先，失败不影响操作结果）
	dnsSvc.OnChange = func(accountID uint, zone string) {
		if _, err := zoneSvc.SyncRecordsFor(accountID, zone); err != nil {
			logger.L().Warn("刷新解析记录镜像失败",
				zap.Uint("account", accountID), zap.String("zone", zone), zap.Error(err))
		}
	}
	userSvc := service.NewUserService(repo.NewUserRepo(db), grantRepo, auditRepo)
	snapshotSvc := service.NewSnapshotService(repo.NewSnapshotRepo(db), alertRepo, dnsSvc)
	settingsSvc := service.NewSettingsService(repo.NewSettingRepo(db))
	tokenSvc := service.NewTokenService(repo.NewApiTokenRepo(db), repo.NewUserRepo(db))
	certSvc := service.NewCertService(repo.NewCertRepo(db), domainRepo, alertRepo, repo.NewDnsRecordRepo(db))
	acmeSvc := service.NewAcmeService(
		repo.NewAcmeAccountRepo(db), repo.NewIssuedCertRepo(db),
		repo.NewZoneRepo(db), dnsSvc, accountRepo, cipher)
	// 证书部署（三期）：签发/续期成功后自动下发到 CDN / SSH 主机
	deploySvc := service.NewCertDeployService(
		repo.NewCertDeployRepo(db), repo.NewIssuedCertRepo(db),
		accountRepo, auditRepo, cipher, alertRepo)
	acmeSvc.OnIssued = func(certID uint) { go deploySvc.RunForCert(certID) }
	if scheduleApplier != nil {
		settingsSvc.SetScheduler(scheduleApplier) // 设置页保存任务计划后热生效
	}

	accountH := handler.NewCloudAccountHandler(accountSvc)
	domainH := handler.NewDomainHandler(domainRepo, accountSvc, repo.NewZoneRepo(db))
	alertH := handler.NewAlertHandler(alertRepo, alertSvc)
	dnsH := handler.NewDNSHandler(dnsSvc, zoneSvc, certSvc)
	auditH := handler.NewAuditHandler(auditRepo)
	userH := handler.NewUserHandler(userSvc, authSvc)
	snapshotH := handler.NewSnapshotHandler(snapshotSvc)
	settingsH := handler.NewSettingsHandler(settingsSvc, db)
	zoneH := handler.NewZoneHandler(zoneSvc)
	certH := handler.NewCertHandler(certSvc)
	acmeH := handler.NewAcmeHandler(acmeSvc)
	templateH := handler.NewTemplateHandler(repo.NewRecordTemplateRepo(db), dnsSvc, zoneSvc)
	deployH := handler.NewDeployHandler(deploySvc)

	adminOnly := middleware.RequireRole(model.RoleAdmin)
	writeAccess := middleware.RequireRole(model.RoleAdmin, model.RoleOperator)

	auth := api.Group("/auth")
	{
		auth.POST("/login", authH.Login)
		auth.POST("/logout", authH.Logout)
		// M4：GitHub OAuth（无需 JWT）
		oauthSvc := service.NewOAuthService(
			service.GitHubOAuthConf{
				Enabled:      cfg.OAuth.GitHub.Enabled,
				ClientID:     cfg.OAuth.GitHub.ClientID,
				ClientSecret: cfg.OAuth.GitHub.ClientSecret,
			},
			repo.NewUserRepo(db), db, cfg.JWT.Secret, cfg.JWT.ExpireHours)
		oauthH := handler.NewOAuthHandler(oauthSvc)
		auth.GET("/oauth/providers", oauthH.Providers)
		auth.GET("/oauth/github", oauthH.GitHubStart)
		auth.GET("/oauth/github/callback", oauthH.GitHubCallback)
	}

	protected := api.Group("")
	// 旧 token 无 role 声明时回源数据库补齐，并校验账号启用状态
	userLookup := func(uid uint) (string, bool) {
		u, err := repo.NewUserRepo(db).FindByID(uid)
		if err != nil {
			return "", false
		}
		return u.Role, u.Status == 1
	}
	protected.Use(middleware.JWT(cfg.JWT.Secret, userLookup, tokenSvc.Resolve))
	{
		protected.GET("/auth/me", authH.Me)
		protected.GET("/dashboard/summary", dashH.Summary)
		protected.GET("/dashboard/stats", dashH.Stats)

		// 云账号：读所有登录用户可见（AK 已脱敏），写需 operator+；用户/凭证管理 admin 专属
		accountGroup := protected.Group("/accounts")
		{
			accountGroup.GET("", accountH.List)
			accountGroup.POST("", writeAccess, accountH.Create)
			accountGroup.PUT("/:id", writeAccess, accountH.Update)
			accountGroup.DELETE("/:id", adminOnly, accountH.Delete)
			accountGroup.POST("/:id/check", writeAccess, accountH.Check)
			accountGroup.POST("/:id/sync", writeAccess, accountH.Sync)
		}

		protected.GET("/domains", domainH.List)
		protected.POST("/domains/sync", writeAccess, domainH.SyncAll)

		channelGroup := protected.Group("/channels")
		{
			channelGroup.GET("", alertH.ListChannels)
			channelGroup.POST("", adminOnly, alertH.CreateChannel)
			channelGroup.POST("/test", writeAccess, alertH.TestChannel)
			channelGroup.POST("/:id/test", writeAccess, alertH.TestChannelByID)
			channelGroup.PUT("/:id", adminOnly, alertH.UpdateChannel)
			channelGroup.DELETE("/:id", adminOnly, alertH.DeleteChannel)
		}

		ruleGroup := protected.Group("/alert-rules")
		{
			ruleGroup.GET("", alertH.ListRules)
			ruleGroup.POST("", adminOnly, alertH.CreateRule)
			ruleGroup.PUT("/:id", adminOnly, alertH.UpdateRule)
			ruleGroup.DELETE("/:id", adminOnly, alertH.DeleteRule)
		}

		protected.POST("/alerts/check", writeAccess, alertH.RunCheck)
		protected.GET("/alerts/logs", alertH.ListLogs)

		// M6：SSL 证书监控（读需登录，写需 operator+）
		protected.GET("/certs", certH.List)
		protected.POST("/certs", writeAccess, certH.AddManual)
		protected.POST("/certs/check", writeAccess, certH.RunCheck)
		protected.POST("/certs/:id/check", writeAccess, certH.CheckOne)
		protected.PUT("/certs/:id/excluded", writeAccess, certH.SetExcluded)
		protected.DELETE("/certs/:id", writeAccess, certH.Delete)

		// 证书申请（ACME 免费证书，DNS-01 复用云账号解析通道）
		protected.GET("/certs-issued/cas", acmeH.ListCAs)
		protected.GET("/certs-issued", acmeH.List)
		protected.GET("/certs-issued/:id", acmeH.Get)
		protected.POST("/certs-issued/apply", writeAccess, acmeH.Apply)
		protected.POST("/certs-issued/:id/renew", writeAccess, acmeH.Renew)
		protected.PUT("/certs-issued/:id/auto-renew", writeAccess, acmeH.SetAutoRenew)
		protected.DELETE("/certs-issued/:id", writeAccess, acmeH.Delete)
		protected.GET("/certs-issued/:id/download", acmeH.Download)

		// 证书部署（CDN / SSH 主机下发）
		protected.GET("/certs-issued/:id/deploys", deployH.List)
		protected.POST("/certs-issued/:id/deploys", writeAccess, deployH.Save)
		protected.PUT("/certs/deploys/:id", writeAccess, deployH.Save)
		protected.DELETE("/certs/deploys/:id", writeAccess, deployH.Delete)
		protected.POST("/certs/deploys/:id/run", writeAccess, deployH.Run)

		// M2：DNS 解析管理（写权限在 service 层按 Zone 授权判定）
		// zones/records 走本地镜像（秒开），sync 回源厂商 API；记录操作仍实时
		protected.GET("/dns/zones", zoneH.ListCached)
		protected.POST("/dns/zones/refresh", writeAccess, zoneH.Refresh)
		protected.GET("/dns/records-cached", dnsH.ListCached)
		protected.POST("/dns/records/sync", writeAccess, dnsH.SyncRecords)
		protected.GET("/dns/records", dnsH.ListRecords)
		protected.POST("/dns/records", writeAccess, dnsH.CreateRecord)
		protected.PUT("/dns/records", writeAccess, dnsH.UpdateRecord)
		protected.DELETE("/dns/records", writeAccess, dnsH.DeleteRecord)
		protected.POST("/dns/plan", dnsH.Plan)
		protected.POST("/dns/push", writeAccess, dnsH.Push)

		// M6：解析记录模板（CRUD + 一键下发预览，执行复用 /dns/push）
		protected.GET("/record-templates", templateH.List)
		protected.POST("/record-templates", writeAccess, templateH.Create)
		protected.PUT("/record-templates/:id", writeAccess, templateH.Update)
		protected.DELETE("/record-templates/:id", writeAccess, templateH.Delete)
		protected.POST("/record-templates/:id/apply", writeAccess, templateH.Apply)

		// M6：跨 Zone 全局搜索（顶栏搜索框）
		protected.GET("/search", zoneH.Search)

		// M5：API Token（个人管理，dht_ 前缀凭据供 CI/自动化调用）
		tokenH := handler.NewTokenHandler(tokenSvc)
		tokenGroup := protected.Group("/tokens")
		{
			tokenGroup.GET("", tokenH.List)
			tokenGroup.POST("", writeAccess, tokenH.Create)
			tokenGroup.DELETE("/:id", tokenH.Revoke)
		}

		// M2：审计日志（admin 专属）
		protected.GET("/audit-logs", adminOnly, auditH.List)

		// M3：用户管理与个人改密
		protected.GET("/users", adminOnly, userH.List)
		protected.POST("/users", adminOnly, userH.Create)
		protected.PUT("/users/:id", adminOnly, userH.Update)
		protected.DELETE("/users/:id", adminOnly, userH.Delete)
		protected.GET("/users/:id/zones", adminOnly, userH.Grants)
		protected.PUT("/users/:id/zones", adminOnly, userH.SetGrants)
		protected.POST("/users/me/password", userH.ChangePassword)

		// M4：解析记录快照（读需登录，写需 operator+）
		protected.GET("/dns/snapshots", snapshotH.List)
		protected.GET("/dns/snapshots/:id", snapshotH.Get)
		protected.POST("/dns/snapshots", writeAccess, snapshotH.Capture)
		protected.POST("/dns/snapshots/diff", snapshotH.Diff)
		protected.POST("/dns/snapshots/restore-plan", writeAccess, snapshotH.RestorePlan)

		// M4：域名标签/备注 + 系统设置
		protected.PATCH("/domains/:id", writeAccess, domainH.UpdateMeta)
		protected.GET("/settings", adminOnly, settingsH.Get)
		protected.PUT("/settings/schedules", adminOnly, settingsH.Update)
	}

	// 健康检查（免鉴权）：探活 + DB 连通性；DB 不可用时返回 503
	r.GET("/healthz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if err := db.WithContext(ctx).Exec("SELECT 1").Error; err != nil {
			logger.L().Error("健康检查失败：数据库不可用", zap.Error(err))
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "degraded", "db": "error"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok", "db": "ok"})
	})

	// 前端静态资源（embed），非 /api 路径回退到 index.html（SPA）
	if staticFS != nil {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "接口不存在"})
				return
			}
			serveStatic(c, staticFS)
		})
	}

	return r
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logger.L().Info("http",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
		)
	}
}
