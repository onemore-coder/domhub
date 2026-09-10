package repo

import (
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
)

// DnsRecordRepo 解析记录本地镜像仓库。
type DnsRecordRepo struct{ db *gorm.DB }

func NewDnsRecordRepo(db *gorm.DB) *DnsRecordRepo { return &DnsRecordRepo{db: db} }

// ReplaceAllFor 整体替换某 Zone 的镜像记录（事务内先删后插）。
// 镜像语义是「当前快照」，不存在需要保留历史行号的场景，整体替换最简单可靠。
func (r *DnsRecordRepo) ReplaceAllFor(accountID uint, zone string, recs []model.DnsRecord) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("cloud_account_id = ? AND zone_name = ?", accountID, zone).
			Delete(&model.DnsRecord{}).Error; err != nil {
			return err
		}
		if len(recs) == 0 {
			return nil
		}
		return tx.CreateInBatches(recs, 200).Error
	})
}

// ListForZone 某 Zone 的镜像记录（按类型、主机排序）。
func (r *DnsRecordRepo) ListForZone(accountID uint, zone string) ([]model.DnsRecord, error) {
	var list []model.DnsRecord
	err := r.db.Where("cloud_account_id = ? AND zone_name = ?", accountID, zone).
		Order("type ASC, name ASC, id ASC").Find(&list).Error
	return list, err
}

// ZoneRef 跨 Zone 查询的授权范围单元。
type ZoneRef struct {
	CloudAccountID uint
	Zone           string
}

// ListFiltered 跨 Zone 镜像查询：账号、Zone 范围、关键字（主机/记录值）、类型。
// zoneRefs 为空时返回空结果（非 admin 无任何授权的场景）。
func (r *DnsRecordRepo) ListFiltered(accountID uint, zoneRefs []ZoneRef, q, rtype string, limit int) ([]model.DnsRecord, error) {
	if limit <= 0 || limit > 2000 {
		limit = 500
	}
	tx := r.db.Model(&model.DnsRecord{})
	if accountID > 0 {
		tx = tx.Where("cloud_account_id = ?", accountID)
	}
	if len(zoneRefs) == 0 && accountID == 0 {
		return []model.DnsRecord{}, nil
	}
	if len(zoneRefs) > 0 {
		// (account, zone) 授权范围过滤
		where := ""
		args := make([]any, 0, len(zoneRefs)*2)
		for i, zr := range zoneRefs {
			if i > 0 {
				where += " OR "
			}
			where += "(cloud_account_id = ? AND zone_name = ?)"
			args = append(args, zr.CloudAccountID, zr.Zone)
		}
		tx = tx.Where("("+where+")", args...)
	}
	if q != "" {
		like := "%" + q + "%"
		tx = tx.Where("name LIKE ? OR value LIKE ?", like, like)
	}
	if rtype != "" {
		tx = tx.Where("type = ?", rtype)
	}
	var list []model.DnsRecord
	err := tx.Order("zone_name ASC, type ASC, name ASC").Limit(limit).Find(&list).Error
	return list, err
}

// CertHostRow 证书监控主机发现行。
type CertHostRow struct {
	CloudAccountID uint
	ZoneName       string
	Name           string
}

// CertHostRows 提供证书监控的主机发现源：A/AAAA/CNAME 记录的主机部分。
func (r *DnsRecordRepo) CertHostRows() ([]CertHostRow, error) {
	var rows []CertHostRow
	err := r.db.Model(&model.DnsRecord{}).
		Select("DISTINCT cloud_account_id, zone_name, name").
		Where("type IN ?", []string{"A", "AAAA", "CNAME"}).
		Order("zone_name ASC, name ASC").
		Scan(&rows).Error
	return rows, err
}
