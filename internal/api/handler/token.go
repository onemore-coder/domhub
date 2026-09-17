package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/service"
)

// TokenHandler 个人 API Token 管理（仅操作自己的令牌）。
type TokenHandler struct {
	svc *service.TokenService
}

func NewTokenHandler(svc *service.TokenService) *TokenHandler {
	return &TokenHandler{svc: svc}
}

// List GET /api/v1/tokens —— 当前用户的令牌列表。
func (h *TokenHandler) List(c *gin.Context) {
	actor := ctxActor(c)
	list, err := h.svc.List(actor.ID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if list == nil {
		list = []model.ApiToken{}
	}
	// TokenHash 带 json:"-"，输出天然不含哈希
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

// Create POST /api/v1/tokens {"name":"ci","expire_days":30}
// 明文 token 仅在本响应返回一次。
func (h *TokenHandler) Create(c *gin.Context) {
	actor := ctxActor(c)
	var req struct {
		Name       string `json:"name"`
		ExpireDays int    `json:"expire_days"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	res, err := h.svc.Create(service.CreateInput{
		UserID: actor.ID, Username: actor.Username, Name: req.Name, ExpireDays: req.ExpireDays,
	})
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": res})
}

// Revoke DELETE /api/v1/tokens/:id
func (h *TokenHandler) Revoke(c *gin.Context) {
	actor := ctxActor(c)
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.svc.Revoke(uint(id), actor.ID); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": "吊销失败"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已吊销"})
}
