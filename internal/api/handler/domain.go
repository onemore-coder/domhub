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
	domains    *repo.DomainRepo
	accountSvc *service.CloudAccountService
	zones      *repo.ZoneRepo // 域名↔Zone 关联视图：解析归属标注
}

func NewDomainHandler(domains *repo.DomainRepo, accountSvc *service.CloudAccountService, zones *repo.ZoneRepo) *DomainHandler {
	return &DomainHandler{domains: domains, accountSvc: accountSvc, zones: zones}
}

// List GET /api/v1/domains
// 返回 items 的同时附带 zone_owners：域名 → 托管其解析的账号列表，
// 用于台账页标注解析归属（注册在 A 家、解析托管在 B 家是多云常态）。
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

	// 解析归属映射：zone 名 → [{account_name, provider}]
	type zoneOwner struct {
		CloudAccountID uint   `json:"cloud_account_id"`
		AccountName    string `json:"account_name"`
		Provider       string `json:"provider"`
		RecordCount    int    `json:"record_count"`
	}
	owners := map[string][]zoneOwner{}
	if views, err := h.zones.ListViews(); err == nil {
		for _, v := range views {
			owners[v.Name] = append(owners[v.Name], zoneOwner{
				CloudAccountID: v.CloudAccountID,
				AccountName:    v.AccountName,
				Provider:       v.Provider,
				RecordCount:    v.RecordCount,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"items": list, "total": total, "zone_owners": owners,
	}})
}

// SyncAll POST /api/v1/domains/sync 全量同步所有账号。
func (h *DomainHandler) SyncAll(c *gin.Context) {
	go h.accountSvc.SyncAll()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "同步任务已触发"})
}

// UpdateMeta PATCH /api/v1/domains/:id —— 更新本地标签/备注。
func (h *DomainHandler) UpdateMeta(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效 ID"})
		return
	}
	var req struct {
		Tags   *string `json:"tags"` // 逗号分隔；传空串清空
		Remark *string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if err := h.domains.UpdateMeta(uint(id), req.Tags, req.Remark); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已保存"})
}
