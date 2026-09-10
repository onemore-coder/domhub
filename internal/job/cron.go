// Package job 定时任务调度。
package job

import (
	"fmt"
	"strings"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"

	"github.com/domhub-io/domhub/internal/pkg/cronutil"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/service"
)

// Job 名称常量（对应 system_settings 的 key）。
const (
	JobExpiryCheck = "expiry_check" // 域名到期检查
	JobSyncDomains = "sync_domains" // 域名台账自动同步
	JobDriftCheck  = "drift_check"  // DNS 漂移检测
	JobSyncZones   = "sync_zones"   // 托管域名元数据缓存刷新
)

// Scheduler 定时任务调度器。
type Scheduler struct {
	c       *cron.Cron
	runners Runners
	entries map[string]cron.EntryID
}

// Runners 各任务的可执行体。
type Runners struct {
	AlertSvc    *service.AlertService
	AccountSvc  *service.CloudAccountService
	SnapshotSvc *service.SnapshotService
	ZoneSvc     *service.ZoneService
}

// NewScheduler 创建（不启动）调度器。
func NewScheduler(runners Runners) *Scheduler {
	return &Scheduler{
		c:       cron.New(cron.WithSeconds()),
		runners: runners,
		entries: make(map[string]cron.EntryID),
	}
}

func (s *Scheduler) jobFunc(name string) func() {
	switch name {
	case JobExpiryCheck:
		return func() {
			sent, err := s.runners.AlertSvc.RunExpiryCheck(nil)
			if err != nil {
				logger.L().Error("到期检查任务失败", zap.Error(err))
				return
			}
			if sent > 0 {
				logger.L().Info("到期检查完成", zap.Int("alerts_sent", sent))
			}
		}
	case JobSyncDomains:
		return func() {
			logger.L().Info("开始定时同步域名台账")
			s.runners.AccountSvc.SyncAll()
		}
	case JobDriftCheck:
		return func() {
			n, err := s.runners.SnapshotSvc.RunDriftCheck(nil)
			if err != nil {
				logger.L().Error("漂移检测任务失败", zap.Error(err))
				return
			}
			if n > 0 {
				logger.L().Warn("漂移检测发现配置偏离", zap.Int("zones", n))
			} else {
				logger.L().Info("漂移检测完成，无偏离")
			}
		}
	case JobSyncZones:
		return func() {
			accs, zones, err := s.runners.ZoneSvc.Refresh(0)
			if err != nil {
				logger.L().Error("Zone 缓存刷新任务失败", zap.Error(err))
				return
			}
			logger.L().Info("Zone 缓存刷新完成",
				zap.Int("accounts", accs), zap.Int("zones", zones))
		}
	}
	return nil
}

// ApplySchedules 注册/更新任务计划（实现 service.ScheduleApplier）。
// spec 为空或 "-" 表示禁用该任务；支持 5 位或 7 位 cron 表达式。
// specs 可只传变更项；其余保持不变。
// key 兼容任务名（expiry_check）与 settings 键（expiry_check_cron）两种写法。
func (s *Scheduler) ApplySchedules(specs map[string]string) error {
	for key, spec := range specs {
		name := strings.TrimSuffix(key, "_cron")
		fn := s.jobFunc(name)
		if fn == nil {
			return fmt.Errorf("未知任务: %s", name)
		}
		if spec == "" || spec == "-" {
			if id, exists := s.entries[name]; exists {
				s.c.Remove(id)
				delete(s.entries, name)
				logger.L().Info("已停用定时任务", zap.String("job", name))
			}
			continue
		}
		normalized := cronutil.Normalize(spec)
		if err := cronutil.Validate(normalized); err != nil {
			return fmt.Errorf("任务 %s 的表达式无效: %w", name, err)
		}
		// 先移除旧计划再注册
		if id, exists := s.entries[name]; exists {
			s.c.Remove(id)
		}
		id, err := s.c.AddFunc(normalized, fn)
		if err != nil {
			return err
		}
		s.entries[name] = id
		logger.L().Info("已注册定时任务", zap.String("job", name), zap.String("spec", normalized))
	}
	return nil
}

// Start 启动调度器。
func (s *Scheduler) Start() {
	s.c.Start()
}

// Stop 停止调度（等待进行中的任务结束）。
func (s *Scheduler) Stop() {
	ctx := s.c.Stop()
	<-ctx.Done()
}
