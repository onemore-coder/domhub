package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/service"
)

// DeployHandler 证书部署目标接口。
type DeployHandler struct {
	svc *service.CertDeployService
}

func NewDeployHandler(svc *service.CertDeployService) *DeployHandler {
	return &DeployHandler{svc: svc}
}

// List GET /certs-issued/:id/deploys
func (h *DeployHandler) List(c *gin.Context) {
	certID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if certID == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效证书 ID"})
		return
	}
	list, err := h.svc.List(uint(certID))
	if err != nil {
		c.JSON(502, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": list})
}

type deployReq struct {
	CertID    uint           `json:"cert_id"`
	Type      string         `json:"type"`
	Name      string         `json:"name"`
	AccountID uint           `json:"account_id"`
	Config    map[string]any `json:"config"`
	Secret    map[string]any `json:"secret"`
}

// Save POST /certs-issued/:id/deploys（创建，:id 为证书 ID）；
// PUT /certs/deploys/:id（更新，:id 为部署目标 ID）。
func (h *DeployHandler) Save(c *gin.Context) {
	var req deployReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Type == "" {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	var (
		targetID uint
		certID   = req.CertID
	)
	if pathID := reqID(c); pathID > 0 {
		if c.Request.Method == http.MethodPut {
			targetID = pathID // 更新目标：路径是目标 ID，证书归属由服务层回填
		} else {
			certID = pathID // 创建：路径是证书 ID
		}
	}
	d, err := h.svc.SaveTarget(targetID, certID, req.Type, req.Name, req.AccountID, req.Config, req.Secret)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已保存", "data": d})
}

// Delete DELETE /certs/deploys/:id
func (h *DeployHandler) Delete(c *gin.Context) {
	id := reqID(c)
	if id == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效部署目标 ID"})
		return
	}
	if err := h.svc.Delete(id); err != nil {
		c.JSON(502, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已删除"})
}

// Run POST /certs/deploys/:id/run
func (h *DeployHandler) Run(c *gin.Context) {
	id := reqID(c)
	if id == 0 {
		c.JSON(400, gin.H{"code": 400, "message": "无效部署目标 ID"})
		return
	}
	if err := h.svc.Run(id, ctxActor(c)); err != nil {
		c.JSON(502, gin.H{"code": 502, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "部署已执行，结果见目标状态"})
}

func reqID(c *gin.Context) uint {
	v, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(v)
}
