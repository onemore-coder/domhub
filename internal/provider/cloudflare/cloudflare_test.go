package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/onemore-coder/domhub/internal/provider"
)

// mockCF 模拟 Cloudflare v4 API，返回每次请求的路径记录与预置响应。
type mockCF struct {
	t       *testing.T
	srv     *httptest.Server
	paths   []string
	bodies  []string
	handler func(w http.ResponseWriter, r *http.Request, path string)
}

func newMockCF(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, path string)) *mockCF {
	m := &mockCF{t: t, handler: handler}
	m.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		m.paths = append(m.paths, r.URL.Path+"?"+r.URL.RawQuery)
		if r.Body != nil {
			buf := make([]byte, 4096)
			n, _ := r.Body.Read(buf)
			if n > 0 {
				m.bodies = append(m.bodies, string(buf[:n]))
			}
		}
		handler(w, r, r.URL.Path)
	}))
	t.Cleanup(m.srv.Close)
	apiBase = m.srv.URL
	return m
}

func writeEnvelope(w http.ResponseWriter, result any) {
	w.Header().Set("Content-Type", "application/json")
	b, _ := json.Marshal(result)
	w.Write([]byte(`{"success":true,"errors":[],"result_info":{"page":1,"per_page":50,"total_pages":1},"result":` + string(b) + `}`))
}

