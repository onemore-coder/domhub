// Package notify 多渠道通知分发：webhook / 钉钉 / 企业微信 / 邮件 / Telegram。
package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

// Notifier 通知渠道接口。
type Notifier interface {
	Send(title, content string) error
}

var httpClient = &http.Client{Timeout: 15 * time.Second}

// Build 按渠道类型和 JSON 配置构建 Notifier。
func Build(chType, config string) (Notifier, error) {
	switch chType {
	case "webhook":
		var cfg struct {
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, fmt.Errorf("webhook 配置解析失败: %w", err)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("webhook 缺少 url")
		}
		return &webhookNotifier{url: cfg.URL, headers: cfg.Headers}, nil

	case "dingtalk":
		var cfg struct {
			Webhook string `json:"webhook"` // 钉钉机器人 Webhook 地址（含 access_token）
			Secret  string `json:"secret"`  // 可选：加签密钥
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, fmt.Errorf("钉钉配置解析失败: %w", err)
		}
		if cfg.Webhook == "" {
			return nil, fmt.Errorf("钉钉缺少 webhook")
		}
		return &dingtalkNotifier{webhook: cfg.Webhook, secret: cfg.Secret}, nil

	case "wecom":
		var cfg struct {
			Webhook string `json:"webhook"` // 企业微信群机器人 Webhook
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, fmt.Errorf("企业微信配置解析失败: %w", err)
		}
		if cfg.Webhook == "" {
			return nil, fmt.Errorf("企业微信缺少 webhook")
		}
		return &wecomNotifier{webhook: cfg.Webhook}, nil

	case "email":
		var cfg struct {
			Host     string   `json:"host"` // SMTP 主机
			Port     int      `json:"port"` // 默认 465/587
			Username string   `json:"username"`
			Password string   `json:"password"`
			From     string   `json:"from"`
			To       []string `json:"to"`
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, fmt.Errorf("邮件配置解析失败: %w", err)
		}
		if cfg.Host == "" || len(cfg.To) == 0 {
			return nil, fmt.Errorf("邮件缺少 host 或收件人")
		}
		if cfg.Port == 0 {
			cfg.Port = 587
		}
		return &emailNotifier{cfg: cfg}, nil

	case "telegram":
		var cfg struct {
			BotToken string `json:"bot_token"`
			ChatID   string `json:"chat_id"`
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			return nil, fmt.Errorf("Telegram 配置解析失败: %w", err)
		}
		if cfg.BotToken == "" || cfg.ChatID == "" {
			return nil, fmt.Errorf("Telegram 缺少 bot_token 或 chat_id")
		}
		return &telegramNotifier{cfg: cfg}, nil

	default:
		return nil, fmt.Errorf("不支持的通知渠道类型: %s", chType)
	}
}

// postJSON 通用 POST。
func postJSON(url string, headers map[string]string, payload any) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

// ---- webhook ----

type webhookNotifier struct {
	url     string
	headers map[string]string
}

func (w *webhookNotifier) Send(title, content string) error {
	return postJSON(w.url, w.headers, map[string]string{"title": title, "content": content, "timestamp": time.Now().Format(time.RFC3339)})
}

// ---- 钉钉 ----

type dingtalkNotifier struct {
	webhook string
	secret  string
}

func (d *dingtalkNotifier) Send(title, content string) error {
	return postJSON(d.webhook, nil, map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  fmt.Sprintf("### %s\n\n%s", title, strings.ReplaceAll(content, "\n", "\n\n")),
		},
	})
}

// ---- 企业微信 ----

type wecomNotifier struct {
	webhook string
}

func (w *wecomNotifier) Send(title, content string) error {
	return postJSON(w.webhook, nil, map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": fmt.Sprintf("**%s**\n%s", title, content),
		},
	})
}

// ---- 邮件 ----

type emailConfig struct {
	Host     string   `json:"host"` // SMTP 主机
	Port     int      `json:"port"` // 默认 587
	Username string   `json:"username"`
	Password string   `json:"password"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

type emailNotifier struct {
	cfg emailConfig
}

func (e *emailNotifier) Send(title, content string) error {
	from := e.cfg.From
	if from == "" {
		from = e.cfg.Username
	}
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + strings.Join(e.cfg.To, ","),
		"Subject: " + title,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		content,
	}, "\r\n")
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	auth := smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	return smtp.SendMail(addr, auth, from, e.cfg.To, []byte(msg))
}

// ---- Telegram ----

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type telegramNotifier struct {
	cfg telegramConfig
}

func (t *telegramNotifier) Send(title, content string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.cfg.BotToken)
	return postJSON(url, nil, map[string]string{
		"chat_id": t.cfg.ChatID,
		"text":    title + "\n" + content,
	})
}
