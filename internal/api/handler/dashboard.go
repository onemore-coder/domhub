package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// DashboardHandler 仪表盘接口（M0 占位，后续接真实统计）。
type DashboardHandler struct{}

func NewDashboardHandler() *DashboardHandler { return &DashboardHandler{} }

// Summary GET /api/v1/dashboard/summary
func (h *DashboardHandler) Summary(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"domain_total":    0,
		"zone_total":      0,
		"account_total":   0,
		"expiring_in_30d": 0,
	}})
}
