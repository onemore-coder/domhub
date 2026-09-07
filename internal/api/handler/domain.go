package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/repo"
	"github.com/domhub-io/domhub/internal/service"
)

// DomainHandler 域名台账接口。
type DomainHandler struct {
	domains     *repo.DomainRepo
	accountSvc  *service.CloudAccountService
}

func NewDomainHandler(domains *repo.DomainRepo, accountSvc *service.CloudAccountService) *DomainHandler {
	return &DomainHandler{domains: domains, accountSvc: accountSvc}
}

// List GET /api/v1/domains
func (h *DomainHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	expiring, _ := strconv.Atoi(c.Query("expiring_days"))
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)

	list, total, err := h.domains.List(repo.DomainFilter{
		Keyword:        c.Query("keyword"),
		Provider:       c.Query("provider"),
		CloudAccountID: uint(accountID),
		Kind:           c.Query("kind"),
		Tag:            c.Query("tag"),
		ExpiringDays:   expiring,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"items": list, "total": total,
	}})
}

// SyncAll POST /api/v1/domains/sync 全量同步所有账号。
func (h *DomainHandler) SyncAll(c *gin.Context) {
	go h.accountSvc.SyncAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "同步任务已触发"})
}
