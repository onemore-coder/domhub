package model

import "time"

// Domain 域名资产台账（本地缓存，来自云厂商同步）。
type Domain struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CloudAccountID uint       `gorm:"not null;uniqueIndex:idx_account_name_kind" json:"cloud_account_id"`
	Name           string     `gorm:"size:255;not null;uniqueIndex:idx_account_name_kind" json:"name"`
	Kind           string     `gorm:"size:16;not null;default:domain;uniqueIndex:idx_account_name_kind" json:"kind"` // domain | zone
	Provider       string     `gorm:"size:32;not null;index" json:"provider"`
	Registrar      string     `gorm:"size:128" json:"registrar"`
	Status         string     `gorm:"size:64" json:"status"`
	RegisteredAt   *time.Time `json:"registered_at"`
	ExpireAt       *time.Time `json:"expire_at"`            // 空 = 未知（如托管 Zone）
	Tags           string     `gorm:"size:255" json:"tags"` // 逗号分隔
	Remark         string     `gorm:"size:255" json:"remark"`
	LastSyncedAt   time.Time  `json:"last_synced_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
