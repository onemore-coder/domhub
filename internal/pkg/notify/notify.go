// Package notify 多渠道通知分发：webhook / 钉钉 / 企业微信 / 邮件 / Telegram。
package notify

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
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
			// 兼容直接填裸 URL 的用法
			cfg.URL = strings.TrimSpace(config)
		}
		if cfg.URL == "" {
			return nil, fmt.Errorf("webhook 缺少 url")
		}
		return &webhookNotifier{url: cfg.URL, headers: cfg.Headers}, nil

	case "dingtalk":
		var cfg dingtalkConfig
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			// 兼容直接填裸 Webhook 地址的旧用法
			cfg = dingtalkConfig{Webhook: strings.TrimSpace(config)}
		}
		if cfg.Webhook == "" {
			return nil, fmt.Errorf("钉钉缺少 webhook")
		}
		return &dingtalkNotifier{webhook: cfg.Webhook, secret: cfg.Secret, keyword: cfg.Keyword}, nil

	case "wecom":
		var cfg struct {
			Webhook string `json:"webhook"` // 企业微信群机器人 Webhook
		}
		if err := json.Unmarshal([]byte(config), &cfg); err != nil {
			// 兼容直接填裸 Webhook 地址的用法
			cfg.Webhook = strings.TrimSpace(config)
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

// postJSON 通用 POST，返回响应体（机器人接口失败时 HTTP 仍为 200，
// 业务结果需由调用方解析响应体判断）。
func postJSON(url string, headers map[string]string, payload any) ([]byte, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	respBody := buf.Bytes()
	if resp.StatusCode >= 300 {
		return respBody, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return respBody, nil
}

// ---- webhook ----

type webhookNotifier struct {
	url     string
	headers map[string]string
}

func (w *webhookNotifier) Send(title, content string) error {
	_, err := postJSON(w.url, w.headers, map[string]string{"title": title, "content": content, "timestamp": time.Now().Format(time.RFC3339)})
	return err
}

// ---- 钉钉 ----

type dingtalkConfig struct {
	Webhook string `json:"webhook"` // 机器人 Webhook 地址（含 access_token）
	Secret  string `json:"secret"`  // 加签密钥（SEC 开头），安全设置选「加签」时必填
	Keyword string `json:"keyword"` // 自定义关键词，安全设置选「自定义关键词」时必填
}

type dingtalkNotifier struct {
	webhook string
	secret  string
	keyword string
}

// signedURL 安全设置为「加签」时，按官方规范对 timestamp+"\n"+secret 计算
// HMAC-SHA256 并 base64，追加 timestamp / sign 参数。
func (d *dingtalkNotifier) signedURL() string {
	if d.secret == "" {
		return d.webhook
	}
	ts := time.Now().UnixMilli()
	mac := hmac.New(sha256.New, []byte(d.secret))
	mac.Write([]byte(strconv.FormatInt(ts, 10) + "\n" + d.secret))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	sep := "?"
	if strings.Contains(d.webhook, "?") {
		sep = "&"
	}
	return fmt.Sprintf("%s%stimestamp=%d&sign=%s", d.webhook, sep, ts, url.QueryEscape(sign))
}

func (d *dingtalkNotifier) Send(title, content string) error {
	text := fmt.Sprintf("### %s\n\n%s", title, strings.ReplaceAll(content, "\n", "\n\n"))
	// 安全设置为「自定义关键词」时，消息必须包含关键词：缺失则自动补在开头
	if d.keyword != "" && !strings.Contains(text, d.keyword) {
		text = "**" + d.keyword + "**\n\n" + text
	}
	body, err := postJSON(d.signedURL(), nil, map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": title,
			"text":  text,
		},
	})
	if err != nil {
		return err
	}
	// 钉钉失败时 HTTP 仍为 200，需解析 errcode
	var r struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &r); err == nil && r.Errcode != 0 {
		return fmt.Errorf("钉钉返回 errcode=%d: %s", r.Errcode, r.Errmsg)
	}
	return nil
}

// ---- 企业微信 ----

type wecomNotifier struct {
	webhook string
}

