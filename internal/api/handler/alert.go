package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
)

// AlertHandler 告警渠道 / 规则 / 检查接口。
type AlertHandler struct {
	alertRepo *repo.AlertRepo
	alertSvc  *service.AlertService
}

func NewAlertHandler(alertRepo *repo.AlertRepo, alertSvc *service.AlertService) *AlertHandler {
	return &AlertHandler{alertRepo: alertRepo, alertSvc: alertSvc}
}

// ---- 渠道 ----

// ListChannels GET /api/v1/channels
func (h *AlertHandler) ListChannels(c *gin.Context) {
	list, err := h.alertRepo.ListChannels()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"items": list, "total": len(list)}})
}

// CreateChannel POST /api/v1/channels
func (h *AlertHandler) CreateChannel(c *gin.Context) {
	var ch model.AlertChannel
	if err := c.ShouldBindJSON(&ch); err != nil || ch.Name == "" || ch.Type == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误：name/type 必填"})
		return
	}
	if err := h.alertRepo.CreateChannel(&ch); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": ch})
}

// UpdateChannel PUT /api/v1/channels/:id
func (h *AlertHandler) UpdateChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	existing, err := h.alertRepo.FindChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "渠道不存在"})
		return
	}
	var req model.AlertChannel
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	existing.Name = req.Name
	existing.Type = req.Type
	existing.Config = req.Config
	existing.Enabled = req.Enabled
	if err := h.alertRepo.UpdateChannel(existing); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// DeleteChannel DELETE /api/v1/channels/:id
func (h *AlertHandler) DeleteChannel(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	if err := h.alertRepo.DeleteChannel(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ---- 规则 ----

// ListRules GET /api/v1/alert-rules
func (h *AlertHandler) ListRules(c *gin.Context) {
	list, err := h.alertRepo.ListRules()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"items": list, "total": len(list)}})
}

// CreateRule POST /api/v1/alert-rules
func (h *AlertHandler) CreateRule(c *gin.Context) {
	var rule model.AlertRule
	if err := c.ShouldBindJSON(&rule); err != nil || rule.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误：name 必填"})
		return
	}
	if rule.Kind == "" {
		rule.Kind = "domain_expire"
	}
	if err := h.alertRepo.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": rule})
}

// UpdateRule PUT /api/v1/alert-rules/:id
func (h *AlertHandler) UpdateRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	var req model.AlertRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	rule := &model.AlertRule{
		ID: uint(id), Name: req.Name, Kind: req.Kind,
		Offsets: req.Offsets, ChannelIDs: req.ChannelIDs, Enabled: req.Enabled,
	}
	if err := h.alertRepo.UpdateRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// DeleteRule DELETE /api/v1/alert-rules/:id
func (h *AlertHandler) DeleteRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	if err := h.alertRepo.DeleteRule(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// ---- 检查与日志 ----

// RunCheck POST /api/v1/alerts/check 手动触发一轮到期检查。
func (h *AlertHandler) RunCheck(c *gin.Context) {
	sent, err := h.alertSvc.RunExpiryCheck(nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"alerts_sent": sent}})
}

// ListLogs GET /api/v1/alerts/logs
func (h *AlertHandler) ListLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.alertRepo.ListLogs(limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"items": list, "total": len(list)}})
}