func TestTokenAuthHeader(t *testing.T) {
	var gotAuth string
	newMockCF(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		gotAuth = r.Header.Get("Authorization")
		writeEnvelope(w, map[string]any{"status": "active"})
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok-123"}}
	if err := p.CheckConnection(context.Background()); err != nil {
		t.Fatalf("CheckConnection 失败: %v", err)
	}
	if gotAuth != "Bearer tok-123" {
		t.Fatalf("Token 认证头错误: %q", gotAuth)
	}
}

func TestGlobalKeyAuthHeader(t *testing.T) {
	var email, key string
	newMockCF(t, func(w http.ResponseWriter, r *http.Request, _ string) {
		email = r.Header.Get("X-Auth-Email")
		key = r.Header.Get("X-Auth-Key")
		writeEnvelope(w, []any{})
	})
	p := &Provider{cred: provider.Credential{AccessKey: "a@b.com", SecretKey: "global-key"}}
	if err := p.CheckConnection(context.Background()); err != nil {
		t.Fatalf("CheckConnection 失败: %v", err)
	}
	if email != "a@b.com" || key != "global-key" {
		t.Fatalf("Global Key 认证头错误: %q / %q", email, key)
	}
}

func TestListZonesAndRecords(t *testing.T) {
	newMockCF(t, func(w http.ResponseWriter, r *http.Request, path string) {
		switch {
		case strings.HasPrefix(path, "/zones/z1/dns_records"):
			writeEnvelope(w, []map[string]any{
				{"id": "r1", "type": "A", "name": "example.com", "content": "1.2.3.4", "ttl": 300},
				{"id": "r2", "type": "CNAME", "name": "www.example.com", "content": "example.com", "ttl": 1, "proxied": true},
				{"id": "r3", "type": "MX", "name": "example.com", "content": "mail.example.com", "ttl": 600, "priority": 10},
			})
		case strings.HasPrefix(path, "/zones") && r.URL.Query().Get("name") != "":
			// resolveZoneID 的精确查询
			writeEnvelope(w, []map[string]any{
				{"id": "z1", "name": "example.com", "status": "active"},
			})
		case strings.HasPrefix(path, "/zones") && r.URL.Query().Get("name") == "":
			writeEnvelope(w, []map[string]any{
				{"id": "z1", "name": "example.com", "status": "active"},
			})
		default:
			writeEnvelope(w, []any{})
		}
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok"}}

	zones, err := p.ListZones(context.Background())
	if err != nil || len(zones) != 1 || zones[0].Name != "example.com" {
		t.Fatalf("ListZones 异常: %v %+v", err, zones)
	}

	records, err := p.ListRecords(context.Background(), "example.com")
	if err != nil {
		t.Fatalf("ListRecords 失败: %v", err)
	}
	if len(records) != 3 {
		t.Fatalf("期望 3 条记录，得到 %d", len(records))
	}
	if records[0].Name != "@" || records[0].Value != "1.2.3.4" {
		t.Fatalf("apex 记录映射错误: %+v", records[0])
	}
	if !records[1].Proxied || records[1].TTL != 1 {
		t.Fatalf("proxied 记录映射错误: %+v", records[1])
	}
	if records[0].Proxied {
		t.Fatalf("未代理记录 Proxied 应为 false: %+v", records[0])
	}
	if records[2].Priority != 10 || records[2].Value != "mail.example.com" {
		t.Fatalf("MX 优先级映射错误: %+v", records[2])
	}
}

func TestCreateRecordMXBody(t *testing.T) {
	m := newMockCF(t, func(w http.ResponseWriter, r *http.Request, path string) {
		switch {
		case strings.HasPrefix(path, "/zones") && r.URL.Query().Get("name") != "":
			writeEnvelope(w, []map[string]any{{"id": "z1", "name": "example.com"}})
		case strings.HasPrefix(path, "/zones/z1/dns_records") && r.Method == http.MethodPost:
			writeEnvelope(w, map[string]any{"id": "new-id", "type": "MX"})
		default:
			writeEnvelope(w, []any{})
		}
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok"}}

	id, err := p.CreateRecord(context.Background(), "example.com", provider.RecordInfo{
		Name: "@", Type: "MX", Value: "mail.example.com", Priority: 20, TTL: 600,
	})
	if err != nil || id != "new-id" {
		t.Fatalf("CreateRecord 失败: id=%q err=%v", id, err)
	}
	if len(m.bodies) == 0 {
		t.Fatal("未捕获到请求体")
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(m.bodies[0]), &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if body["name"] != "example.com" {
		t.Fatalf("@ 未转换为 FQDN: %v", body["name"])
	}
	if body["priority"].(float64) != 20 {
		t.Fatalf("MX 优先级错误: %v", body["priority"])
	}
}

func TestCreateRecordProxiedBody(t *testing.T) {
	m := newMockCF(t, func(w http.ResponseWriter, r *http.Request, path string) {
		switch {
		case strings.HasPrefix(path, "/zones") && r.URL.Query().Get("name") != "":
			writeEnvelope(w, []map[string]any{{"id": "z1", "name": "example.com"}})
		case strings.HasPrefix(path, "/zones/z1/dns_records") && r.Method == http.MethodPost:
			writeEnvelope(w, map[string]any{"id": "new-id"})
		default:
			writeEnvelope(w, []any{})
		}
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok"}}

	// 开启代理：proxied=true，TTL 强制 auto(1)
	_, err := p.CreateRecord(context.Background(), "example.com", provider.RecordInfo{
		Name: "www", Type: "A", Value: "1.2.3.4", TTL: 600, Proxied: true,
	})
	if err != nil {
		t.Fatalf("CreateRecord(proxied) 失败: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(m.bodies[len(m.bodies)-1]), &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if body["proxied"] != true {
		t.Fatalf("proxied 应为 true: %v", body["proxied"])
	}
	if body["ttl"].(float64) != 1 {
		t.Fatalf("代理记录 TTL 应强制为 auto(1): %v", body["ttl"])
	}

	// DNS only：proxied=false 也要显式传（避免保留云端旧状态）
	_, err = p.CreateRecord(context.Background(), "example.com", provider.RecordInfo{
		Name: "@", Type: "A", Value: "1.2.3.4", TTL: 300,
	})
	if err != nil {
		t.Fatalf("CreateRecord(dns only) 失败: %v", err)
	}
	body = map[string]any{} // 注意：Unmarshal 对已有 map 是合并语义，必须重置
	if err := json.Unmarshal([]byte(m.bodies[len(m.bodies)-1]), &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if body["proxied"] != false {
		t.Fatalf("proxied 应为 false: %v", body["proxied"])
	}
	if body["ttl"].(float64) != 300 {
		t.Fatalf("DNS only 记录 TTL 不应被改写: %v", body["ttl"])
	}

	// 不可代理类型（TXT）：不应携带 proxied 字段
	_, err = p.CreateRecord(context.Background(), "example.com", provider.RecordInfo{
		Name: "_test", Type: "TXT", Value: "hello", TTL: 300,
	})
	if err != nil {
		t.Fatalf("CreateRecord(TXT) 失败: %v", err)
	}
	body = map[string]any{}
	if err := json.Unmarshal([]byte(m.bodies[len(m.bodies)-1]), &body); err != nil {
		t.Fatalf("请求体解析失败: %v", err)
	}
	if _, ok := body["proxied"]; ok {
		t.Fatalf("TXT 记录不应携带 proxied 字段: %v", body)
	}
}

func TestListDomainsWithRegistrar(t *testing.T) {
	newMockCF(t, func(w http.ResponseWriter, r *http.Request, path string) {
		if strings.Contains(path, "/registrar/domains") {
			writeEnvelope(w, []map[string]any{
				{"name": "example.com", "status": "active", "expires_at": "2027-01-01T00:00:00Z"},
			})
		} else {
			writeEnvelope(w, []map[string]any{{"id": "z1", "name": "example.com", "status": "active"}})
		}
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok", Region: "acc-001"}}

	domains, err := p.ListDomains(context.Background())
	if err != nil {
		t.Fatalf("ListDomains 失败: %v", err)
	}
	var hasZone, hasDomain bool
	for _, d := range domains {
		if d.Kind == provider.KindZone && d.Name == "example.com" {
			hasZone = true
		}
		if d.Kind == provider.KindDomain && d.Name == "example.com" && d.ExpireAt.Year() == 2027 {
			hasDomain = true
		}
	}
	if !hasZone || !hasDomain {
		t.Fatalf("Zone/Registrar 域名缺失: %+v", domains)
	}
}

func TestAPIErrorSurface(t *testing.T) {
	newMockCF(t, func(w http.ResponseWriter, _ *http.Request, _ string) {
		w.Write([]byte(`{"success":false,"errors":[{"code":9103,"message":"Unknown X-Auth-Key or X-Auth-Email"}]}`))
	})
	p := &Provider{cred: provider.Credential{AccessKey: "tok"}}
	err := p.CheckConnection(context.Background())
	if err == nil || !strings.Contains(err.Error(), "9103") {
		t.Fatalf("API 错误未透出: %v", err)
	}
}
