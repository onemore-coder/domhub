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
	"github.com/domhub-io/domhub/internal/pkg/config"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
)

// NewRouter 构建 gin 引擎。
func NewRouter(db *gorm.DB, cfg *config.Config, staticFS fs.FS) *gin.Engine {
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

	dashH := handler.NewDashboardHandler(accountRepo, domainRepo)

	accountSvc := service.NewCloudAccountService(accountRepo, domainRepo, taskRepo, cipher)
	alertSvc := service.NewAlertService(alertRepo, domainRepo)
	dnsSvc := service.NewDNSService(accountRepo, cipher, auditRepo)

	accountH := handler.NewCloudAccountHandler(accountSvc)
	domainH := handler.NewDomainHandler(domainRepo, accountSvc)
	alertH := handler.NewAlertHandler(alertRepo, alertSvc)
	dnsH := handler.NewDNSHandler(dnsSvc)
	auditH := handler.NewAuditHandler(auditRepo)

	auth := api.Group("/auth")
	{
		auth.POST("/login", authH.Login)
		auth.POST("/logout", authH.Logout)
	}

	protected := api.Group("")
	protected.Use(middleware.JWT(cfg.JWT.Secret))
	{
		protected.GET("/auth/me", authH.Me)
		protected.GET("/dashboard/summary", dashH.Summary)

		protected.GET("/accounts", accountH.List)
		protected.POST("/accounts", accountH.Create)
		protected.PUT("/accounts/:id", accountH.Update)
		protected.DELETE("/accounts/:id", accountH.Delete)
		protected.POST("/accounts/:id/check", accountH.Check)
		protected.POST("/accounts/:id/sync", accountH.Sync)

		protected.GET("/domains", domainH.List)
		protected.POST("/domains/sync", domainH.SyncAll)

		protected.GET("/channels", alertH.ListChannels)
		protected.POST("/channels", alertH.CreateChannel)
		protected.PUT("/channels/:id", alertH.UpdateChannel)
		protected.DELETE("/channels/:id", alertH.DeleteChannel)

		protected.GET("/alert-rules", alertH.ListRules)
		protected.POST("/alert-rules", alertH.CreateRule)
		protected.PUT("/alert-rules/:id", alertH.UpdateRule)
		protected.DELETE("/alert-rules/:id", alertH.DeleteRule)

		protected.POST("/alerts/check", alertH.RunCheck)
		protected.GET("/alerts/logs", alertH.ListLogs)

		// M2：DNS 解析管理 + 审计
		protected.GET("/dns/zones", dnsH.ListZones)
		protected.GET("/dns/records", dnsH.ListRecords)
		protected.POST("/dns/records", dnsH.CreateRecord)
		protected.PUT("/dns/records", dnsH.UpdateRecord)
		protected.DELETE("/dns/records", dnsH.DeleteRecord)
		protected.POST("/dns/plan", dnsH.Plan)
		protected.POST("/dns/push", dnsH.Push)

		protected.GET("/audit-logs", auditH.List)
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
