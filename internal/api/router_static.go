package api

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// serveStatic 托管内嵌的前端资源；未命中文件时回退 index.html（SPA 路由）。
// 注意：不用 c.FileFromFS，因为它底层是 http.FileServer，
// 对 index.html 会触发 "…/index.html → …/" 的 301 重定向，导致首页循环重定向。
func serveStatic(c *gin.Context, root fs.FS) {
	urlPath := path.Clean(c.Request.URL.Path)
	rel := strings.TrimPrefix(urlPath, "/")
	if rel == "" || rel == "." {
		rel = "index.html"
	}

	// 命中真实文件
	if f, err := root.Open(rel); err == nil {
		stat, statErr := f.Stat()
		if statErr == nil && !stat.IsDir() {
			data, readErr := fs.ReadFile(root, rel)
			_ = f.Close()
			if readErr == nil {
				c.Data(http.StatusOK, mime.TypeByExtension(path.Ext(rel)), data)
				return
			}
		}
		_ = f.Close()
	}

	// SPA 回退：未命中的一律返回 index.html
	data, err := fs.ReadFile(root, "index.html")
	if err != nil {
		c.String(http.StatusNotFound, "frontend assets not built")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}
