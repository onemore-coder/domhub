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

// CertService SSL 证书监控：TLS 拨测获取证书到期时间，按规则告警。
type CertService struct {
	certs   *repo.CertRepo
	domains *repo.DomainRepo
	rules   *repo.AlertRepo
}

func NewCertService(certs *repo.CertRepo, domains *repo.DomainRepo, rules *repo.AlertRepo) *CertService {
	return &CertService{certs: certs, domains: domains, rules: rules}
}

// probe 对单个域名做 TLS 拨测，读取证书信息。
// 跳过证书链校验（我们只关心到期时间，自签/过期证书同样要能读到）。
func probeCert(domain string) (notAfter time.Time, issuer, subject string, err error) {
	dialer := &net.Dialer{Timeout: 8 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", domain+":443", &tls.Config{
		ServerName:         domain,
		InsecureSkipVerify: true, //nolint:gosec // 仅读取证书元数据，不做链校验
	})
	if err != nil {
		return time.Time{}, "", "", err
	}
	defer conn.Close()
	leaf := conn.ConnectionState().PeerCertificates[0]
	return leaf.NotAfter, leaf.Issuer.CommonName, leaf.Subject.CommonName, nil
}

// ListStatus 返回全部域名的证书状态（供列表页展示）。
func (s *CertService) ListStatus() ([]model.CertStatus, error) {
	return s.certs.List()
}

// probeAndStore 探测单域名并落库（保留已告警档位）。
func (s *CertService) probeAndStore(d model.Domain) (*model.CertStatus, error) {
	cs := &model.CertStatus{
		DomainID: d.ID,
		Name:     d.Name,
		OK:       true,
	}
	notAfter, issuer, subject, err := probeCert(d.Name)
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
	if prev, e := s.certs.FindByDomainID(d.ID); e == nil {
		if prev.NotAfter != nil && cs.NotAfter != nil && prev.NotAfter.Equal(*cs.NotAfter) {
			cs.AlertedOffsets = prev.AlertedOffsets
		}
	}
	cs.CheckedAt = time.Now()
	err = s.certs.Upsert(cs)
	return cs, err
}

// CheckDomains 执行一轮检查：探测全部域名 → 按启用的 cert_expire 规则告警。
// 返回 (检查数, 告警数, 错误)。仅探测指定 domainID 时传 id > 0。
func (s *CertService) CheckDomains(ctx context.Context, domainID uint) (checked, alerted int, err error) {
	domains, err := s.domains.ListWithExpiry()
	if err != nil {
		return 0, 0, err
	}
	// 只检查注册域（kind=domain）；托管 Zone 没有独立证书
	filtered := make([]model.Domain, 0, len(domains))
	for _, d := range domains {
		if d.Kind == "domain" && (domainID == 0 || d.ID == domainID) {
			filtered = append(filtered, d)
		}
	}

	statuses := make([]*model.CertStatus, 0, len(filtered))
	for _, d := range filtered {
		select {
		case <-ctx.Done():
			return checked, alerted, ctx.Err()
		default:
		}
		cs, e := s.probeAndStore(d)
		if e != nil {
			logger.L().Warn("证书检查落库失败", zap.String("domain", d.Name), zap.Error(e))
			continue
		}
		checked++
		statuses = append(statuses, cs)
		time.Sleep(200 * time.Millisecond) // 温和限速
	}

	alerted, err = s.dispatchAlerts(statuses)
	return checked, alerted, err
}

// dispatchAlerts 按启用的 cert_expire 规则发送告警。
// 同一证书（NotAfter）同一档位只发一次。
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
					"域名 %s 的 HTTPS 证书将于 %s 到期，剩余约 %d 天。\n签发者：%s\n请及时续期证书。",
					cs.Name, cs.NotAfter.Format("2006-01-02"), cs.DaysLeft, cs.Issuer,
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
					logger.L().Error("更新证书告警档位失败", zap.String("domain", cs.Name), zap.Error(e))
				}
				if e := s.rules.CreateLog(&model.AlertLog{
					DomainID:   cs.DomainID,
					DomainName: cs.Name,
					Offset:     offset,
					ExpYear:    cs.NotAfter.Year(),
					Channels:   strings.Join(notified, ","),
					Message:    fmt.Sprintf("证书告警 | 已通知: %s", strings.Join(notified, ", ")),
					SentAt:     time.Now(),
				}); e != nil {
					logger.L().Warn("写入证书告警日志失败", zap.String("domain", cs.Name), zap.Error(e))
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
