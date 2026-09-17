// demoseed 演示环境自举工具：建表 + 初始管理员 + 幂等灌入演示数据。
//
// 用途：无服务器容器平台部署演示实例。容器文件系统可能是临时的
// （冷启动后 sqlite 文件丢失/为空），故每次启动前执行本工具：
// 已有数据则跳过，无数据则重建，保证演示站任何环境下都有完整观感。
//
// 数据全部为 example.com 系保留域名假数据，不含真实资产。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/bootstrap"
	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/pkg/config"
	applog "github.com/onemore-coder/domhub/internal/pkg/logger"
)

func main() {
	cfg := config.Load()
	applog.Init("release")
	db := bootstrap.InitDB(cfg)
	bootstrap.Seed(db, cfg)
	seedDemo(db)
}

func seedDemo(db *gorm.DB) {
	var n int64
	db.Model(&model.CloudAccount{}).Count(&n)
	if n > 0 {
		applog.L().Info("演示数据已存在，跳过 seeding", zap.Int64("count", n))
		return
	}

	now := time.Now()
	hAgo := func(h int) *time.Time { t := now.Add(-time.Duration(h) * time.Hour); return &t }
	dLater := func(d int) *time.Time { t := now.AddDate(0, 0, d); return &t }
	dateAt := func(s string) *time.Time { t, _ := time.Parse(time.DateOnly, s); return &t }
	must := func(msg string, err error) {
		if err != nil {
			panic(fmt.Sprintf("%s: %v", msg, err))
		}
	}

	// ---- 云账号 ----
	accounts := []model.CloudAccount{
		{Name: "腾讯云主账号", Provider: "tencent", AccessKey: "AKIDdemo0000000000000000", SecretKey: "enc-demo", Region: "ap-guangzhou", Status: 1, LastCheckAt: hAgo(2), LastCheckOK: true, LastCheckMsg: "凭证校验通过", LastSyncAt: hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{Name: "阿里云账号A", Provider: "aliyun", AccessKey: "LTAIdemo000000000000", SecretKey: "enc-demo", Region: "cn-hangzhou", Status: 1, LastCheckAt: hAgo(2), LastCheckOK: true, LastCheckMsg: "凭证校验通过", LastSyncAt: hAgo(2), CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: *hAgo(2)},
		{Name: "Cloudflare", Provider: "cloudflare", AccessKey: "cf-demo-token-000000", Region: "", Status: 1, LastCheckAt: hAgo(3), LastCheckOK: true, LastCheckMsg: "凭证校验通过", LastSyncAt: hAgo(3), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(3)},
		{Name: "AWS Route53", Provider: "aws", AccessKey: "AKIAdemo000000000000", SecretKey: "enc-demo", Region: "us-east-1", Status: 1, LastCheckAt: hAgo(5), LastCheckOK: false, LastCheckMsg: "凭证校验失败: 权限不足", LastSyncAt: hAgo(5), CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: *hAgo(5)},
	}
	must("插入云账号", db.Create(&accounts).Error)

	// ---- 域名台账 ----
	domains := []model.Domain{
		{CloudAccountID: 1, Name: "example.com", Kind: "domain", Provider: "tencent", Registrar: "腾讯云", Status: "ok", RegisteredAt: dateAt("2022-03-15"), ExpireAt: dLater(820), Tags: "生产", Remark: "主站域名", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 1, Name: "example.cn", Kind: "domain", Provider: "tencent", Registrar: "腾讯云", Status: "ok", RegisteredAt: dateAt("2023-06-20"), ExpireAt: dLater(25), Tags: "生产", Remark: "即将续费", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 2, Name: "example.net", Kind: "domain", Provider: "aliyun", Registrar: "阿里云", Status: "ok", RegisteredAt: dateAt("2021-11-01"), ExpireAt: dLater(415), Tags: "生产,API", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 2, Name: "shop-demo.cn", Kind: "domain", Provider: "aliyun", Registrar: "阿里云", Status: "ok", RegisteredAt: dateAt("2024-01-10"), ExpireAt: dLater(12), Tags: "电商", Remark: "尽快续费！", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 3, Name: "example.dev", Kind: "domain", Provider: "cloudflare", Registrar: "Cloudflare", Status: "ok", RegisteredAt: dateAt("2023-09-08"), ExpireAt: dLater(600), Tags: "测试", LastSyncedAt: *hAgo(3), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(3)},
		{CloudAccountID: 3, Name: "example.app", Kind: "domain", Provider: "cloudflare", Registrar: "Cloudflare", Status: "ok", RegisteredAt: dateAt("2024-04-22"), ExpireAt: dLater(980), Tags: "测试", LastSyncedAt: *hAgo(3), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(3)},
		{CloudAccountID: 4, Name: "example.io", Kind: "domain", Provider: "aws", Registrar: "MarkMonitor", Status: "ok", RegisteredAt: dateAt("2020-07-30"), ExpireAt: dLater(180), Tags: "海外", LastSyncedAt: *hAgo(5), CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: *hAgo(5)},
		{CloudAccountID: 4, Name: "demo-site.com", Kind: "domain", Provider: "aws", Registrar: "Namecheap", Status: "ok", RegisteredAt: dateAt("2025-02-14"), ExpireAt: dLater(45), Tags: "海外,博客", LastSyncedAt: *hAgo(5), CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: *hAgo(5)},
		// 托管 Zone（仪表盘「托管 Zone」统计 kind=zone 行）
		{CloudAccountID: 1, Name: "example.com", Kind: "zone", Provider: "tencent", Status: "active", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 1, Name: "example.cn", Kind: "zone", Provider: "tencent", Status: "active", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 2, Name: "example.net", Kind: "zone", Provider: "aliyun", Status: "active", LastSyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 3, Name: "example.dev", Kind: "zone", Provider: "cloudflare", Status: "active", LastSyncedAt: *hAgo(3), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(3)},
		{CloudAccountID: 4, Name: "example.io", Kind: "zone", Provider: "aws", Status: "active", LastSyncedAt: *hAgo(5), CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: *hAgo(5)},
	}
	must("插入域名", db.Create(&domains).Error)

	// ---- 托管 Zone 缓存 ----
	zones := []model.Zone{
		{CloudAccountID: 1, Name: "example.com", RecordCount: 8, SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 1, Name: "example.cn", RecordCount: 5, SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 2, Name: "example.net", RecordCount: 6, SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -25), UpdatedAt: *hAgo(2)},
		{CloudAccountID: 3, Name: "example.dev", RecordCount: 4, SyncedAt: *hAgo(3), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(3)},
		{CloudAccountID: 4, Name: "example.io", RecordCount: 7, SyncedAt: *hAgo(5), CreatedAt: now.AddDate(0, 0, -15), UpdatedAt: *hAgo(5)},
	}
	must("插入 Zone", db.Create(&zones).Error)

	// ---- 解析记录 ----
	recKey := func(name, typ, value, line string) string {
		sum := sha256.Sum256([]byte(name + "|" + typ + "|" + value + "|" + line))
		return hex.EncodeToString(sum[:])
	}
	records := []model.DnsRecord{
		{CloudAccountID: 1, ZoneName: "example.com", ProviderRecordID: "rec-1001", Name: "example.com", Type: "A", Value: "203.0.113.10", TTL: 600, Line: "默认", Status: "ENABLE", Remark: "主站入口", SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30)},
		{CloudAccountID: 1, ZoneName: "example.com", ProviderRecordID: "rec-1002", Name: "www", Type: "A", Value: "203.0.113.10", TTL: 600, Line: "默认", Status: "ENABLE", SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30)},
		{CloudAccountID: 1, ZoneName: "example.com", ProviderRecordID: "rec-1003", Name: "@", Type: "MX", Value: "mail.example.com", TTL: 600, Priority: 10, Line: "默认", Status: "ENABLE", Remark: "企业邮箱", SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30)},
		{CloudAccountID: 1, ZoneName: "example.com", ProviderRecordID: "rec-1004", Name: "@", Type: "TXT", Value: "v=spf1 include:example.com -all", TTL: 600, Line: "默认", Status: "ENABLE", Remark: "SPF", SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30)},
		{CloudAccountID: 1, ZoneName: "example.com", ProviderRecordID: "rec-1005", Name: "api", Type: "CNAME", Value: "example.com", TTL: 300, Line: "默认", Status: "ENABLE", Remark: "API 网关", SyncedAt: *hAgo(2), CreatedAt: now.AddDate(0, 0, -30)},
	}
	for i := range records {
		r := &records[i]
		r.RecordKey = recKey(r.Name, r.Type, r.Value, r.Line)
	}
	must("插入解析记录", db.Create(&records).Error)

	// ---- 审计日志 ----
	audits := []model.AuditLog{
		{UserID: 1, Username: "admin", Action: "dns.update", Resource: "/zones/example.com/records/api", Detail: `{"before":{"ttl":600},"after":{"ttl":300}}`, Status: "success", CreatedAt: now.Add(-26 * time.Hour)},
		{UserID: 1, Username: "admin", Action: "dns.create", Resource: "/zones/example.com/records/cdn", Detail: `{"type":"CNAME","value":"cdn.example.com"}`, Status: "success", CreatedAt: now.Add(-25 * time.Hour)},
		{UserID: 1, Username: "admin", Action: "cert.deploy", Resource: "/certs-issued/3/deploys/1", Detail: `{"target":"腾讯云 CDN","domain":"example.com"}`, Status: "success", CreatedAt: now.Add(-20 * time.Hour)},
		{UserID: 1, Username: "admin", Action: "domain.sync", Resource: "/accounts/2/sync", Detail: `{"added":1,"updated":2}`, Status: "success", CreatedAt: now.Add(-3 * time.Hour)},
		{UserID: 1, Username: "admin", Action: "dns.delete", Resource: "/zones/example.cn/records/old", Detail: `{"type":"A","value":"203.0.113.99"}`, Status: "success", CreatedAt: now.Add(-2 * time.Hour)},
	}
	must("插入审计日志", db.Create(&audits).Error)

	// ---- 证书签发记录 ----
	certs := []model.IssuedCert{
		{PrimaryDomain: "example.com", SANs: "example.com,*.example.com", DNSAccountID: 1, DNSProvider: "tencent", AcmeEmail: "admin@example.com", DirectoryURL: "letsencrypt", CAName: "Let's Encrypt", Status: "issued", AutoRenew: true, CertChain: "-----BEGIN CERTIFICATE-----demo", PrivateKeyEnc: "enc", NotBefore: hAgo(24 * 40), NotAfter: dLater(50), LastMessage: "签发成功", RenewCount: 2, CreatedAt: now.AddDate(0, 0, -80), UpdatedAt: *hAgo(24 * 40)},
		{PrimaryDomain: "example.net", SANs: "example.net", DNSAccountID: 2, DNSProvider: "aliyun", AcmeEmail: "admin@example.net", DirectoryURL: "zerossl", CAName: "ZeroSSL", Status: "issued", AutoRenew: true, CertChain: "-----BEGIN CERTIFICATE-----demo", PrivateKeyEnc: "enc", NotBefore: hAgo(24 * 20), NotAfter: dLater(70), LastMessage: "签发成功", RenewCount: 1, CreatedAt: now.AddDate(0, 0, -50), UpdatedAt: *hAgo(24 * 20)},
		{PrimaryDomain: "example.dev", SANs: "example.dev,*.example.dev", DNSAccountID: 3, DNSProvider: "cloudflare", AcmeEmail: "admin@example.dev", DirectoryURL: "letsencrypt", CAName: "Let's Encrypt", Status: "issued", AutoRenew: true, CertChain: "-----BEGIN CERTIFICATE-----demo", PrivateKeyEnc: "enc", NotBefore: hAgo(24 * 5), NotAfter: dLater(85), LastMessage: "签发成功", CreatedAt: now.AddDate(0, 0, -5), UpdatedAt: *hAgo(24 * 5)},
	}
	must("插入证书", db.Create(&certs).Error)

	// ---- 证书监控主机 ----
	certStatuses := []model.CertStatus{
		{Host: "example.com", DomainID: 1, DomainName: "example.com", Source: "auto", NotAfter: dLater(50), Issuer: "Let's Encrypt", Subject: "example.com", DaysLeft: 50, OK: true, CheckedAt: *hAgo(1), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(1)},
		{Host: "www.example.com", DomainID: 1, DomainName: "example.com", Source: "auto", NotAfter: dLater(50), Issuer: "Let's Encrypt", Subject: "example.com", DaysLeft: 50, OK: true, CheckedAt: *hAgo(1), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(1)},
		{Host: "api.example.net", DomainID: 3, DomainName: "example.net", Source: "auto", NotAfter: dLater(15), Issuer: "ZeroSSL", Subject: "example.net", DaysLeft: 15, OK: true, AlertedOffsets: "[30]", CheckedAt: *hAgo(1), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(1)},
		{Host: "example.dev", DomainID: 5, DomainName: "example.dev", Source: "auto", NotAfter: dLater(85), Issuer: "Let's Encrypt", Subject: "example.dev", DaysLeft: 85, OK: true, CheckedAt: *hAgo(1), CreatedAt: now.AddDate(0, 0, -20), UpdatedAt: *hAgo(1)},
	}
	must("插入证书监控", db.Create(&certStatuses).Error)

	// ---- 告警渠道与规则 ----
	channels := []model.AlertChannel{
		{Name: "运维钉钉群", Type: "dingtalk", Config: `{"webhook":"https://oapi.dingtalk.com/robot/send?access_token=demo"}`, Enabled: true, CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)},
		{Name: "值班企业微信", Type: "wecom", Config: `{"webhook":"https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=demo"}`, Enabled: true, CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)},
	}
	must("插入告警渠道", db.Create(&channels).Error)
	rules := []model.AlertRule{
		{Name: "域名到期提醒", Kind: "domain_expire", Offsets: "60,30,7,1", ChannelIDs: "[1,2]", Enabled: true, CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)},
		{Name: "证书到期提醒", Kind: "cert_expire", Offsets: "30,14,7", ChannelIDs: "[1]", Enabled: true, CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now.AddDate(0, 0, -30)},
	}
	must("插入告警规则", db.Create(&rules).Error)

	// ---- API Token（哈希为假值，仅展示列表观感） ----
	fakeHash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	exp180 := dLater(180)
	tokens := []model.ApiToken{
		{UserID: 1, Name: "CI 部署令牌", TokenHash: fakeHash("dht_demo_ci_token"), Prefix: "dht_ci5K", ExpireAt: exp180, LastUsedAt: hAgo(4), CreatedAt: now.AddDate(0, 0, -60), UpdatedAt: *hAgo(4)},
		{UserID: 1, Name: "监控脚本", TokenHash: fakeHash("dht_demo_monitor_token"), Prefix: "dht_mo2X", LastUsedAt: hAgo(24), CreatedAt: now.AddDate(0, 0, -45), UpdatedAt: *hAgo(24)},
	}
	must("插入 API Token", db.Create(&tokens).Error)

	// ---- 演示用户（ops/demo123456、viewer/demo123456） ----
	hash, err := bcrypt.GenerateFromPassword([]byte("demo123456"), bcrypt.DefaultCost)
	must("生成演示用户密码哈希", err)
	lastLogin := hAgo(8)
	extraUsers := []model.User{
		{Username: "ops", PasswordHash: string(hash), Role: "operator", Status: 1, LastLoginAt: lastLogin, CreatedAt: now.AddDate(0, 0, -40), UpdatedAt: *lastLogin},
	}
	lastLogin2 := hAgo(48)
	extraUsers = append(extraUsers, model.User{Username: "viewer", PasswordHash: string(hash), Role: "viewer", Status: 1, LastLoginAt: lastLogin2, CreatedAt: now.AddDate(0, 0, -35), UpdatedAt: *lastLogin2})
	must("插入演示用户", db.Create(&extraUsers).Error)

	applog.L().Info("演示数据 seeding 完成",
		zap.Int("domains", len(domains)),
		zap.Int("records", len(records)),
		zap.Int("certs", len(certs)))
}
