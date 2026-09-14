// deploy.go 腾讯云 CDN 证书上传与绑定（证书部署三期）。
// 流程：SSL 云证书上传（UploadServerCertificate）拿 CertId →
// CDN 域名配置更新（UpdateDomainConfig）绑定该证书并开启 HTTPS。
package tencent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/domhub-io/domhub/internal/provider"
)

var (
	targetSSL = apiTarget{host: "ssl.tencentcloudapi.com", service: "ssl", version: "2019-12-05"}
	targetCDN = apiTarget{host: "cdn.tencentcloudapi.com", service: "cdn", version: "2018-06-06"}
)

// NewCertDeployClient 构建用于证书部署的客户端（复用同一主账号 AK）。
func NewCertDeployClient(cred provider.Credential) *Provider {
	return &Provider{cred: cred, client: &http.Client{Timeout: 60 * time.Second}}
}

// UploadServerCert 上传证书到 SSL 云证书，返回 CertId。
func (p *Provider) UploadServerCert(ctx context.Context, certName, chainPEM, keyPEM string) (string, error) {
	out, err := p.call(ctx, targetSSL, "UploadServerCertificate", map[string]any{
		"CertificatePublicKey":  chainPEM,
		"CertificatePrivateKey": keyPEM,
		"Alias":                 certName,
	})
	if err != nil {
		return "", fmt.Errorf("上传云证书失败: %w", err)
	}
	certID := jsonStrField(out, "CertId")
	if certID == "" {
		return "", fmt.Errorf("上传云证书成功但响应缺少 CertId")
	}
	return certID, nil
}

// jsonStrField 从响应字段中提取字符串值。
func jsonStrField(out map[string]json.RawMessage, key string) string {
	raw, ok := out[key]
	if !ok {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return strings.Trim(string(raw), `"`)
}

// BindCDNCert 将云证书绑定到 CDN 域名并开启 HTTPS（部分更新，不影响其他配置）。
func (p *Provider) BindCDNCert(ctx context.Context, domain, certID string) error {
	_, err := p.call(ctx, targetCDN, "UpdateDomainConfig", map[string]any{
		"Domain": domain,
		"Https": map[string]any{
			"Switch":   "on",
			"CertInfo": map[string]any{"CertId": certID},
		},
	})
	return err
}

// DeployCDNCert 完整部署：上传 → 绑定。
func (p *Provider) DeployCDNCert(ctx context.Context, domain, certName, chainPEM, keyPEM string) error {
	certID, err := p.UploadServerCert(ctx, certName, chainPEM, keyPEM)
	if err != nil {
		return err
	}
	return p.BindCDNCert(ctx, domain, certID)
}
