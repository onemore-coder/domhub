package repo

import (
	"time"

	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
)

// CloudAccountRepo 云账号数据访问。
type CloudAccountRepo struct {
	db *gorm.DB
}

func NewCloudAccountRepo(db *gorm.DB) *CloudAccountRepo { return &CloudAccountRepo{db: db} }

func (r *CloudAccountRepo) Create(a *model.CloudAccount) error { return r.db.Create(a).Error }

func (r *CloudAccountRepo) Update(a *model.CloudAccount) error { return r.db.Save(a).Error }

func (r *CloudAccountRepo) Delete(id uint) error { return r.db.Delete(&model.CloudAccount{}, id).Error }

func (r *CloudAccountRepo) FindByID(id uint) (*model.CloudAccount, error) {
	var a model.CloudAccount
	err := r.db.First(&a, id).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *CloudAccountRepo) List() ([]model.CloudAccount, error) {
	var list []model.CloudAccount
	err := r.db.Order("id").Find(&list).Error
	return list, err
}

// Count 统计账号数。
func (r *CloudAccountRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.CloudAccount{}).Count(&n).Error
	return n, err
}

// DomainRepo 域名台账数据访问。
type DomainRepo struct {
	db *gorm.DB
}

func NewDomainRepo(db *gorm.DB) *DomainRepo { return &DomainRepo{db: db} }

