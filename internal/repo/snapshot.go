package repo

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
	"github.com/onemore-coder/domhub/internal/provider"
)

// SnapshotRepo 解析记录快照仓库。
type SnapshotRepo struct {
	db *gorm.DB
}

func NewSnapshotRepo(db *gorm.DB) *SnapshotRepo {
	return &SnapshotRepo{db: db}
}

// Save 保存一份快照。
func (r *SnapshotRepo) Save(s *model.DNSRecordSnapshot) error {
	return r.db.Create(s).Error
}

// List 按 Zone 倒序列出快照（不含记录正文）。
func (r *SnapshotRepo) List(accountID uint, zone string, limit int) ([]model.DNSRecordSnapshot, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var list []model.DNSRecordSnapshot
	err := r.db.Where("account_id = ? AND zone = ?", accountID, zone).
		Order("id DESC").Limit(limit).Find(&list).Error
	return list, err
}

// FindByID 取单份快照（含记录正文）。
func (r *SnapshotRepo) FindByID(id uint) (*model.DNSRecordSnapshot, error) {
	var s model.DNSRecordSnapshot
	if err := r.db.First(&s, id).Error; err != nil {
		return nil, err
	}
	return &s, nil
}

// DistinctZones 列出已有快照的 Zone（用于漂移检测遍历）。
func (r *SnapshotRepo) DistinctZones() ([]model.DNSRecordSnapshot, error) {
	var out []model.DNSRecordSnapshot
	err := r.db.Model(&model.DNSRecordSnapshot{}).
		Select("DISTINCT account_id, zone").
		Find(&out).Error
	return out, err
}

// Latest 取 Zone 最新一份快照。
func (r *SnapshotRepo) Latest(accountID uint, zone string) (*model.DNSRecordSnapshot, error) {
	var s model.DNSRecordSnapshot
	err := r.db.Where("account_id = ? AND zone = ?", accountID, zone).
		Order("id DESC").First(&s).Error
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Delete 清理快照（保留最近 keep 份，返回删除数量）。
func (r *SnapshotRepo) Delete(accountID uint, zone string, keep int) (int64, error) {
	var ids []uint
	if err := r.db.Model(&model.DNSRecordSnapshot{}).
		Where("account_id = ? AND zone = ?", accountID, zone).
		Order("id DESC").Offset(keep).Limit(500).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.Where("id IN ?", ids).Delete(&model.DNSRecordSnapshot{})
	return res.RowsAffected, res.Error
}

// DecodeRecords 解析快照正文为记录列表。
func DecodeRecords(raw string) []provider.RecordInfo {
	var out []provider.RecordInfo
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// EncodeRecords 序列化记录列表。
func EncodeRecords(records []provider.RecordInfo) string {
	b, err := json.Marshal(records)
	if err != nil {
		return "[]"
	}
	return string(b)
}

// SettingRepo 系统 KV 设置仓库。
type SettingRepo struct {
	db *gorm.DB
}

func NewSettingRepo(db *gorm.DB) *SettingRepo {
	return &SettingRepo{db: db}
}

// Get 取设置值，不存在返回空串。
func (r *SettingRepo) Get(key string) (string, error) {
	var s model.SystemSetting
	if err := r.db.Where("`key` = ?", key).First(&s).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", err
	}
	return s.Value, nil
}

// Set 写入设置（upsert）。
func (r *SettingRepo) Set(key, value string) error {
	return r.db.Where("`key` = ?", key).
		Assign(map[string]any{"value": value}).
		FirstOrCreate(&model.SystemSetting{Key: key, Value: value}).Error
}

// All 取全部设置。
func (r *SettingRepo) All() ([]model.SystemSetting, error) {
	var list []model.SystemSetting
	err := r.db.Order("`key` ASC").Find(&list).Error
	return list, err
}
