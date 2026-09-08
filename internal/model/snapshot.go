package model

import "time"

// DNSRecordSnapshot 解析记录快照（Zone 全量记录的某一时刻副本）。
type DNSRecordSnapshot struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	AccountID  uint      `gorm:"not null;index:idx_snap_zone" json:"account_id"`
	Zone       string    `gorm:"size:255;not null;index:idx_snap_zone" json:"zone"`
	Source     string    `gorm:"size:16;not null;default:view" json:"source"` // view 查看时 | manual 手动 | drift 漂移留存
	RecordJSON string    `gorm:"type:text;not null" json:"-"`                 // provider.RecordInfo 数组
	Count      int       `json:"count"`                                       // 记录条数
	Note       string    `gorm:"size:255" json:"note"`
	CreatedBy  uint      `json:"created_by"` // 0 = 系统定时任务
	CreatedAt  time.Time `gorm:"index" json:"created_at"`
}

// SystemSetting 系统 KV 设置。
type SystemSetting struct {
	Key       string    `gorm:"primaryKey;size:64" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
