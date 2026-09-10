package model

import "time"

// SyncTask 同步任务记录。
type SyncTask struct {
	ID             uint       `gorm:"primaryKey" json:"id"`
	CloudAccountID uint       `gorm:"not null;index" json:"cloud_account_id"`
	Type           string     `gorm:"size:32;not null" json:"type"`   // sync
	Status         string     `gorm:"size:32;not null" json:"status"` // running | success | failed
	Message        string     `gorm:"size:512" json:"message"`
	DomainCount    int        `json:"domain_count"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
}
