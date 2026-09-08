package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/api/middleware"
	"github.com/domhub-io/domhub/internal/provider"
	"github.com/domhub-io/domhub/internal/service"
)

// DNSHandler 解析管理接口。
type DNSHandler struct {
	dnsSvc *service.DNSService
}

func NewDNSHandler(dnsSvc *service.DNSService) *DNSHandler {
	return &DNSHandler{dnsSvc: dnsSvc}
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
		ID: req.RecordID,
		Name: req.Name, Type: req.Type, Value: req.Value,
		TTL: req.TTL, Priority: req.Priority, Line: req.Line,
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
	AccountID uint                   `json:"account_id"`
	Zone      string                 `json:"zone"`
	Desired   []provider.RecordInfo  `json:"desired"`
	Actions   []service.PlanAction   `json:"actions"` // push 用
	Mode      string                 `json:"mode"`    // preview | push
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
