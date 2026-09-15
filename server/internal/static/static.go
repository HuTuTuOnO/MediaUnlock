// Package static 把前端构建产物嵌进 Server 二进制,单文件部署。
//
// 构建流程见 scripts/build.sh:先 `npm run build` 生成 web/dist,
// 再同步到 server/internal/static/web,最后 `go build`。
// web/ 整个目录是 gitignore 的(只留一个 .gitkeep 让目录存在),
// 所以仓库里不带构建产物;没构建前端时首页会给提示页,不会白屏。
package static

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// 只嵌 web/ 这一个子目录。用 all:* 会把本文件自己也嵌进去(GET /static.go 能拿到源码),
// 换成 * 也一样 —— * 只排除 . 和 _ 开头的文件,并不排除 .go。
//
//go:embed all:web
var embedded embed.FS

var staticFS fs.FS

func init() {
	// 剥掉 web/ 前缀,后面按 URL 路径直接查。目录一定存在(embed 指令已保证),不会失败。
	staticFS, _ = fs.Sub(embedded, "web")
}

// placeholder 前端还没构建时(web/ 里只有 .gitkeep)回给首页的提示。
const placeholder = `<!doctype html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <title>MediaUnlock</title>
    <style>
      body { font-family: system-ui, -apple-system, sans-serif; max-width: 40rem;
             margin: 6rem auto; padding: 0 1.5rem; line-height: 1.8; color: #18181b }
      code, pre { background: #f4f4f5; border-radius: .375rem; font-size: .875rem }
      code { padding: .125rem .375rem }
      pre { padding: 1rem; overflow-x: auto }
    </style>
  </head>
  <body>
    <h1>前端还没有构建</h1>
    <p>Server 会把前端产物嵌进二进制，但现在 <code>server/internal/static/web</code> 里只有占位文件。</p>
    <p>构建一次即可（脚本会构建前端、同步进去，再编译 Server 与 Agent）：</p>
    <pre>./scripts/build.sh</pre>
    <p>或者只构建前端：</p>
    <pre>cd web &amp;&amp; npm ci &amp;&amp; npm run build</pre>
  </body>
</html>
`

// Register 把前端静态资源挂到引擎:
//   - /api/* 已由业务路由处理,未命中则返回 JSON 404(不落到 SPA);
//   - 命中 static 内真实文件 → 直接返回(带正确 Content-Type / 缓存头);
//   - 其余 GET → 回退 index.html,交给前端路由;
//   - 连 index.html 都没有(还没构建前端)→ 返回提示页。
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
			if _, err := fs.Stat(staticFS, name); err != nil {
				c.Header("Cache-Control", "no-cache")
				c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(placeholder))
				return
			}
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
