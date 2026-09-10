package service

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/pkg/notify"
	"github.com/domhub-io/domhub/internal/repo"
)

// CertService SSL 证书监控：发现主机名（注册域 apex + 解析记录镜像中的子域名），
// TLS 拨测读取证书到期时间，按 cert_expire 规则告警。
//
// 子域名不单独建存储：主机名直接来自 dns_records 本地镜像（A/AAAA/CNAME），
// 镜像由 sync_records 任务与变更回调保持新鲜；发现结果落在
// cert_statuses（host 唯一），支持手动添加与排除。
type CertService struct {
	certs   *repo.CertRepo
	domains *repo.DomainRepo
	rules   *repo.AlertRepo
	records *repo.DnsRecordRepo
}

func NewCertService(certs *repo.CertRepo, domains *repo.DomainRepo, rules *repo.AlertRepo,
	records *repo.DnsRecordRepo) *CertService {
	return &CertService{certs: certs, domains: domains, rules: rules, records: records}
}

// certTarget 一个待探测的主机名。
type certTarget struct {
	host       string
	domainID   uint
	domainName string
}

// probeCert 对主机名做 TLS 拨测，读取证书信息。
// 跳过证书链校验（只关心到期时间，自签/过期证书同样要能读到）。
func probeCert(host string) (notAfter time.Time, issuer, subject string, err error) {
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", host+":443", &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: true, //nolint:gosec // 仅读取证书元数据，不做链校验
	})
	if err != nil {
		return time.Time{}, "", "", err
	}
	defer conn.Close()
	leaf := conn.ConnectionState().PeerCertificates[0]
	return leaf.NotAfter, leaf.Issuer.CommonName, leaf.Subject.CommonName, nil
}

// discoverHosts 汇总监控目标：
//   - 注册域（kind=domain）→ apex 主机名
//   - 解析记录镜像（dns_records）→ A/AAAA/CNAME 主机名（跳过 * 与 _ 前缀）
func (s *CertService) discoverHosts(ctx context.Context) ([]certTarget, error) {
	seen := make(map[string]struct{})
	targets := make([]certTarget, 0, 16)

	add := func(t certTarget) {
		if t.host == "" {
			return
		}
		if _, dup := seen[t.host]; dup {
			return
		}
		seen[t.host] = struct{}{}
		targets = append(targets, t)
	}

	// 1) 主域 apex
	domains, err := s.domains.ListAllByKind("domain")
	if err != nil {
		return nil, err
	}
	for _, d := range domains {
		add(certTarget{host: d.Name, domainID: d.ID, domainName: d.Name})
	}

	// 2) 解析记录镜像中的子域名（本地查询，零 API 成本）
	rows, err := s.records.CertHostRows()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		select {
		case <-ctx.Done():
			return targets, ctx.Err()
		default:
		}
		host := CertHost(row.Name, row.ZoneName)
		if host == "" {
			continue
		}
		add(certTarget{host: host, domainName: row.ZoneName})
	}
	return targets, nil
}

// CertHost 把记录主机名转换为完整域名。
// 过滤规则：@ → zone 本身；* 泛解析与 _ 开头（_acme-challenge 等）跳过。
func CertHost(name, zone string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "*" || strings.HasPrefix(name, "_") {
		return ""
	}
	if name == "@" {
		return zone
	}
	// 兼容个别厂商直接返回 FQDN 的情况
	if name == zone || strings.HasSuffix(name, "."+zone) {
		return name
	}
	return name + "." + zone
}

// MapByZone 返回某 Zone 关联主机的证书状态（host → status），供记录列表关联展示。
func (s *CertService) MapByZone(zone string) map[string]model.CertStatus {
	all, err := s.certs.List()
	if err != nil {
		return nil
	}
	out := make(map[string]model.CertStatus)
	for _, cs := range all {
		if cs.Host == zone || strings.HasSuffix(cs.Host, "."+zone) {
			out[cs.Host] = cs
		}
	}
	return out
}

