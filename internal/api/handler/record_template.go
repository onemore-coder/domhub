package handler

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/provider"
	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
)

// TemplateHandler 解析记录模板接口：模板 CRUD + 一键下发（生成变更计划，执行复用 /dns/push）。
type TemplateHandler struct {
	templates *repo.RecordTemplateRepo
	dnsSvc    *service.DNSService
	zoneSvc   *service.ZoneService
}

func NewTemplateHandler(templates *repo.RecordTemplateRepo, dnsSvc *service.DNSService, zoneSvc *service.ZoneService) *TemplateHandler {
	return &TemplateHandler{templates: templates, dnsSvc: dnsSvc, zoneSvc: zoneSvc}
}

type templateReq struct {
	Name   string               `json:"name"`
	Remark string               `json:"remark"`
	Items  []model.TemplateItem `json:"items"`
}

var validRecordTypes = map[string]bool{
	"A": true, "AAAA": true, "CNAME": true, "TXT": true,
	"MX": true, "NS": true, "CAA": true, "SRV": true,
}

func (h *TemplateHandler) validateItems(items []model.TemplateItem) error {
	if len(items) == 0 {
		return errBadRequest("模板至少包含一条记录")
	}
	for i, it := range items {
		if strings.TrimSpace(it.Name) == "" {
			return errBadRequestf("第 %d 条记录缺少主机记录", i+1)
		}
		if !validRecordTypes[strings.ToUpper(it.Type)] {
			return errBadRequestf("第 %d 条记录类型不支持: %s", i+1, it.Type)
		}
		if strings.TrimSpace(it.Value) == "" {
			return errBadRequestf("第 %d 条记录缺少记录值", i+1)
		}
	}
	return nil
}

// List GET /api/v1/record-templates
func (h *TemplateHandler) List(c *gin.Context) {
	list, err := h.templates.List()
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if list == nil {
		list = []model.RecordTemplate{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

// Create POST /api/v1/record-templates
func (h *TemplateHandler) Create(c *gin.Context) {
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": 400, "message": "模板名称不能为空"})
		return
	}
	if err := h.validateItems(req.Items); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	raw, _ := json.Marshal(req.Items)
	t := &model.RecordTemplate{Name: strings.TrimSpace(req.Name), Remark: req.Remark, Items: string(raw)}
	if err := h.templates.Create(t); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "创建成功", "data": t})
}

// Update PUT /api/v1/record-templates/:id
func (h *TemplateHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	t, err := h.templates.FindByID(uint(id))
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	var req templateReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		c.JSON(400, gin.H{"code": 400, "message": "模板名称不能为空"})
		return
	}
	if err := h.validateItems(req.Items); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	raw, _ := json.Marshal(req.Items)
	t.Name, t.Remark, t.Items = strings.TrimSpace(req.Name), req.Remark, string(raw)
	if err := h.templates.Update(t); err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已更新"})
}

// Delete DELETE /api/v1/record-templates/:id
func (h *TemplateHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.templates.Delete(uint(id)); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已删除"})
}

type applyTemplateReq struct {
	AccountID uint   `json:"account_id"`
	Zone      string `json:"zone"`
}

// Apply POST /api/v1/record-templates/:id/apply
// 将模板条目合并进现网记录生成变更计划（预览）；执行复用 /dns/push，
// 以复用其 NS 高危确认与执行后快照链路。
func (h *TemplateHandler) Apply(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req applyTemplateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountID == 0 || req.Zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "需要 account_id 与 zone"})
		return
	}
	t, err := h.templates.FindByID(uint(id))
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	items, err := repo.ParseItems(t.Items)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	op := ctxActor(c)
	// 与记录操作同源的授权校验：非授权 Zone 直接拒绝
	if ok, err := h.zoneSvc.CheckZoneAccess(op, req.AccountID, req.Zone); err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	} else if !ok {
		c.JSON(403, gin.H{"code": 403, "message": "无权访问该域名（需要管理员授权）"})
		return
	}
	actual, err := h.dnsSvc.ListRecords(req.AccountID, req.Zone, op)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}

	// 合并：同名同类型的既有记录跳过（不覆盖现网），其余转为新增期望记录
	desired := make([]provider.RecordInfo, len(actual), len(actual)+len(items))
	copy(desired, actual)
	skipped := make([]string, 0, len(items))
	for _, it := range items {
		it.Type = strings.ToUpper(it.Type)
		it.Name = normalizeHostName(it.Name, req.Zone)
		it.Value = strings.ReplaceAll(it.Value, "{zone}", req.Zone)
		conflict := false
		for _, a := range actual {
			if a.Name == it.Name && a.Type == it.Type {
				conflict = true
				break
			}
		}
		if conflict {
			skipped = append(skipped, it.Type+" "+it.Name)
			continue
		}
		desired = append(desired, provider.RecordInfo{
			Name: it.Name, Type: it.Type, Value: it.Value,
			TTL: it.TTL, Priority: it.Priority, Line: it.Line,
		})
	}
	plan, err := service.BuildPlan(actual, desired)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if plan == nil {
		plan = []service.PlanAction{}
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"plan": plan, "skipped": skipped,
	}})
}

// normalizeHostName 兼容模板里写全域名（www.example.com）或相对名（www）。
func normalizeHostName(name, zone string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".")
	if name == zone || name == "@" {
		return "@"
	}
	return strings.TrimSuffix(name, "."+zone)
}

// errBadRequest / errBadRequestf 便捷构造（模板校验复用）。
type badRequestError struct{ msg string }

func (e *badRequestError) Error() string { return e.msg }

func errBadRequest(msg string) error          { return &badRequestError{msg} }
func errBadRequestf(f string, a ...any) error { return &badRequestError{fmt.Sprintf(f, a...)} }
