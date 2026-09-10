package model

import "time"

// AuditLog 操作审计日志。
type AuditLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"user_id"`
	Username  string    `gorm:"size:64" json:"username"`
	Action    string    `gorm:"size:64;not null;index" json:"action"` // dns.create / dns.update / dns.delete / dns.push / account.create ...
	Resource  string    `gorm:"size:256;index" json:"resource"`       // 如 aliyun/daydayops.com/A www
	Detail    string    `gorm:"type:text" json:"detail"`              // 变更前后 JSON
	Status    string    `gorm:"size:32;not null" json:"status"`       // success | failed
	Message   string    `gorm:"size:512" json:"message"`
	CreatedAt time.Time `gorm:"index" json:"created_at"`
}
