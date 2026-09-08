package handler

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/service"
)

// SettingsHandler 系统设置接口。
type SettingsHandler struct {
	svc *service.SettingsService
	db  *gorm.DB
}

func NewSettingsHandler(svc *service.SettingsService, db *gorm.DB) *SettingsHandler {
	return &SettingsHandler{svc: svc, db: db}
}

// Get GET /settings —— 任务计划 + 系统信息。
func (h *SettingsHandler) Get(c *gin.Context) {
	schedules, err := h.svc.AllSchedules()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"schedules": schedules,
		"info":      h.svc.Info(h.db),
	}})
}

// Update PUT /settings/schedules {"expiry_check_cron":"...","sync_domains_cron":"-","drift_check_cron":"..."}
func (h *SettingsHandler) Update(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil || len(req) == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.svc.UpdateSchedules(req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已保存并生效"})
}
