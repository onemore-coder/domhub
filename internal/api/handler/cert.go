package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/service"
)

// CertHandler SSL 证书监控接口。
type CertHandler struct {
	svc *service.CertService
}

func NewCertHandler(svc *service.CertService) *CertHandler {
	return &CertHandler{svc: svc}
}

// List GET /api/v1/certs —— 全部域名证书状态。
func (h *CertHandler) List(c *gin.Context) {
	list, err := h.svc.ListStatus()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if list == nil {
		list = []model.CertStatus{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"items": list, "total": len(list)}})
}

// RunCheck POST /api/v1/certs/check {"domain_id": 0} —— 手动检查（0=全部）。
func (h *CertHandler) RunCheck(c *gin.Context) {
	var req struct {
		DomainID uint `json:"domain_id"`
	}
	_ = c.ShouldBindJSON(&req)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	checked, alerted, err := h.svc.CheckDomains(ctx, req.DomainID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"checked": checked, "alerts_sent": alerted,
	}})
}
