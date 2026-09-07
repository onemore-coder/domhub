// Package model 定义 GORM 数据模型。
package model

import "time"

// User 本地用户账号。
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:128;not null" json:"-"`
	Role         string     `gorm:"size:32;not null;default:admin" json:"role"` // admin | viewer
	Status       int        `gorm:"not null;default:1" json:"status"`           // 1 启用 0 禁用
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}