// UpsertBatch 批量 upsert：以 (cloud_account_id, name, kind) 为唯一键。
func (r *DomainRepo) UpsertBatch(items []model.Domain) error {
	if len(items) == 0 {
		return nil
	}
	now := time.Now()
	for i := range items {
		items[i].LastSyncedAt = now
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for i := range items {
			var existing model.Domain
			err := tx.Where("cloud_account_id = ? AND name = ? AND kind = ?",
				items[i].CloudAccountID, items[i].Name, items[i].Kind).First(&existing).Error
			if err == gorm.ErrRecordNotFound {
				if err := tx.Create(&items[i]).Error; err != nil {
					return err
				}
				continue
			}
			if err != nil {
				return err
			}
			// 更新远端字段，保留本地编辑字段（tags/remark）
			if err := tx.Model(&existing).Updates(map[string]any{
				"provider":       items[i].Provider,
				"registrar":      items[i].Registrar,
				"status":         items[i].Status,
				"registered_at":  items[i].RegisteredAt,
				"expire_at":      items[i].ExpireAt,
				"last_synced_at": items[i].LastSyncedAt,
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DomainFilter 域名查询条件。
type DomainFilter struct {
	Keyword        string
	Provider       string
	CloudAccountID uint
	Kind           string
	Tag            string
	ExpiringDays   int // 只看 N 天内到期的（0 = 不过滤）
	Page           int
	PageSize       int
}

// List 按条件查询 + 分页。
func (r *DomainRepo) List(f DomainFilter) ([]model.Domain, int64, error) {
	q := r.db.Model(&model.Domain{})
	if f.Keyword != "" {
		like := "%" + f.Keyword + "%"
		q = q.Where("name LIKE ? OR remark LIKE ?", like, like)
	}
	if f.Provider != "" {
		q = q.Where("provider = ?", f.Provider)
	}
	if f.CloudAccountID > 0 {
		q = q.Where("cloud_account_id = ?", f.CloudAccountID)
	}
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	if f.Tag != "" {
		q = q.Where("tags LIKE ?", "%"+f.Tag+"%")
	}
	if f.ExpiringDays > 0 {
		deadline := time.Now().AddDate(0, 0, f.ExpiringDays)
		q = q.Where("expire_at IS NOT NULL AND expire_at <= ?", deadline)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if f.Page <= 0 {
		f.Page = 1
	}
	if f.PageSize <= 0 || f.PageSize > 200 {
		f.PageSize = 20
	}
	var list []model.Domain
	err := q.Order("expire_at IS NULL, expire_at ASC").Order("id DESC").
		Offset((f.Page - 1) * f.PageSize).Limit(f.PageSize).Find(&list).Error
	return list, total, err
}

// Count 统计域名数。
func (r *DomainRepo) Count() (int64, error) {
	var n int64
	err := r.db.Model(&model.Domain{}).Count(&n).Error
	return n, err
}

// UpdateMeta 更新本地维护字段（标签/备注），不影响云同步。
func (r *DomainRepo) UpdateMeta(id uint, tags, remark *string) error {
	updates := map[string]any{}
	if tags != nil {
		updates["tags"] = *tags
	}
	if remark != nil {
		updates["remark"] = *remark
	}
	if len(updates) == 0 {
		return nil
	}
	return r.db.Model(&model.Domain{}).Where("id = ?", id).Updates(updates).Error
}

// CountByKind 按类型统计（domain | zone）。
func (r *DomainRepo) CountByKind(kind string) (int64, error) {
	var n int64
	err := r.db.Model(&model.Domain{}).Where("kind = ?", kind).Count(&n).Error
	return n, err
}

// CountExpiringIn 统计 N 天内到期的注册域名数。
func (r *DomainRepo) CountExpiringIn(days int) (int64, error) {
	var n int64
	deadline := time.Now().AddDate(0, 0, days)
	err := r.db.Model(&model.Domain{}).
		Where("kind = ? AND expire_at IS NOT NULL AND expire_at <= ?", "domain", deadline).
		Count(&n).Error
	return n, err
}

// ProviderCount 厂商分布统计项。
type ProviderCount struct {
	Provider string `json:"provider"`
	Count    int64  `json:"count"`
}

// CountByProvider 台账按厂商分布（仪表盘）。
func (r *DomainRepo) CountByProvider() ([]ProviderCount, error) {
	var out []ProviderCount
	err := r.db.Model(&model.Domain{}).
		Select("provider, COUNT(*) AS count").
		Group("provider").Order("count DESC").
		Scan(&out).Error
	return out, err
}

// DayCount 逐日计数项（到期时间线）。
type DayCount struct {
	Date  string `json:"date"` // YYYY-MM-DD
	Count int64  `json:"count"`
}

// ExpiryTimeline 未来 N 天逐日到期数（无到期的日期补零，前端可直接渲染）。
func (r *DomainRepo) ExpiryTimeline(days int) ([]DayCount, error) {
	today := time.Now().Truncate(24 * time.Hour)
	deadline := today.AddDate(0, 0, days)
	var rows []DayCount
	err := r.db.Model(&model.Domain{}).
		Select("DATE(expire_at) AS date, COUNT(*) AS count").
		Where("kind = ? AND expire_at IS NOT NULL AND expire_at >= ? AND expire_at < ?", "domain", today, deadline).
		Group("DATE(expire_at)").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	byDate := map[string]int64{}
	for _, r := range rows {
		byDate[r.Date] = r.Count
	}
	out := make([]DayCount, 0, days)
	for i := 0; i < days; i++ {
		d := today.AddDate(0, 0, i).Format("2006-01-02")
		out = append(out, DayCount{Date: d, Count: byDate[d]})
	}
	return out, nil
}

// ListWithExpiry 查询全部有到期时间的注册域名（供到期检查任务用）。
func (r *DomainRepo) ListWithExpiry() ([]model.Domain, error) {
	var list []model.Domain
	err := r.db.Where("kind = ? AND expire_at IS NOT NULL", "domain").Find(&list).Error
	return list, err
}

// ListAllByKind 按类型查询全部域名/Zone（供证书监控做主机名发现）。
func (r *DomainRepo) ListAllByKind(kind string) ([]model.Domain, error) {
	var list []model.Domain
	err := r.db.Where("kind = ?", kind).Find(&list).Error
	return list, err
}

// FindByID 按主键查询单个域名/Zone。
func (r *DomainRepo) FindByID(id uint) (*model.Domain, error) {
	var d model.Domain
	err := r.db.First(&d, id).Error
	if err != nil {
		return nil, err
	}
	return &d, nil
}

// DeleteByAccount 删除某账号下的全部域名。
func (r *DomainRepo) DeleteByAccount(accountID uint) error {
	return r.db.Where("cloud_account_id = ?", accountID).Delete(&model.Domain{}).Error
}

// SyncTaskRepo 同步任务数据访问。
type SyncTaskRepo struct {
	db *gorm.DB
}

func NewSyncTaskRepo(db *gorm.DB) *SyncTaskRepo { return &SyncTaskRepo{db: db} }

func (r *SyncTaskRepo) Create(t *model.SyncTask) error { return r.db.Create(t).Error }

func (r *SyncTaskRepo) Finish(t *model.SyncTask) error { return r.db.Save(t).Error }
