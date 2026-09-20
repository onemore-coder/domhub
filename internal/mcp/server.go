// Package mcp 内置 MCP（Model Context Protocol）服务器：
// 客户的 AI 助手（Claude / Cursor / WorkBuddy 等）通过标准 MCP 协议
// 直接查询 DomHub 的域名 / DNS / 证书 / 告警数据。
//
// 接入方式：Streamable HTTP（端点 /mcp），凭据复用 dht_ 前缀 API Token，
// 通过 Authorization: Bearer dht_xxx 或 X-Api-Key 头传入。
// 首期仅开放只读工具；写入类工具后续按 scope 模型另行开放。
package mcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/provider"
	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
)

// Version MCP 服务器版本（随工具集演进递增）。
const Version = "0.2.0"

// Deps MCP 服务器依赖。
type Deps struct {
	Tokens   *service.TokenService
	Domains  *repo.DomainRepo
	Accounts *repo.CloudAccountRepo
	Alerts   *repo.AlertRepo
	Zones    *service.ZoneService
	Certs    *service.CertService
	DNS      *service.DNSService    // 写工具：记录增删改（复用 Zone 授权与审计）
	Records  *repo.DnsRecordRepo    // 写工具：镜像记录定位（record_id → 账号/Zone/厂商记录 ID）
}

// NewServer 构建注册全部工具的 MCP Server（导出以便测试直连）。
func NewServer(d Deps) *mcp.Server {
	// fail-fast：写工具依赖缺失时启动即报错，避免请求期空指针
	if d.Tokens == nil || d.DNS == nil || d.Records == nil || d.Zones == nil {
		panic("mcp.Deps 配置不完整：Tokens / DNS / Records / Zones 均不可为 nil")
	}
	s := mcp.NewServer(&mcp.Implementation{Name: "domhub", Version: Version}, nil)
	registerTools(s, d)
	return s
}

// Handler 返回挂载在 /mcp 的 Streamable HTTP 处理器。
func Handler(d Deps) http.Handler {
	s := NewServer(d)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
}

func registerTools(s *mcp.Server, d Deps) {
	add(s, d, "overview", "DomHub 数据概览：域名厂商分布、30 天内到期域名数、Zone/云账号/证书数量与临期证书统计", d.overview)
	add(s, d, "list_domains", "查询域名台账（支持关键词/厂商/到期时间过滤，分页）", d.listDomains)
	add(s, d, "list_zones", "查询已托管的 DNS Zone 列表（含归属云账号与记录数）", d.listZones)
	add(s, d, "list_dns_records", "查询 DNS 解析记录镜像（可按账号/Zone/关键词/记录类型过滤）", d.listRecords)
	add(s, d, "search_dns", "跨 Zone 全局搜索：按关键词同时匹配 Zone 名称与解析记录", d.searchDNS)
	add(s, d, "list_certs", "查询 SSL 证书监控状态（含剩余天数，可只看 N 天内到期）", d.listCerts)
	add(s, d, "list_cloud_accounts", "查询已接入的云账号列表（密钥脱敏，仅元信息）", d.listAccounts)
	add(s, d, "list_alert_logs", "查询最近的告警发送记录（域名/证书到期提醒）", d.listAlertLogs)
	// 写工具（readwrite Token 专属；记录定位用镜像表主键 record_id，
	// 可先调 list_dns_records 拿到 ID；全部操作写入审计日志）
	add(s, d, "create_dns_record", "创建 DNS 解析记录（写操作，readwrite Token）：指定 Zone + 主机记录/类型/值", d.createRecord)
	add(s, d, "update_dns_record", "修改 DNS 解析记录（写操作，readwrite Token）：按镜像 record_id 定位，可改记录值/TTL/优先级/代理状态", d.updateRecord)
	add(s, d, "delete_dns_record", "删除 DNS 解析记录（写操作，readwrite Token）：按镜像 record_id 定位，删除前应与用户确认", d.deleteRecord)
}

