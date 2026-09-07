// Package bootstrap 应用启动编排：数据库初始化、迁移、种子数据。
package bootstrap

import (
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/glebarez/sqlite"
	"go.uber.org/zap"

	"github.com/domhub-io/domhub/internal/model"
	"github.com/domhub-io/domhub/internal/pkg/config"
	applog "github.com/domhub-io/domhub/internal/pkg/logger"
)

// InitDB 按配置初始化数据库连接并执行迁移。
func InitDB(cfg *config.Config) *gorm.DB {
	var (
		db  *gorm.DB
		err error
	)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	switch cfg.DB.Driver {
	case "mysql":
		db, err = gorm.Open(mysql.Open(cfg.DB.DSN), gormCfg)
	case "sqlite":
		// 开发/演示模式的轻量降级，生产环境建议 mysql
		db, err = gorm.Open(sqlite.Open(cfg.DB.DSN), gormCfg)
	default:
		panic(fmt.Sprintf("不支持的数据库驱动: %s", cfg.DB.Driver))
	}
	if err != nil {
		panic("数据库连接失败: " + err.Error())
	}

	if err := db.AutoMigrate(
		&model.User{},
		&model.CloudAccount{},
		&model.Domain{},
		&model.AlertChannel{},
		&model.AlertRule{},
		&model.AlertLog{},
		&model.SyncTask{},
	); err != nil {
		panic("数据库迁移失败: " + err.Error())
	}

	applog.L().Info("数据库初始化完成", zap.String("driver", cfg.DB.Driver))
	return db
}
