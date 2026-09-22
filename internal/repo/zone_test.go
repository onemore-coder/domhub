package repo

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
)

func newZoneTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存库失败: %v", err)
	}
	if err := db.AutoMigrate(&model.Zone{}, &model.DnsRecord{}, &model.CloudAccount{}); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	return db
}

// 回归：厂商 ListZones 的 record_count 不可靠（Cloudflare 恒为 0），
// UpsertBatch 不得覆盖镜像同步（TouchRecordCount/ApplyMirrorCounts）回写的真实值。
func TestUpsertBatchPreservesRecordCount(t *testing.T) {
	db := newZoneTestDB(t)
	r := NewZoneRepo(db)
	acct := uint(1)

	// 第一轮：厂商返回 count=0，写入缓存（ListViews 需要关联启用的云账号）
	if err := db.Create(&model.CloudAccount{ID: acct, Name: "cf", Provider: "cloudflare", Status: 1}).Error; err != nil {
		t.Fatalf("造账号失败: %v", err)
	}
	if err := r.UpsertBatch(acct, []model.Zone{{Name: "bisfin.hk", RecordCount: 0}}, time.Now()); err != nil {
		t.Fatalf("UpsertBatch 失败: %v", err)
	}

	// 模拟解析记录镜像同步：11 条记录 + 回写
	if err := r.TouchRecordCount(acct, "bisfin.hk", 11, time.Now()); err != nil {
		t.Fatalf("TouchRecordCount 失败: %v", err)
	}

	// 第二轮 Zone 刷新：厂商仍返回 0，不得冲掉 11
	if err := r.UpsertBatch(acct, []model.Zone{{Name: "bisfin.hk", RecordCount: 0}}, time.Now()); err != nil {
		t.Fatalf("UpsertBatch 二次刷新失败: %v", err)
	}
	views, err := r.ListViews()
	if err != nil {
		t.Fatalf("ListViews 失败: %v", err)
	}
	if len(views) != 1 || views[0].RecordCount != 11 {
		t.Fatalf("Zone 刷新不应覆盖镜像回写的记录数: %+v", views)
	}

	// ApplyMirrorCounts：以本地镜像为准修正（含真实记录行时）
	records := []model.DnsRecord{
		{CloudAccountID: acct, ZoneName: "bisfin.hk"},
		{CloudAccountID: acct, ZoneName: "bisfin.hk", RecordKey: "k2"},
		{CloudAccountID: acct, ZoneName: "bisfin.hk", RecordKey: "k3"},
	}
	for i := range records {
		if err := db.Create(&records[i]).Error; err != nil {
			t.Fatalf("造记录失败: %v", err)
		}
	}
	if err := r.ApplyMirrorCounts(acct); err != nil {
		t.Fatalf("ApplyMirrorCounts 失败: %v", err)
	}
	views, _ = r.ListViews()
	if views[0].RecordCount != 3 {
		t.Fatalf("ApplyMirrorCounts 应按镜像修正为 3: %+v", views)
	}
}
