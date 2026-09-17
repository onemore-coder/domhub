// Package version 提供应用版本号。
// 默认 "dev"；构建时通过 ldflags 注入，例如：
//
//	go build -ldflags "-X github.com/onemore-coder/domhub/internal/pkg/version.Version=$(git describe --tags --always)" ./cmd/server
package version

// Version 应用版本号，构建时由 ldflags 注入。
var Version = "dev"
