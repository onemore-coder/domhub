package config

import (
	"fmt"
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
	Crypto struct {
		Key string `mapstructure:"key"` // 云凭证加密密钥；默认取 DOMHUB_CRYPTO_KEY，再退回 jwt.secret 派生
	} `mapstructure:"crypto"`
	Job struct {
		CheckCron string `mapstructure:"check_cron"` // 到期检查，cron 表达式（带秒位），空 = 关闭
		SyncCron  string `mapstructure:"sync_cron"`  // 台账定时同步，空 = 关闭
	} `mapstructure:"job"`
	Admin struct {
		Username string `mapstructure:"username"`
		Password string `mapstructure:"password"`
	} `mapstructure:"admin"`
	OAuth struct {
		GitHub struct {
			Enabled      bool   `mapstructure:"enabled"`
			ClientID     string `mapstructure:"client_id"`
			ClientSecret string `mapstructure:"client_secret"`
		} `mapstructure:"github"`
	} `mapstructure:"oauth"`
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
	v.SetDefault("crypto.key", "")
	v.SetDefault("job.check_cron", "0 0 9 * * *") // 每天 09:00
	v.SetDefault("job.sync_cron", "")             // 默认关闭
	v.SetDefault("admin.username", "admin")
	v.SetDefault("admin.password", "admin123")

	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件缺失时，仅当 DB 至少一项通过环境变量显式指定才允许启动
			// （Docker / 环境变量部署场景）；否则拒绝启动，避免从错误的工作目录
			// 启动时静默回退到 sqlite 空库（曾导致"数据全没了"的假象）。
			if os.Getenv("DOMHUB_DB_DRIVER") == "" && os.Getenv("DOMHUB_DB_DSN") == "" {
				cwd, _ := os.Getwd()
				panic(fmt.Sprintf(
					"未找到配置文件 config.yaml（已在 %s 与其 ./config 子目录下查找，当前工作目录: %s）。"+
						"请在项目目录启动，或显式设置 DOMHUB_DB_DRIVER / DOMHUB_DB_DSN 以环境变量模式运行。",
					cwd, cwd))
			}
		} else {
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

	// 云凭证加密密钥：crypto.key → DOMHUB_CRYPTO_KEY → jwt.secret
	if cfg.Crypto.Key == "" {
		cfg.Crypto.Key = os.Getenv("DOMHUB_CRYPTO_KEY")
	}
	if cfg.Crypto.Key == "" {
		cfg.Crypto.Key = cfg.JWT.Secret
	}

	return &cfg
}
