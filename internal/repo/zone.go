package repo

import (
	"time"

	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
)

// ZoneView 缓存 Zone + 归属账号信息（join 查询结果）。
type ZoneView struct {
	ZoneID        uint      `json:"id"`
	CloudAccountID uint     `json:"cloud_account_id"`
	AccountName   string    `json:"account_name"`
	Provider      string    `json:"provider"`
	Name          string    `json:"name"`
	RecordCount   int       `json:"record_count"`
	SyncedAt      time.Time `json:"synced_at"`
}

// ZoneRepo Zone 元数据缓存仓库。
type ZoneRepo struct {
	db *gorm.DB
}

func NewZoneRepo(db *gorm.DB) *ZoneRepo { return &ZoneRepo{db: db} }

// UpsertBatch 批量写入/更新某账号的 Zone 缓存，并删除本轮未出现的过期条目。
// batchStart 用于识别陈旧数据：本轮同步前已存在、且本轮未刷新到的即已从厂商侧消失。
func (r *ZoneRepo) UpsertBatch(accountID uint, zones []model.Zone, batchStart time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range zones {
			zones[i].CloudAccountID = accountID
			zones[i].SyncedAt = time.Now()
			if err := tx.Where(model.Zone{CloudAccountID: accountID, Name: zones[i].Name}).
				Assign(map[string]any{"record_count": zones[i].RecordCount, "synced_at": zones[i].SyncedAt}).
				FirstOrCreate(&zones[i]).Error; err != nil {
				return err
			}
		}
		// 清理已从厂商侧消失的 Zone
		if err := tx.Where("cloud_account_id = ? AND synced_at < ?", accountID, batchStart).
			Delete(&model.Zone{}).Error; err != nil {
			return err
		}
		return nil
	})
}

// ListViews 全量缓存视图（含账号信息），按账号+域名排序。
func (r *ZoneRepo) ListViews() ([]ZoneView, error) {
	var out []ZoneView
	err := r.db.Table("zones").
		Select("zones.id AS zone_id, zones.cloud_account_id, cloud_accounts.name AS account_name, "+
			"cloud_accounts.provider, zones.name, zones.record_count, zones.synced_at").
		Joins("JOIN cloud_accounts ON cloud_accounts.id = zones.cloud_account_id AND cloud_accounts.status = 1").
		Order("zones.cloud_account_id, zones.name").
		Scan(&out).Error
	return out, err
}

// CountByAccount 某账号缓存条数（刷新前后对比可用）。
func (r *ZoneRepo) CountByAccount(accountID uint) (int64, error) {
	var n int64
	err := r.db.Model(&model.Zone{}).Where("cloud_account_id = ?", accountID).Count(&n).Error
	return n, err
}
