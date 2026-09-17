package repo

import (
	"github.com/onemore-coder/domhub/internal/model"
	"gorm.io/gorm"
)

// CertDeployRepo 证书部署目标存储。
type CertDeployRepo struct{ db *gorm.DB }

func NewCertDeployRepo(db *gorm.DB) *CertDeployRepo { return &CertDeployRepo{db: db} }

func (r *CertDeployRepo) FindByID(id uint) (*model.CertDeploy, error) {
	var d model.CertDeploy
	if err := r.db.First(&d, id).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

// ListByCert 某证书的全部部署目标。
func (r *CertDeployRepo) ListByCert(certID uint) ([]model.CertDeploy, error) {
	var out []model.CertDeploy
	err := r.db.Where("cert_id = ?", certID).Order("id asc").Find(&out).Error
	return out, err
}

func (r *CertDeployRepo) Create(d *model.CertDeploy) error { return r.db.Create(d).Error }

func (r *CertDeployRepo) Update(d *model.CertDeploy) error { return r.db.Save(d).Error }

func (r *CertDeployRepo) Delete(id uint) error { return r.db.Delete(&model.CertDeploy{}, id).Error }
