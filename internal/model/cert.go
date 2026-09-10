package model

import "time"

// CertStatus 主机名级 TLS 证书状态（每次检查覆盖更新）。
// 监控对象包含注册域 apex 与从解析记录/快照发现的子域名。
type CertStatus struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Host     string `gorm:"size:255;not null;uniqueIndex" json:"host"` // 监控的完整主机名
	DomainID uint   `gorm:"index" json:"domain_id"`                    // 关联的主域 ID（0=手动添加）
	DomainName string `gorm:"size:255" json:"domain_name"`             // 所属主域（展示用）
	Source   string `gorm:"size:16;not null;default:auto" json:"source"` // auto 自动发现 | manual 手动添加
	Excluded bool   `gorm:"not null;default:false" json:"excluded"`    // 排除后不探测不告警

	NotAfter *time.Time `json:"not_after"` // 证书到期时间
	Issuer   string     `gorm:"size:255" json:"issuer"`
	Subject  string     `gorm:"size:255" json:"subject"`
	DaysLeft int        `json:"days_left"` // 剩余天数（检查失败为 -1）
	OK       bool       `gorm:"not null" json:"ok"`
	Error    string     `gorm:"size:255" json:"error"`

	// 已发送告警的档位（当前证书 NotAfter 内去重），JSON 数组如 [30,7]；
	// 证书更换（NotAfter 变化）后自动重置
	AlertedOffsets string    `gorm:"size:128" json:"alerted_offsets"`
	CheckedAt      time.Time `json:"checked_at"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (CertStatus) TableName() string { return "cert_statuses" }
