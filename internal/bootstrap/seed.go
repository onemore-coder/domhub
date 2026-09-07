package bootstrap

import (
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/config"
	"github.com/domhub-io/domhub/internal/pkg/logger"
	"github.com/domhub-io/domhub/internal/repo"
)

// Seed 初始数据：确保管理员账号存在（仅当用户表为空时创建）。
func Seed(db *gorm.DB, cfg *config.Config) {
	users := repo.NewUserRepo(db)

	u, err := users.FindByUsername(cfg.Admin.Username)
	if err != nil {
		panic("查询管理员账号失败: " + err.Error())
	}
	if u != nil {
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(cfg.Admin.Password), bcrypt.DefaultCost)
	if err != nil {
		panic("生成密码哈希失败: " + err.Error())
	}

	admin := &model.User{
		Username:     cfg.Admin.Username,
		PasswordHash: string(hash),
		Role:         "admin",
		Status:       1,
	}
	if err := db.Create(admin).Error; err != nil {
		panic("创建管理员账号失败: " + err.Error())
	}
	logger.L().Info("已创建初始管理员账号", zap.String("username", admin.Username))
}
