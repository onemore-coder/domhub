package repo

import (
	"time"

	"github.com/onemore-coder/domhub/internal/model"
	"gorm.io/gorm"
)

// AcmeAccountRepo ACME 账户存储。
type AcmeAccountRepo struct{ db *gorm.DB }

func NewAcmeAccountRepo(db *gorm.DB) *AcmeAccountRepo { return &AcmeAccountRepo{db: db} }

// FindByDirAndEmail 按目录+邮箱定位账户（不存在返回 gorm.ErrRecordNotFound）。
func (r *AcmeAccountRepo) FindByDirAndEmail(dir, email string) (*model.AcmeAccount, error) {
	var a model.AcmeAccount
	err := r.db.Where("directory_url = ? AND email = ?", dir, email).First(&a).Error
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *AcmeAccountRepo) Create(a *model.AcmeAccount) error { return r.db.Create(a).Error }

func (r *AcmeAccountRepo) Update(a *model.AcmeAccount) error { return r.db.Save(a).Error }

// IssuedCertRepo 已签发/申请中证书存储。
type IssuedCertRepo struct{ db *gorm.DB }

func NewIssuedCertRepo(db *gorm.DB) *IssuedCertRepo { return &IssuedCertRepo{db: db} }

func (r *IssuedCertRepo) FindByID(id uint) (*model.IssuedCert, error) {
	var c model.IssuedCert
	if err := r.db.First(&c, id).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

// List 按 status 过滤（空为全部），主域名升序。
func (r *IssuedCertRepo) List(status string) ([]model.IssuedCert, error) {
	q := r.db.Order("primary_domain asc, id asc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var out []model.IssuedCert
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// DueForRenewal 需要自动续期的证书：开启自动续期、已签发、withinDays 天内到期。
func (r *IssuedCertRepo) DueForRenewal(withinDays int) ([]model.IssuedCert, error) {
	var out []model.IssuedCert
	err := r.db.Where(
		"auto_renew = ? AND status = ? AND not_after IS NOT NULL AND not_after <= ?",
		true, model.CertApplyIssued, time.Now().AddDate(0, 0, withinDays),
	).Find(&out).Error
	return out, err
}

func (r *IssuedCertRepo) Create(c *model.IssuedCert) error { return r.db.Create(c).Error }

// Update 全量保存（含进度日志等大字段）。
func (r *IssuedCertRepo) Update(c *model.IssuedCert) error { return r.db.Save(c).Error }

// Delete 删除记录。
func (r *IssuedCertRepo) Delete(id uint) error { return r.db.Delete(&model.IssuedCert{}, id).Error }
