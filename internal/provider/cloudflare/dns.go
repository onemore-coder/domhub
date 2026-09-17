package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/onemore-coder/domhub/internal/provider"
)

// ---- DNS 记录管理（Cloudflare v4 /zones/{id}/dns_records）----

type cfDNSRecord struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Name     string `json:"name"` // FQDN
	Content  string `json:"content"`
	TTL      int    `json:"ttl"` // 1 = auto
	Priority int    `json:"priority"`
	Proxied  bool   `json:"proxied"`
	Comment  string `json:"comment"`
}

// resolveZoneID 把 zone 名称解析为 Zone ID。
func (p *Provider) resolveZoneID(ctx context.Context, zone string) (string, error) {
	zone = strings.TrimSuffix(zone, ".")
	var zoneID string
	u := "/zones?" + url.Values{"name": {zone}}.Encode()
	result, _, err := p.do(ctx, "", u, nil)
	if err != nil {
		return "", err
	}
	var zones []cfZone
	if err := json.Unmarshal(result, &zones); err != nil {
		return "", err
	}
	for _, z := range zones {
		if strings.EqualFold(z.Name, zone) {
			zoneID = z.ID
			break
		}
	}
	if zoneID == "" {
		return "", fmt.Errorf("Cloudflare 未找到 Zone: %s", zone)
	}
	return zoneID, nil
}

// ListZones 列出账号下全部托管 Zone（record_count 需逐 Zone 查询，展示为 0）。
func (p *Provider) ListZones(ctx context.Context) ([]provider.ZoneInfo, error) {
	var out []provider.ZoneInfo
	err := p.doList(ctx, "/zones", 50, func(page json.RawMessage) error {
		var zones []cfZone
		if err := json.Unmarshal(page, &zones); err != nil {
			return err
		}
		for _, z := range zones {
			remark := z.Status
			if z.Paused {
				remark = "paused"
			}
			out = append(out, provider.ZoneInfo{
				Name:   z.Name,
				Remark: remark,
			})
		}
		return nil
	})
	return out, err
}

// relativeName 把 FQDN 转为相对主机名（@ 表示根）。
func relativeName(name, zone string) string {
	name = strings.TrimSuffix(name, ".")
	zone = strings.TrimSuffix(zone, ".")
	if strings.EqualFold(name, zone) {
		return "@"
	}
	if strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(zone)) {
		return name[:len(name)-len(zone)-1]
	}
	return name
}

// fqdnName 把相对主机名转为 FQDN。
func fqdnName(name, zone string) string {
	zone = strings.TrimSuffix(zone, ".")
	if name == "@" || name == "" {
		return zone
	}
	if strings.HasSuffix(strings.ToLower(name), "."+strings.ToLower(zone)) || strings.EqualFold(name, zone) {
		return name
	}
	return name + "." + zone
}

// ListRecords 拉取 Zone 下全部解析记录（分页，每页 100）。
func (p *Provider) ListRecords(ctx context.Context, zone string) ([]provider.RecordInfo, error) {
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return nil, err
	}
	var out []provider.RecordInfo
	err = p.doList(ctx, "/zones/"+zoneID+"/dns_records", 100, func(page json.RawMessage) error {
		var records []cfDNSRecord
		if err := json.Unmarshal(page, &records); err != nil {
			return err
		}
		for _, r := range records {
			out = append(out, provider.RecordInfo{
				ID:       r.ID,
				Name:     relativeName(r.Name, zone),
				Type:     r.Type,
				Value:    r.Content,
				TTL:      r.TTL,
				Priority: r.Priority,
				Line:     "default",
				Proxied:  r.Proxied,
				Remark:   r.Comment,
			})
		}
		return nil
	})
	return out, err
}

// proxiableTypes 支持 CDN 代理（橙云）的记录类型。
var proxiableTypes = map[string]bool{"A": true, "AAAA": true, "CNAME": true}

