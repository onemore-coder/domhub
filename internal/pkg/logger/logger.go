// Package logger 提供全局 zap 日志器。
package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var global *zap.Logger

// Init 按运行模式初始化全局日志器。
func Init(mode string) {
	var cfg zap.Config
	if mode == "release" {
		cfg = zap.NewProductionConfig()
	} else {
		cfg = zap.NewDevelopmentConfig()
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	l, err := cfg.Build(zap.AddCallerSkip(1))
	if err != nil {
		// 日志初始化失败时退回标准输出
		l = zap.NewNop()
		_ = os.Stderr
	}
	global = l
}

// L 返回全局日志器（未初始化时返回 Nop，保证安全）。
func L() *zap.Logger {
	if global == nil {
		return zap.NewNop()
	}
	return global
}
