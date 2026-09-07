// Package aliyun 阿里云 Provider：RPC 签名直调域名 API（HMAC-SHA1），避免引入大 SDK。
package aliyun

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/domhub-io/domhub/internal/provider"
)

const (
	apiBaseURL = "https://domain.aliyuncs.com"
	apiVersion = "2018-01-29"
)

func init() {
	provider.Register("aliyun", func(cred provider.Credential) (provider.DomainProvider, error) {
		return &Provider{cred: cred, client: &http.Client{Timeout: 30 * time.Second}}, nil
	})
}

// Provider 阿里云域名 Provider。
type Provider struct {
	cred   provider.Credential
	client *http.Client
}

var cst = time.FixedZone("CST", 8*3600)

// percentEncode 阿里云 POP 协议的 URL 编码规则。
func percentEncode(s string) string {
	e := url.QueryEscape(s)
	e = strings.ReplaceAll(e, "+", "%20")
	e = strings.ReplaceAll(e, "*", "%2A")
	e = strings.ReplaceAll(e, "%7E", "~")
	return e
}

// sign 计算 RPC 签名。
func sign(secret string, params url.Values) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(percentEncode(k))
		sb.WriteByte('=')
		sb.WriteString(percentEncode(params.Get(k)))
	}
	stringToSign := "GET&%2F&" + percentEncode(sb.String())

	m := hmac.New(sha1.New, []byte(secret+"&"))
	m.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

func nonce() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// call 调用阿里云 RPC API。
func (p *Provider) call(ctx context.Context, action string, extra map[string]string) (map[string]json.RawMessage, error) {
	params := url.Values{}
	params.Set("Format", "JSON")
	params.Set("Version", apiVersion)
	params.Set("AccessKeyId", p.cred.AccessKey)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", nonce())
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("Action", action)
	for k, v := range extra {
		params.Set(k, v)
	}
	params.Set("Signature", sign(p.cred.SecretKey, params))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiBaseURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求阿里云 API 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var out map[string]json.RawMessage
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body=%s", err, truncate(body))
	}
	// 阿里云出错时响应体中带 Code/Message 字段
	if rawCode, ok := out["Code"]; ok {
		if code := jsonStr(rawCode); code != "" {
			msg := ""
			if rawMsg, ok2 := out["Message"]; ok2 {
				msg = jsonStr(rawMsg)
			}
			return nil, fmt.Errorf("阿里云 API 错误 %s: %s", code, msg)
		}
	}
	return out, nil
}

func jsonStr(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}
