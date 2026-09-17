// Package middleware Gin 中间件。
package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/pkg/jwtx"
)

const (
	CtxUserID   = "userID"
	CtxUsername = "username"
	CtxRole     = "role"
)

// RoleLookup 按用户 ID 查询角色与状态（用于兼容无 role 声明的旧 token）。
// 返回 (role, active)；active=false 表示账号被禁用。
type RoleLookup func(userID uint) (role string, active bool)

// ApiTokenLookup API Token 鉴权回调：明文 token → 用户三要素。
type ApiTokenLookup func(token string) (userID uint, username, role string, ok bool)

// JWT 校验 Bearer Token，将用户信息注入上下文。
// 支持两种凭据：
//   - JWT（登录会话）
//   - API Token（dht_ 前缀，走 apiTokenLookup，供 CI/自动化脚本使用）
//
// lookup 非 nil 时：旧 token（无 role 声明）会回源数据库补齐角色，
// 同时校验账号是否被禁用，禁用用户的 token 立即失效。
func JWT(secret string, lookup RoleLookup, apiTokenLookup ApiTokenLookup) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 网关兼容：部分托管平台/CDN 网关会在转发时改写 Authorization 头
		// （塞入自身边缘凭据）甚至丢弃非标准自定义头。收集全部凭据候选，
		// 逐个尝试解析，任一成功即通过：
		// X-Api-Key 头 → Authorization 头 → domhub_token Cookie。
		var candidates []string
		if xk := c.GetHeader("X-Api-Key"); xk != "" {
			candidates = append(candidates, xk)
		}
		if auth := c.GetHeader("Authorization"); auth != "" {
			candidates = append(candidates, strings.TrimPrefix(auth, "Bearer "))
		}
		if ck, err := c.Cookie("domhub_token"); err == nil && ck != "" {
			candidates = append(candidates, ck)
		}
		if len(candidates) == 0 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "未登录"})
			return
		}

		// API Token 路径：任一候选命中 dht_ 前缀即走 API Token 鉴权
		for _, cand := range candidates {
			if !strings.HasPrefix(cand, "dht_") {
				continue
			}
			if apiTokenLookup == nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "API Token 未启用"})
				return
			}
			uid, username, role, ok := apiTokenLookup(cand)
			if !ok {
				continue
			}
			c.Set(CtxUserID, uid)
			c.Set(CtxUsername, username)
			c.Set(CtxRole, role)
			c.Next()
			return
		}

		var claims *jwtx.Claims
		for _, cand := range candidates {
			if parsed, err := jwtx.ParseToken(cand, secret); err == nil {
				claims = parsed
				break
			}
		}
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "登录已失效，请重新登录"})
			return
		}
		// 2FA 登录中间态预令牌：只能用于完成两步验证接口，不能访问业务接口
		if claims.Typ == "2fa" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "请先完成两步验证"})
			return
		}

		role := claims.Role
		if role == "" && lookup != nil {
			// 兼容 M3 之前签发的旧 token：回源补齐角色
			dbRole, active := lookup(claims.UserID)
			if !active {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "账号已被禁用，请联系管理员"})
				return
			}
			role = dbRole
		}

		c.Set(CtxUserID, claims.UserID)
		c.Set(CtxUsername, claims.Username)
		c.Set(CtxRole, role)
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
