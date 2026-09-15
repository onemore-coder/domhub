// deploy.go 阿里云 CDN 证书上传与绑定（证书部署三期）。
// POP RPC 支持 POST 表单提交，签名串前缀为 POST&%2F&；
// 证书链 + 私钥体积较大，GET 查询串有 URL 长度风险，故用 POST。
package aliyun

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
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
	cdnBaseURL = "https://cdn.aliyuncs.com"
	cdnVersion = "2018-05-10"
)

// NewCertDeployClient 构建用于证书部署的客户端（复用域名/CDN 同一主账号 AK）。
func NewCertDeployClient(cred provider.Credential) *Provider {
	return &Provider{cred: cred, client: &http.Client{Timeout: 60 * time.Second}}
}

// signPOST 计算 POST 表单 RPC 的签名（与 GET 版差异仅在签名串前缀）。
func signPOST(secret string, params url.Values) string {
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
	stringToSign := "POST&%2F&" + percentEncode(sb.String())

	return popSign(secret, stringToSign)
}

// popSign 按 POP 规则对已拼好的签名串计算 HMAC-SHA1。
func popSign(secret, stringToSign string) string {
	m := hmac.New(sha1.New, []byte(secret+"&"))
	m.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

// callCDN 以 POST 表单调用阿里云 CDN RPC API。
func (p *Provider) callCDN(ctx context.Context, action string, extra map[string]string) (map[string]json.RawMessage, error) {
	params := url.Values{}
	params.Set("Format", "JSON")
	params.Set("Version", cdnVersion)
	params.Set("AccessKeyId", p.cred.AccessKey)
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", nonce())
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("Action", action)
	for k, v := range extra {
		params.Set(k, v)
	}
	params.Set("Signature", signPOST(p.cred.SecretKey, params))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cdnBaseURL,
		strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求阿里云 CDN API 失败: %w", err)
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
	if rawCode, ok := out["Code"]; ok {
		if code := jsonStr(rawCode); code != "" {
			msg := ""
			if rawMsg, ok2 := out["Message"]; ok2 {
				msg = jsonStr(rawMsg)
			}
			return nil, fmt.Errorf("阿里云 CDN API 错误 %s: %s", code, msg)
		}
	}
	return out, nil
}

// SetCDNCert 上传证书并绑定到 CDN 域名（CertType=upload）。
// SSLProtocol 为必填参数（MissingSSLProtocol），固定传 on 开启 HTTPS。
func (p *Provider) SetCDNCert(ctx context.Context, domain, certName, chainPEM, keyPEM string) error {
	pri, err := ensurePKCS8(keyPEM)
	if err != nil {
		return fmt.Errorf("私钥格式转换失败: %w", err)
	}
	_, err = p.callCDN(ctx, "SetCdnDomainSSLCertificate", map[string]string{
		"DomainName":  domain,
		"CertName":    certName,
		"CertType":    "upload",
		"SSLProtocol": "on",
		"SSLPub":      chainPEM,
		"SSLPri":      pri,
	})
	return err
}

// ensurePKCS8 将 PKCS#1（BEGIN RSA PRIVATE KEY）私钥转换为阿里云要求的
// PKCS#8（BEGIN PRIVATE KEY）；已是 PKCS#8 / EC 格式则原样返回。
func ensurePKCS8(pemStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", fmt.Errorf("私钥不是有效的 PEM")
	}
	switch block.Type {
	case "PRIVATE KEY": // 已是 PKCS#8
		return pemStr, nil
	case "RSA PRIVATE KEY": // PKCS#1 → PKCS#8
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return "", err
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return "", err
		}
		der, err := x509.MarshalPKCS8PrivateKey(key)
		if err != nil {
			return "", err
		}
		return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
	default:
		return "", fmt.Errorf("不支持的私钥类型: %s", block.Type)
	}
}
