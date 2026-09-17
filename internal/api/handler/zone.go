package handler

import (
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
)

// ZoneHandler 托管域名缓存接口。
type ZoneHandler struct {
	svc *service.ZoneService
}

func NewZoneHandler(svc *service.ZoneService) *ZoneHandler {
	return &ZoneHandler{svc: svc}
}

// ListCached GET /api/v1/dns/zones —— 缓存视图（admin 全量，其他角色按授权过滤）。
func (h *ZoneHandler) ListCached(c *gin.Context) {
	views, err := h.svc.ListCached(ctxActor(c))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if views == nil {
		views = []repo.ZoneView{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": views})
}

// Refresh POST /api/v1/dns/zones/refresh {"account_id": 0} —— 同步 Zone 缓存（0=全部账号）。
func (h *ZoneHandler) Refresh(c *gin.Context) {
	var req struct {
		AccountID uint `json:"account_id"`
	}
	_ = c.ShouldBindJSON(&req) // body 可省略，缺省刷新全部
	accounts, zones, err := h.svc.Refresh(req.AccountID)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"accounts": accounts, "zones": zones,
	}})
}

// Search GET /api/v1/search?q=xxx —— 跨 Zone 全局搜索（顶栏搜索框）。
// 命中两类：托管 Zone（本地缓存）与解析记录（本地镜像，按主机/记录值模糊）。
// 非 admin 用户自动限定在其授权范围内。
func (h *ZoneHandler) Search(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"zones": []repo.ZoneView{}, "records": []any{}}})
		return
	}
	op := ctxActor(c)

	zones, err := h.svc.ListCached(op)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	kw := strings.ToLower(q)
	zoneHits := make([]repo.ZoneView, 0, 5)
	for _, z := range zones {
		if strings.Contains(strings.ToLower(z.Name), kw) {
			zoneHits = append(zoneHits, z)
			if len(zoneHits) >= 5 {
				break
			}
		}
	}

	// 记录命中：走授权感知的跨 Zone 镜像查询，并补充账号名便于展示与跳转
	records, err := h.svc.ListRecordsCached(op, 0, "", q, "", 10)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	views, _ := h.svc.ListCached(op)
	accName := map[uint]string{}
	for _, v := range views {
		accName[v.CloudAccountID] = v.AccountName
	}
	type recordHit struct {
		ID             uint   `json:"id"`
		CloudAccountID uint   `json:"cloud_account_id"`
		AccountName    string `json:"account_name"`
		ZoneName       string `json:"zone_name"`
		Name           string `json:"name"`
		Type           string `json:"type"`
		Value          string `json:"value"`
		TTL            int    `json:"ttl"`
	}
	hits := make([]recordHit, 0, len(records))
	for _, r := range records {
		hits = append(hits, recordHit{
			ID: r.ID, CloudAccountID: r.CloudAccountID,
			AccountName: accName[r.CloudAccountID], ZoneName: r.ZoneName,
			Name: r.Name, Type: r.Type, Value: r.Value, TTL: r.TTL,
		})
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"zones": zoneHits, "records": hits,
	}})
}
