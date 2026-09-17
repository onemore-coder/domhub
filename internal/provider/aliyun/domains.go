package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onemore-coder/domhub/internal/provider"
)

// domainItem QueryDomainList 返回的域名条目。
// 日期优先用 *DateLong（UTC 毫秒时间戳），字符串格式历史上有多种（"2020-09-10"、"Nov 02,2019 04:00:45"）不可靠。
type domainItem struct {
	DomainName           string `json:"DomainName"`
	DomainStatus         string `json:"DomainStatus"` // "1" 急需续费 / "2" 急需赎回 / "3" 正常
	RegistrationDateLong int64  `json:"RegistrationDateLong"`
	ExpirationDateLong   int64  `json:"ExpirationDateLong"`
}

// ListDomains 拉取注册域名列表（分页全量）。
// 接口：domain.aliyuncs.com QueryDomainList（Version 2018-01-29）。
// 注意：历史上 Data 有两种结构（对象含 Domain 数组 / 直接数组），两者都兼容。
func (p *Provider) ListDomains(ctx context.Context) ([]provider.DomainInfo, error) {
	var out []provider.DomainInfo
	pageNum := 1
	pageSize := 20
	for {
		fields, err := p.call(ctx, "QueryDomainList", map[string]string{
			"PageNum":  fmt.Sprintf("%d", pageNum),
			"PageSize": fmt.Sprintf("%d", pageSize),
			"Lang":     "zh",
		})
		if err != nil {
			return nil, err
		}

		items, err := parseDomainList(fields["Data"])
		if err != nil {
			return nil, fmt.Errorf("解析域名列表失败: %w", err)
		}

		for _, d := range items {
			info := provider.DomainInfo{
				Name:      d.DomainName,
				Kind:      provider.KindDomain,
				Registrar: "阿里云",
				Status:    mapStatus(d.DomainStatus),
			}
			if d.RegistrationDateLong > 0 {
				info.RegisteredAt = msToTime(d.RegistrationDateLong)
			}
			if d.ExpirationDateLong > 0 {
				info.ExpireAt = msToTime(d.ExpirationDateLong)
			}
			out = append(out, info)
		}

		// 总数在顶层 TotalItemNum 字段
		total := 0
		if rawTotal, ok := fields["TotalItemNum"]; ok {
			_ = json.Unmarshal(rawTotal, &total)
		}
		if len(items) < pageSize || (total > 0 && pageNum*pageSize >= total) || pageNum > 100 {
			break
		}
		pageNum++
	}
	return out, nil
}

// parseDomainList 兼容两种 Data 结构：
// 旧版：{"Domain": [...]}；新版：直接是 [...] 数组。
func parseDomainList(raw json.RawMessage) ([]domainItem, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	// 先尝试对象结构
	var obj struct {
		Domain []domainItem `json:"Domain"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.Domain != nil {
		return obj.Domain, nil
	}
	// 再尝试数组结构
	var arr []domainItem
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr, nil
	}
	return nil, fmt.Errorf("无法识别的 Data 结构: %s", truncate(raw))
}

// mapStatus 把 DomainStatus 数字码映射为可读状态。
func mapStatus(s string) string {
	switch s {
	case "1":
		return "急需续费"
	case "2":
		return "急需赎回"
	case "3", "":
		return "ok"
	default:
		return s
	}
}

func msToTime(ms int64) time.Time {
	return time.UnixMilli(ms).In(cst)
}

// CheckConnection 连通性检测：查询 1 条域名列表。
func (p *Provider) CheckConnection(ctx context.Context) error {
	_, err := p.call(ctx, "QueryDomainList", map[string]string{
		"PageNum":  "1",
		"PageSize": "1",
		"Lang":     "zh",
	})
	return err
}
