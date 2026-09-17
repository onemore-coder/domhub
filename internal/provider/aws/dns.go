package aws

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/onemore-coder/domhub/internal/provider"
)

// ---- Route53 DNS 托管区 ----

// r53Key AWS Route53 无独立记录 ID，用组合键编码（base64(JSON)）承载删除/更新所需全部信息。
type r53Key struct {
	Name   string   `json:"n"` // FQDN（带尾点）
	Type   string   `json:"t"`
	TTL    int64    `json:"ttl"`
	Values []string `json:"v"` // 原始值（TXT 保持引号形式，删除时需要精确匹配）
}

func encodeR53Key(k r53Key) string {
	b, _ := json.Marshal(k)
	return base64.StdEncoding.EncodeToString(b)
}

func decodeR53Key(s string) (r53Key, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return r53Key{}, fmt.Errorf("无效的记录标识: %w", err)
	}
	var k r53Key
	if err := json.Unmarshal(b, &k); err != nil {
		return r53Key{}, fmt.Errorf("无效的记录标识: %w", err)
	}
	return k, nil
}

// r53Client 构建 route53 客户端（托管区为全局服务，region 仅影响调用端点偏好）。
func (p *Provider) r53Client(ctx context.Context) (*route53.Client, error) {
	region := p.region
	if region == "" {
		region = "us-east-1"
	}
	sdkCfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion(region),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			p.cred.AccessKey, p.cred.SecretKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 AWS 配置失败: %w", err)
	}
	return route53.NewFromConfig(sdkCfg), nil
}

// resolveZoneID 把 zone 名称解析为 Hosted Zone ID（Route53 API 以 ID 定位）。
func (p *Provider) resolveZoneID(ctx context.Context, zone string) (string, error) {
	client, err := p.r53Client(ctx)
	if err != nil {
		return "", err
	}
	want := strings.TrimSuffix(zone, ".")
	paginator := route53.NewListHostedZonesPaginator(client, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("AWS 拉取托管区失败: %w", err)
		}
		for _, z := range page.HostedZones {
			if strings.TrimSuffix(*z.Name, ".") == want {
				return *z.Id, nil
			}
		}
	}
	return "", fmt.Errorf("未找到托管区: %s", zone)
}

// ListZones 列出托管 Zone。
func (p *Provider) ListZones(ctx context.Context) ([]provider.ZoneInfo, error) {
	client, err := p.r53Client(ctx)
	if err != nil {
		return nil, err
	}
	var out []provider.ZoneInfo
	paginator := route53.NewListHostedZonesPaginator(client, &route53.ListHostedZonesInput{})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("AWS 连接失败: %w", err)
		}
		for _, z := range page.HostedZones {
			comment := ""
			if z.Config != nil && z.Config.Comment != nil {
				comment = *z.Config.Comment
			}
			recordCount := int64(0)
			if z.ResourceRecordSetCount != nil {
				recordCount = *z.ResourceRecordSetCount
			}
			out = append(out, provider.ZoneInfo{
				Name:        strings.TrimSuffix(*z.Name, "."),
				RecordCount: int(recordCount),
				Remark:      comment,
			})
		}
	}
	return out, nil
}

// zoneFQDN 把 zone 转成 FQDN（带尾点）。
func zoneFQDN(zone string) string { return strings.TrimSuffix(zone, ".") + "." }

// relativeName 把 FQDN 转为相对主机名（@ 表示根）。
func relativeName(name, zone string) string {
	suffix := "." + strings.TrimSuffix(zone, ".")
	if name == strings.TrimSuffix(zone, ".") {
		return "@"
	}
	if strings.HasSuffix(name, suffix) {
		return strings.TrimSuffix(name, suffix)
	}
	return name
}

// ListRecords 拉取 Zone 下全部记录集。别名记录只读展示。
func (p *Provider) ListRecords(ctx context.Context, zone string) ([]provider.RecordInfo, error) {
	client, err := p.r53Client(ctx)
	if err != nil {
		return nil, err
	}
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return nil, err
	}
	fqdn := zoneFQDN(zone)

	var out []provider.RecordInfo
	paginator := route53.NewListResourceRecordSetsPaginator(client, &route53.ListResourceRecordSetsInput{
		HostedZoneId: &zoneID,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("AWS 拉取记录失败: %w", err)
		}
		for _, r := range page.ResourceRecordSets {
			name := relativeName(strings.TrimSuffix(*r.Name, "."), strings.TrimSuffix(fqdn, "."))
			if r.AliasTarget != nil {
				out = append(out, provider.RecordInfo{
					Name:   name,
					Type:   string(r.Type),
					Value:  "<alias → " + *r.AliasTarget.DNSName + ">",
					ID:     "",
					Status: "alias",
				})
				continue
			}
			var values []string
			for _, rr := range r.ResourceRecords {
				values = append(values, *rr.Value)
			}
			var ttl int64
			if r.TTL != nil {
				ttl = *r.TTL
			}
			key := r53Key{Name: *r.Name, Type: string(r.Type), TTL: ttl, Values: values}
			out = append(out, provider.RecordInfo{
				ID:    encodeR53Key(key),
				Name:  name,
				Type:  string(r.Type),
				Value: strings.Join(values, "\n"),
				TTL:   int(ttl),
			})
		}
	}
	return out, nil
}

