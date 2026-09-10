package model

import "time"

// DnsRecord 解析记录本地镜像（dns_records）。
//
// 定位：只读缓存 + 监控数据源，不是权威数据源（权威在云厂商）。
//   - 读路径：列表/搜索/证书监控主机发现走本表，规模大也秒开
//   - 写路径：变更操作实时写云端（DNSProvider），成功后回源刷新本表；
//     另有 sync_records 定时任务周期校准
//
// 唯一键 = 账号 + Zone + 内容键（RecordKey，sha256(name|type|value|line)）。
// 用内容键而非厂商记录 ID：AWS Route53 没有独立记录 ID。
type DnsRecord struct {
	ID               uint   `gorm:"primaryKey" json:"id"`
	CloudAccountID   uint   `gorm:"uniqueIndex:idx_rec_uniq,priority:1;not null" json:"cloud_account_id"`
	ZoneName         string `gorm:"uniqueIndex:idx_rec_uniq,priority:2;size:255;not null" json:"zone_name"`
	RecordKey        string `gorm:"uniqueIndex:idx_rec_uniq,priority:3;size:64;not null" json:"-"`
	ProviderRecordID string `gorm:"size:64" json:"provider_record_id"` // 厂商记录标识（AWS 为组合键编码）
	Name             string `gorm:"size:255" json:"name"`              // 主机记录：@ / www / 前缀
	Type             string `gorm:"size:16;index" json:"type"`
	Value            string `gorm:"type:text" json:"value"`
	TTL              int    `json:"ttl"`
	Priority         int    `json:"priority"` // MX/SRV
	Line             string `gorm:"size:32" json:"line"`
	Status           string `gorm:"size:32" json:"status"` // 厂商侧启用/暂停语义
	Remark           string `gorm:"size:255" json:"remark"`
	SyncedAt         time.Time `json:"synced_at"` // 本镜像条目同步时间
	CreatedAt        time.Time `json:"created_at"`
}

func (DnsRecord) TableName() string { return "dns_records" }
