package config

import (
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config 全局配置。
type Config struct {
	Server struct {
		Port int    `mapstructure:"port"`
		Mode string `mapstructure:"mode"` // debug | release
	} `mapstructure:"server"`
	DB struct {
		Driver string `mapstructure:"driver"` // mysql | sqlite
		DSN    string `mapstructure:"dsn"`    // mysql DSN 或 sqlite 文件路径
	} `mapstructure:"db"`
	JWT struct {
		Secret      string `mapstructure:"secret"`
		ExpireHours int    `mapstructure:"expire_hours"`
	} `mapstructure:"jwt"`
	Admin struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"admin"`
}

// Load 读取配置：默认值 < config.yaml < 环境变量。
// 环境变量命名：DOMHUB_SERVER_PORT / DOMHUB_DB_DRIVER / DOMHUB_DB_DSN /
// DOMHUB_JWT_SECRET / DOMHUB_ADMIN_USERNAME / DOMHUB_ADMIN_PASSWORD
func Load() *Config {
	v := viper.New()

	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("db.driver", "sqlite")
	v.SetDefault("db.dsn", "domhub.db")
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.expire_hours", 24)
	v.SetDefault("admin.username", "admin")
	v.SetDefault("admin.password", "admin123")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	if err := v.ReadInConfig(); err != nil {
		// 配置文件可选：不存在时使用默认值 + 环境变量
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic("读取配置文件失败: " + err.Error())
		}
	}

	v.SetEnvPrefix("DOMHUB")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic("解析配置失败: " + err.Error())
	}

	// JWT secret 专项兜底：显式检查环境变量，避免 Unmarshal 拿不到嵌套 env
	if cfg.JWT.Secret == "" {
		if s := os.Getenv("DOMHUB_JWT_SECRET"); s != "" {
			cfg.JWT.Secret = s
		} else {
			// 开发兜底；生产环境必须显式配置
			cfg.JWT.Secret = "domhub-dev-secret-do-not-use-in-prod"
		}
	}

	return &cfg
}
