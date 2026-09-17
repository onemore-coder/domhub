package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/logger"
	"github.com/onemore-coder/domhub/internal/pkg/notify"
	"github.com/onemore-coder/domhub/internal/repo"
)

// AlertService 到期告警业务。
type AlertService struct {
	rules   *repo.AlertRepo
	domains *repo.DomainRepo
}

func NewAlertService(rules *repo.AlertRepo, domains *repo.DomainRepo) *AlertService {
	return &AlertService{rules: rules, domains: domains}
}

// RunExpiryCheck 执行一轮域名到期检查：
// 遍历启用的规则 → 匹配达到提前档位且未发送过的域名 → 通过渠道发送 → 记录日志。
// 返回本次发送的告警条数。
func (s *AlertService) RunExpiryCheck(_ context.Context) (int, error) {
	rules, err := s.rules.ListEnabledRules()
	if err != nil {
		return 0, err
	}
	domains, err := s.domains.ListWithExpiry()
	if err != nil {
		return 0, err
	}

	sent := 0
	for _, rule := range rules {
		offsets := parseOffsets(rule.Offsets)
		channelIDs := parseUintIDs(rule.ChannelIDs)
		if len(offsets) == 0 || len(channelIDs) == 0 {
			continue
		}

		channels, err := s.rules.FindChannelsByIDs(channelIDs)
		if err != nil || len(channels) == 0 {
			logger.L().Warn("告警规则没有可用渠道", zap.String("rule", rule.Name))
			continue
		}

		for _, d := range domains {
			daysLeft := int(time.Until(*d.ExpireAt).Hours() / 24)
			for _, offset := range offsets {
				if daysLeft > offset {
					continue
				}
				expYear := d.ExpireAt.Year()
				exists, err := s.rules.HasLog(d.ID, offset, expYear)
				if err != nil {
					logger.L().Error("查询告警日志失败", zap.Error(err))
					continue
				}
				if exists {
					continue
				}

				title := fmt.Sprintf("【DomHub】域名到期提醒（提前 %d 天）", offset)
				content := fmt.Sprintf(
					"域名 %s 将于 %s 到期，剩余约 %d 天。\n厂商：%s\n请及时处理续费或下线。",
					d.Name, d.ExpireAt.Format("2006-01-02"), daysLeft, d.Provider,
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

				msg := fmt.Sprintf("已通知: %s", strings.Join(notified, ", "))
				if len(errs) > 0 {
					msg += "；失败: " + strings.Join(errs, "; ")
				}
				if err := s.rules.CreateLog(&model.AlertLog{
					DomainID:   d.ID,
					DomainName: d.Name,
					Offset:     offset,
					ExpYear:    expYear,
					Channels:   strings.Join(notified, ","),
					Message:    msg,
					SentAt:     time.Now(),
				}); err != nil {
					logger.L().Error("写入告警日志失败", zap.Error(err))
				}
				sent++
			}
		}
	}
	return sent, nil
}

func parseOffsets(s string) []int {
	parts := strings.Split(strings.ReplaceAll(s, " ", ""), ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

// parseUintIDs 解析渠道 ID（JSON 数组，兼容逗号分隔写法）。
func parseUintIDs(s string) []uint {
	var ids []uint
	if err := json.Unmarshal([]byte(s), &ids); err != nil || len(ids) == 0 {
		ids = nil
		for _, p := range strings.Split(s, ",") {
			if n, err := strconv.ParseUint(strings.TrimSpace(p), 10, 64); err == nil {
				ids = append(ids, uint(n))
			}
		}
	}
	return ids
}
