package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/repo"
)

// AuditHandler 审计日志接口。
type AuditHandler struct {
	audit *repo.AuditRepo
}

func NewAuditHandler(audit *repo.AuditRepo) *AuditHandler {
	return &AuditHandler{audit: audit}
}

// List GET /api/v1/audit-logs
func (h *AuditHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	list, total, err := h.audit.List(c.Query("action"), c.Query("username"), c.Query("zone"), c.Query("keyword"), page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(200, gin.H{"code": 0, "message": "ok", "data": gin.H{
		"items": list, "total": total,
	}})
}
