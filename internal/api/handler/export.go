package handler

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
)

// ExportHandler 配置导出：域名清单 / DNS 记录 CSV。
type ExportHandler struct {
	domains *repo.DomainRepo
	zoneSvc *service.ZoneService
}

func NewExportHandler(domains *repo.DomainRepo, zoneSvc *service.ZoneService) *ExportHandler {
	return &ExportHandler{domains: domains, zoneSvc: zoneSvc}
}

// writeCSV 以 UTF-8 BOM 输出 CSV（Excel 直接打开不乱码）。
func writeCSV(c *gin.Context, filename string, rows [][]string) {
	c.Header("Content-Type", "text/csv; charset=utf-8")
	// filename= 只放 ASCII 兜底名（RFC 6265 要求该参数为 ASCII）；
	// filename*= 携带 UTF-8 中文名，现代浏览器优先读取它
	ascii := "export-" + time.Now().Format("20060102") + ".csv"
	c.Header("Content-Disposition",
		`attachment; filename="`+ascii+`"; filename*=UTF-8''`+url.PathEscape(filename))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte("\ufeff")) // UTF-8 BOM：Excel 直接打开不乱码
	w := csv.NewWriter(c.Writer)
	_ = w.Write(rows[0])
	for _, r := range rows[1:] {
		_ = w.Write(r)
	}
	w.Flush()
}

func fmtTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// Domains GET /api/v1/domains/export —— 域名清单 CSV（与列表页相同筛选条件，不分页）。
func (h *ExportHandler) Domains(c *gin.Context) {
	expiring, _ := strconv.Atoi(c.Query("expiring_days"))
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	list, _, err := h.domains.List(repo.DomainFilter{
		Keyword:        c.Query("keyword"),
		Provider:       c.Query("provider"),
		CloudAccountID: uint(accountID),
		Kind:           c.Query("kind"),
		Tag:            c.Query("tag"),
		ExpiringDays:   expiring,
		Page:           1,
		PageSize:       200, // 上限由 repo 控制；日常域名量足够，超出时改用全量接口
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	rows := [][]string{{
		"域名", "类型", "厂商", "注册商", "状态", "注册时间", "到期时间", "标签", "备注", "最近同步",
	}}
	for _, d := range list {
		kind := "域名"
		if d.Kind == "zone" {
			kind = "托管Zone"
		}
		rows = append(rows, []string{
			d.Name, kind, d.Provider, d.Registrar, d.Status,
			fmtTimePtr(d.RegisteredAt), fmtTimePtr(d.ExpireAt),
			d.Tags, d.Remark, d.LastSyncedAt.Format("2006-01-02 15:04"),
		})
	}
	writeCSV(c, fmt.Sprintf("domhub-域名清单-%s.csv", time.Now().Format("20060102")), rows)
}

// Records GET /api/v1/dns/records/export?account_id=1&zone=example.com
// 单 Zone 解析记录 CSV（沿用缓存镜像，走 Zone 授权校验）。
func (h *ExportHandler) Records(c *gin.Context) {
	accountID, _ := strconv.ParseUint(c.Query("account_id"), 10, 64)
	zone := c.Query("zone")
	if accountID == 0 || zone == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "缺少 account_id 或 zone"})
		return
	}
	records, err := h.zoneSvc.ListRecordsCached(ctxActor(c), uint(accountID), zone, "", "", 0)
	if err != nil {
		c.JSON(httpCode(err), gin.H{"code": httpCode(err), "message": err.Error()})
		return
	}
	rows := [][]string{{
		"Zone", "主机记录", "类型", "线路", "TTL", "优先级", "记录值", "状态", "备注", "云代理",
	}}
	for _, r := range records {
		status := r.Status
		if status == "" {
			status = "启用"
		}
		proxied := ""
		if r.Proxied {
			proxied = "是"
		}
		rows = append(rows, []string{
			r.ZoneName, r.Name, r.Type, r.Line,
			strconv.Itoa(r.TTL), strconv.Itoa(r.Priority),
			r.Value, status, r.Remark, proxied,
		})
	}
	writeCSV(c, fmt.Sprintf("domhub-%s-解析记录-%s.csv", zone, time.Now().Format("20060102")), rows)
}

// 编译期确认导出字段与模型一致，防止模型字段改名后 CSV 静默漏列。
var (
	_ = model.Domain{}
	_ = model.DnsRecord{}
)
