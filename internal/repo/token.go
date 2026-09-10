package repo

import (
	"time"

	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
)

// ApiTokenRepo API 令牌仓库。
type ApiTokenRepo struct {
	db *gorm.DB
}

func NewApiTokenRepo(db *gorm.DB) *ApiTokenRepo { return &ApiTokenRepo{db: db} }

// Create 写入新令牌。
func (r *ApiTokenRepo) Create(t *model.ApiToken) error { return r.db.Create(t).Error }

// Delete 删除令牌（吊销），仅限属主。
func (r *ApiTokenRepo) Delete(id, userID uint) error {
	return r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&model.ApiToken{}).Error
}

// ListByUser 用户自己的令牌列表。
func (r *ApiTokenRepo) ListByUser(userID uint) ([]model.ApiToken, error) {
	var list []model.ApiToken
	err := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&list).Error
	return list, err
}

// FindValidByHash 按哈希查有效令牌（未过期）。
func (r *ApiTokenRepo) FindValidByHash(hash string) (*model.ApiToken, error) {
	var t model.ApiToken
	err := r.db.Where("token_hash = ? AND (expire_at IS NULL OR expire_at > ?)", hash, time.Now()).
		First(&t).Error
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// Touch 更新最后使用时间（失败忽略，不影响请求）。
func (r *ApiTokenRepo) Touch(id uint) {
	now := time.Now()
	r.db.Model(&model.ApiToken{}).Where("id = ?", id).Update("last_used_at", &now)
}
