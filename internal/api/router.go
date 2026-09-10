// Package router 路由注册与静态资源托管。
package api

import (
	"io/fs"
	"net/http"
	"strings"

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

	dashH := handler.NewDashboardHandler(accountRepo, domainRepo)

	accountSvc := service.NewCloudAccountService(accountRepo, domainRepo, taskRepo, cipher)
	alertSvc := service.NewAlertService(alertRepo, domainRepo)
	dnsSvc := service.NewDNSService(accountRepo, cipher, auditRepo, grantRepo)
	zoneSvc := service.NewZoneService(accountRepo, repo.NewZoneRepo(db), grantRepo, dnsSvc)
	userSvc := service.NewUserService(repo.NewUserRepo(db), grantRepo, auditRepo)
	snapshotSvc := service.NewSnapshotService(repo.NewSnapshotRepo(db), alertRepo, dnsSvc)
	settingsSvc := service.NewSettingsService(repo.NewSettingRepo(db))
	if scheduleApplier != nil {
		settingsSvc.SetScheduler(scheduleApplier) // 设置页保存任务计划后热生效
	}

	accountH := handler.NewCloudAccountHandler(accountSvc)
	domainH := handler.NewDomainHandler(domainRepo, accountSvc)
	alertH := handler.NewAlertHandler(alertRepo, alertSvc)
	dnsH := handler.NewDNSHandler(dnsSvc)
	auditH := handler.NewAuditHandler(auditRepo)
	userH := handler.NewUserHandler(userSvc, authSvc)
	snapshotH := handler.NewSnapshotHandler(snapshotSvc)
	settingsH := handler.NewSettingsHandler(settingsSvc, db)
	zoneH := handler.NewZoneHandler(zoneSvc)

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
	protected.Use(middleware.JWT(cfg.JWT.Secret, userLookup))
	{
		protected.GET("/auth/me", authH.Me)
		protected.GET("/dashboard/summary", dashH.Summary)

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

	// M2：DNS 解析管理（写权限在 service 层按 Zone 授权判定）
	// zones 走本地缓存（秒开），refresh 回源厂商 API；记录操作仍实时
	protected.GET("/dns/zones", zoneH.ListCached)
	protected.POST("/dns/zones/refresh", writeAccess, zoneH.Refresh)
	protected.GET("/dns/records", dnsH.ListRecords)
	protected.POST("/dns/records", writeAccess, dnsH.CreateRecord)
	protected.PUT("/dns/records", writeAccess, dnsH.UpdateRecord)
	protected.DELETE("/dns/records", writeAccess, dnsH.DeleteRecord)
	protected.POST("/dns/plan", dnsH.Plan)
	protected.POST("/dns/push", writeAccess, dnsH.Push)

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