// add 统一的注册包装：鉴权（dht_ Token → Actor）+ 错误包装，减少样板代码。
func add[In any](s *mcp.Server, d Deps, name, desc string, run func(ctx context.Context, op service.Actor, in In) (any, error)) {
	mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			op, err := actorFromReq(d, req)
			if err != nil {
				return nil, nil, err
			}
			out, err := run(ctx, op, in)
			if err != nil {
				return nil, nil, err
			}
			return nil, out, nil
		})
}

// actorFromReq 从 MCP 请求透传的 HTTP 头解析 dht_ API Token → 用户三要素。
// 与 web 中间件一致：X-Api-Key 头 → Authorization 头。
func actorFromReq(d Deps, req *mcp.CallToolRequest) (service.Actor, error) {
	var cands []string
	if req.Extra != nil && req.Extra.Header != nil {
		if xk := req.Extra.Header.Get("X-Api-Key"); xk != "" {
			cands = append(cands, xk)
		}
		if auth := req.Extra.Header.Get("Authorization"); auth != "" {
			cands = append(cands, strings.TrimPrefix(auth, "Bearer "))
		}
	}
	for _, c := range cands {
		if !strings.HasPrefix(c, model.ApiTokenPrefix) {
			continue
		}
		if uid, username, role, scope, ok := d.Tokens.Resolve(c); ok {
			return service.Actor{ID: uid, Username: username, Role: role, Scope: scope}, nil
		}
	}
	return service.Actor{}, fmt.Errorf("鉴权失败：需要有效的 dht_ API Token，"+
		"请在 DomHub「安全设置 → API Token」页生成，并以 Authorization: Bearer dht_xxx 头传入")
}

// requireWrite 写工具前置校验：仅 readwrite Token 可执行写操作。
func requireWrite(op service.Actor) error {
	if op.Scope == model.TokenScopeReadWrite {
		return nil
	}
	return fmt.Errorf("当前 API Token 为只读（readonly）权限，无法执行写操作；" +
		"请在 DomHub「安全设置 → API Token」生成读写（readwrite）令牌后重试")
}

// ---- 工具实现 ----

// overviewOut 概览结果。
type overviewOut struct {
	DomainsByProvider  []repo.ProviderCount `json:"domains_by_provider"`
	DomainsExpiring30d int64                `json:"domains_expiring_in_30d"`
	Zones              int                  `json:"zones"`
	CloudAccounts      int                  `json:"cloud_accounts"`
	CertsMonitored     int                  `json:"certs_monitored"`
	CertsExpiring30d   int                  `json:"certs_expiring_in_30d"`
}

func (d Deps) overview(_ context.Context, op service.Actor, _ struct{}) (any, error) {
	providers, err := d.Domains.CountByProvider()
	if err != nil {
		return nil, err
	}
	_, expiring, err := d.Domains.List(repo.DomainFilter{ExpiringDays: 30, Page: 1, PageSize: 1})
	if err != nil {
		return nil, err
	}
	zones, err := d.Zones.ListCached(op)
	if err != nil {
		return nil, err
	}
	accounts, err := d.Accounts.List()
	if err != nil {
		return nil, err
	}
	certs, err := d.Certs.ListStatus()
	if err != nil {
		return nil, err
	}
	certsSoon := 0
	for _, c := range certs {
		if c.OK && c.DaysLeft >= 0 && c.DaysLeft <= 30 {
			certsSoon++
		}
	}
	return overviewOut{
		DomainsByProvider:  providers,
		DomainsExpiring30d: expiring,
		Zones:              len(zones),
		CloudAccounts:      len(accounts),
		CertsMonitored:     len(certs),
		CertsExpiring30d:   certsSoon,
	}, nil
}

