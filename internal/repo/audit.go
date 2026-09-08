package repo

import (
	"github.com/domhub-io/domhub/internal/model"
	"gorm.io/gorm"
)

// AuditRepo 审计日志仓库。
type AuditRepo struct {
	db *gorm.DB
}

func NewAuditRepo(db *gorm.DB) *AuditRepo { return &AuditRepo{db: db} }

// Create 写入审计日志。
func (r *AuditRepo) Create(log *model.AuditLog) error {
	return r.db.Create(log).Error
}

// List 分页查询，可按 action / username / 关键字过滤。
func (r *AuditRepo) List(action, username, keyword string, page, pageSize int) ([]model.AuditLog, int64, error) {
	q := r.db.Model(&model.AuditLog{})
	if action != "" {
		q = q.Where("action LIKE ?", action+"%")
	}
	if username != "" {
		q = q.Where("username = ?", username)
	}
	if keyword != "" {
		q = q.Where("resource LIKE ? OR detail LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var list []model.AuditLog
	err := q.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&list).Error
	return list, total, err
}
