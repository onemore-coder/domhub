package model

import "time"

// Zone 托管域名元数据缓存（列表走本地库秒开，操作仍实时调厂商 API）。
type Zone struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	CloudAccountID uint      `gorm:"not null;uniqueIndex:idx_account_zone" json:"cloud_account_id"`
	Name           string    `gorm:"size:255;not null;uniqueIndex:idx_account_zone" json:"name"` // 主域名，如 example.com
	RecordCount    int       `json:"record_count"`
	SyncedAt       time.Time `json:"synced_at"` // 本次缓存拉取时间（数据新鲜度）
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
