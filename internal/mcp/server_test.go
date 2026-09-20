package mcp

import (
	"context"
	"net/http"
	"strings"
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
	deps   Deps
	plain  string // 明文 Token（readonly）
	userID uint
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
		Certs:   service.NewCertService(repo.NewCertRepo(db), repo.NewDomainRepo(db), repo.NewAlertRepo(db), repo.NewDnsRecordRepo(db)),
		DNS:     service.NewDNSService(repo.NewCloudAccountRepo(db), nil, repo.NewAuditRepo(db), repo.NewGrantRepo(db)),
		Records: repo.NewDnsRecordRepo(db),
	}
	res, err := deps.Tokens.Create(service.CreateInput{UserID: u.ID, Username: u.Username, Name: "mcp-test"})
	if err != nil {
		t.Fatalf("签发 Token 失败: %v", err)
	}
	return &testEnv{deps: deps, plain: res.Plain, userID: u.ID}
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
	if op.Scope != model.TokenScopeReadOnly {
		t.Fatalf("默认签发应为只读 Token: %+v", op)
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

// issueToken 在既有环境上追加签发指定 scope 的 Token。
func (e *testEnv) issueToken(t *testing.T, scope string) string {
	t.Helper()
	res, err := e.deps.Tokens.Create(service.CreateInput{
		UserID: e.userID, Username: "tester", Name: "mcp-" + scope, Scope: scope,
	})
	if err != nil {
		t.Fatalf("签发 %s Token 失败: %v", scope, err)
	}
	return res.Plain
}

func TestWriteToolsScopeGate(t *testing.T) {
	env := newTestEnv(t)
	ro := service.Actor{ID: env.userID, Username: "tester", Role: model.RoleAdmin, Scope: model.TokenScopeReadOnly}
	rw := service.Actor{ID: env.userID, Username: "tester", Role: model.RoleAdmin, Scope: model.TokenScopeReadWrite}

	// 只读 Actor：三个写工具全部被 scope 拦截
	if _, err := env.deps.createRecord(context.Background(), ro, createRecordIn{Zone: "example.com", Type: "A", Value: "1.1.1.1"}); err == nil {
		t.Fatal("readonly 调 create_dns_record 应被拒绝")
	} else if !strings.Contains(err.Error(), "readwrite") {
		t.Fatalf("报错应提示需要 readwrite Token: %v", err)
	}
	if _, err := env.deps.updateRecord(context.Background(), ro, updateRecordIn{RecordID: 1, Value: "2.2.2.2"}); err == nil {
		t.Fatal("readonly 调 update_dns_record 应被拒绝")
	}
	if _, err := env.deps.deleteRecord(context.Background(), ro, deleteRecordIn{RecordID: 1}); err == nil {
		t.Fatal("readonly 调 delete_dns_record 应被拒绝")
	}

	// 读写 Actor：镜像不存在的 record_id 应报「不存在」，而非穿透到云厂商
	if _, err := env.deps.deleteRecord(context.Background(), rw, deleteRecordIn{RecordID: 999}); err == nil {
		t.Fatal("不存在的 record_id 应报错")
	} else if !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("报错应提示记录不存在: %v", err)
	}
	if _, err := env.deps.updateRecord(context.Background(), rw, updateRecordIn{RecordID: 999}); err == nil {
		t.Fatal("不存在的 record_id 应报错")
	}

	// 读写 Actor：缺 Zone/Type 参数应在解析账号前报参数错误
	if _, err := env.deps.createRecord(context.Background(), rw, createRecordIn{Zone: "", Type: "A", Value: "1.1.1.1"}); err == nil {
		t.Fatal("缺 zone 应报参数错误")
	}
}

func TestTokenScopeEndToEnd(t *testing.T) {
	env := newTestEnv(t)
	rwPlain := env.issueToken(t, model.TokenScopeReadWrite)

	// readwrite Token 经 actorFromReq 应解析出 readwrite scope
	req := &mcp.CallToolRequest{Extra: &mcp.RequestExtra{Header: http.Header{"X-Api-Key": {rwPlain}}}}
	op, err := actorFromReq(env.deps, req)
	if err != nil {
		t.Fatalf("readwrite Token 应解析成功: %v", err)
	}
	if op.Scope != model.TokenScopeReadWrite {
		t.Fatalf("scope 应为 readwrite: %+v", op)
	}

	// 非法 scope 签发应被拒绝
	if _, err := env.deps.Tokens.Create(service.CreateInput{
		UserID: env.userID, Username: "tester", Name: "bad", Scope: "admin",
	}); err == nil {
		t.Fatal("非法 scope 应拒绝签发")
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
