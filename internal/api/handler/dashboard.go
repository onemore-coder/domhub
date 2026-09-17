package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/repo"
)

// DashboardHandler 仪表盘接口。
type DashboardHandler struct {
	accounts *repo.CloudAccountRepo
	domains  *repo.DomainRepo
	audits   *repo.AuditRepo
}

func NewDashboardHandler(accounts *repo.CloudAccountRepo, domains *repo.DomainRepo, audits *repo.AuditRepo) *DashboardHandler {
	return &DashboardHandler{accounts: accounts, domains: domains, audits: audits}
}

// Summary GET /api/v1/dashboard/summary
func (h *DashboardHandler) Summary(c *gin.Context) {
	accountTotal, _ := h.accounts.Count()
	domainTotal, _ := h.domains.Count()
	zoneTotal, _ := h.domains.CountByKind("zone")
	expiring30, _ := h.domains.CountExpiringIn(30)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"domain_total":    domainTotal - zoneTotal,
		"zone_total":      zoneTotal,
		"account_total":   accountTotal,
		"expiring_in_30d": expiring30,
	}})
}

// Stats GET /api/v1/dashboard/stats —— 厂商分布 / 30 天到期时间线 / 最近变更流。
func (h *DashboardHandler) Stats(c *gin.Context) {
	providers, err := h.domains.CountByProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	timeline, err := h.domains.ExpiryTimeline(30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	recent, _, err := h.audits.List("", "", "", "", 1, 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"providers":       providers,
		"expiry_timeline": timeline,
		"recent_changes":  recent,
	}})
}
