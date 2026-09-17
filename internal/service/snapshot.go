package service

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/logger"
	"github.com/onemore-coder/domhub/internal/pkg/notify"
	"github.com/onemore-coder/domhub/internal/repo"
)

// SnapshotService 解析记录快照与漂移检测业务。
type SnapshotService struct {
	snapshots *repo.SnapshotRepo
	alerts    *repo.AlertRepo
	dns       *DNSService
}

func NewSnapshotService(snapshots *repo.SnapshotRepo, alerts *repo.AlertRepo, dns *DNSService) *SnapshotService {
	return &SnapshotService{snapshots: snapshots, alerts: alerts, dns: dns}
}

// Capture 拉取现网记录并保存一份快照。
func (s *SnapshotService) Capture(accountID uint, zone, source, note string, userID uint) (*model.DNSRecordSnapshot, error) {
	records, err := s.dns.ListRecords(accountID, zone, Actor{ID: userID, Role: model.RoleAdmin, Username: "system"})
	if err != nil {
		return nil, err
	}
	snap := &model.DNSRecordSnapshot{
		AccountID:  accountID,
		Zone:       zone,
		Source:     source,
		RecordJSON: repo.EncodeRecords(records),
		Count:      len(records),
		Note:       note,
		CreatedBy:  userID,
	}
	if err := s.snapshots.Save(snap); err != nil {
		return nil, err
	}
	// 超过 50 份时清理旧快照
	if n, err := s.snapshots.Delete(accountID, zone, 50); err == nil && n > 0 {
		logger.L().Info("清理旧快照", zap.String("zone", zone), zap.Int64("deleted", n))
	}
	snap.RecordJSON = ""
	return snap, nil
}

// List 列出 Zone 的快照（不含正文）。
func (s *SnapshotService) List(accountID uint, zone string, limit int) ([]model.DNSRecordSnapshot, error) {
	return s.snapshots.List(accountID, zone, limit)
}

// Get 取单份快照（含记录正文）。
func (s *SnapshotService) Get(id uint) (*model.DNSRecordSnapshot, error) {
	return s.snapshots.FindByID(id)
}

// DiffSnapshots 比较两份快照：base（旧）→ target（新），生成与 /dns/plan 相同结构的变更计划。
// 计划含义：对 base 应用这些动作可得到 target。
func (s *SnapshotService) DiffSnapshots(baseID, targetID uint) (base, target *model.DNSRecordSnapshot, plan []PlanAction, err error) {
	base, err = s.snapshots.FindByID(baseID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("基准快照不存在")
	}
	target, err = s.snapshots.FindByID(targetID)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("目标快照不存在")
	}
	plan, err = BuildPlan(repo.DecodeRecords(base.RecordJSON), repo.DecodeRecords(target.RecordJSON))
	if err != nil {
		return nil, nil, nil, err
	}
	if plan == nil {
		plan = []PlanAction{}
	}
	return base, target, plan, nil
}

// RestorePlan 以指定快照为目标，生成"现网 → 快照"的恢复计划（不执行，走 /dns/push 执行）。
func (s *SnapshotService) RestorePlan(accountID uint, zone string, snapshotID uint, op Actor) ([]PlanAction, error) {
	snap, err := s.snapshots.FindByID(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("快照不存在")
	}
	if snap.AccountID != accountID || snap.Zone != zone {
		return nil, fmt.Errorf("快照与 Zone 不匹配")
	}
	live, err := s.dns.ListRecords(accountID, zone, op)
	if err != nil {
		return nil, err
	}
	plan, err := BuildPlan(live, repo.DecodeRecords(snap.RecordJSON))
	if err != nil {
		return nil, err
	}
	if plan == nil {
		plan = []PlanAction{}
	}
	return plan, nil
}

// RunDriftCheck 对所有有快照的 Zone 做一轮漂移检测：
// 现网 vs 最新快照，有差异 → 保存漂移快照（作为下次基线）+ 通知所有启用渠道。
// 返回发生漂移的 Zone 数。
func (s *SnapshotService) RunDriftCheck(ctx context.Context) (int, error) {
	zones, err := s.snapshots.DistinctZones()
	if err != nil {
		return 0, err
	}
	drifted := 0
	for _, z := range zones {
		latest, err := s.snapshots.Latest(z.AccountID, z.Zone)
		if err != nil {
			continue
		}
		live, err := s.dns.ListRecords(z.AccountID, z.Zone, Actor{Role: model.RoleAdmin, Username: "drift-check"})
		if err != nil {
			logger.L().Warn("漂移检测拉取现网失败", zap.String("zone", z.Zone), zap.Error(err))
			continue
		}
		plan, err := BuildPlan(repo.DecodeRecords(latest.RecordJSON), live)
		if err != nil || len(plan) == 0 {
			continue // 无漂移
		}

		// 保存漂移快照作为新基线，避免重复告警
		if _, err := s.Capture(z.AccountID, z.Zone, "drift", fmt.Sprintf("检测到 %d 项漂移", len(plan)), 0); err != nil {
			logger.L().Error("保存漂移快照失败", zap.String("zone", z.Zone), zap.Error(err))
		}

		// 通知
		var b strings.Builder
		for _, a := range plan {
			fmt.Fprintf(&b, "· %s %s %s %s\n", strings.ToUpper(a.Action), a.Record.Type, a.Record.Name, truncateStr(a.Record.Value, 40))
		}
		title := fmt.Sprintf("【DomHub】DNS 漂移告警：%s", z.Zone)
		content := fmt.Sprintf("检测到 %s 的解析记录与最近快照存在 %d 处差异：\n%s\n当前记录已自动留存为新快照。如非本人操作，请检查云账号安全。",
			z.Zone, len(plan), b.String())
		s.notifyAll(title, content)
		drifted++
	}
	return drifted, nil
}

func (s *SnapshotService) notifyAll(title, content string) {
	channels, err := s.alerts.ListChannels()
	if err != nil {
		return
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		n, err := notify.Build(ch.Type, ch.Config)
		if err != nil {
			continue
		}
		if err := n.Send(title, content); err != nil {
			logger.L().Warn("漂移告警发送失败", zap.String("channel", ch.Name), zap.Error(err))
		}
	}
}

func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
