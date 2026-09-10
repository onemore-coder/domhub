package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/repo"
)

// ZoneService 托管域名元数据缓存：列表读缓存秒开，刷新任务回源厂商 API。
// 操作类接口（记录 CRUD/plan/push）不走本缓存，保持实时。
//
// 解析记录镜像（dns_records）：同「列表走缓存、操作走实时」的分工——
// 读（列表/搜索/证书监控）走镜像，写（CRUD/push）实时写云端，
// 成功后经 DNSService.OnChange 回调回源刷新镜像。
type ZoneService struct {
	accounts *repo.CloudAccountRepo
	zones    *repo.ZoneRepo
	records  *repo.DnsRecordRepo
	grants   *repo.GrantRepo
	dns      *DNSService // 复用其 Provider 构建与授权过滤逻辑
}

func NewZoneService(accounts *repo.CloudAccountRepo, zones *repo.ZoneRepo, records *repo.DnsRecordRepo,
	grants *repo.GrantRepo, dns *DNSService) *ZoneService {
	return &ZoneService{accounts: accounts, zones: zones, records: records, grants: grants, dns: dns}
}

// accountRateLimit 多账号串行刷新时的间隔，规避厂商 API 限流。
const accountRateLimit = 300 * time.Millisecond

// Refresh 刷新指定账号（accountID=0 表示全部启用账号）的 Zone 缓存。
// 返回刷新的账号数与 Zone 总数。
func (s *ZoneService) Refresh(accountID uint) (int, int, error) {
	var targets []model.CloudAccount
	if accountID > 0 {
		a, err := s.accounts.FindByID(accountID)
		if err != nil {
			return 0, 0, err
		}
		targets = []model.CloudAccount{*a}
	} else {
		list, err := s.accounts.List()
		if err != nil {
			return 0, 0, err
		}
		for _, a := range list {
			if a.Status == 1 {
				targets = append(targets, a)
			}
		}
	}

	totalZones, okAccounts := 0, 0
	var failed []string
	for i, a := range targets {
		if i > 0 {
			time.Sleep(accountRateLimit)
		}
		n, err := s.refreshAccount(&a)
		if err != nil {
			logger.L().Warn("刷新 Zone 缓存失败", zap.String("account", a.Name), zap.Error(err))
			failed = append(failed, a.Name)
			continue
		}
		okAccounts++
		totalZones += n
	}
	if len(failed) > 0 && okAccounts == 0 {
		return 0, 0, fmt.Errorf("全部账号刷新失败: %s", joinNames(failed))
	}
	return okAccounts, totalZones, nil
}

// refreshAccount 拉取单账号 Zone 列表并写入缓存，返回条数。
func (s *ZoneService) refreshAccount(a *model.CloudAccount) (int, error) {
	dnsProvider, err := s.dns.buildDNSProvider(a)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	list, err := dnsProvider.ListZones(ctx)
	if err != nil {
		return 0, err
	}
	batchStart := time.Now()
	zones := make([]model.Zone, 0, len(list))
	for _, z := range list {
		zones = append(zones, model.Zone{Name: z.Name, RecordCount: z.RecordCount})
	}
	if err := s.zones.UpsertBatch(a.ID, zones, batchStart); err != nil {
		return 0, err
	}
	return len(zones), nil
}

// ListCached 返回缓存视图（admin 全量；其他角色按 user_zones 授权过滤）。
func (s *ZoneService) ListCached(op Actor) ([]repo.ZoneView, error) {
	views, err := s.zones.ListViews()
	if err != nil {
		return nil, err
	}
	if op.Role == model.RoleAdmin {
		return views, nil
	}
	grants, err := s.grants.ZonesByUser(op.ID)
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(grants))
	for _, g := range grants {
		allowed[fmt.Sprintf("%d:%s", g.CloudAccountID, g.Zone)] = true
	}
	filtered := make([]repo.ZoneView, 0, len(views))
	for _, v := range views {
		if allowed[fmt.Sprintf("%d:%s", v.CloudAccountID, v.Name)] {
			filtered = append(filtered, v)
		}
	}
	return filtered, nil
}

func joinNames(names []string) string {
	return strings.Join(names, "、")
}

// CheckZoneAccess 判断操作者是否有某 Zone 的访问权（admin 直通，其他按授权）。
func (s *ZoneService) CheckZoneAccess(op Actor, accountID uint, zone string) (bool, error) {
	if op.Role == model.RoleAdmin {
		return true, nil
	}
	return s.grants.Exists(op.ID, accountID, zone)
}

