// Package cronutil cron 表达式规范化与校验（service 与 job 共用，避免循环依赖）。
package cronutil

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

var parser = cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// Normalize 5 位表达式补秒位（分 时 日 月 周 → 秒 分 时 日 月 周），兼容用户输入习惯。
func Normalize(spec string) string {
	spaces := 0
	for _, ch := range spec {
		if ch == ' ' || ch == '\t' {
			spaces++
		}
	}
	if spaces == 4 {
		return "0 " + spec
	}
	return spec
}

// Validate 校验表达式；空串或 "-" 表示禁用，视为合法。
func Validate(spec string) error {
	if spec == "" || spec == "-" {
		return nil
	}
	if _, err := parser.Parse(Normalize(spec)); err != nil {
		return fmt.Errorf("表达式无效: %w", err)
	}
	return nil
}
