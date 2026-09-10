package handler

import (
	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
)

// ZoneHandler 托管域名缓存接口。
type ZoneHandler struct {
	svc *service.ZoneService
}

func NewZoneHandler(svc *service.ZoneService) *ZoneHandler {
	return &ZoneHandler{svc: svc}
}

// ListCached GET /api/v1/dns/zones —— 缓存视图（admin 全量，其他角色按授权过滤）。
func (h *ZoneHandler) ListCached(c *gin.Context) {
	views, err := h.svc.ListCached(ctxActor(c))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if views == nil {
		views = []repo.ZoneView{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": views})
}

// Refresh POST /api/v1/dns/zones/refresh {"account_id": 0} —— 同步 Zone 缓存（0=全部账号）。
func (h *ZoneHandler) Refresh(c *gin.Context) {
	var req struct {
		AccountID uint `json:"account_id"`
	}
	_ = c.ShouldBindJSON(&req) // body 可省略，缺省刷新全部
	accounts, zones, err := h.svc.Refresh(req.AccountID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"accounts": accounts, "zones": zones,
	}})
}
