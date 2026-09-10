package service

import (
	"context"
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
type ZoneService struct {
	accounts *repo.CloudAccountRepo
	zones    *repo.ZoneRepo
	grants   *repo.GrantRepo
	dns      *DNSService // 复用其 Provider 构建与授权过滤逻辑
}

func NewZoneService(accounts *repo.CloudAccountRepo, zones *repo.ZoneRepo, grants *repo.GrantRepo, dns *DNSService) *ZoneService {
	return &ZoneService{accounts: accounts, zones: zones, grants: grants, dns: dns}
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
