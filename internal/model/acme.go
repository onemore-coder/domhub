package model

import "time"

// AcmeAccount ACME 客户端账户（按 CA 目录 + 邮箱唯一）。
// 账户私钥加密存储；RegistrationURI 为 CA 返回的账户资源定位。
type AcmeAccount struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	DirectoryURL    string    `gorm:"size:255;uniqueIndex:idx_acme_uniq,priority:1" json:"directory_url"`
	Email           string    `gorm:"size:255;uniqueIndex:idx_acme_uniq,priority:2" json:"email"`
	KeyEnc          string    `gorm:"type:text" json:"-"` // 账户私钥 PEM（AES-GCM 加密）
	RegistrationURI string    `gorm:"size:255" json:"registration_uri"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

func (AcmeAccount) TableName() string { return "acme_accounts" }

// IssuedCert 证书申请/签发记录（证书申请一期）。
//
// 私钥 AES-GCM 加密落库；证书链为公开数据明文存储。
// 申请流程异步执行：创建记录（pending）→ 后台跑 ACME → issued/failed。
type IssuedCert struct {
	ID            uint       `gorm:"primaryKey" json:"id"`
	PrimaryDomain string     `gorm:"size:255;index" json:"primary_domain"` // 主域名（CN）
	SANs          string     `gorm:"size:1023" json:"sans"`                // 附加域名，逗号分隔（含主域名）
	DNSAccountID  uint       `gorm:"index" json:"dns_account_id"`          // DNS-01 使用的云账号
	DNSProvider   string     `gorm:"size:32" json:"dns_provider"`          // 冗余展示
	AcmeEmail     string     `gorm:"size:255" json:"acme_email"`           // ACME 账户联系邮箱
	DirectoryURL  string     `gorm:"size:255" json:"directory_url"`        // ACME CA 目录
	CAName        string     `gorm:"size:64" json:"ca_name"`               // 展示名：Let's Encrypt (staging) 等
	Status        string     `gorm:"size:16;index" json:"status"`          // pending / issued / failed / renewing
	AutoRenew     bool       `json:"auto_renew"`
	CertChain     string     `gorm:"type:text" json:"-"` // 叶子证书 + 中级链 PEM（明文，公开数据）
	PrivateKeyEnc string     `gorm:"type:text" json:"-"` // 私钥 PEM（加密）
	NotBefore     *time.Time `json:"not_before"`
	NotAfter      *time.Time `json:"not_after"`
	LastMessage   string     `gorm:"size:1023" json:"last_message"` // 最近一次结果/进度摘要
	ProgressLog   string     `gorm:"type:text" json:"progress_log"` // 申请过程流水（换行分隔）
	RenewCount    int        `json:"renew_count"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (IssuedCert) TableName() string { return "issued_certs" }

// 证书申请状态常量。
const (
	CertApplyPending  = "pending"
	CertApplyIssued   = "issued"
	CertApplyFailed   = "failed"
	CertApplyRenewing = "renewing"
)
