// Package middleware Gin 中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/pkg/jwtx"
)

const (
	CtxUserID   = "userID"
	CtxUsername = "username"
	CtxRole     = "role"
)

// JWT 校验 Bearer Token，将用户信息注入上下文。
func JWT(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
			return
		}
		token := strings.TrimPrefix(auth, "Bearer ")
		claims, err := jwtx.ParseToken(token, secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "登录已失效，请重新登录"})
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, claims.Role)
		c.Next()
	}
}

// RequireRole 校验当前用户角色，仅允许列出的角色通过。
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		role, _ := c.Get(CtxRole)
		rs, _ := role.(string)
		if !allowed[rs] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "权限不足（角色: " + orDefault(rs, "未知") + "）"})
			return
		}
		c.Next()
	}
}

// CtxUser 取当前登录用户三要素。
func CtxUser(c *gin.Context) (uint, string, string) {
	id, _ := c.Get(CtxUserID)
	uid, _ := id.(uint)
	username, _ := c.Get(CtxUsername)
	un, _ := username.(string)
	role, _ := c.Get(CtxRole)
	rs, _ := role.(string)
	return uid, un, rs
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
