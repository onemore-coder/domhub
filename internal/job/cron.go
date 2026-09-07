// Package job 定时任务调度。
package job

import (
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/service"
)

// Scheduler 定时任务调度器。
type Scheduler struct {
	c *cron.Cron
}

// Start 启动定时任务。
// checkCron：到期检查（默认每天 09:00）；syncCron：台账自动同步（默认关闭）。
func Start(alertSvc *service.AlertService, accountSvc *service.CloudAccountService, checkCron, syncCron string) *Scheduler {
	c := cron.New(cron.WithSeconds())

	if checkCron != "" {
		if _, err := c.AddFunc(checkCron, func() {
			sent, err := alertSvc.RunExpiryCheck(nil)
			if err != nil {
				logger.L().Error("到期检查任务失败", zap.Error(err))
				return
			}
			if sent > 0 {
				logger.L().Info("到期检查完成", zap.Int("alerts_sent", sent))
			}
		}); err != nil {
			logger.L().Error("到期检查任务注册失败", zap.String("cron", checkCron), zap.Error(err))
		} else {
			logger.L().Info("已注册到期检查定时任务", zap.String("cron", checkCron))
		}
	}

	if syncCron != "" {
		if _, err := c.AddFunc(syncCron, func() {
			logger.L().Info("开始定时同步域名台账")
			accountSvc.SyncAll()
		}); err != nil {
			logger.L().Error("台账同步任务注册失败", zap.String("cron", syncCron), zap.Error(err))
		} else {
			logger.L().Info("已注册台账同步定时任务", zap.String("cron", syncCron))
		}
	}

	c.Start()
	return &Scheduler{c: c}
}

// Stop 停止调度（等待进行中的任务结束）。
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}
