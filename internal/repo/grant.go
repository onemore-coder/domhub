package repo

import (
	"github.com/onemore-coder/domhub/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GrantRepo Zone 授权数据访问。
type GrantRepo struct {
	db *gorm.DB
}

func NewGrantRepo(db *gorm.DB) *GrantRepo { return &GrantRepo{db: db} }

// ZonesByUser 查询用户的全部授权。
func (r *GrantRepo) ZonesByUser(userID uint) ([]model.UserZone, error) {
	var list []model.UserZone
	err := r.db.Where("user_id = ?", userID).Order("cloud_account_id, zone").Find(&list).Error
	return list, err
}

// Exists 判断用户是否被授权某 账号+Zone。
func (r *GrantRepo) Exists(userID uint, accountID uint, zone string) (bool, error) {
	var n int64
	err := r.db.Model(&model.UserZone{}).
		Where("user_id = ? AND cloud_account_id = ? AND zone = ?", userID, accountID, zone).
		Count(&n).Error
	return n > 0, err
}

// AccountsByUser 用户被授权的账号 ID 集合。
func (r *GrantRepo) AccountsByUser(userID uint) (map[uint]bool, error) {
	var list []model.UserZone
	err := r.db.Where("user_id = ?", userID).Find(&list).Error
	if err != nil {
		return nil, err
	}
	m := make(map[uint]bool, len(list))
	for _, z := range list {
		m[z.CloudAccountID] = true
	}
	return m, nil
}

// ZonesByUserAndAccount 用户在指定账号下的授权 Zone 集合。
func (r *GrantRepo) ZonesByUserAndAccount(userID, accountID uint) (map[string]bool, error) {
	var list []model.UserZone
	err := r.db.Where("user_id = ? AND cloud_account_id = ?", userID, accountID).Find(&list).Error
	if err != nil {
		return nil, err
	}
	m := make(map[string]bool, len(list))
	for _, z := range list {
		m[z.Zone] = true
	}
	return m, nil
}

// ReplaceByUser 全量替换用户的授权列表。
func (r *GrantRepo) ReplaceByUser(userID uint, zones []model.UserZone) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserZone{}).Error; err != nil {
			return err
		}
		if len(zones) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&zones).Error
	})
}
