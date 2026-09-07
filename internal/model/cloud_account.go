package model

import "time"

// CloudAccount 云厂商账号（凭证加密存储）。
type CloudAccount struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Name         string     `gorm:"size:64;not null" json:"name"`
	Provider     string     `gorm:"size:32;not null;index" json:"provider"` // tencent | aliyun | aws
	AccessKey    string     `gorm:"size:512;not null" json:"access_key"`    // AES 加密存储
	SecretKey    string     `gorm:"size:512;not null" json:"-"`             // AES 加密存储，绝不返回
	Region       string     `gorm:"size:64" json:"region"`
	Status       int        `gorm:"not null;default:1" json:"status"` // 1 启用 0 禁用
	LastCheckAt  *time.Time `json:"last_check_at"`
	LastCheckOK  bool       `json:"last_check_ok"`
	LastCheckMsg string     `gorm:"size:512" json:"last_check_msg"`
	LastSyncAt   *time.Time `json:"last_sync_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
