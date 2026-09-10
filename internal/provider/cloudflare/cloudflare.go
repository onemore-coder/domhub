// Package cloudflare Cloudflare Provider：API Token / Global API Key 认证，
// 拉取托管 Zone（DNS）与注册域名（Registrar，可选）。
//
// 凭证映射：
//   - AccessKey = API Token（推荐，权限最小化）或 Account Email
//   - SecretKey = 空（Token 认证）或 Global API Key（与 Email 搭配的旧版认证）
//   - Region    = Cloudflare Account ID（可选；填写后额外同步 Registrar 注册域名）
package cloudflare

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/domhub-io/domhub/internal/provider"
)

// apiBase Cloudflare v4 API 地址（测试中可替换为 httptest 服务）。
var apiBase = "https://api.cloudflare.com/client/v4"

func init() {
	provider.Register("cloudflare", func(cred provider.Credential) (provider.DomainProvider, error) {
		return &Provider{cred: cred}, nil
	})
	provider.RegisterDNS("cloudflare", func(cred provider.Credential) (provider.DNSProvider, error) {
		return &Provider{cred: cred}, nil
	})
}

// Provider Cloudflare 域名/DNS Provider。
type Provider struct {
	cred provider.Credential
}

// ---- v4 API 响应封装 ----

type cfEnvelope struct {
	Success bool `json:"success"`
	Errors  []struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"errors"`
	Result     json.RawMessage `json:"result"`
	ResultInfo cfResultInfo    `json:"result_info"`
}

type cfResultInfo struct {
	Page       int `json:"page"`
	PerPage    int `json:"per_page"`
	TotalPages int `json:"total_pages"`
}

func (e *cfEnvelope) err() error {
	if e.Success {
		return nil
	}
	msgs := make([]string, 0, len(e.Errors))
	for _, er := range e.Errors {
		msgs = append(msgs, fmt.Sprintf("%d: %s", er.Code, er.Message))
	}
	if len(msgs) == 0 {
		msgs = append(msgs, "未知错误")
	}
	return fmt.Errorf("Cloudflare API 错误: %s", strings.Join(msgs, "; "))
}

// do 执行一次 v4 API 调用并解析响应（method 为空表示 GET）。
// 返回 result 原文与 result_info 分页信息。
func (p *Provider) do(ctx context.Context, method, path string, body any) (json.RawMessage, cfResultInfo, error) {
	if method == "" {
		method = http.MethodGet
	}
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, cfResultInfo{}, err
		}
		reader = strings.NewReader(string(b))
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
	if err != nil {
		return nil, cfResultInfo{}, err
	}
	p.applyAuth(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, cfResultInfo{}, fmt.Errorf("Cloudflare 请求失败: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, cfResultInfo{}, err
	}
	var env cfEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, cfResultInfo{}, fmt.Errorf("Cloudflare 响应解析失败 (HTTP %d): %w", resp.StatusCode, err)
	}
	if err := env.err(); err != nil {
		return nil, cfResultInfo{}, err
	}
	return env.Result, env.ResultInfo, nil
}

// doList 分页拉取：逐页调用 path（附加 page 参数），直到 last page。
// onPage 在每页 result（数组原文）上被调用。
func (p *Provider) doList(ctx context.Context, path string, perPage int, onPage func(page json.RawMessage) error) error {
	for page := 1; ; page++ {
		u, _ := url.Parse(path)
		q := u.Query()
		q.Set("page", fmt.Sprint(page))
		q.Set("per_page", fmt.Sprint(perPage))
		u.RawQuery = q.Encode()

		result, info, err := p.do(ctx, "", u.String(), nil)
		if err != nil {
			return err
		}
		if err := onPage(result); err != nil {
			return err
		}
		if info.TotalPages <= page || info.TotalPages == 0 {
			return nil
		}
	}
}

// applyAuth 设置认证头：优先 API Token（SK 为空），否则 Email + Global API Key。
func (p *Provider) applyAuth(req *http.Request) {
	if p.cred.SecretKey == "" {
		req.Header.Set("Authorization", "Bearer "+p.cred.AccessKey)
		return
	}
	req.Header.Set("X-Auth-Email", p.cred.AccessKey)
	req.Header.Set("X-Auth-Key", p.cred.SecretKey)
}

// ---- Cloudflare 资源结构 ----

type cfZone struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"` // active / pending / moved
	Paused    bool   `json:"paused"`
	CreatedOn string `json:"created_on"`
}

type cfRegistrarDomain struct {
	Name         string `json:"name"`
	Status       string `json:"status"`
	RegisteredAt string `json:"registered_at"`
	ExpiresAt    string `json:"expires_at"`
	AutoRenew    bool   `json:"auto_renew"`
}

// ListDomains 拉取托管 Zone（kind=zone）；Region 填了 Account ID 时附带
// Registrar 注册域名（kind=domain，含注册/到期时间）。
func (p *Provider) ListDomains(ctx context.Context) ([]provider.DomainInfo, error) {
	var out []provider.DomainInfo

	err := p.doList(ctx, "/zones", 50, func(page json.RawMessage) error {
		var zones []cfZone
		if err := json.Unmarshal(page, &zones); err != nil {
			return err
		}
		for _, z := range zones {
			status := z.Status
			if z.Paused {
				status = "paused"
			}
			out = append(out, provider.DomainInfo{
				Name:      z.Name,
				Kind:      provider.KindZone,
				Registrar: "Cloudflare",
				Status:    status,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Registrar：需要 Account ID（存放在 Region 字段）
	if accountID := strings.TrimSpace(p.cred.Region); accountID != "" {
		path := "/accounts/" + url.PathEscape(accountID) + "/registrar/domains"
		err := p.doList(ctx, path, 50, func(page json.RawMessage) error {
			var domains []cfRegistrarDomain
			if err := json.Unmarshal(page, &domains); err != nil {
				return err
			}
			for _, d := range domains {
				info := provider.DomainInfo{
					Name:      d.Name,
					Kind:      provider.KindDomain,
					Registrar: "Cloudflare",
					Status:    d.Status,
				}
				if t, e := time.Parse(time.RFC3339, d.RegisteredAt); e == nil {
					info.RegisteredAt = t
				}
				if t, e := time.Parse(time.RFC3339, d.ExpiresAt); e == nil {
					info.ExpireAt = t
				}
				out = append(out, info)
			}
			return nil
		})
		// Registrar 仅附加信息：失败不阻塞 Zone 同步，但空结果时给出行内提示
		if err != nil {
			if out == nil {
				return nil, fmt.Errorf("Registrar 域名拉取失败（检查 Region 是否为 Account ID 或移除）: %w", err)
			}
			out = append(out, provider.DomainInfo{
				Name:      "(registrar 拉取失败: " + err.Error() + ")",
				Kind:      provider.KindZone,
				Registrar: "Cloudflare",
				Status:    "registrar_error",
			})
		}
	}

	return out, nil
}

// CheckConnection 连通性检测：Token 认证走 /user/tokens/verify，否则拉 Zone 列表。
func (p *Provider) CheckConnection(ctx context.Context) error {
	if p.cred.SecretKey == "" {
		result, _, err := p.do(ctx, "", "/user/tokens/verify", nil)
		if err != nil {
			return err
		}
		var v struct {
			Status string `json:"status"` // active / disabled
		}
		if err := json.Unmarshal(result, &v); err == nil && v.Status == "disabled" {
			return fmt.Errorf("Cloudflare API Token 已被禁用")
		}
		return nil
	}
	if _, _, err := p.do(ctx, "", "/zones?per_page=1", nil); err != nil {
		return err
	}
	return nil
}
