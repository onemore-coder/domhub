package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// serveStatic 托管内嵌的前端资源；未命中文件时回退 index.html（SPA 路由）。
func serveStatic(c *gin.Context, root fs.FS) {
	urlPath := path.Clean(c.Request.URL.Path)
	if urlPath == "." || urlPath == "/" {
		urlPath = "index.html"
	}
	// 去掉开头的 /
	rel := strings.TrimPrefix(urlPath, "/")

	if f, err := root.Open(rel); err == nil {
		stat, _ := f.Stat()
		_ = f.Close()
		if stat != nil && !stat.IsDir() {
			c.FileFromFS(rel, http.FS(root))
			return
		}
	}

	// 回退 index.html
	index := "index.html"
	if _, err := fs.Stat(root, index); err != nil {
		c.String(http.StatusNotFound, "frontend assets not built")
		return
	}
	c.FileFromFS(index, http.FS(root))
}