// SyncRecordsFor 回源刷新单个 Zone 的解析记录镜像（云端优先：先取厂商实时数据再覆盖本地）。
// 变更操作成功后与 sync_records 定时任务共用此路径。
func (s *ZoneService) SyncRecordsFor(accountID uint, zone string) (int, error) {
	a, err := s.accounts.FindByID(accountID)
	if err != nil {
		return 0, err
	}
	p, err := s.dns.buildDNSProvider(a)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	list, err := p.ListRecords(ctx, zone)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	rows := make([]model.DnsRecord, 0, len(list))
	for _, r := range list {
		rows = append(rows, model.DnsRecord{
			CloudAccountID:   accountID,
			ZoneName:         zone,
			RecordKey:        recordContentKey(r.Name, r.Type, r.Value, r.Line),
			ProviderRecordID: r.ID,
			Name:             r.Name,
			Type:             r.Type,
			Value:            r.Value,
			TTL:              r.TTL,
			Priority:         r.Priority,
			Line:             r.Line,
			Status:           r.Status,
			Remark:           r.Remark,
			Proxied:          r.Proxied,
			SyncedAt:         now,
		})
	}
	if err := s.records.ReplaceAllFor(accountID, zone, rows); err != nil {
		return 0, err
	}
	// 同步回写 Zone 缓存的记录数（比 ListZones 返回的 count 更实时）
	if err := s.zones.TouchRecordCount(accountID, zone, len(rows), now); err != nil {
		logger.L().Warn("回写 Zone 记录数失败", zap.String("zone", zone), zap.Error(err))
	}
	return len(rows), nil
}

// SyncAllRecords 回源刷新解析记录镜像：accountID>0 限该账号，0 表示全部启用账号。
// 逐 Zone 串行 + 限流；单 Zone 失败不阻塞其余。返回同步的 Zone 数与记录总数。
func (s *ZoneService) SyncAllRecords(accountID uint) (int, int, error) {
	views, err := s.zones.ListViews()
	if err != nil {
		return 0, 0, err
	}
	totalZones, totalRecords := 0, 0
	var failed []string
	for i, v := range views {
		if accountID > 0 && v.CloudAccountID != accountID {
			continue
		}
		if i > 0 {
			time.Sleep(accountRateLimit)
		}
		n, err := s.SyncRecordsFor(v.CloudAccountID, v.Name)
		if err != nil {
			logger.L().Warn("解析记录镜像同步失败",
				zap.Uint("account", v.CloudAccountID), zap.String("zone", v.Name), zap.Error(err))
			failed = append(failed, v.Name)
			continue
		}
		totalZones++
		totalRecords += n
	}
	if len(failed) > 0 && totalZones == 0 {
		return 0, 0, fmt.Errorf("全部 Zone 同步失败: %s", joinNames(failed))
	}
	return totalZones, totalRecords, nil
}

// ListRecordsCached 读取解析记录镜像。zone 非空时按 Zone 查询（校验授权），
// 为空时跨 Zone 查询（非 admin 限定在授权范围内）。q 支持主机/记录值模糊匹配。
func (s *ZoneService) ListRecordsCached(op Actor, accountID uint, zone, q, rtype string, limit int) ([]model.DnsRecord, error) {
	if zone != "" {
		if op.Role != model.RoleAdmin {
			ok, err := s.grants.Exists(op.ID, accountID, zone)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, fmt.Errorf("无权访问该域名（需要管理员授权）")
			}
		}
		return s.records.ListForZone(accountID, zone)
	}
	refs := make([]repo.ZoneRef, 0, 8)
	if op.Role == model.RoleAdmin {
		views, err := s.zones.ListViews()
		if err != nil {
			return nil, err
		}
		for _, v := range views {
			if accountID > 0 && v.CloudAccountID != accountID {
				continue
			}
			refs = append(refs, repo.ZoneRef{CloudAccountID: v.CloudAccountID, Zone: v.Name})
		}
	} else {
		grants, err := s.grants.ZonesByUser(op.ID)
		if err != nil {
			return nil, err
		}
		for _, g := range grants {
			if accountID > 0 && g.CloudAccountID != accountID {
				continue
			}
			refs = append(refs, repo.ZoneRef{CloudAccountID: g.CloudAccountID, Zone: g.Zone})
		}
	}
	return s.records.ListFiltered(accountID, refs, q, rtype, limit)
}

// recordContentKey 镜像记录唯一键：sha256(name|type|value|line)。
// 用内容键而非厂商记录 ID，规避 AWS Route53 无记录 ID 的差异。
func recordContentKey(name, rtype, value, line string) string {
	h := sha256.Sum256([]byte(name + "|" + rtype + "|" + value + "|" + line))
	return hex.EncodeToString(h[:])
}
