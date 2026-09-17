package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/notify"
	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
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

// TestChannelPayload 测试发送的请求体。
type TestChannelPayload struct {
	Type   string `json:"type"`
	Config string `json:"config"`
}

// sendTest 用给定渠道类型/配置发送一条测试消息，返回错误（nil 表示成功）。
func sendTest(chType, config string) error {
	n, err := notify.Build(chType, config)
	if err != nil {
		return err
	}
	return n.Send("DomHub 测试消息", "这是一条来自 DomHub 的渠道测试消息，收到即表示渠道配置有效。")
}

// TestChannelByID POST /api/v1/channels/:id/test —— 用已保存配置发送测试消息。
func (h *AlertHandler) TestChannelByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	ch, err := h.alertRepo.FindChannel(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "渠道不存在"})
		return
	}
	if err := sendTest(ch.Type, ch.Config); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "发送失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "测试消息已发送，请查收"})
}

// TestChannel POST /api/v1/channels/test —— 保存前用表单配置发送测试消息。
func (h *AlertHandler) TestChannel(c *gin.Context) {
	var req TestChannelPayload
	if err := c.ShouldBindJSON(&req); err != nil || req.Type == "" || req.Config == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误：type/config 必填"})
		return
	}
	if err := sendTest(req.Type, req.Config); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 1, "message": "发送失败：" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "测试消息已发送，请查收"})
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