type listDomainsIn struct {
	Keyword      string `json:"keyword,omitempty" jsonschema:"域名关键词，模糊匹配，可空"`
	Provider     string `json:"provider,omitempty" jsonschema:"厂商过滤：tencent / aliyun / aws，可空"`
	ExpiringDays int    `json:"expiring_days,omitempty" jsonschema:"只看 N 天内到期的域名，0 表示不过滤"`
	Page         int    `json:"page,omitempty" jsonschema:"页码，从 1 开始，默认 1"`
	PageSize     int    `json:"page_size,omitempty" jsonschema:"每页数量，默认 20，最大 100"`
}

func (d Deps) listDomains(_ context.Context, _ service.Actor, in listDomainsIn) (any, error) {
	page := in.Page
	if page < 1 {
		page = 1
	}
	size := in.PageSize
	if size < 1 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	items, total, err := d.Domains.List(repo.DomainFilter{
		Keyword:      in.Keyword,
		Provider:     in.Provider,
		ExpiringDays: in.ExpiringDays,
		Page:         page,
		PageSize:     size,
	})
	if err != nil {
		return nil, err
	}
	return ginH{"total": total, "page": page, "page_size": size, "items": items}, nil
}

func (d Deps) listZones(_ context.Context, op service.Actor, _ struct{}) (any, error) {
	return d.Zones.ListCached(op)
}

type listRecordsIn struct {
	AccountID  uint   `json:"account_id,omitempty" jsonschema:"云账号 ID，可空"`
	Zone       string `json:"zone,omitempty" jsonschema:"Zone 名称（如 example.com），可空表示跨 Zone 查询"`
	Keyword    string `json:"keyword,omitempty" jsonschema:"关键词，匹配主机记录或记录值，可空"`
	RecordType string `json:"record_type,omitempty" jsonschema:"记录类型：A / AAAA / CNAME / MX / TXT 等，可空"`
	Limit      int    `json:"limit,omitempty" jsonschema:"返回条数上限，默认 50，最大 200"`
}

func (d Deps) listRecords(_ context.Context, op service.Actor, in listRecordsIn) (any, error) {
	limit := in.Limit
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	accountID, err := d.resolveAccountID(op, in.Zone, in.AccountID)
	if err != nil {
		return nil, err
	}
	return d.Zones.ListRecordsCached(op, accountID, in.Zone, in.Keyword, in.RecordType, limit)
}

// resolveAccountID 仅指定 Zone 未指定账号时，从授权可见的 Zone 视图自动解析归属账号。
func (d Deps) resolveAccountID(op service.Actor, zone string, accountID uint) (uint, error) {
	if zone == "" || accountID != 0 {
		return accountID, nil
	}
	views, err := d.Zones.ListCached(op)
	if err != nil {
		return 0, err
	}
	for _, v := range views {
		if v.Name == zone {
			return v.CloudAccountID, nil
		}
	}
	return 0, fmt.Errorf("未找到 Zone %s（请检查名称，或确认该 Zone 归属的云账号已接入）", zone)
}

type searchDNSIn struct {
	Query string `json:"query" jsonschema:"搜索关键词（域名 / 主机记录 / 记录值片段）"`
}

