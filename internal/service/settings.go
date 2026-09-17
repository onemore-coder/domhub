package service

import (
	"fmt"
	"runtime/debug"
	"time"

	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/pkg/cronutil"
	"github.com/domhub-io/domhub/internal/pkg/version"
	"github.com/domhub-io/domhub/internal/repo"
)

// 可配置的定时任务 key 与默认值（7 位 cron，带秒）。
const (
	KeyExpiryCheckCron = "expiry_check_cron"
	KeySyncCron        = "sync_domains_cron"
	KeyDriftCheckCron  = "drift_check_cron"
	KeySyncZonesCron   = "sync_zones_cron"
	KeySyncRecordsCron = "sync_records_cron"
	KeyCertCheckCron   = "cert_check_cron"
	KeyCertRenewCron   = "cert_renew_cron"
)

// scheduleDefaults 任务默认计划（空串 = 默认禁用）。
var scheduleDefaults = map[string]string{
	KeyExpiryCheckCron: "0 0 9 * * *",    // 每天 09:00
	KeySyncCron:        "",               // 默认关闭
	KeyDriftCheckCron:  "",               // 默认关闭
	KeySyncZonesCron:   "0 0 */2 * * *",  // 默认每 2 小时刷新 Zone 缓存
	KeySyncRecordsCron: "0 30 */2 * * *", // 每 2 小时刷新解析记录镜像（与 Zone 错峰）
	KeyCertCheckCron:   "0 0 8 * * *",    // 每天 08:00 检查证书
	KeyCertRenewCron:   "0 30 8 * * *",   // 每天 08:30 自动续期快到期的已签发证书
}

// ScheduleApplier 设置更新后热生效（由 job.Scheduler 实现）。
type ScheduleApplier interface {
	ApplySchedules(specs map[string]string) error
}

// SettingsService 系统设置业务。
type SettingsService struct {
	settings  *repo.SettingRepo
	scheduler ScheduleApplier
}

func NewSettingsService(settings *repo.SettingRepo) *SettingsService {
	return &SettingsService{settings: settings}
}

// SetScheduler 绑定调度器（后置注入，避免循环依赖）。
func (s *SettingsService) SetScheduler(ap ScheduleApplier) {
	s.scheduler = ap
}

// Schedule 获取某任务计划，未配置返回默认值。
func (s *SettingsService) Schedule(key string) (string, error) {
	v, err := s.settings.Get(key)
	if err != nil {
		return "", err
	}
	if v == "" {
		return scheduleDefaults[key], nil
	}
	return v, nil
}

// AllSchedules 读取全部任务计划（值为 "-" 表示禁用）。
func (s *SettingsService) AllSchedules() (map[string]string, error) {
	out := make(map[string]string, len(scheduleDefaults))
	for k := range scheduleDefaults {
		v, err := s.Schedule(k)
		if err != nil {
			return nil, err
		}
		if v == "" {
			v = "-"
		}
		out[k] = v
	}
	return out, nil
}

// UpdateSchedules 更新任务计划并热生效。先整体校验再落库，避免半更新。
func (s *SettingsService) UpdateSchedules(specs map[string]string) error {
	for k, v := range specs {
		if _, ok := scheduleDefaults[k]; !ok {
			return fmt.Errorf("不支持的任务: %s", k)
		}
		if err := cronutil.Validate(v); err != nil {
			return fmt.Errorf("%s %w", k, err)
		}
	}
	for k, v := range specs {
		if err := s.settings.Set(k, v); err != nil {
			return err
		}
	}
	if s.scheduler != nil {
		if err := s.scheduler.ApplySchedules(specs); err != nil {
			return fmt.Errorf("计划已保存但热更新失败: %w", err)
		}
	}
	return nil
}

// SystemInfo 系统信息（设置页展示）。
type SystemInfo struct {
	Version   string    `json:"version"`
	GoVersion string    `json:"go_version"`
	StartedAt time.Time `json:"started_at"`
	DBEngine  string    `json:"db_engine"`
}

var startedAt = time.Now()

// Info 汇总系统信息。
func (s *SettingsService) Info(db *gorm.DB) SystemInfo {
	engine := "unknown"
	if db != nil && db.Dialector != nil {
		engine = db.Dialector.Name()
	}
	return SystemInfo{
		Version:   version.Version,
		GoVersion: goVersion(),
		StartedAt: startedAt,
		DBEngine:  engine,
	}
}

func goVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if ok {
		return bi.GoVersion
	}
	return "unknown"
}
