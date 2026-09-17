package repo

import (
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
)

// RecordTemplateRepo 解析记录模板仓库。
type RecordTemplateRepo struct{ db *gorm.DB }

func NewRecordTemplateRepo(db *gorm.DB) *RecordTemplateRepo { return &RecordTemplateRepo{db: db} }

func (r *RecordTemplateRepo) List() ([]model.RecordTemplate, error) {
	var list []model.RecordTemplate
	err := r.db.Order("id ASC").Find(&list).Error
	return list, err
}

func (r *RecordTemplateRepo) FindByID(id uint) (*model.RecordTemplate, error) {
	var t model.RecordTemplate
	if err := r.db.First(&t, id).Error; err != nil {
		return nil, fmt.Errorf("模板不存在")
	}
	return &t, nil
}

func (r *RecordTemplateRepo) Create(t *model.RecordTemplate) error { return r.db.Create(t).Error }

func (r *RecordTemplateRepo) Update(t *model.RecordTemplate) error { return r.db.Save(t).Error }

func (r *RecordTemplateRepo) Delete(id uint) error {
	res := r.db.Delete(&model.RecordTemplate{}, id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("模板不存在")
	}
	return nil
}

// ParseItems 解析模板条目 JSON。
func ParseItems(raw string) ([]model.TemplateItem, error) {
	var items []model.TemplateItem
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return nil, fmt.Errorf("模板条目格式错误: %w", err)
	}
	return items, nil
}
