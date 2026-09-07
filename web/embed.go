package web

import "embed"

// Dist 内嵌前端构建产物（web/dist）。
// 构建后端前请先执行 `cd web && npm run build`；
// dist 目录不存在时需先创建（CI/Docker 构建流程会自动处理）。
//
//go:embed all:dist
var Dist embed.FS

// Static 返回剔除 dist 前缀的文件系统，供路由直接使用。
func Static() (embed.FS, error) {
	return Dist, nil
}