// recordHit 搜索结果中的记录命中项（补充账号名便于展示）。
type recordHit struct {
	ID          uint   `json:"id"`
	AccountName string `json:"account_name"`
	ZoneName    string `json:"zone_name"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Value       string `json:"value"`
	TTL         int    `json:"ttl"`
}

func (d Deps) searchDNS(_ context.Context, op service.Actor, in searchDNSIn) (any, error) {
	q := strings.TrimSpace(in.Query)
	if q == "" {
		return ginH{"zones": []repo.ZoneView{}, "records": []any{}}, nil
	}
	zones, err := d.Zones.ListCached(op)
	if err != nil {
		return nil, err
	}
	kw := strings.ToLower(q)
	zoneHits := make([]repo.ZoneView, 0, 5)
	for _, z := range zones {
		if strings.Contains(strings.ToLower(z.Name), kw) {
			zoneHits = append(zoneHits, z)
			if len(zoneHits) >= 5 {
				break
			}
		}
	}
	records, err := d.Zones.ListRecordsCached(op, 0, "", q, "", 10)
	if err != nil {
		return nil, err
	}
	accName := map[uint]string{}
	for _, v := range zones {
		accName[v.CloudAccountID] = v.AccountName
	}
	hits := make([]recordHit, 0, len(records))
	for _, r := range records {
		hits = append(hits, recordHit{
			ID: r.ID, AccountName: accName[r.CloudAccountID], ZoneName: r.ZoneName,
			Name: r.Name, Type: r.Type, Value: r.Value, TTL: r.TTL,
		})
	}
	return ginH{"zones": zoneHits, "records": hits}, nil
}

type listCertsIn struct {
	ExpiringDays int `json:"expiring_days,omitempty" jsonschema:"只看 N 天内到期（含已过期）的证书，0 表示全部"`
}

func (d Deps) listCerts(_ context.Context, _ service.Actor, in listCertsIn) (any, error) {
	certs, err := d.Certs.ListStatus()
	if err != nil {
		return nil, err
	}
	if in.ExpiringDays > 0 {
		filtered := certs[:0]
		for _, c := range certs {
			if c.DaysLeft <= in.ExpiringDays {
				filtered = append(filtered, c)
			}
		}
		certs = filtered
	}
	return certs, nil
}

// accountView 云账号元信息（不含任何密钥字段）。
type accountView struct {
	ID           uint   `json:"id"`
	Name         string `json:"name"`
	Provider     string `json:"provider"`
	Region       string `json:"region"`
	Status       int    `json:"status"`
	LastCheckOK  bool   `json:"last_check_ok"`
	LastCheckMsg string `json:"last_check_msg"`
}

func (d Deps) listAccounts(_ context.Context, _ service.Actor, _ struct{}) (any, error) {
	accounts, err := d.Accounts.List()
	if err != nil {
		return nil, err
	}
	views := make([]accountView, 0, len(accounts))
	for _, a := range accounts {
		views = append(views, accountView{
			ID: a.ID, Name: a.Name, Provider: a.Provider, Region: a.Region,
			Status: a.Status, LastCheckOK: a.LastCheckOK, LastCheckMsg: a.LastCheckMsg,
		})
	}
	return views, nil
}

type listAlertLogsIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"返回条数上限，默认 20，最大 100"`
}

func (d Deps) listAlertLogs(_ context.Context, _ service.Actor, in listAlertLogsIn) (any, error) {
	limit := in.Limit
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return d.Alerts.ListLogs(limit)
}

// ginH 轻量 map（避免 mcp 包直接依赖 gin）。
type ginH = map[string]any

// ---- 写工具（readwrite Token 专属）----

type createRecordIn struct {
	AccountID uint   `json:"account_id,omitempty" jsonschema:"云账号 ID，可空（仅给 Zone 时自动按归属账号解析）"`
	Zone      string `json:"zone" jsonschema:"Zone 名称（如 example.com）"`
	Name      string `json:"name" jsonschema:"主机记录：@ 表示根域名，www 为子域前缀"`
	Type      string `json:"type" jsonschema:"记录类型：A / AAAA / CNAME / TXT / MX 等"`
	Value     string `json:"value" jsonschema:"记录值（多值以换行分隔）"`
	TTL       int    `json:"ttl,omitempty" jsonschema:"TTL 秒数，默认 600"`
	Priority  int    `json:"priority,omitempty" jsonschema:"MX/SRV 优先级，其他类型留 0"`
	Line      string `json:"line,omitempty" jsonschema:"运营商线路（仅国内厂商）：default / 移动 / 联通 / 电信，可空"`
	Proxied   bool   `json:"proxied,omitempty" jsonschema:"是否开启 CDN 代理（仅 Cloudflare 橙云记录）"`
}

