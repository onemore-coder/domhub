package notify

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
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
