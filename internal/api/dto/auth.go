// Package dto API 请求/响应结构。
package dto

import "time"

// LoginRequest 登录请求。
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应。
type LoginResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

// UserResponse 用户信息（脱敏）。
type UserResponse struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Role        string     `json:"role"`
	Status      int        `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
}