func (d Deps) createRecord(_ context.Context, op service.Actor, in createRecordIn) (any, error) {
	if err := requireWrite(op); err != nil {
		return nil, err
	}
	if strings.TrimSpace(in.Zone) == "" || strings.TrimSpace(in.Type) == "" {
		return nil, fmt.Errorf("zone 与 type 为必填参数")
	}
	accountID, err := d.resolveAccountID(op, in.Zone, in.AccountID)
	if err != nil {
		return nil, err
	}
	rec := provider.RecordInfo{
		Name: in.Name, Type: strings.ToUpper(in.Type), Value: in.Value,
		TTL: in.TTL, Priority: in.Priority, Line: in.Line, Proxied: in.Proxied,
	}
	id, err := d.DNS.CreateRecord(accountID, in.Zone, rec, op)
	if err != nil {
		return nil, err
	}
	return ginH{"success": true, "provider_record_id": id,
		"record": rec, "zone": in.Zone, "account_id": accountID}, nil
}

type updateRecordIn struct {
	RecordID uint   `json:"record_id" jsonschema:"解析记录的镜像 ID（先调 list_dns_records 获取）"`
	Value    string `json:"value,omitempty" jsonschema:"新记录值，留空保持不变"`
	TTL      int    `json:"ttl,omitempty" jsonschema:"新 TTL 秒数，0 表示保持不变"`
	Priority *int   `json:"priority,omitempty" jsonschema:"新 MX/SRV 优先级（指针区分未传），可空"`
	Proxied  *bool  `json:"proxied,omitempty" jsonschema:"CDN 代理开关（指针区分未传），可空"`
}

func (d Deps) updateRecord(_ context.Context, op service.Actor, in updateRecordIn) (any, error) {
	if err := requireWrite(op); err != nil {
		return nil, err
	}
	rec, err := d.Records.FindByID(in.RecordID)
	if err != nil {
		return nil, fmt.Errorf("镜像记录 %d 不存在（record_id 需先通过 list_dns_records 查询获取）", in.RecordID)
	}
	updated := provider.RecordInfo{
		ID: rec.ProviderRecordID, Name: rec.Name, Type: rec.Type, Value: rec.Value,
		TTL: rec.TTL, Priority: rec.Priority, Line: rec.Line, Status: rec.Status,
		Remark: rec.Remark, Proxied: rec.Proxied,
	}
	if in.Value != "" {
		updated.Value = in.Value
	}
	if in.TTL > 0 {
		updated.TTL = in.TTL
	}
	if in.Priority != nil {
		updated.Priority = *in.Priority
	}
	if in.Proxied != nil {
		updated.Proxied = *in.Proxied
	}
	if err := d.DNS.UpdateRecord(rec.CloudAccountID, rec.ZoneName, updated, op); err != nil {
		return nil, err
	}
	return ginH{"success": true, "record": updated,
		"zone": rec.ZoneName, "account_id": rec.CloudAccountID}, nil
}

type deleteRecordIn struct {
	RecordID uint `json:"record_id" jsonschema:"解析记录的镜像 ID（先调 list_dns_records 获取；删除前应与用户确认）"`
}

func (d Deps) deleteRecord(_ context.Context, op service.Actor, in deleteRecordIn) (any, error) {
	if err := requireWrite(op); err != nil {
		return nil, err
	}
	rec, err := d.Records.FindByID(in.RecordID)
	if err != nil {
		return nil, fmt.Errorf("镜像记录 %d 不存在（record_id 需先通过 list_dns_records 查询获取）", in.RecordID)
	}
	if err := d.DNS.DeleteRecord(rec.CloudAccountID, rec.ZoneName, rec.ProviderRecordID,
		rec.Type+" "+rec.Name, op); err != nil {
		return nil, err
	}
	return ginH{"success": true, "deleted": ginH{
		"id": rec.ID, "zone": rec.ZoneName, "name": rec.Name, "type": rec.Type, "value": rec.Value,
	}}, nil
}