// ListStatus 返回全部主机名的证书状态（供列表页展示）。
func (s *CertService) ListStatus() ([]model.CertStatus, error) {
	return s.certs.List()
}

// AddManual 手动添加监控主机名。
func (s *CertService) AddManual(host, domainName string) (*model.CertStatus, error) {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" || !strings.Contains(host, ".") {
		return nil, fmt.Errorf("主机名格式不正确")
	}
	if prev, err := s.certs.FindByHost(host); err == nil && prev != nil {
		return nil, fmt.Errorf("该主机已在监控列表中")
	}
	cs := &model.CertStatus{
		Host: host, DomainName: domainName, Source: "manual",
		DaysLeft: -1, CheckedAt: time.Now(),
	}
	if err := s.certs.Upsert(cs); err != nil {
		return nil, err
	}
	return cs, nil
}

// Delete 删除监控条目（手动条目与被排除的自动条目）。
func (s *CertService) Delete(id uint) error { return s.certs.Delete(id) }

// SetExcluded 设置/取消排除。
func (s *CertService) SetExcluded(id uint, excluded bool) error {
	return s.certs.SetExcluded(id, excluded)
}

// CheckDomains 执行一轮检查：发现主机名 → 逐个探测 → 清理消失的自动条目 → 告警。
// domainID > 0 时只检查该主域（apex + 同名 Zone 的子域名）。
func (s *CertService) CheckDomains(ctx context.Context, domainID uint) (checked, alerted int, err error) {
	targets, err := s.discoverHosts(ctx)
	if err != nil {
		return 0, 0, err
	}
	if domainID > 0 {
		if d, e := s.domains.FindByID(domainID); e == nil {
			filtered := targets[:0]
			for _, t := range targets {
				if t.domainName == d.Name {
					filtered = append(filtered, t)
				}
			}
			targets = filtered
		}
	}

	statuses := make([]*model.CertStatus, 0, len(targets))
	seenHosts := make([]string, 0, len(targets))
	for _, t := range targets {
		select {
		case <-ctx.Done():
			return checked, alerted, ctx.Err()
		default:
		}

		// 已排除的条目不探测（保留人工决策），但计为已见、不清理
		if prev, e := s.certs.FindByHost(t.host); e == nil && prev != nil && prev.Excluded {
			seenHosts = append(seenHosts, t.host)
			continue
		}

		cs, e := s.probeAndStore(t)
		if e != nil {
			logger.L().Warn("证书检查落库失败", zap.String("host", t.host), zap.Error(e))
			continue
		}
		seenHosts = append(seenHosts, t.host)
		statuses = append(statuses, cs)
		checked++
		time.Sleep(200 * time.Millisecond) // 温和限速
	}

	// DNS 上已消失且未被排除的自动条目一并清理（排除项保留，避免反复重生）
	if n, e := s.certs.DeleteUnseenAuto(seenHosts); e != nil {
		logger.L().Warn("清理失效证书监控条目失败", zap.Error(e))
	} else if n > 0 {
		logger.L().Info("清理失效证书监控条目", zap.Int64("count", n))
	}

	alerted, err = s.dispatchAlerts(statuses)
	return checked, alerted, err
}

// probeAndStore 探测单个主机名并落库（保留已告警档位；证书更换后重置）。
func (s *CertService) probeAndStore(t certTarget) (*model.CertStatus, error) {
	cs := &model.CertStatus{
		Host: t.host, DomainID: t.domainID, DomainName: t.domainName,
		Source: "auto", OK: true,
	}
	notAfter, issuer, subject, err := probeCert(t.host)
	if err != nil {
		cs.OK = false
		cs.DaysLeft = -1
		cs.Error = err.Error()
	} else {
		cs.NotAfter = &notAfter
		cs.Issuer = issuer
		cs.Subject = subject
		cs.DaysLeft = int(time.Until(notAfter).Hours() / 24)
	}

	// 保留已告警档位；证书更换（NotAfter 变化）后重置
	if prev, e := s.certs.FindByHost(t.host); e == nil && prev != nil {
		if prev.NotAfter != nil && cs.NotAfter != nil && prev.NotAfter.Equal(*cs.NotAfter) {
			cs.AlertedOffsets = prev.AlertedOffsets
		}
	}
	cs.CheckedAt = time.Now()
	err = s.certs.Upsert(cs)
	return cs, err
}

