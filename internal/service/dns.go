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
}

func NewDNSService(accounts *repo.CloudAccountRepo, cipher *cryptox.Cipher, audit *repo.AuditRepo) *DNSService {
	return &DNSService{accounts: accounts, cipher: cipher, audit: audit}
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

// ListZones 列出账号下托管解析的 Zone。
func (s *DNSService) ListZones(accountID uint) ([]provider.ZoneInfo, error) {
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
	return p.ListZones(ctx)
}

// ListRecords 拉取 Zone 的全部解析记录。
func (s *DNSService) ListRecords(accountID uint, zone string) ([]provider.RecordInfo, error) {
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

// CreateRecord 创建解析记录并审计。
func (s *DNSService) CreateRecord(accountID uint, zone string, rec provider.RecordInfo, userID uint, username string) (string, error) {
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
	s.writeAudit(userID, username, "dns.create", a.Provider+"/"+zone+"/"+rec.Type+" "+rec.Name, rec, err)
	return id, err
}

// UpdateRecord 更新解析记录并审计。
func (s *DNSService) UpdateRecord(accountID uint, zone string, rec provider.RecordInfo, userID uint, username string) error {
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
	s.writeAudit(userID, username, "dns.update", a.Provider+"/"+zone+"/"+rec.Type+" "+rec.Name, rec, err)
	return err
}

// DeleteRecord 删除解析记录并审计。
func (s *DNSService) DeleteRecord(accountID uint, zone, recordID, desc string, userID uint, username string) error {
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
	s.writeAudit(userID, username, "dns.delete", a.Provider+"/"+zone+"/"+desc, map[string]string{"record_id": recordID}, err)
	return err
}

// BuildPlan 比对现网记录与期望记录，生成变更计划（DNSControl 式 preview）。
//
// 规则：
//   - 期望记录带 ID   → 与现网同 ID 记录比对，有差异为 update
//   - 期望记录无 ID   → create
//   - 现网记录未被引用 → delete（现网 ID 为空，如 AWS 别名记录，不可操作，跳过）
func BuildPlan(actual, desired []provider.RecordInfo) ([]PlanAction, error) {
	byID := make(map[string]provider.RecordInfo, len(actual))
	for _, r := range actual {
		if r.ID != "" {
			byID[r.ID] = r
		}
	}
	used := make(map[string]bool, len(actual))

	var plan []PlanAction
	for _, d := range desired {
		if d.ID == "" {
			plan = append(plan, PlanAction{Action: "create", Record: d})
			continue
		}
		actualRec, ok := byID[d.ID]
		if !ok {
			return nil, fmt.Errorf("记录不存在或不可编辑: %s %s", d.Type, d.Name)
		}
		used[d.ID] = true
		if !recordEqual(actualRec, d) {
			plan = append(plan, PlanAction{Action: "update", Record: d})
		}
	}
	for _, r := range actual {
		if r.ID == "" || used[r.ID] {
			continue
		}
		plan = append(plan, PlanAction{Action: "delete", Record: r})
	}
	return plan, nil
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

// Push 执行变更计划并逐条审计。
func (s *DNSService) Push(accountID uint, zone string, actions []PlanAction, userID uint, username string) ([]PlanResult, error) {
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
		s.writeAudit(userID, username, "dns."+act.Action,
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
