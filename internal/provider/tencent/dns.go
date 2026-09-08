package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/domhub-io/domhub/internal/provider"
)

// ---- DNSPod（2021-03-23）----

// ListZones 列出 DNSPod 托管的域名。
func (p *Provider) ListZones(ctx context.Context) ([]provider.ZoneInfo, error) {
	var out []provider.ZoneInfo
	offset := 0
	limit := 100
	for {
		fields, err := p.call(ctx, targetDNSPod, "DescribeDomainList", map[string]any{
			"Offset": offset,
			"Limit":  limit,
		})
		if err != nil {
			return nil, err
		}
		var resp struct {
			DomainList []struct {
				Name        string `json:"Name"`
				PunycodeName string `json:"PunycodeName"`
				RecordCount int    `json:"RecordCount"`
				Remark      string `json:"Remark"`
			} `json:"DomainList"`
			TotalCount int `json:"TotalCount"`
		}
		if raw, ok := fields["DomainList"]; ok {
			if err := json.Unmarshal(raw, &resp.DomainList); err != nil {
				return nil, fmt.Errorf("解析域名列表失败: %w", err)
			}
		}
		if rawTotal, ok := fields["TotalCount"]; ok {
			_ = json.Unmarshal(rawTotal, &resp.TotalCount)
		}

		for _, z := range resp.DomainList {
			out = append(out, provider.ZoneInfo{
				Name:         z.Name,
				RecordCount:  z.RecordCount,
				Remark:       z.Remark,
				PunycodeName: z.PunycodeName,
			})
		}

		offset += limit
		if len(resp.DomainList) < limit || (resp.TotalCount > 0 && offset >= resp.TotalCount) {
			break
		}
	}
	return out, nil
}

// dnspodRecord DNSPod 记录结构。
type dnspodRecord struct {
	Id     any    `json:"Id"` // 数字或字符串，统一处理
	Name   string `json:"Name"`
	Type   string `json:"Type"`
	Value  string `json:"Value"`
	TTL    int    `json:"TTL"`
	MX     int    `json:"MX"`
	Line   string `json:"Line"`
	Status string `json:"Status"` // ENABLE / DISABLE
	Remark string `json:"Remark"`
}

func (r dnspodRecord) recordID() string {
	switch v := r.Id.(type) {
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case string:
		return v
	default:
		return ""
	}
}

// ListRecords 拉取某域名的全部解析记录。
func (p *Provider) ListRecords(ctx context.Context, zone string) ([]provider.RecordInfo, error) {
	var out []provider.RecordInfo
	offset := 0
	limit := 3000 // DNSPod 单页上限
	for {
		fields, err := p.call(ctx, targetDNSPod, "DescribeRecordList", map[string]any{
			"Domain": zone,
			"Offset": offset,
			"Limit":  limit,
		})
		if err != nil {
			return nil, err
		}
		var resp struct {
			RecordList []dnspodRecord `json:"RecordList"`
		}
		raw, ok := fields["RecordList"]
		if !ok {
			// 域名下无解析记录时不返回 RecordList
			return out, nil
		}
		if err := json.Unmarshal(raw, &resp.RecordList); err != nil {
			return nil, fmt.Errorf("解析记录列表失败: %w", err)
		}

		for _, r := range resp.RecordList {
			out = append(out, provider.RecordInfo{
				ID:       r.recordID(),
				Name:     r.Name,
				Type:     r.Type,
				Value:    r.Value,
				TTL:      r.TTL,
				Priority: r.MX,
				Line:     r.Line,
				Status:   r.Status,
				Remark:   r.Remark,
			})
		}

		if len(resp.RecordList) < limit {
			break
		}
		offset += limit
	}
	return out, nil
}

// CreateRecord 创建解析记录。
func (p *Provider) CreateRecord(ctx context.Context, zone string, rec provider.RecordInfo) (string, error) {
	params := map[string]any{
		"Domain": zone,
		"Name":   rec.Name,
		"Type":   rec.Type,
		"Value":  rec.Value,
		"TTL":    rec.TTL,
		"Line":   rec.Line,
	}
	if rec.Line == "" {
		params["Line"] = "默认"
	}
	if rec.Priority > 0 {
		params["MX"] = rec.Priority
	}
	fields, err := p.call(ctx, targetDNSPod, "CreateRecord", params)
	if err != nil {
		return "", err
	}
	var resp struct {
		RecordId any `json:"RecordId"`
	}
	if raw, ok := fields["RecordId"]; ok {
		_ = json.Unmarshal(raw, &resp.RecordId)
	}
	// 通过临时 dnspodRecord 复用 ID 归一化逻辑
	return dnspodRecord{Id: resp.RecordId}.recordID(), nil
}

// UpdateRecord 更新解析记录。
func (p *Provider) UpdateRecord(ctx context.Context, zone string, rec provider.RecordInfo) error {
	id, _ := strconv.ParseInt(rec.ID, 10, 64)
	params := map[string]any{
		"Domain":   zone,
		"RecordId": id,
		"Name":     rec.Name,
		"Type":     rec.Type,
		"Value":    rec.Value,
		"TTL":      rec.TTL,
		"Line":     rec.Line,
	}
	if rec.Line == "" {
		params["Line"] = "默认"
	}
	if rec.Priority > 0 {
		params["MX"] = rec.Priority
	}
	_, err := p.call(ctx, targetDNSPod, "ModifyRecord", params)
	return err
}

// DeleteRecord 删除解析记录。
func (p *Provider) DeleteRecord(ctx context.Context, zone, recordID string) error {
	id, _ := strconv.ParseInt(recordID, 10, 64)
	_, err := p.call(ctx, targetDNSPod, "DeleteRecord", map[string]any{
		"Domain":   zone,
		"RecordId": id,
	})
	return err
}

// CheckConnection DNS 连通性检测。
func (p *Provider) checkDNSConnection(ctx context.Context) error {
	_, err := p.call(ctx, targetDNSPod, "DescribeDomainList", map[string]any{"Offset": 0, "Limit": 1})
	return err
}

// dnsProvider 包装器：覆盖 CheckConnection，让 DNS 接口走 DNSPod 而非域名注册 API。
type dnsProvider struct {
	*Provider
}

func (d *dnsProvider) CheckConnection(ctx context.Context) error {
	return d.checkDNSConnection(ctx)
}
