package model

import "time"

// CertStatus 域名 TLS 证书状态（每次检查覆盖更新）。
type CertStatus struct {
	ID       uint       `gorm:"primaryKey" json:"id"`
	DomainID uint       `gorm:"not null;uniqueIndex" json:"domain_id"`
	Name     string     `gorm:"size:255;not null" json:"name"` // 冗余域名，便于展示
	NotAfter *time.Time `json:"not_after"`                     // 证书到期时间
	Issuer   string     `gorm:"size:255" json:"issuer"`        // 签发者
	Subject  string     `gorm:"size:255" json:"subject"`       // 证书主体
	DaysLeft int        `json:"days_left"`                     // 剩余天数（检查失败为 -1）
	OK       bool       `gorm:"not null" json:"ok"`            // 检测到有效 HTTPS 服务
	Error    string     `gorm:"size:255" json:"error"`         // 检查失败原因
	// 已发送告警的档位（当前证书 NotAfter 内去重），JSON 数组如 [30,7]
	AlertedOffsets string    `gorm:"size:128" json:"alerted_offsets"`
	CheckedAt      time.Time `json:"checked_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (CertStatus) TableName() string { return "cert_statuses" }
