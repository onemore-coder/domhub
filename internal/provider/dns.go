package provider

import (
	"context"
	"fmt"
	"sync"
)

// ZoneInfo 托管解析 Zone 信息。
type ZoneInfo struct {
	Name         string `json:"name"` // 如 example.com
	RecordCount  int    `json:"record_count"`
	Remark       string `json:"remark"`
	PunycodeName string `json:"punycode_name"` // 中文域名的 punycode 形式（无则为空）
}

// RecordInfo 统一的解析记录信息。
type RecordInfo struct {
	ID       string `json:"id"`       // Provider 记录标识（阿里/腾讯为记录 ID；AWS 为编码的组合键）
	Name     string `json:"name"`     // 主机记录：@ / www / 子域名前缀（不含 zone 后缀）
	Type     string `json:"type"`     // A / AAAA / CNAME / TXT / MX / NS / CAA / SRV ...
	Value    string `json:"value"`    // 记录值；多值记录以 \n 分隔
	TTL      int    `json:"ttl"`      // 秒
	Priority int    `json:"priority"` // MX/SRV 优先级，无则为 0
	Line     string `json:"line"`     // 运营商线路：default / 移动 / 联通 / 电信（仅国内厂商）
	Status   string `json:"status"`   // 启用/暂停状态（Provider 语义，可为空）
	Remark   string `json:"remark"`   // 备注
	Proxied  bool   `json:"proxied"`  // CDN 代理状态（仅 Cloudflare 橙云：A/AAAA/CNAME 可代理）
}

// DNSProvider 解析记录管理接口。
type DNSProvider interface {
	// ListZones 列出账号下托管解析的 Zone。
	ListZones(ctx context.Context) ([]ZoneInfo, error)
	// ListRecords 拉取某 Zone 的全部解析记录。
	ListRecords(ctx context.Context, zone string) ([]RecordInfo, error)
	// CreateRecord 创建解析记录，返回 Provider 记录 ID。
	CreateRecord(ctx context.Context, zone string, rec RecordInfo) (string, error)
	// UpdateRecord 更新解析记录（rec.ID 为 Provider 记录标识）。
	UpdateRecord(ctx context.Context, zone string, rec RecordInfo) error
	// DeleteRecord 删除解析记录。
	DeleteRecord(ctx context.Context, zone, recordID string) error
	// CheckConnection 连通性与权限检测。
	CheckConnection(ctx context.Context) error
}

// DNSFactory 由凭证构建 DNSProvider 实例。
type DNSFactory func(cred Credential) (DNSProvider, error)

var (
	dnsMu       sync.RWMutex
	dnsRegistry = map[string]DNSFactory{}
)

// RegisterDNS 注册 DNS Provider 实现（在实现包的 init 中调用）。
func RegisterDNS(name string, f DNSFactory) {
	dnsMu.Lock()
	defer dnsMu.Unlock()
	dnsRegistry[name] = f
}

// GetDNS 获取 DNS Provider 工厂。
func GetDNS(name string) (DNSFactory, error) {
	dnsMu.RLock()
	defer dnsMu.RUnlock()
	f, ok := dnsRegistry[name]
	if !ok {
		return nil, fmt.Errorf("该云厂商暂不支持 DNS 解析管理: %s", name)
	}
	return f, nil
}

// SupportedDNS 返回已注册的 DNS Provider 列表（有序）。
func SupportedDNS() []string {
	dnsMu.RLock()
	defer dnsMu.RUnlock()
	names := make([]string, 0, len(dnsRegistry))
	for n := range dnsRegistry {
		names = append(names, n)
	}
	sortStrings(names)
	return names
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
