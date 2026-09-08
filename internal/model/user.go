package model

import "time"

// User 本地用户账号。
type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:64;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:128;not null" json:"-"`
	Role         string     `gorm:"size:32;not null;default:admin" json:"role"` // admin | operator | viewer
	Status       int        `gorm:"not null;default:1" json:"status"`           // 1 启用 0 禁用
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// 角色常量。
const (
	RoleAdmin    = "admin"    // 全部权限
	RoleOperator = "operator" // DNS/台账操作（受 Zone 授权约束），不能管理用户与云账号
	RoleViewer   = "viewer"   // 只读
)

// UserZone Zone 授权：非 admin 用户可访问的 账号+Zone 组合。
type UserZone struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	UserID         uint      `gorm:"not null;uniqueIndex:idx_user_account_zone" json:"user_id"`
	CloudAccountID uint      `gorm:"not null;uniqueIndex:idx_user_account_zone" json:"cloud_account_id"`
	Zone           string    `gorm:"size:253;not null;uniqueIndex:idx_user_account_zone" json:"zone"`
	CreatedAt      time.Time `json:"created_at"`
}