// dispatchAlerts 按启用的 cert_expire 规则发送告警（同一证书同档位只发一次）。
func (s *CertService) dispatchAlerts(statuses []*model.CertStatus) (int, error) {
	rules, err := s.rules.ListEnabledRules()
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, rule := range rules {
		if rule.Kind != "cert_expire" {
			continue
		}
		offsets := parseOffsets(rule.Offsets)
		channelIDs := parseUintIDs(rule.ChannelIDs)
		if len(offsets) == 0 || len(channelIDs) == 0 {
			continue
		}
		channels, err := s.rules.FindChannelsByIDs(channelIDs)
		if err != nil || len(channels) == 0 {
			logger.L().Warn("证书告警规则没有可用渠道", zap.String("rule", rule.Name))
			continue
		}

		for _, cs := range statuses {
			if !cs.OK || cs.NotAfter == nil {
				continue
			}
			alerted := parseAlerted(cs.AlertedOffsets)
			for _, offset := range offsets {
				if cs.DaysLeft > offset {
					continue
				}
				if _, ok := alerted[offset]; ok {
					continue
				}

				title := fmt.Sprintf("【DomHub】证书到期提醒（提前 %d 天）", offset)
				content := fmt.Sprintf(
					"主机 %s 的 HTTPS 证书将于 %s 到期，剩余约 %d 天。\n签发者：%s\n请及时续期证书。",
					cs.Host, cs.NotAfter.Format("2006-01-02"), cs.DaysLeft, cs.Issuer,
				)

				notified := make([]string, 0, len(channels))
				var errs []string
				for _, ch := range channels {
					n, err := notify.Build(ch.Type, ch.Config)
					if err != nil {
						errs = append(errs, fmt.Sprintf("%s(配置错误: %v)", ch.Name, err))
						continue
					}
					if err := n.Send(title, content); err != nil {
						errs = append(errs, fmt.Sprintf("%s(发送失败: %v)", ch.Name, err))
						continue
					}
					notified = append(notified, ch.Name)
				}

				// 记录档位，本证书内不再重发
				alerted[offset] = struct{}{}
				out, _ := json.Marshal(alertedKeys(alerted))
				cs.AlertedOffsets = string(out)
				if e := s.certs.Upsert(cs); e != nil {
					logger.L().Error("更新证书告警档位失败", zap.String("host", cs.Host), zap.Error(e))
				}
				if e := s.rules.CreateLog(&model.AlertLog{
					DomainID:   cs.DomainID,
					DomainName: cs.Host,
					Offset:     offset,
					ExpYear:    cs.NotAfter.Year(),
					Channels:   strings.Join(notified, ","),
					Message:    fmt.Sprintf("证书告警 | 已通知: %s", strings.Join(notified, ", ")),
					SentAt:     time.Now(),
				}); e != nil {
					logger.L().Warn("写入证书告警日志失败", zap.String("host", cs.Host), zap.Error(e))
				}
				sent++
			}
		}
	}
	return sent, nil
}

func parseAlerted(s string) map[int]struct{} {
	var arr []int
	_ = json.Unmarshal([]byte(s), &arr)
	m := make(map[int]struct{}, len(arr))
	for _, v := range arr {
		m[v] = struct{}{}
	}
	return m
}

func alertedKeys(m map[int]struct{}) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
