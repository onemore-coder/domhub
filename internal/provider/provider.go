// Package provider 云厂商抽象层（参考 DNSControl 的 Provider 插件模型）。
// 新增云厂商 = 实现 DomainProvider 接口并在各自包的 init() 中自注册。
package provider

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Credential 云厂商访问凭证（已解密的明文，仅在内存中流转）。
type Credential struct {
	AccessKey string
	SecretKey string
	Region    string
}

// DomainKind 域名条目类型。
const (
	KindDomain = "domain" // 注册域名（含到期时间）
	KindZone   = "zone"   // 托管解析 Zone（无到期时间）
)

// DomainInfo 统一的域名/Zone 信息。
type DomainInfo struct {
	Name         string
	Kind         string
	Registrar    string
	Status       string
	RegisteredAt time.Time // 零值表示未知
	ExpireAt     time.Time // 零值表示未知
}

// DomainProvider 域名资产提供者接口。
type DomainProvider interface {
	// ListDomains 拉取账号下的域名与托管 Zone。
	ListDomains(ctx context.Context) ([]DomainInfo, error)
	// CheckConnection 连通性与权限检测。
	CheckConnection(ctx context.Context) error
}

// Factory 由凭证构建 Provider 实例。
type Factory func(cred Credential) (DomainProvider, error)

var (
	mu       sync.RWMutex
	registry = map[string]Factory{}
)

// Register 注册厂商实现（在实现包的 init 中调用）。
func Register(name string, f Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = f
}

// Get 获取厂商工厂。
func Get(name string) (Factory, error) {
	mu.RLock()
	defer mu.RUnlock()
	f, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("不支持的云厂商: %s", name)
	}
	return f, nil
}

// Supported 返回已注册的厂商列表（有序）。
func Supported() []string {
	mu.RLock()
	defer mu.RUnlock()
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
