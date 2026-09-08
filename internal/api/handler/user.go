package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/api/middleware"
	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/service"
)

// UserHandler 用户管理与授权接口（admin）。
type UserHandler struct {
	userSvc *service.UserService
	auth    *service.AuthService
}

func NewUserHandler(userSvc *service.UserService, auth *service.AuthService) *UserHandler {
	return &UserHandler{userSvc: userSvc, auth: auth}
}

// List GET /api/v1/users
func (h *UserHandler) List(c *gin.Context) {
	list, err := h.userSvc.List()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

type createUserReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// Create POST /api/v1/users
func (h *UserHandler) Create(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	uid, username, _ := middleware.CtxUser(c)
	u, err := h.userSvc.Create(req.Username, req.Password, req.Role, uid, username)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "创建成功", "data": u})
}

type updateUserReq struct {
	Role     string `json:"role"`
	Status   *int   `json:"status"`
	Password string `json:"password"` // 重置密码，留空不改
}

// Update PUT /api/v1/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	uid, username, _ := middleware.CtxUser(c)
	if err := h.userSvc.Update(uint(id), req.Role, req.Status, req.Password, uid, username); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "更新成功"})
}

// Delete DELETE /api/v1/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	uid, username, _ := middleware.CtxUser(c)
	if err := h.userSvc.Delete(uint(id), uid, username); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "删除成功"})
}

// Grants GET /api/v1/users/:id/zones
func (h *UserHandler) Grants(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := h.userSvc.Grants(uint(id))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

type setGrantsReq struct {
	Grants []struct {
		CloudAccountID uint   `json:"account_id"`
		Zone           string `json:"zone"`
	} `json:"grants"`
}

// SetGrants PUT /api/v1/users/:id/zones
func (h *UserHandler) SetGrants(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req setGrantsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	items := make([]model.UserZone, 0, len(req.Grants))
	for _, g := range req.Grants {
		if g.Zone == "" || g.CloudAccountID == 0 {
			continue
		}
		items = append(items, model.UserZone{
			CloudAccountID: g.CloudAccountID,
			Zone:           g.Zone,
		})
	}
	uid, username, _ := middleware.CtxUser(c)
	if err := h.userSvc.SetGrants(uint(id), items, uid, username); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "授权已更新"})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword POST /api/v1/users/me/password（所有登录用户）
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	uid, _, _ := middleware.CtxUser(c)
	if err := h.auth.ChangePassword(uid, req.OldPassword, req.NewPassword); err != nil {
		if errors.Is(err, service.ErrUserNotFound) {
			c.JSON(401, gin.H{"code": 401, "message": err.Error()})
			return
		}
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "密码已修改"})
}
