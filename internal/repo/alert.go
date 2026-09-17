package repo

import (
	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
)

// AlertRepo 告警渠道 / 规则 / 日志数据访问。
type AlertRepo struct {
	db *gorm.DB
}

func NewAlertRepo(db *gorm.DB) *AlertRepo { return &AlertRepo{db: db} }

// ---- 渠道 ----

func (r *AlertRepo) CreateChannel(c *model.AlertChannel) error { return r.db.Create(c).Error }
func (r *AlertRepo) UpdateChannel(c *model.AlertChannel) error { return r.db.Save(c).Error }
func (r *AlertRepo) DeleteChannel(id uint) error               { return r.db.Delete(&model.AlertChannel{}, id).Error }

func (r *AlertRepo) ListChannels() ([]model.AlertChannel, error) {
	var list []model.AlertChannel
	err := r.db.Order("id").Find(&list).Error
	return list, err
}

func (r *AlertRepo) FindChannel(id uint) (*model.AlertChannel, error) {
	var c model.AlertChannel
	err := r.db.First(&c, id).Error
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *AlertRepo) FindChannelsByIDs(ids []uint) ([]model.AlertChannel, error) {
	var list []model.AlertChannel
	if len(ids) == 0 {
		return list, nil
	}
	err := r.db.Where("id IN ? AND enabled = ?", ids, true).Find(&list).Error
	return list, err
}

// ---- 规则 ----

func (r *AlertRepo) CreateRule(rule *model.AlertRule) error { return r.db.Create(rule).Error }
func (r *AlertRepo) UpdateRule(rule *model.AlertRule) error { return r.db.Save(rule).Error }
func (r *AlertRepo) DeleteRule(id uint) error               { return r.db.Delete(&model.AlertRule{}, id).Error }

func (r *AlertRepo) ListRules() ([]model.AlertRule, error) {
	var list []model.AlertRule
	err := r.db.Order("id").Find(&list).Error
	return list, err
}

// ListEnabledRules 查询启用的规则（到期检查任务用）。
func (r *AlertRepo) ListEnabledRules() ([]model.AlertRule, error) {
	var list []model.AlertRule
	err := r.db.Where("enabled = ?", true).Find(&list).Error
	return list, err
}

// ---- 日志 ----

// HasLog 是否已发送过（域名+档位+到期年份 去重）。
func (r *AlertRepo) HasLog(domainID uint, offset, expYear int) (bool, error) {
	var n int64
	err := r.db.Model(&model.AlertLog{}).
		Where("domain_id = ? AND offset = ? AND exp_year = ?", domainID, offset, expYear).
		Count(&n).Error
	return n > 0, err
}

func (r *AlertRepo) CreateLog(log *model.AlertLog) error { return r.db.Create(log).Error }

// ListLogs 最近的告警记录。
func (r *AlertRepo) ListLogs(limit int) ([]model.AlertLog, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var list []model.AlertLog
	err := r.db.Order("sent_at DESC").Limit(limit).Find(&list).Error
	return list, err
}
