package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/domhub-io/domhub/internal/provider"
)

// ListDomains 拉取注册域名列表（分页全量）。
// 接口：domain.aliyuncs.com DescribeDomainList（Version 2018-01-29）。
func (p *Provider) ListDomains(ctx context.Context) ([]provider.DomainInfo, error) {
	var out []provider.DomainInfo
	pageNum := 1
	pageSize := 20
	for {
		fields, err := p.call(ctx, "DescribeDomainList", map[string]string{
			"PageNum":  fmt.Sprintf("%d", pageNum),
			"PageSize": fmt.Sprintf("%d", pageSize),
		})
		if err != nil {
			return nil, err
		}
		var resp describeDomainListResp
		rawData, ok := fields["Data"]
		if !ok {
			return out, nil
		}
		if err := json.Unmarshal(rawData, &resp); err != nil {
			return nil, fmt.Errorf("解析域名数据失败: %w", err)
		}
		for _, d := range resp.Domain {
			info := provider.DomainInfo{
				Name:      d.DomainName,
				Kind:      provider.KindDomain,
				Registrar: "阿里云",
				Status:    "ok",
			}
			if t := parseDate(d.RegistrationDate); !t.IsZero() {
				info.RegisteredAt = t
			}
			if t := parseDate(d.ExpireDate); !t.IsZero() {
				info.ExpireAt = t
			}
			out = append(out, info)
		}

		total := 0
		if rawTotal, ok := fields["TotalCount"]; ok {
			_ = json.Unmarshal(rawTotal, &total)
		}
		pageNum++
		if len(resp.Domain) < pageSize || (total > 0 && pageNum*pageSize > total+pageSize) || pageNum > 100 {
			break
		}
	}
	return out, nil
}

type domainItem struct {
	DomainName       string `json:"DomainName"`
	ExpireDate       string `json:"ExpireDate"` // "2020-09-10"
	RegistrationDate string `json:"RegistrationDate"`
}

type describeDomainListResp struct {
	Domain []domainItem `json:"Domain"`
}

// CheckConnection 连通性检测。
func (p *Provider) CheckConnection(ctx context.Context) error {
	_, err := p.call(ctx, "DescribeDomainList", map[string]string{"PageNum": "1", "PageSize": "1"})
	return err
}

// parseDate 解析 "2006-01-02" 日期（按北京时间零点）。
func parseDate(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02", s, cst)
	if err != nil {
		return time.Time{}
	}
	return t
}
