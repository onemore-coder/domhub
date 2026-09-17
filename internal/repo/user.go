// Package repo 数据访问层。
package repo

import (
	"errors"

	"gorm.io/gorm"

	"github.com/onemore-coder/domhub/internal/model"
)

// UserRepo 用户数据访问。
type UserRepo struct {
	db *gorm.DB
}

func NewUserRepo(db *gorm.DB) *UserRepo { return &UserRepo{db: db} }

func (r *UserRepo) FindByUsername(username string) (*model.User, error) {
	var u model.User
	err := r.db.Where("username = ?", username).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UserRepo) FindByID(id uint) (*model.User, error) {
	var u model.User
	err := r.db.First(&u, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *UserRepo) Update(u *model.User) error {
	return r.db.Save(u).Error
}

// List 全部用户（按 ID 升序）。
func (r *UserRepo) List() ([]model.User, error) {
	var list []model.User
	err := r.db.Order("id ASC").Find(&list).Error
	return list, err
}

// Create 创建用户。
func (r *UserRepo) Create(u *model.User) error {
	return r.db.Create(u).Error
}

// Delete 删除用户及其授权。
func (r *UserRepo) Delete(id uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&model.UserZone{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, id).Error
	})
}

// CountAdmins 统计启用的 admin 数量（防止删掉最后一个管理员）。
func (r *UserRepo) CountAdmins() (int64, error) {
	var n int64
	err := r.db.Model(&model.User{}).Where("role = ? AND status = 1", model.RoleAdmin).Count(&n).Error
	return n, err
}
