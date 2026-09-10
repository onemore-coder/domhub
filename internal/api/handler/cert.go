package handler

import (
	"context"
	"strconv"
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

// List GET /api/v1/certs —— 全部监控主机的证书状态。
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Minute)
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

// AddManual POST /api/v1/certs {"host": "api.example.com", "domain_name": "example.com"}
// 手动添加监控主机（自动发现覆盖不到的场景，如外部 CDN 域名）。
func (h *CertHandler) AddManual(c *gin.Context) {
	var req struct {
		Host       string `json:"host"`
		DomainName string `json:"domain_name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Host == "" {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误：host 必填"})
		return
	}
	cs, err := h.svc.AddManual(req.Host, req.DomainName)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": cs})
}

// Delete DELETE /api/v1/certs/:id —— 删除监控条目。
func (h *CertHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

// SetExcluded PUT /api/v1/certs/:id/excluded {"excluded": true}
// 排除后不探测、不告警；自动发现不会重新加入。
func (h *CertHandler) SetExcluded(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	var req struct {
		Excluded bool `json:"excluded"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.svc.SetExcluded(uint(id), req.Excluded); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}
