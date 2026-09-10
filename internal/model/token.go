package model

import "time"

// ApiToken API 访问令牌（用于 CI/自动化脚本调用 DomHub API）。
// 明文仅在创建时返回一次，库里只存 SHA-256 哈希。
type ApiToken struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	UserID      uint       `gorm:"not null;index" json:"user_id"`
	Name        string     `gorm:"size:64;not null" json:"name"`
	TokenHash   string     `gorm:"size:64;not null;uniqueIndex" json:"-"` // sha256 hex
	Prefix      string     `gorm:"size:16;not null" json:"prefix"`        // 明文前缀（dht_ + 8 位），用于列表辨识
	ExpireAt    *time.Time `json:"expire_at"`                             // nil = 永不过期
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Token 明文前缀（与 ApiToken.Prefix 一致），鉴权中间件据此识别 API Token。
const ApiTokenPrefix = "dht_"
