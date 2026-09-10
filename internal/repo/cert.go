package repo

import (
	"github.com/domhub-io/domhub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CertRepo struct{ db *gorm.DB }

func NewCertRepo(db *gorm.DB) *CertRepo { return &CertRepo{db: db} }

// Upsert 按主机名 upsert 证书状态。
func (r *CertRepo) Upsert(cs *model.CertStatus) error {
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "host"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"domain_id", "domain_name", "source", "excluded",
			"not_after", "issuer", "subject", "days_left", "ok", "error",
			"alerted_offsets", "checked_at",
		}),
	}).Create(cs).Error
}

// List 全量列表（按剩余天数升序，失败项 -1 排在前，便于先看到问题）。
func (r *CertRepo) List() ([]model.CertStatus, error) {
	var list []model.CertStatus
	err := r.db.Order("ok DESC, days_left ASC, host ASC").Find(&list).Error
	return list, err
}

func (r *CertRepo) FindByHost(host string) (*model.CertStatus, error) {
	var cs model.CertStatus
	err := r.db.Where("host = ?", host).First(&cs).Error
	if err != nil {
		return nil, err
	}
	return &cs, nil
}

// ListManual 手动添加的监控主机（不参与自动发现，批量检查需单独并入探测队列）。
func (r *CertRepo) ListManual() ([]model.CertStatus, error) {
	var list []model.CertStatus
	err := r.db.Where("source = ?", "manual").Find(&list).Error
	return list, err
}

func (r *CertRepo) FindByID(id uint) (*model.CertStatus, error) {
	var cs model.CertStatus
	err := r.db.First(&cs, id).Error
	if err != nil {
		return nil, err
	}
	return &cs, nil
}

// DeleteUnseenAuto 删除本轮探测中未再发现、且未被排除的自动发现条目。
func (r *CertRepo) DeleteUnseenAuto(seenHosts []string) (int64, error) {
	q := r.db.Where("source = ? AND excluded = ?", "auto", false)
	if len(seenHosts) > 0 {
		q = q.Where("host NOT IN ?", seenHosts)
	} else {
		q = q.Where("1 = 1")
	}
	res := q.Delete(&model.CertStatus{})
	return res.RowsAffected, res.Error
}

func (r *CertRepo) Delete(id uint) error {
	return r.db.Delete(&model.CertStatus{}, id).Error
}

// SetExcluded 设置/取消排除标志。排除后不探测、不告警，保留人工决策。
func (r *CertRepo) SetExcluded(id uint, excluded bool) error {
	return r.db.Model(&model.CertStatus{}).Where("id = ?", id).
		Updates(map[string]any{"excluded": excluded}).Error
}
