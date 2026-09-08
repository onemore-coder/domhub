// Package tencent 腾讯云 Provider：直调 API（TC3-HMAC-SHA256 签名），避免引入超大 SDK。
package tencent

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/domhub-io/domhub/internal/provider"
)

var cst *time.Location

func init() {
	cst, _ = time.LoadLocation("Asia/Shanghai")
	if cst == nil {
		cst = time.FixedZone("CST", 8*3600)
	}
	provider.Register("tencent", func(cred provider.Credential) (provider.DomainProvider, error) {
		return &Provider{cred: cred, client: &http.Client{Timeout: 30 * time.Second}}, nil
	})
	provider.RegisterDNS("tencent", func(cred provider.Credential) (provider.DNSProvider, error) {
		return &dnsProvider{Provider: &Provider{cred: cred, client: &http.Client{Timeout: 30 * time.Second}}}, nil
	})
}

// Provider 腾讯云 Provider（域名注册 + DNSPod 解析）。
type Provider struct {
	cred   provider.Credential
	client *http.Client
}

// tc3Sign 计算 TC3-HMAC-SHA256 签名（腾讯云官方算法）。
func tc3Sign(secret, date, svc, stringToSign string) string {
	h := func(key, data []byte) []byte {
		m := hmac.New(sha256.New, key)
		m.Write(data)
		return m.Sum(nil)
	}
	kDate := h([]byte("TC3"+secret), []byte(date))
	kService := h(kDate, []byte(svc))
	kSigning := h(kService, []byte("tc3_request"))
	return hex.EncodeToString(h(kSigning, []byte(stringToSign)))
}

// apiTarget 一个腾讯云 API 产品的接入信息。
type apiTarget struct {
	host    string
	service string
	version string
}

var (
	targetDomain = apiTarget{host: "domain.tencentcloudapi.com", service: "domain", version: "2018-08-08"}
	targetDNSPod = apiTarget{host: "dnspod.tencentcloudapi.com", service: "dnspod", version: "2021-03-23"}
)

// call 调用腾讯云 API，返回 Response 内除 Error/RequestId 外的字段。
func (p *Provider) call(ctx context.Context, t apiTarget, action string, params map[string]any) (map[string]json.RawMessage, error) {
	payload, _ := json.Marshal(params)
	ts := time.Now().UTC()
	timestamp := fmt.Sprintf("%d", ts.Unix())
	date := ts.Format("2006-01-02")

	hashedPayload := sha256.Sum256(payload)
	canonicalRequest := fmt.Sprintf("POST\n/\n\ncontent-type:application/json; charset=utf-8\nhost:%s\n\ncontent-type;host\n%s",
		t.host, hex.EncodeToString(hashedPayload[:]))

	stringToSign := fmt.Sprintf("TC3-HMAC-SHA256\n%s\n%s/%s/tc3_request\n%s",
		timestamp, date, t.service, sha256Hex([]byte(canonicalRequest)))
	signature := tc3Sign(p.cred.SecretKey, date, t.service, stringToSign)

	auth := fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s/%s/tc3_request, SignedHeaders=content-type;host, Signature=%s",
		p.cred.AccessKey, date, t.service, signature)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+t.host, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Host", t.host)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", t.version)
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("Authorization", auth)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求腾讯云 API 失败: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}

	var out struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error,omitempty"`
			RequestID string `json:"RequestId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w, body=%s", err, truncate(body))
	}
	if out.Response.Error != nil {
		return nil, fmt.Errorf("腾讯云 API 错误 %s: %s", out.Response.Error.Code, out.Response.Error.Message)
	}

	fields := map[string]json.RawMessage{}
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, err
	}
	respFields, ok := fields["Response"]
	if !ok {
		return nil, fmt.Errorf("响应缺少 Response 字段")
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(respFields, &inner); err != nil {
		return nil, err
	}
	delete(inner, "Error")
	delete(inner, "RequestId")
	return inner, nil
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func truncate(b []byte) string {
	if len(b) > 200 {
		return string(b[:200]) + "..."
	}
	return string(b)
}

// parseCST 解析 "2006-01-02 15:04:05" 格式的北京时间。
func parseCST(s string) time.Time {
	t, err := time.ParseInLocation("2006-01-02 15:04:05", s, cst)
	if err != nil {
		return time.Time{}
	}
	return t
}
