package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/domhub-io/domhub/internal/service"
)

// CloudAccountHandler 云账号接口。
type CloudAccountHandler struct {
	svc *service.CloudAccountService
}

func NewCloudAccountHandler(svc *service.CloudAccountService) *CloudAccountHandler {
	return &CloudAccountHandler{svc: svc}
}

type accountReq struct {
	Name     string `json:"name"`
	Provider string `json:"provider"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Region   string `json:"region"`
	Status   *int   `json:"status"`
}

// Create POST /api/v1/accounts
func (h *CloudAccountHandler) Create(c *gin.Context) {
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	a, err := h.svc.Create(req.Name, req.Provider, req.AccessKey, req.SecretKey, req.Region)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": a})
}

// List GET /api/v1/accounts
func (h *CloudAccountHandler) List(c *gin.Context) {
	list, err := h.svc.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"items":      list,
		"total":      len(list),
		"connectors": service.SupportedProviders(),
	}})
}

// Update PUT /api/v1/accounts/:id
func (h *CloudAccountHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	var req accountReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误"})
		return
	}
	if err := h.svc.Update(uint(id), req.Name, req.Region, req.AccessKey, req.SecretKey, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// Delete DELETE /api/v1/accounts/:id
func (h *CloudAccountHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok"})
}

// Check POST /api/v1/accounts/:id/check
func (h *CloudAccountHandler) Check(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	a, err := h.svc.Check(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"last_check_ok":  a.LastCheckOK,
		"last_check_msg": a.LastCheckMsg,
	}})
}

// Sync POST /api/v1/accounts/:id/sync
func (h *CloudAccountHandler) Sync(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "ID 非法"})
		return
	}
	task, err := h.svc.Sync(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": task})
}
