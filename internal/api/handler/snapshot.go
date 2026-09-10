package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/api/middleware"
	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/service"
)

// SnapshotHandler 解析记录快照接口。
type SnapshotHandler struct {
	svc *service.SnapshotService
}

func NewSnapshotHandler(svc *service.SnapshotService) *SnapshotHandler {
	return &SnapshotHandler{svc: svc}
}

// List GET /dns/snapshots?account_id=1&zone=example.com&limit=50
func (h *SnapshotHandler) List(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	zone := c.Query("zone")
	if accountID == 0 || zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "缺少 account_id 或 zone"})
		return
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.svc.List(uint(accountID), zone, limit)
	if err != nil {
		c.JSON(502, gin.H{"code": 502, "message": err.Error()})
		return
	}
	if list == nil {
		list = make([]model.DNSRecordSnapshot, 0)
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

// Capture POST /dns/snapshots {"account_id","zone","note"}
func (h *SnapshotHandler) Capture(c *gin.Context) {
	var req struct {
		AccountID uint   `json:"account_id"`
		Zone      string `json:"zone"`
		Note      string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountID == 0 || req.Zone == "" {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	uid, _, _ := middleware.CtxUser(c)
	snap, err := h.svc.Capture(req.AccountID, req.Zone, "manual", req.Note, uid)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "快照已保存", "data": snap})
}

// Get GET /dns/snapshots/:id（含记录正文）
func (h *SnapshotHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效快照 ID"})
		return
	}
	snap, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "快照不存在"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": snap})
}

type diffReq struct {
	BaseID   uint `json:"base_id"`
	TargetID uint `json:"target_id"`
}

// Diff POST /dns/snapshots/diff
func (h *SnapshotHandler) Diff(c *gin.Context) {
	var req diffReq
	if err := c.ShouldBindJSON(&req); err != nil || req.BaseID == 0 || req.TargetID == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	base, target, plan, err := h.svc.DiffSnapshots(req.BaseID, req.TargetID)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"base":   gin.H{"id": base.ID, "created_at": base.CreatedAt, "count": base.Count},
		"target": gin.H{"id": target.ID, "created_at": target.CreatedAt, "count": target.Count},
		"plan":   plan,
	}})
}

type restoreReq struct {
	AccountID  uint   `json:"account_id"`
	Zone       string `json:"zone"`
	SnapshotID uint   `json:"snapshot_id"`
}

// RestorePlan POST /dns/snapshots/restore-plan —— 生成"现网 → 快照"恢复计划（不执行）。
func (h *SnapshotHandler) RestorePlan(c *gin.Context) {
	var req restoreReq
	if err := c.ShouldBindJSON(&req); err != nil || req.AccountID == 0 || req.Zone == "" || req.SnapshotID == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	plan, err := h.svc.RestorePlan(req.AccountID, req.Zone, req.SnapshotID, ctxActor(c))
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": plan})
}
