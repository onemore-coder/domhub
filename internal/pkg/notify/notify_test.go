package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// 钉钉加签：官方算法为 HMAC-SHA256(secret, "timestamp\nsecret") 后 base64，
// 拼接参数为 timestamp（毫秒）与 urlencoded(sign)。
func TestDingtalkSignedURL(t *testing.T) {
	d := &dingtalkNotifier{
		webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc123",
		secret:  "SECxxxxxxxx",
	}
	got := d.signedURL()

	if !strings.HasPrefix(got, d.webhook+"&timestamp=") {
		t.Fatalf("签名 URL 前缀不符: %s", got)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("签名 URL 解析失败: %v", err)
	}
	q := u.Query()
	ts, sign := q.Get("timestamp"), q.Get("sign")
	if ts == "" || sign == "" {
		t.Fatalf("缺少 timestamp/sign 参数: %s", got)
	}
	mac := hmac.New(sha256.New, []byte(d.secret))
	mac.Write([]byte(ts + "\n" + d.secret))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	if sign != want {
		t.Fatalf("签名不匹配: got %s want %s", sign, want)
	}
}

// 无 secret 时 URL 不变。
func TestDingtalkUnsignedURL(t *testing.T) {
	d := &dingtalkNotifier{webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc123"}
	if got := d.signedURL(); got != d.webhook {
		t.Fatalf("无密钥时不应改写 URL: %s", got)
	}
}

// 裸 Webhook 地址（旧用法）应可解析。
func TestBuildDingtalkBareURL(t *testing.T) {
	n, err := Build("dingtalk", "https://oapi.dingtalk.com/robot/send?access_token=abc123")
	if err != nil {
		t.Fatalf("裸 URL 应兼容: %v", err)
	}
	d, ok := n.(*dingtalkNotifier)
	if !ok || d.webhook == "" || d.secret != "" {
		t.Fatalf("裸 URL 解析结果异常: %+v", d)
	}
}

// JSON 配置应完整解析 webhook/secret/keyword。
func TestBuildDingtalkJSON(t *testing.T) {
	cfg, _ := json.Marshal(dingtalkConfig{
		Webhook: "https://oapi.dingtalk.com/robot/send?access_token=abc123",
		Secret:  "SECxxx",
		Keyword: "DomHub",
	})
	n, err := Build("dingtalk", string(cfg))
	if err != nil {
		t.Fatalf("JSON 配置解析失败: %v", err)
	}
	d := n.(*dingtalkNotifier)
	if d.secret != "SECxxx" || d.keyword != "DomHub" {
		t.Fatalf("secret/keyword 解析异常: %+v", d)
	}
}

// 机器人接口失败时 HTTP 仍为 200 + errcode!=0，Send 必须报错而非误报成功。
func TestDingtalkErrcodeCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":310000,"errmsg":"sign not match"}`))
	}))
	defer srv.Close()

	d := &dingtalkNotifier{webhook: srv.URL}
	if err := d.Send("测试", "内容"); err == nil {
		t.Fatal("errcode!=0 应返回错误")
	} else if !strings.Contains(err.Error(), "310000") {
		t.Fatalf("错误信息应包含 errcode: %v", err)
	}
}

// errcode=0 应成功。
func TestDingtalkSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	defer srv.Close()

	d := &dingtalkNotifier{webhook: srv.URL, keyword: "DomHub"}
	if err := d.Send("测试", "内容"); err != nil {
		t.Fatalf("errcode=0 不应报错: %v", err)
	}
}

// 企业微信 errcode!=0 应报错。
func TestWecomErrcodeCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"errcode":93000,"errmsg":"invalid webhook"}`))
	}))
	defer srv.Close()

	w := &wecomNotifier{webhook: srv.URL}
	if err := w.Send("测试", "内容"); err == nil || !strings.Contains(err.Error(), "93000") {
		t.Fatalf("企业微信 errcode!=0 应报错: %v", err)
	}
}

// Telegram ok=false 应报错。
func TestTelegramOKCheck(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"error_code":401,"description":"Unauthorized"}`))
	}))
	defer srv.Close()

	tg := &telegramNotifier{cfg: telegramConfig{BotToken: "x", ChatID: "1"}}
	// 替换 API 域名为本地测试服务器
	old := apiBase
	apiBase = srv.URL
	defer func() { apiBase = old }()
	if err := tg.Send("测试", "内容"); err == nil || !strings.Contains(err.Error(), "Unauthorized") {
		t.Fatalf("Telegram ok=false 应报错: %v", err)
	}
}
