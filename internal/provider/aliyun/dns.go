package aliyun

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/onemore-coder/domhub/internal/provider"
)

// ---- 云解析 DNS（alidns, 2015-01-09）----

// dnsProvider 包装器：覆盖 CheckConnection，让 DNS 接口走 alidns 而非域名注册 API。
type dnsProvider struct {
	*Provider
}

func (d *dnsProvider) CheckConnection(ctx context.Context) error {
	_, err := d.callAPI(ctx, alidnsBaseURL, alidnsVersion, "DescribeDomains", map[string]string{
		"PageNumber": "1", "PageSize": "1",
	})
	return err
}

// ListZones 列出云解析托管的域名。
func (p *Provider) ListZones(ctx context.Context) ([]provider.ZoneInfo, error) {
	var out []provider.ZoneInfo
	pageNumber := 1
	pageSize := 100
	for {
		fields, err := p.callAPI(ctx, alidnsBaseURL, alidnsVersion, "DescribeDomains", map[string]string{
			"PageNumber": strconv.Itoa(pageNumber),
			"PageSize":   strconv.Itoa(pageSize),
			"Lang":       "zh",
		})
		if err != nil {
			return nil, err
		}
		var resp struct {
			Domains struct {
				Domain []struct {
					DomainName  string `json:"DomainName"`
					Punycode    string `json:"Punycode"`
					RecordCount int    `json:"RecordCount"`
				} `json:"Domain"`
			} `json:"Domains"`
			TotalCount int `json:"TotalCount"`
		}
		if raw, ok := fields["Domains"]; ok {
			if err := json.Unmarshal(raw, &resp.Domains); err != nil {
				return nil, fmt.Errorf("解析域名列表失败: %w", err)
			}
		}
		if rawTotal, ok := fields["TotalCount"]; ok {
			_ = json.Unmarshal(rawTotal, &resp.TotalCount)
		}

		for _, z := range resp.Domains.Domain {
			out = append(out, provider.ZoneInfo{
				Name:         z.DomainName,
				RecordCount:  z.RecordCount,
				PunycodeName: z.Punycode,
			})
		}

		if len(resp.Domains.Domain) < pageSize || (resp.TotalCount > 0 && pageNumber*pageSize >= resp.TotalCount) || pageNumber > 100 {
			break
		}
		pageNumber++
	}
	return out, nil
}

// alidnsRecord 云解析记录结构。
// 注意：阿里云返回的 RecordId 是字符串形式（"12345"），用 any 兼容数字/字符串。
type alidnsRecord struct {
	RecordId any    `json:"RecordId"`
	RR       string `json:"RR"`
	Type     string `json:"Type"`
	Value    string `json:"Value"`
	TTL      int    `json:"TTL"`
	Priority int    `json:"Priority"`
	Line     string `json:"Line"`
	Status   string `json:"Status"` // ENABLE / PAUSE
	Remark   string `json:"Remark"`
}

// parseAliyunID 归一化阿里云的 ID 字段（可能是数字或字符串）。
func parseAliyunID(v any) string {
	switch x := v.(type) {
	case float64:
		return strconv.FormatInt(int64(x), 10)
	case string:
		return x
	default:
		return ""
	}
}

// ListRecords 拉取某域名的全部解析记录（分页全量）。
func (p *Provider) ListRecords(ctx context.Context, zone string) ([]provider.RecordInfo, error) {
	var out []provider.RecordInfo
	pageNumber := 1
	pageSize := 100
	for {
		fields, err := p.callAPI(ctx, alidnsBaseURL, alidnsVersion, "DescribeDomainRecords", map[string]string{
			"DomainName": zone,
			"PageNumber": strconv.Itoa(pageNumber),
			"PageSize":   strconv.Itoa(pageSize),
			"Lang":       "zh",
		})
		if err != nil {
			return nil, err
		}
		var resp struct {
			DomainRecords struct {
				Record []alidnsRecord `json:"Record"`
			} `json:"DomainRecords"`
			TotalCount int `json:"TotalCount"`
		}
		if raw, ok := fields["DomainRecords"]; ok {
			if err := json.Unmarshal(raw, &resp.DomainRecords); err != nil {
				return nil, fmt.Errorf("解析记录列表失败: %w", err)
			}
		}
		if rawTotal, ok := fields["TotalCount"]; ok {
			_ = json.Unmarshal(rawTotal, &resp.TotalCount)
		}

		for _, r := range resp.DomainRecords.Record {
			out = append(out, provider.RecordInfo{
				ID:       parseAliyunID(r.RecordId),
				Name:     r.RR,
				Type:     r.Type,
				Value:    r.Value,
				TTL:      r.TTL,
				Priority: r.Priority,
				Line:     r.Line,
				Status:   r.Status,
				Remark:   r.Remark,
			})
		}

		if len(resp.DomainRecords.Record) < pageSize || (resp.TotalCount > 0 && pageNumber*pageSize >= resp.TotalCount) || pageNumber > 100 {
			break
		}
		pageNumber++
	}
	return out, nil
}

// CreateRecord 创建解析记录。
func (p *Provider) CreateRecord(ctx context.Context, zone string, rec provider.RecordInfo) (string, error) {
	extra := map[string]string{
		"DomainName": zone,
		"RR":         rec.Name,
		"Type":       rec.Type,
		"Value":      rec.Value,
		"Line":       rec.Line,
	}
	if extra["Line"] == "" {
		extra["Line"] = "default"
	}
	if rec.TTL > 0 {
		extra["TTL"] = strconv.Itoa(rec.TTL)
	}
	if rec.Priority > 0 {
		extra["Priority"] = strconv.Itoa(rec.Priority)
	}
	fields, err := p.callAPI(ctx, alidnsBaseURL, alidnsVersion, "AddDomainRecord", extra)
	if err != nil {
		return "", err
	}
	var resp struct {
		RecordId    any    `json:"RecordId"`
		RecordIdStr string `json:"RecordIdStr"`
	}
	if raw, ok := fields["RecordId"]; ok {
		_ = json.Unmarshal(raw, &resp.RecordId)
	}
	if raw, ok := fields["RecordIdStr"]; ok {
		_ = json.Unmarshal(raw, &resp.RecordIdStr)
	}
	if resp.RecordIdStr != "" {
		return resp.RecordIdStr, nil
	}
	return parseAliyunID(resp.RecordId), nil
}

// UpdateRecord 更新解析记录。
func (p *Provider) UpdateRecord(ctx context.Context, zone string, rec provider.RecordInfo) error {
	extra := map[string]string{
		"RecordId": rec.ID,
		"RR":       rec.Name,
		"Type":     rec.Type,
		"Value":    rec.Value,
		"Line":     rec.Line,
	}
	if extra["Line"] == "" {
		extra["Line"] = "default"
	}
	if rec.TTL > 0 {
		extra["TTL"] = strconv.Itoa(rec.TTL)
	}
	if rec.Priority > 0 {
		extra["Priority"] = strconv.Itoa(rec.Priority)
	}
	_, err := p.callAPI(ctx, alidnsBaseURL, alidnsVersion, "UpdateDomainRecord", extra)
	return err
}

// DeleteRecord 删除解析记录。
func (p *Provider) DeleteRecord(ctx context.Context, zone, recordID string) error {
	_, err := p.callAPI(ctx, alidnsBaseURL, alidnsVersion, "DeleteDomainRecord", map[string]string{
		"RecordId": recordID,
	})
	return err
}
