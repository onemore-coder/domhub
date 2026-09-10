package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/api/middleware"
	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/provider"
	"github.com/domhub-io/domhub/internal/service"
)

// DNSHandler 解析管理接口。
type DNSHandler struct {
	dnsSvc  *service.DNSService
	zoneSvc *service.ZoneService // 解析记录镜像（列表/同步）
	certSvc *service.CertService // 记录关联展示证书状态
}

func NewDNSHandler(dnsSvc *service.DNSService, zoneSvc *service.ZoneService, certSvc *service.CertService) *DNSHandler {
	return &DNSHandler{dnsSvc: dnsSvc, zoneSvc: zoneSvc, certSvc: certSvc}
}

func ctxActor(c *gin.Context) service.Actor {
	uid, username, role := middleware.CtxUser(c)
	return service.Actor{ID: uid, Username: username, Role: role}
}

// ListZones GET /api/v1/dns/zones?account_id=1
func (h *DNSHandler) ListZones(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	if accountID == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 account_id"})
		return
	}
	zones, err := h.dnsSvc.ListZones(uint(accountID), ctxActor(c))
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	if zones == nil {
		zones = []provider.ZoneInfo{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": zones})
}

// ListCached GET /api/v1/dns/records-cached?account_id=1&zone=example.com
// 读取本地镜像（秒开）；A/AAAA/CNAME 记录关联返回主机证书剩余天数。
func (h *DNSHandler) ListCached(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	zone := c.Query("zone")
	if accountID == 0 || zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 account_id 或 zone"})
		return
	}
	records, err := h.zoneSvc.ListRecordsCached(ctxActor(c), uint(accountID), zone, "", "", 0)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	// 关联证书状态：主机名 → 剩余天数（仅 A/AAAA/CNAME 且有检查结果的行）
	certMap := h.certSvc.MapByZone(zone)
	type recordItem struct {
		model.DnsRecord
		CertOK   bool `json:"cert_ok"`
		CertDays *int `json:"cert_days"`
	}
	items := make([]recordItem, 0, len(records))
	var lastSync time.Time
	for _, r := range records {
		item := recordItem{DnsRecord: r}
		if r.SyncedAt.After(lastSync) {
			lastSync = r.SyncedAt
		}
		if r.Type == "A" || r.Type == "AAAA" || r.Type == "CNAME" {
			if cs, ok := certMap[service.CertHost(r.Name, zone)]; ok && !cs.Excluded {
				days := cs.DaysLeft
				item.CertOK = cs.OK
				item.CertDays = &days
			}
		}
		items = append(items, item)
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"items":     items,
		"synced_at": lastSync,
	}})
}

// SyncRecords POST /api/v1/dns/records/sync {"account_id":1,"zone":"example.com"}
// 从云端回源刷新该 Zone 的记录镜像。
func (h *DNSHandler) SyncRecords(c *gin.Context) {
	var req struct {
		AccountID uint   `json:"account_id"`
		Zone      string `json:"zone"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountID == 0 || req.Zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "需要 account_id 与 zone"})
		return
	}
	if ok, err := h.zoneSvc.CheckZoneAccess(ctxActor(c), req.AccountID, req.Zone); err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	} else if !ok {
		c.JSON(403, gin.H{"code": 403, "message": "无权访问该域名（需要管理员授权）"})
		return
	}
	n, err := h.zoneSvc.SyncRecordsFor(req.AccountID, req.Zone)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"records": n}})
}

// ListRecords GET /api/v1/dns/records?account_id=1&zone=example.com
func (h *DNSHandler) ListRecords(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	zone := c.Query("zone")
	if accountID == 0 || zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 account_id 或 zone"})
		return
	}
	records, err := h.dnsSvc.ListRecords(uint(accountID), zone, ctxActor(c))
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	if records == nil {
		records = []provider.RecordInfo{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": records})
}

type recordReq struct {
	AccountID uint   `json:"account_id"`
	Zone      string `json:"zone"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	TTL       int    `json:"ttl"`
	Priority  int    `json:"priority"`
	Line      string `json:"line"`
	Proxied   *bool  `json:"proxied"`   // Cloudflare 橙云代理（nil 视为 false）
	RecordID  string `json:"record_id"` // update/delete 用
	Desc      string `json:"desc"`      // delete 审计用描述
}

// CreateRecord POST /api/v1/dns/records
func (h *DNSHandler) CreateRecord(c *gin.Context) {
	var req recordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	op := ctxActor(c)
	rec := provider.RecordInfo{
		Name: req.Name, Type: req.Type, Value: req.Value,
		TTL: req.TTL, Priority: req.Priority, Line: req.Line,
		Proxied: req.Proxied != nil && *req.Proxied,
	}
	id, err := h.dnsSvc.CreateRecord(req.AccountID, req.Zone, rec, op)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "创建成功", "data": gin.H{"record_id": id}})
}

// UpdateRecord PUT /api/v1/dns/records
func (h *DNSHandler) UpdateRecord(c *gin.Context) {
	var req recordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if req.RecordID == "" {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 record_id"})
		return
	}
	op := ctxActor(c)
	rec := provider.RecordInfo{
		ID:   req.RecordID,
		Name: req.Name, Type: req.Type, Value: req.Value,
		TTL: req.TTL, Priority: req.Priority, Line: req.Line,
		Proxied: req.Proxied != nil && *req.Proxied,
	}
	if err := h.dnsSvc.UpdateRecord(req.AccountID, req.Zone, rec, op); err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteRecord DELETE /api/v1/dns/records
func (h *DNSHandler) DeleteRecord(c *gin.Context) {
	var req recordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if req.RecordID == "" {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 record_id"})
		return
	}
	op := ctxActor(c)
	if err := h.dnsSvc.DeleteRecord(req.AccountID, req.Zone, req.RecordID, req.Desc, op); err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "删除成功"})
}

type planReq struct {
	AccountID uint                  `json:"account_id"`
	Zone      string                `json:"zone"`
	Desired   []provider.RecordInfo `json:"desired"`
	Actions   []service.PlanAction  `json:"actions"` // push 用
	Mode      string                `json:"mode"`    // preview | push
}

// Plan POST /api/v1/dns/plan
func (h *DNSHandler) Plan(c *gin.Context) {
	var req planReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	actual, err := h.dnsSvc.ListRecords(req.AccountID, req.Zone, ctxActor(c))
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	plan, err := service.BuildPlan(actual, req.Desired)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if plan == nil {
		plan = []service.PlanAction{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": plan})
}

// Push POST /api/v1/dns/push
func (h *DNSHandler) Push(c *gin.Context) {
	var req planReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	if len(req.Actions) == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "变更计划为空"})
		return
	}
	op := ctxActor(c)
	results, err := h.dnsSvc.Push(req.AccountID, req.Zone, req.Actions, op)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": results})
}

// httpCode 根据错误类型映射 HTTP 状态码。
func httpCode(err error) int {
	if err == nil {
		return 200
	}
	// 参数/不存在类错误 → 400；其余（上游 API 错误）→ 502
	msg := err.Error()
	for _, kw := range []string{"无效", "缺少", "不存在", "不支持的", "未找到", "解密失败"} {
		if len(kw) > 0 && contains(msg, kw) {
			return 400
		}
	}
	return 502
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
