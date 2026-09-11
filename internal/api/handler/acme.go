package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/service"
)

// AcmeHandler 证书申请（ACME 免费证书）接口。
type AcmeHandler struct{ svc *service.AcmeService }

func NewAcmeHandler(svc *service.AcmeService) *AcmeHandler { return &AcmeHandler{svc: svc} }

type applyReq struct {
	DNSAccountID uint     `json:"dns_account_id"`
	Domains      []string `json:"domains"` // 第一个为主域名，支持通配符 *.example.com
	Email        string   `json:"email"`
	CA           string   `json:"ca"` // letsencrypt / letsencrypt_staging / zerossl
	AutoRenew    bool     `json:"auto_renew"`
	EABKid       string   `json:"eab_kid"` // ZeroSSL 等要求 EAB 的 CA 必填
	EABKey       string   `json:"eab_key"`
}

// ListCAs GET /api/v1/certs-issued/cas —— 可用 CA 目录。
func (h *AcmeHandler) ListCAs(c *gin.Context) {
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": service.SupportedCAs()})
}

// Apply POST /api/v1/certs-issued/apply —— 创建申请并异步执行。
func (h *AcmeHandler) Apply(c *gin.Context) {
	var req applyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}
	cert, err := h.svc.Create(req.DNSAccountID, req.Domains, req.Email, req.CA, req.AutoRenew, req.EABKid, req.EABKey)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "申请已提交，正在后台执行", "data": cert})
}

// List GET /api/v1/certs-issued?status=issued
func (h *AcmeHandler) List(c *gin.Context) {
	items, err := h.svc.List(c.Query("status"))
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{"items": items}})
}

// Get GET /api/v1/certs-issued/:id
func (h *AcmeHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	cert, err := h.svc.Get(uint(id))
	if err != nil {
		c.JSON(404, gin.H{"code": 404, "message": "记录不存在"})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": cert})
}

// Renew POST /api/v1/certs-issued/:id/renew
func (h *AcmeHandler) Renew(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.RenewNow(uint(id)); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "续期已启动，请稍后刷新查看进度"})
}

// SetAutoRenew PUT /api/v1/certs-issued/:id/auto-renew
func (h *AcmeHandler) SetAutoRenew(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req struct {
		AutoRenew bool `json:"auto_renew"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.svc.SetAutoRenew(uint(id), req.AutoRenew); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}

// Delete DELETE /api/v1/certs-issued/:id
func (h *AcmeHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "已删除"})
}

// Download GET /api/v1/certs-issued/:id/download?type=chain|key|fullchain
func (h *AcmeHandler) Download(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	kind := c.DefaultQuery("type", "fullchain")
	pem, err := h.svc.ExportPEM(uint(id), kind)
	if err != nil {
		c.JSON(400, gin.H{"code": 400, "message": err.Error()})
		return
	}
	cert, gerr := h.svc.Get(uint(id))
	base := "cert"
	if gerr == nil {
		base = strings.ReplaceAll(cert.PrimaryDomain, "*.", "wildcard.")
	}
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s.%s.pem", base, kind))
	c.Data(http.StatusOK, "application/x-pem-file", []byte(pem))
}
