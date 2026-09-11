package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	// 云厂商 Provider 自注册
	_ "github.com/domhub-io/domhub/internal/provider/aliyun"
	_ "github.com/domhub-io/domhub/internal/provider/aws"
	_ "github.com/domhub-io/domhub/internal/provider/cloudflare"
	_ "github.com/domhub-io/domhub/internal/provider/tencent"

	"github.com/domhub-io/domhub/internal/api"
	"github.com/domhub-io/domhub/internal/bootstrap"
	"github.com/domhub-io/domhub/internal/job"
	"github.com/domhub-io/domhub/internal/pkg/config"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
	web "github.com/domhub-io/domhub/web"
)

func main() {
	cfg := config.Load()
	logger.Init(cfg.Server.Mode)

	db := bootstrap.InitDB(cfg)
	bootstrap.Seed(db, cfg)

	// 内嵌前端资源；dist 缺失时（纯后端开发）不托管静态文件
	var staticFS fs.FS
	if sub, err := fs.Sub(web.Dist, "dist"); err == nil {
		if f, err := sub.Open("index.html"); err == nil {
			_ = f.Close()
			staticFS = sub
		} else {
			logger.L().Warn("前端资源未构建（web/dist/index.html 不存在），仅提供 API")
		}
	}

	// 服务层（路由与定时任务共用）
	cipher, err := cryptox.New(cfg.Crypto.Key)
	if err != nil {
		panic("凭证加密器初始化失败: " + err.Error())
	}
	accountRepo := repo.NewCloudAccountRepo(db)
	domainRepo := repo.NewDomainRepo(db)
	alertRepo := repo.NewAlertRepo(db)
	auditRepo := repo.NewAuditRepo(db)
	grantRepo := repo.NewGrantRepo(db)

	alertSvc := service.NewAlertService(alertRepo, domainRepo)
	accountSvc := service.NewCloudAccountService(accountRepo, domainRepo, repo.NewSyncTaskRepo(db), cipher)
	dnsSvc := service.NewDNSService(accountRepo, cipher, auditRepo, grantRepo)
	zoneSvc := service.NewZoneService(accountRepo, repo.NewZoneRepo(db), repo.NewDnsRecordRepo(db), grantRepo, dnsSvc)
	snapshotSvc := service.NewSnapshotService(repo.NewSnapshotRepo(db), alertRepo, dnsSvc)
	certSvc := service.NewCertService(repo.NewCertRepo(db), domainRepo, alertRepo, repo.NewDnsRecordRepo(db))
	acmeSvc := service.NewAcmeService(
		repo.NewAcmeAccountRepo(db), repo.NewIssuedCertRepo(db),
		repo.NewZoneRepo(db), dnsSvc, accountRepo, cipher)

	// 定时任务：到期检查 / 台账同步 / 漂移检测 / Zone 缓存刷新 / 证书检查 / 证书自动续期
	// 计划存 system_settings（设置页可改，热生效）；缺省值见 service.scheduleDefaults
	scheduler := job.NewScheduler(job.Runners{
		AlertSvc:    alertSvc,
		AccountSvc:  accountSvc,
		SnapshotSvc: snapshotSvc,
		ZoneSvc:     zoneSvc,
		CertSvc:     certSvc,
		AcmeSvc:     acmeSvc,
	})
	specs, err := service.NewSettingsService(repo.NewSettingRepo(db)).AllSchedules()
	if err != nil {
		logger.L().Error("读取任务计划失败", zap.Error(err))
	} else if err := scheduler.ApplySchedules(specs); err != nil {
		logger.L().Error("注册定时任务失败", zap.Error(err))
	}
	scheduler.Start()
	defer scheduler.Stop()

	// 启动预热：Zone 缓存为空时立即刷新，避免重启后归属/搜索/授权整片空窗
	go zoneSvc.WarmupIfEmpty()

	r := api.NewRouter(db, cfg, staticFS, scheduler)

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: r,
	}

	go func() {
		logger.L().Info("DomHub 服务启动",
			zap.Int("port", cfg.Server.Port),
			zap.String("mode", cfg.Server.Mode))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.L().Error("服务启动失败", zap.Error(err))
			os.Exit(1)
		}
	}()

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.L().Info("正在关闭服务...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.L().Error("服务关闭异常", zap.Error(err))
	}
	logger.L().Info("服务已退出")
}