// buildRRSet 由 RecordInfo 构建 ResourceRecordSet（FQDN 已补全）。
func buildRRSet(rec provider.RecordInfo, zone string) (types.ResourceRecordSet, error) {
	fqdn := rec.Name
	if fqdn == "@" || fqdn == "" {
		fqdn = zoneFQDN(zone)
	} else if !strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn + "." + zoneFQDN(zone)
	}
	values := strings.Split(rec.Value, "\n")
	rrs := make([]types.ResourceRecord, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		// TXT 值需要带引号
		if rec.Type == "TXT" && !strings.HasPrefix(v, "\"") {
			v = "\"" + strings.ReplaceAll(v, "\"", "\\\"") + "\""
		}
		rrs = append(rrs, types.ResourceRecord{Value: &v})
	}
	if len(rrs) == 0 {
		return types.ResourceRecordSet{}, fmt.Errorf("记录值不能为空")
	}
	ttl := int64(rec.TTL)
	if ttl <= 0 {
		ttl = 300
	}
	return types.ResourceRecordSet{
		Name:            &fqdn,
		Type:            types.RRType(rec.Type),
		TTL:             &ttl,
		ResourceRecords: rrs,
	}, nil
}

// CreateRecord 创建解析记录。
func (p *Provider) CreateRecord(ctx context.Context, zone string, rec provider.RecordInfo) (string, error) {
	client, err := p.r53Client(ctx)
	if err != nil {
		return "", err
	}
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return "", err
	}
	newSet, err := buildRRSet(rec, zone)
	if err != nil {
		return "", err
	}
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch: &types.ChangeBatch{Changes: []types.Change{
			{Action: types.ChangeActionCreate, ResourceRecordSet: &newSet},
		}},
	})
	if err != nil {
		return "", fmt.Errorf("AWS 创建记录失败: %w", err)
	}
	// 返回组合键（记录值已知）
	var values []string
	for _, v := range newSet.ResourceRecords {
		values = append(values, *v.Value)
	}
	key := r53Key{Name: *newSet.Name, Type: rec.Type, TTL: *newSet.TTL, Values: values}
	return encodeR53Key(key), nil
}

// UpdateRecord 更新解析记录（DELETE 原值 + UPSERT 新值，Route53 标准做法）。
func (p *Provider) UpdateRecord(ctx context.Context, zone string, rec provider.RecordInfo) error {
	key, err := decodeR53Key(rec.ID)
	if err != nil {
		return err
	}
	client, err := p.r53Client(ctx)
	if err != nil {
		return err
	}
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return err
	}
	newSet, err := buildRRSet(rec, zone)
	if err != nil {
		return err
	}

	oldRRs := make([]types.ResourceRecord, 0, len(key.Values))
	for _, v := range key.Values {
		vv := v
		oldRRs = append(oldRRs, types.ResourceRecord{Value: &vv})
	}
	oldName := key.Name
	oldTTL := key.TTL
	oldSet := types.ResourceRecordSet{
		Name:            &oldName,
		Type:            types.RRType(key.Type),
		TTL:             &oldTTL,
		ResourceRecords: oldRRs,
	}

	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch: &types.ChangeBatch{Changes: []types.Change{
			{Action: types.ChangeActionDelete, ResourceRecordSet: &oldSet},
			{Action: types.ChangeActionUpsert, ResourceRecordSet: &newSet},
		}},
	})
	if err != nil {
		return fmt.Errorf("AWS 更新记录失败: %w", err)
	}
	return nil
}

// DeleteRecord 删除解析记录。
func (p *Provider) DeleteRecord(ctx context.Context, zone, recordID string) error {
	key, err := decodeR53Key(recordID)
	if err != nil {
		return err
	}
	client, err := p.r53Client(ctx)
	if err != nil {
		return err
	}
	zoneID, err := p.resolveZoneID(ctx, zone)
	if err != nil {
		return err
	}
	oldRRs := make([]types.ResourceRecord, 0, len(key.Values))
	for _, v := range key.Values {
		vv := v
		oldRRs = append(oldRRs, types.ResourceRecord{Value: &vv})
	}
	name := key.Name
	ttl := key.TTL
	_, err = client.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch: &types.ChangeBatch{Changes: []types.Change{
			{Action: types.ChangeActionDelete, ResourceRecordSet: &types.ResourceRecordSet{
				Name:            &name,
				Type:            types.RRType(key.Type),
				TTL:             &ttl,
				ResourceRecords: oldRRs,
			}},
		}},
	})
	if err != nil {
		return fmt.Errorf("AWS 删除记录失败: %w", err)
	}
	return nil
}

// CheckConnection DNS 连通性检测（复用托管区列表）。
func (p *Provider) checkR53Connection(ctx context.Context) error {
	client, err := p.r53Client(ctx)
	if err != nil {
		return err
	}
	_, err = client.ListHostedZones(ctx, &route53.ListHostedZonesInput{})
	if err != nil {
		return fmt.Errorf("AWS 连接失败: %w", err)
	}
	return nil
}
