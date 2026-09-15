// Package static 把前端构建产物嵌进 Server 二进制,单文件部署。
//
// 构建流程见 scripts/build.sh:先 `npm run build` 生成 web/dist,
// 再同步到 server/internal/static,最后 `go build`。仓库里保留一份占位
// index.html,保证未构建前端时 `go build` 依然可用(访问首页给出提示)。
package static

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed all:*
var staticFS embed.FS

// Register 把前端静态资源挂到引擎:
//   - /api/* 已由业务路由处理,未命中则返回 JSON 404(不落到 SPA);
//   - 命中 static 内真实文件 → 直接返回(带正确 Content-Type / 缓存头);
//   - 其余 GET → 回退 index.html,交给前端路由(刷新 /nodes 等不 404)。
func Register(r *gin.Engine) {
	r.NoRoute(func(c *gin.Context) {
		req := c.Request
		if strings.HasPrefix(req.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"code": http.StatusNotFound, "msg": "not found"})
			return
		}
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}

		name := strings.TrimPrefix(path.Clean(req.URL.Path), "/")
		if name == "" {
			name = "index.html"
		}
		if _, statErr := fs.Stat(staticFS, name); statErr != nil {
			// /assets/ 下是构建产物(文件名带内容 hash),不存在就是真的没有。
			// 若回退 index.html,浏览器拿到 text/html 会报 MIME 错误而不是 404,极难排查。
			if strings.HasPrefix(name, "assets/") {
				c.Status(http.StatusNotFound)
				return
			}
			name = "index.html" // SPA 回退
		}
		// 带内容 hash 的构建产物可以长期强缓存;index.html 不能缓存,
		// 否则发版后用户仍拿到旧壳、引用已被替换掉的 assets 文件名。
		if strings.HasPrefix(name, "assets/") {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "no-cache")
		}
		http.ServeFileFS(c.Writer, req, staticFS, name)
	})
}
