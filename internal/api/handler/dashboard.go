package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/repo"
)

// DashboardHandler 仪表盘接口。
type DashboardHandler struct {
	accounts *repo.CloudAccountRepo
	domains  *repo.DomainRepo
}

func NewDashboardHandler(accounts *repo.CloudAccountRepo, domains *repo.DomainRepo) *DashboardHandler {
	return &DashboardHandler{accounts: accounts, domains: domains}
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
