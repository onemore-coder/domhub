package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/service"
)

// OAuthHandler GitHub OAuth 登录接口。
type OAuthHandler struct {
	svc *service.OAuthService
}

func NewOAuthHandler(svc *service.OAuthService) *OAuthHandler {
	return &OAuthHandler{svc: svc}
}

// Providers GET /auth/oauth/providers —— 登录页判断是否展示 GitHub 按钮。
func (h *OAuthHandler) Providers(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"github": h.svc.Enabled(),
	}})
}

// GitHubStart GET /auth/oauth/github —— 302 跳转 GitHub 授权页。
func (h *OAuthHandler) GitHubStart(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if p := c.GetHeader("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	base := scheme + "://" + c.Request.Host
	authURL, _, err := h.svc.AuthURL(base)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.Redirect(http.StatusFound, authURL)
}

// GitHubCallback GET /auth/oauth/github/callback —— 授权回调，签发 JWT 后重定回前端。
func (h *OAuthHandler) GitHubCallback(c *gin.Context) {
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if p := c.GetHeader("X-Forwarded-Proto"); p != "" {
		scheme = p
	}
	frontend := scheme + "://" + c.Request.Host

	if errCode := c.Query("error"); errCode != "" {
		c.Redirect(http.StatusFound, frontend+"/login?oauth_error="+errCode)
		return
	}
	code, state := c.Query("code"), c.Query("state")
	if code == "" || state == "" {
		c.Redirect(http.StatusFound, frontend+"/login?oauth_error=invalid")
		return
	}
	token, _, err := h.svc.Exchange(c.Request.Context(), code, state, frontend)
	if err != nil {
		c.Redirect(http.StatusFound, frontend+"/login?oauth_error="+err.Error())
		return
	}
	c.Redirect(http.StatusFound, frontend+"/oauth/callback#token="+token)
}
