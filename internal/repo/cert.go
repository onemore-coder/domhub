package repo

import (
	"github.com/domhub-io/domhub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CertRepo struct{ db *gorm.DB }

func NewCertRepo(db *gorm.DB) *CertRepo { return &CertRepo{db: db} }

// UpsertInsert 按域名 upsert 证书状态。
func (r *CertRepo) Upsert(cs *model.CertStatus) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "domain_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "not_after", "issuer", "subject", "days_left", "ok", "error", "alerted_offsets", "checked_at"}),
	}).Create(cs).Error
}

// UpsertKeepAlerted 同上，但保留已有 alerted_offsets（供探测更新时使用）。
func (r *CertRepo) UpsertKeepAlerted(cs *model.CertStatus, alerted string) error {
	cs.AlertedOffsets = alerted
	return r.Upsert(cs)
}

func (r *CertRepo) List() ([]model.CertStatus, error) {
	var list []model.CertStatus
	err := r.db.Order("days_left ASC").Find(&list).Error
	return list, err
}

func (r *CertRepo) FindByDomainID(domainID uint) (*model.CertStatus, error) {
	var cs model.CertStatus
	err := r.db.Where("domain_id = ?", domainID).First(&cs).Error
	if err != nil {
		return nil, err
	}
	return &cs, nil
}
