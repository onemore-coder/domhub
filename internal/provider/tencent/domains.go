package tencent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/domhub-io/domhub/internal/provider"
)

// describeResp DescribeDomainNameList 响应字段。
type describeResp struct {
	TotalCount int `json:"TotalCount"`
	DomainList []struct {
		Name        string `json:"Name"`
		Status      string `json:"Status"`
		CreatedTime string `json:"CreatedTime"`
		ExpireTime  string `json:"ExpireTime"`
	} `json:"DomainList"`
}

// ListDomains 拉取注册域名列表（分页全量）。
func (p *Provider) ListDomains(ctx context.Context) ([]provider.DomainInfo, error) {
	var (
		out    []provider.DomainInfo
		offset = 0
		limit  = 100
	)
	for {
		fields, err := p.call(ctx, "DescribeDomainNameList", map[string]any{
			"Offset": offset,
			"Limit":  limit,
		})
		if err != nil {
			return nil, err
		}
		var data describeResp
		raw, ok := fields["DomainList"]
		if !ok {
			// 无域名时接口可能不返回 DomainList
			return out, nil
		}
		if err := json.Unmarshal(raw, &data.DomainList); err != nil {
			return nil, fmt.Errorf("解析域名列表失败: %w", err)
		}
		if total, err := json.Marshal(fields["TotalCount"]); err == nil && total != nil {
			_ = json.Unmarshal(total, &data.TotalCount)
		}

		for _, d := range data.DomainList {
			info := provider.DomainInfo{
				Name:      d.Name,
				Kind:      provider.KindDomain,
				Registrar: "腾讯云",
				Status:    d.Status,
			}
			if t := parseCST(d.CreatedTime); !t.IsZero() {
				info.RegisteredAt = t
			}
			if t := parseCST(d.ExpireTime); !t.IsZero() {
				info.ExpireAt = t
			}
			out = append(out, info)
		}

		offset += limit
		if len(data.DomainList) < limit || (data.TotalCount > 0 && offset >= data.TotalCount) {
			break
		}
	}
	return out, nil
}

// CheckConnection 连通性检测：尝试拉取 1 条域名列表。
func (p *Provider) CheckConnection(ctx context.Context) error {
	_, err := p.call(ctx, "DescribeDomainNameList", map[string]any{"Offset": 0, "Limit": 1})
	return err
}