func (w *wecomNotifier) Send(title, content string) error {
	text := fmt.Sprintf("**%s**\n%s", title, content)
	// 企业微信 markdown 上限 4096 字节，超长按字符截断避免整条被拒
	if len(text) > 4000 {
		runes := []rune(text)
		if len(runes) > 1200 {
			runes = runes[:1200]
		}
		text = string(runes) + "\n\n...(内容过长已截断)"
	}
	body, err := postJSON(w.webhook, nil, map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"content": text,
		},
	})
	if err != nil {
		return err
	}
	// 企业微信失败时 HTTP 仍为 200，需解析 errcode
	var r struct {
		Errcode int    `json:"errcode"`
		Errmsg  string `json:"errmsg"`
	}
	if err := json.Unmarshal(body, &r); err == nil && r.Errcode != 0 {
		return fmt.Errorf("企业微信返回 errcode=%d: %s", r.Errcode, r.Errmsg)
	}
	return nil
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
	// 主题按 RFC 2047 编码，否则中文主题乱码/被 MTA 拒收
	subject := mime.QEncoding.Encode("utf-8", title)
	msg := strings.Join([]string{
		"From: " + from,
		"To: " + strings.Join(e.cfg.To, ","),
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		content,
	}, "\r\n")
	addr := fmt.Sprintf("%s:%d", e.cfg.Host, e.cfg.Port)
	auth := smtp.PlainAuth("", e.cfg.Username, e.cfg.Password, e.cfg.Host)
	// 465 为隐式 TLS 端口（国内邮箱服务商标配），smtp.SendMail 不支持，需 TLS 直连
	if e.cfg.Port == 465 {
		return e.sendImplicitTLS(addr, auth, from, msg)
	}
	if err := smtp.SendMail(addr, auth, from, e.cfg.To, []byte(msg)); err != nil {
		// 部分 SMTP（163/QQ）仅接受 AUTH LOGIN，PLAIN 失败时回退重试
		if strings.Contains(err.Error(), "535") || strings.Contains(err.Error(), "auth") {
			return smtp.SendMail(addr, loginAuth{username: e.cfg.Username, password: e.cfg.Password}, from, e.cfg.To, []byte(msg))
		}
		return err
	}
	return nil
}

// sendImplicitTLS 通过隐式 TLS 连接发送邮件（465 端口）。
func (e *emailNotifier) sendImplicitTLS(addr string, auth smtp.Auth, from, msg string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: e.cfg.Host})
	if err != nil {
		return fmt.Errorf("TLS 连接失败: %w", err)
	}
	defer conn.Close()
	c, err := smtp.NewClient(conn, e.cfg.Host)
	if err != nil {
		return fmt.Errorf("SMTP 客户端创建失败: %w", err)
	}
	defer c.Close()
	if err = c.Auth(auth); err != nil {
		// PLAIN 不被接受时回退 LOGIN
		if err2 := c.Auth(loginAuth{username: e.cfg.Username, password: e.cfg.Password}); err2 != nil {
			return fmt.Errorf("认证失败: %v / %v", err, err2)
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range e.cfg.To {
		if err = c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write([]byte(msg)); err != nil {
		return err
	}
	if err = w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// loginAuth 实现 AUTH LOGIN（国内 SMTP 服务商常用，net/smtp 未内置）。
type loginAuth struct {
	username, password string
}

func (l loginAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {
	return "LOGIN", nil, nil
}

func (l loginAuth) Next(fromServer []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	switch strings.Contains(strings.ToLower(string(fromServer)), "password") {
	case true:
		return []byte(l.password), nil
	default:
		return []byte(l.username), nil
	}
}

// ---- Telegram ----

// apiBase Telegram API 地址，测试时可替换。
var apiBase = "https://api.telegram.org"

type telegramConfig struct {
	BotToken string `json:"bot_token"`
	ChatID   string `json:"chat_id"`
}

type telegramNotifier struct {
	cfg telegramConfig
}

func (t *telegramNotifier) Send(title, content string) error {
	apiURL := fmt.Sprintf("%s/bot%s/sendMessage", apiBase, t.cfg.BotToken)
	body, err := postJSON(apiURL, nil, map[string]string{
		"chat_id": t.cfg.ChatID,
		"text":    title + "\n" + content,
	})
	if err != nil {
		return err
	}
	var r struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(body, &r); err == nil && !r.OK {
		return fmt.Errorf("Telegram 返回失败: %s", r.Description)
	}
	return nil
}
