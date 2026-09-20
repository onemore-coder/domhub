package mcp

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/repo"
	"github.com/onemore-coder/domhub/internal/service"
)

// testEnv 内存库 + 种子数据 + 已签发的 dht_ Token。
type testEnv struct {
	deps  Deps
	plain string // 明文 Token
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("内存库打开失败: %v", err)
	}
	if err := db.AutoMigrate(
		&model.User{}, &model.CloudAccount{}, &model.Domain{}, &model.Zone{},
		&model.DnsRecord{}, &model.CertStatus{}, &model.AlertLog{},
		&model.ApiToken{}, &model.UserZone{},
	); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	u := &model.User{Username: "tester", Role: model.RoleAdmin, Status: 1}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("建用户失败: %v", err)
	}
	if err := db.Create(&model.CloudAccount{Name: "主账号", Provider: "tencent", Status: 1}).Error; err != nil {
		t.Fatalf("建云账号失败: %v", err)
	}
	exp := time.Now().AddDate(0, 0, 15)
	if err := db.Create(&model.Domain{
		CloudAccountID: 1, Name: "example.com", Kind: "domain", Provider: "tencent", ExpireAt: &exp,
	}).Error; err != nil {
		t.Fatalf("建域名失败: %v", err)
	}
	if err := db.Create(&model.Zone{CloudAccountID: 1, Name: "example.com", RecordCount: 2}).Error; err != nil {
		t.Fatalf("建 Zone 失败: %v", err)
	}
	if err := db.Create(&model.DnsRecord{CloudAccountID: 1, ZoneName: "example.com", Name: "www", Type: "A", Value: "1.2.3.4"}).Error; err != nil {
		t.Fatalf("建解析记录失败: %v", err)
	}
	if err := db.Create(&model.CertStatus{Host: "www.example.com", DomainName: "example.com", OK: true, DaysLeft: 20}).Error; err != nil {
		t.Fatalf("建证书失败: %v", err)
	}

	deps := Deps{
		Tokens:   service.NewTokenService(repo.NewApiTokenRepo(db), repo.NewUserRepo(db)),
		Domains:  repo.NewDomainRepo(db),
		Accounts: repo.NewCloudAccountRepo(db),
		Alerts:   repo.NewAlertRepo(db),
		Zones: service.NewZoneService(
			repo.NewCloudAccountRepo(db), repo.NewZoneRepo(db), repo.NewDnsRecordRepo(db),
			repo.NewGrantRepo(db), nil),
		Certs: service.NewCertService(repo.NewCertRepo(db), repo.NewDomainRepo(db), repo.NewAlertRepo(db), repo.NewDnsRecordRepo(db)),
	}
	res, err := deps.Tokens.Create(service.CreateInput{UserID: u.ID, Username: u.Username, Name: "mcp-test"})
	if err != nil {
		t.Fatalf("签发 Token 失败: %v", err)
	}
	return &testEnv{deps: deps, plain: res.Plain}
}

func TestActorFromReq(t *testing.T) {
	env := newTestEnv(t)

	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{"Authorization": {"Bearer " + env.plain}}}}
	op, err := actorFromReq(env.deps, req)
	if err != nil {
		t.Fatalf("有效 Token 应解析成功: %v", err)
	}
	if op.Username != "tester" || op.Role != model.RoleAdmin {
		t.Fatalf("解析出的用户三要素异常: %+v", op)
	}

	req2 := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{"Authorization": {"Bearer dht_wrong"}}}}
	if _, err := actorFromReq(env.deps, req2); err == nil {
		t.Fatal("无效 Token 应报错")
	}

	req3 := &mcp.CallToolRequest{}
	if _, err := actorFromReq(env.deps, req3); err == nil {
		t.Fatal("无头请求应报错")
	}
}

func TestToolsRoundTrip(t *testing.T) {
	env := newTestEnv(t)
	op := service.Actor{ID: 1, Username: "tester", Role: model.RoleAdmin}

	// overview：域名/Zone/证书计数
	out, err := env.deps.overview(context.Background(), op, struct{}{})
	if err != nil {
		t.Fatalf("overview 失败: %v", err)
	}
	ov := out.(overviewOut)
	if ov.DomainsExpiring30d != 1 || ov.Zones != 1 || ov.CloudAccounts != 1 || ov.CertsExpiring30d != 1 {
		t.Fatalf("overview 计数异常: %+v", ov)
	}

	// list_domains：到期过滤
	dom, err := env.deps.listDomains(context.Background(), op, listDomainsIn{ExpiringDays: 30})
	if err != nil {
		t.Fatalf("list_domains 失败: %v", err)
	}
	dm := dom.(map[string]any)
	if dm["total"].(int64) != 1 {
		t.Fatalf("list_domains 应命中 1 条: %+v", dm)
	}

	// list_zones
	zones, err := env.deps.listZones(context.Background(), op, struct{}{})
	if err != nil {
		t.Fatalf("list_zones 失败: %v", err)
	}
	if len(zones.([]repo.ZoneView)) != 1 {
		t.Fatalf("list_zones 应返回 1 条: %+v", zones)
	}

	// list_dns_records：按 Zone 过滤
	recs, err := env.deps.listRecords(context.Background(), op, listRecordsIn{Zone: "example.com"})
	if err != nil {
		t.Fatalf("list_dns_records 失败: %v", err)
	}
	if len(recs.([]model.DnsRecord)) != 1 {
		t.Fatalf("list_dns_records 应返回 1 条: %+v", recs)
	}

	// search_dns：命中记录
	sr, err := env.deps.searchDNS(context.Background(), op, searchDNSIn{Query: "www"})
	if err != nil {
		t.Fatalf("search_dns 失败: %v", err)
	}
	sm := sr.(map[string]any)
	if len(sm["records"].([]recordHit)) != 1 {
		t.Fatalf("search_dns 应命中 1 条记录: %+v", sm)
	}

	// list_certs：30 天临期过滤
	certs, err := env.deps.listCerts(context.Background(), op, listCertsIn{ExpiringDays: 30})
	if err != nil {
		t.Fatalf("list_certs 失败: %v", err)
	}
	if len(certs.([]model.CertStatus)) != 1 {
		t.Fatalf("list_certs 应命中 1 条: %+v", certs)
	}

	// list_cloud_accounts：不含密钥字段
	accs, err := env.deps.listAccounts(context.Background(), op, struct{}{})
	if err != nil {
		t.Fatalf("list_cloud_accounts 失败: %v", err)
	}
	av := accs.([]accountView)
	if len(av) != 1 || av[0].Provider != "tencent" {
		t.Fatalf("list_cloud_accounts 异常: %+v", av)
	}
}
