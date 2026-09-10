package model

import "time"

// AlertChannel 通知渠道。
type AlertChannel struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"size:64;not null" json:"name"`
	Type      string    `gorm:"size:32;not null" json:"type"`     // webhook | dingtalk | wecom | email | telegram
	Config    string    `gorm:"type:text;not null" json:"config"` // JSON，按 type 定义字段
	Enabled   bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertRule 告警规则。
type AlertRule struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Name       string    `gorm:"size:64;not null" json:"name"`
	Kind       string    `gorm:"size:32;not null;default:domain_expire" json:"kind"` // domain_expire
	Offsets    string    `gorm:"size:128;not null;default:60,30,7,1" json:"offsets"` // 逗号分隔的提前天数
	ChannelIDs string    `gorm:"size:255" json:"channel_ids"`                        // JSON 数组 [1,2]
	Enabled    bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AlertLog 告警发送记录（按 域名+档位+到期年份 去重）。
type AlertLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	DomainID   uint      `gorm:"not null;uniqueIndex:idx_domain_offset_year" json:"domain_id"`
	DomainName string    `gorm:"size:255" json:"domain_name"`
	Offset     int       `gorm:"not null;uniqueIndex:idx_domain_offset_year" json:"offset"`
	ExpYear    int       `gorm:"not null;uniqueIndex:idx_domain_offset_year" json:"exp_year"`
	Channels   string    `gorm:"size:255" json:"channels"`
	Message    string    `gorm:"type:text" json:"message"`
	SentAt     time.Time `json:"sent_at"`
}
