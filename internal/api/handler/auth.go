// Package handler HTTP 处理器。
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/api/dto"
	"github.com/onemore-coder/domhub/internal/api/middleware"
	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/service"
)

// AuthHandler 认证相关接口。
type AuthHandler struct {
	auth *service.AuthService
}

func NewAuthHandler(auth *service.AuthService) *AuthHandler { return &AuthHandler{auth: auth} }

// Login POST /api/v1/auth/login
// 开启两步验证的用户：密码校验通过后返回 require_2fa + pre_token（5 分钟），
// 前端凭其调用 POST /auth/2fa/verify 完成登录。
func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	user, token, err := h.auth.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.Err2FARequired) {
			pre, perr := h.auth.Login2FAPreToken(req.Username, req.Password)
			if perr != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": perr.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
				"require_2fa": true, "pre_token": pre,
			}})
			return
		}
		if errors.Is(err, service.ErrInvalidCredentials) || errors.Is(err, service.ErrUserDisabled) {
			c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "服务器内部错误"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": dto.LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	}})
}

// Verify2FA POST /api/v1/auth/2fa/verify {pre_token, code}
func (h *AuthHandler) Verify2FA(c *gin.Context) {
	var req struct {
		PreToken string `json:"pre_token"`
		Code     string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.PreToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	user, token, err := h.auth.Verify2FALogin(req.PreToken, req.Code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": dto.LoginResponse{
		Token: token,
		User:  toUserResponse(user),
	}})
}

// Setup2FA POST /api/v1/auth/2fa/setup —— 生成密钥与二维码（未启用状态）。
func (h *AuthHandler) Setup2FA(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	secret, qr, err := h.auth.Setup2FA(uid)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"secret": secret, "qr": qr,
	}})
}

// Enable2FA POST /api/v1/auth/2fa/enable {code}
func (h *AuthHandler) Enable2FA(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.auth.Enable2FA(c.GetUint(middleware.CtxUserID), req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "两步验证已开启"})
}

// Disable2FA POST /api/v1/auth/2fa/disable {code}
func (h *AuthHandler) Disable2FA(c *gin.Context) {
	var req struct {
		Code string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.auth.Disable2FA(c.GetUint(middleware.CtxUserID), req.Code); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "两步验证已关闭"})
}

// Me GET /api/v1/auth/me
func (h *AuthHandler) Me(c *gin.Context) {
	uid := c.GetUint(middleware.CtxUserID)
	user, err := h.auth.GetByID(uid)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "用户不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": toUserResponse(user)})
}

// Logout POST /api/v1/auth/logout （无状态 JWT，前端清除 token 即可）
func (h *AuthHandler) Logout(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

func toUserResponse(u *model.User) dto.UserResponse {
	return dto.UserResponse{
		ID:          u.ID,
		Username:    u.Username,
		Role:        u.Role,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		TotpEnabled: u.TotpEnabled,
	}
}
