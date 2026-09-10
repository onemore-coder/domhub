package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/cryptox"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/provider"
	"github.com/domhub-io/domhub/internal/repo"
	"go.uber.org/zap"
)

// PlanAction 变更计划中的一项。
type PlanAction struct {
	Action string              `json:"action"` // create | update | delete
	Record provider.RecordInfo `json:"record"` // create/update 为期望值；delete 为现网记录
}

// PlanResult push 执行结果。
type PlanResult struct {
	PlanAction
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// DNSService 解析管理业务。
type DNSService struct {
	accounts *repo.CloudAccountRepo
	cipher   *cryptox.Cipher
	audit    *repo.AuditRepo
	grants   *repo.GrantRepo
}

func NewDNSService(accounts *repo.CloudAccountRepo, cipher *cryptox.Cipher, audit *repo.AuditRepo, grants *repo.GrantRepo) *DNSService {
	return &DNSService{accounts: accounts, cipher: cipher, audit: audit, grants: grants}
}

// hasZoneAccess admin 直通；其他角色按 user_zones 授权判定。
func (s *DNSService) hasZoneAccess(userID uint, role string, accountID uint, zone string) (bool, error) {
	if role == model.RoleAdmin {
		return true, nil
	}
	return s.grants.Exists(userID, accountID, zone)
}

// Actor 操作者三要素（来自 JWT 上下文）。
type Actor struct {
	ID       uint
	Username string
	Role     string
}

// buildDNSProvider 构建账号对应的 DNS Provider。
func (s *DNSService) buildDNSProvider(a *model.CloudAccount) (provider.DNSProvider, error) {
	factory, err := provider.GetDNS(a.Provider)
	if err != nil {
		return nil, err
	}
	ak, err := s.cipher.Decrypt(a.AccessKey)
	if err != nil {
		return nil, fmt.Errorf("AccessKey 解密失败: %w", err)
	}
	sk, err := s.cipher.Decrypt(a.SecretKey)
	if err != nil {
		return nil, fmt.Errorf("SecretKey 解密失败: %w", err)
	}
	return factory(provider.Credential{AccessKey: ak, SecretKey: sk, Region: a.Region})
}

func (s *DNSService) getAccount(id uint) (*model.CloudAccount, error) {
	return s.accounts.FindByID(id)
}

// ListZones 列出账号下托管解析的 Zone（非 admin 仅返回被授权的 Zone）。
func (s *DNSService) ListZones(accountID uint, op Actor) ([]provider.ZoneInfo, error) {
	a, err := s.getAccount(accountID)
	if err != nil {
		return nil, err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	zones, err := p.ListZones(ctx)
	if err != nil {
		return nil, err
	}
	if op.Role != model.RoleAdmin {
		allowed, err := s.grants.ZonesByUserAndAccount(op.ID, accountID)
		if err != nil {
			return nil, err
		}
		filtered := make([]provider.ZoneInfo, 0, len(zones))
		for _, z := range zones {
			if allowed[z.Name] {
				filtered = append(filtered, z)
			}
		}
		zones = filtered
	}
	return zones, nil
}

// ListRecords 拉取 Zone 的全部解析记录（非 admin 需有该 Zone 授权）。
func (s *DNSService) ListRecords(accountID uint, zone string, op Actor) ([]provider.RecordInfo, error) {
	if ok, err := s.hasZoneAccess(op.ID, op.Role, accountID, zone); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("无权访问该域名（需要管理员授权）")
	}
	a, err := s.getAccount(accountID)
	if err != nil {
		return nil, err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return p.ListRecords(ctx, zone)
}

// CreateRecord 创建解析记录并审计（非 admin 需有该 Zone 授权）。
func (s *DNSService) CreateRecord(accountID uint, zone string, rec provider.RecordInfo, op Actor) (string, error) {
	if ok, err := s.hasZoneAccess(op.ID, op.Role, accountID, zone); err != nil {
		return "", err
	} else if !ok {
		return "", fmt.Errorf("无权操作该域名（需要管理员授权）")
	}
	a, err := s.getAccount(accountID)
	if err != nil {
		return "", err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return "", err
	}
	if rec.TTL <= 0 {
		rec.TTL = 600
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	id, err := p.CreateRecord(ctx, zone, rec)
	s.writeAudit(op.ID, op.Username, "dns.create", a.Provider+"/"+zone+"/"+rec.Type+" "+rec.Name, rec, err)
	return id, err
}

// UpdateRecord 更新解析记录并审计（非 admin 需有该 Zone 授权）。
func (s *DNSService) UpdateRecord(accountID uint, zone string, rec provider.RecordInfo, op Actor) error {
	if ok, err := s.hasZoneAccess(op.ID, op.Role, accountID, zone); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("无权操作该域名（需要管理员授权）")
	}
	a, err := s.getAccount(accountID)
	if err != nil {
		return err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return err
	}
	if rec.TTL <= 0 {
		rec.TTL = 600
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = p.UpdateRecord(ctx, zone, rec)
	s.writeAudit(op.ID, op.Username, "dns.update", a.Provider+"/"+zone+"/"+rec.Type+" "+rec.Name, rec, err)
	return err
}

// DeleteRecord 删除解析记录并审计（非 admin 需有该 Zone 授权）。
func (s *DNSService) DeleteRecord(accountID uint, zone, recordID, desc string, op Actor) error {
	if ok, err := s.hasZoneAccess(op.ID, op.Role, accountID, zone); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("无权操作该域名（需要管理员授权）")
	}
	a, err := s.getAccount(accountID)
	if err != nil {
		return err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err = p.DeleteRecord(ctx, zone, recordID)
	if desc == "" {
		desc = recordID
	}
	s.writeAudit(op.ID, op.Username, "dns.delete", a.Provider+"/"+zone+"/"+desc, map[string]string{"record_id": recordID}, err)
	return err
}

// BuildPlan 比对现网记录与期望记录，生成变更计划（DNSControl 式 preview）。
//
// 规则：
//   - 期望记录带 ID 且现网存在  → 与同 ID 记录比对，有差异为 update
//   - 期望记录无 ID / ID 不在现网（跨快照比较的新增记录）→ 先按内容键
//     （主机记录+类型+线路）匹配现网剩余记录，匹配到则比对差异，否则 create
//   - 现网记录未被引用 → delete（现网 ID 为空，如 AWS 别名记录，不可操作，跳过）
func BuildPlan(actual, desired []provider.RecordInfo) ([]PlanAction, error) {
	idxByID := make(map[string]int, len(actual))
	idxByKey := make(map[string][]int, len(actual))
	for i, r := range actual {
		if r.ID != "" {
			idxByID[r.ID] = i
		}
		k := recordKey(r)
		idxByKey[k] = append(idxByKey[k], i)
	}
	used := make(map[int]bool, len(actual))

	var plan []PlanAction
	for _, d := range desired {
		if i, ok := idxByID[d.ID]; d.ID != "" && ok {
			used[i] = true
			if !recordEqual(actual[i], d) {
				plan = append(plan, PlanAction{Action: "update", Record: d})
			}
			continue
		}
		// 无 ID 或 ID 不在现网（跨快照比较）：按内容键兜底匹配
		matched := -1
		for _, i := range idxByKey[recordKey(d)] {
			if !used[i] {
				matched = i
				break
			}
		}
		if matched >= 0 {
			used[matched] = true
			if !recordEqual(actual[matched], d) {
				plan = append(plan, PlanAction{Action: "update", Record: d})
			}
			continue
		}
		c := d
		c.ID = "" // 新建动作不携带记录 ID
		plan = append(plan, PlanAction{Action: "create", Record: c})
	}
	for i, r := range actual {
		if r.ID == "" || used[i] {
			continue
		}
		plan = append(plan, PlanAction{Action: "delete", Record: r})
	}
	return plan, nil
}

// recordKey 记录内容键：主机记录 + 类型 + 线路。
func recordKey(r provider.RecordInfo) string {
	return r.Name + "|" + r.Type + "|" + normalizeLine(r.Line)
}

// recordEqual 比较两条记录的关键字段。
func recordEqual(a, b provider.RecordInfo) bool {
	return a.Name == b.Name &&
		a.Type == b.Type &&
		normalizeValue(a.Value) == normalizeValue(b.Value) &&
		a.TTL == b.TTL &&
		a.Priority == b.Priority &&
		normalizeLine(a.Line) == normalizeLine(b.Line)
}

// normalizeValue 统一多值分隔符与空白。
func normalizeValue(v string) string {
	out := make([]byte, 0, len(v))
	for _, line := range splitLines(v) {
		line = trimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line...)
		out = append(out, '\n')
	}
	return string(out)
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}

// normalizeLine 线路统一小写比较（阿里 default / 腾讯 默认）。
func normalizeLine(l string) string {
	switch l {
	case "", "default", "默认":
		return "default"
	default:
		return l
	}
}

// Push 执行变更计划并逐条审计（非 admin 需有该 Zone 授权）。
func (s *DNSService) Push(accountID uint, zone string, actions []PlanAction, op Actor) ([]PlanResult, error) {
	if ok, err := s.hasZoneAccess(op.ID, op.Role, accountID, zone); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("无权操作该域名（需要管理员授权）")
	}
	a, err := s.getAccount(accountID)
	if err != nil {
		return nil, err
	}
	p, err := s.buildDNSProvider(a)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	results := make([]PlanResult, 0, len(actions))
	for _, act := range actions {
		res := PlanResult{PlanAction: act}
		switch act.Action {
		case "create":
			res.Message, err = p.CreateRecord(ctx, zone, act.Record)
			if err == nil {
				res.Message = "创建成功"
			}
		case "update":
			err = p.UpdateRecord(ctx, zone, act.Record)
		case "delete":
			err = p.DeleteRecord(ctx, zone, act.Record.ID)
		default:
			err = fmt.Errorf("未知操作: %s", act.Action)
		}
		res.Success = err == nil
		if err != nil {
			res.Message = err.Error()
		}
		s.writeAudit(op.ID, op.Username, "dns."+act.Action,
			a.Provider+"/"+zone+"/"+act.Record.Type+" "+act.Record.Name, act.Record, err)
		results = append(results, res)
	}
	return results, nil
}

// writeAudit 记录审计日志（失败不阻断业务）。
func (s *DNSService) writeAudit(userID uint, username, action, resource string, detail any, err error) {
	detailJSON, _ := json.Marshal(detail)
	log := model.AuditLog{
		UserID:   userID,
		Username: username,
		Action:   action,
		Resource: resource,
		Detail:   string(detailJSON),
		Status:   "success",
	}
	if err != nil {
		log.Status = "failed"
		log.Message = err.Error()
	}
	if err := s.audit.Create(&log); err != nil {
		logger.L().Error("写入审计日志失败", zap.Error(err))
	}
}