// recordBody 构建创建/更新的请求体。SRV/CAA 需要结构化 data。
func recordBody(zone string, rec provider.RecordInfo) (map[string]any, error) {
	content := strings.TrimSpace(rec.Value)
	if content == "" {
		return nil, fmt.Errorf("记录值不能为空")
	}
	body := map[string]any{
		"type": rec.Type,
		"name": fqdnName(rec.Name, zone),
		"ttl":  rec.TTL,
	}
	if rec.TTL <= 0 {
		body["ttl"] = 1 // Cloudflare auto TTL
	}
	if proxiableTypes[rec.Type] {
		body["proxied"] = rec.Proxied
		// Cloudflare 规则：开启代理的记录 TTL 必须为 auto（1）
		if rec.Proxied {
			body["ttl"] = 1
		}
	}
	if rec.Remark != "" {
		body["comment"] = rec.Remark
	}

	switch rec.Type {
	case "MX":
		body["content"] = strings.TrimSuffix(content, ".")
		body["priority"] = rec.Priority
	case "SRV":
		// 统一模型 content: "priority weight port target"
		fields := strings.Fields(content)
		if len(fields) != 4 {
			return nil, fmt.Errorf("SRV 记录值格式应为: 优先级 权重 端口 目标")
		}
		priority := rec.Priority
		if priority == 0 {
			fmt.Sscanf(fields[0], "%d", &priority)
		}
		var weight, port int
		fmt.Sscanf(fields[1], "%d", &weight)
		fmt.Sscanf(fields[2], "%d", &port)
		body["data"] = map[string]any{
			"priority": priority, "weight": weight, "port": port, "target": strings.TrimSuffix(fields[3], "."),
		}
		delete(body, "content")
	case "CAA":
		// 统一模型 content: `flags tag "value"`
		fields := strings.SplitN(content, " ", 3)
		if len(fields) != 3 {
			return nil, fmt.Errorf("CAA 记录值格式应为: 0 issue \"letsencrypt.org\"")
		}
		var flags int
		fmt.Sscanf(fields[0], "%d", &flags)
		value := strings.Trim(fields[2], "\"")
		body["data"] = map[string]any{"flags": flags, "tag": fields[1], "value": value}
		delete(body, "content")
	default:
		body["content"] = content
	}
	return body, nil
}

// CreateRecord 创建解析记录，返回 Cloudflare 记录 ID。
func (p *Provider) CreateRecord(ctx context.Context, zone string, rec provider.RecordInfo) (string, error) {
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return "", err
	}
	body, err := recordBody(zone, rec)
	if err != nil {
		return "", err
	}
	result, _, err := p.do(ctx, http.MethodPost, "/zones/"+zoneID+"/dns_records", body)
	if err != nil {
		return "", fmt.Errorf("Cloudflare 创建记录失败: %w", err)
	}
	var created cfDNSRecord
	if err := json.Unmarshal(result, &created); err != nil || created.ID == "" {
		return "", fmt.Errorf("Cloudflare 创建记录响应异常: %w", err)
	}
	return created.ID, nil
}

// UpdateRecord 更新解析记录（全量 PUT，记录 ID 定位）。
func (p *Provider) UpdateRecord(ctx context.Context, zone string, rec provider.RecordInfo) error {
	if rec.ID == "" {
		return fmt.Errorf("缺少记录 ID，无法更新")
	}
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return err
	}
	body, err := recordBody(zone, rec)
	if err != nil {
		return err
	}
	if _, _, err := p.do(ctx, http.MethodPut, "/zones/"+zoneID+"/dns_records/"+url.PathEscape(rec.ID), body); err != nil {
		return fmt.Errorf("Cloudflare 更新记录失败: %w", err)
	}
	return nil
}

// DeleteRecord 删除解析记录。
func (p *Provider) DeleteRecord(ctx context.Context, zone, recordID string) error {
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return err
	}
	if _, _, err := p.do(ctx, http.MethodDelete, "/zones/"+zoneID+"/dns_records/"+url.PathEscape(recordID), nil); err != nil {
		return fmt.Errorf("Cloudflare 删除记录失败: %w", err)
	}
	return nil
}
