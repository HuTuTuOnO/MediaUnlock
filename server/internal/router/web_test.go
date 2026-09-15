package router

import (
	"net/http"
	"strings"
	"testing"
)

// 前端托管:首页返回嵌入的 index.html。
func TestWebIndex(t *testing.T) {
	r, _, _ := setup(t)
	w := do(t, r, http.MethodGet, "/", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("want html content-type, got %q", ct)
	}
	if !strings.Contains(w.Body.String(), `id="root"`) {
		t.Fatalf("index.html not served: %s", w.Body.String())
	}
}

// SPA 回退:未知的前端路由(非 /api、非真实文件)返回 index.html,不 404。
func TestWebSPAFallback(t *testing.T) {
	r, _, _ := setup(t)
	w := do(t, r, http.MethodGet, "/nodes", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 (SPA fallback), got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `id="root"`) {
		t.Fatalf("SPA fallback should serve index.html: %s", w.Body.String())
	}
}

// 未命中的 /api 路由不落到 SPA,返回 JSON 404。
func TestWebAPINotFoundStaysJSON(t *testing.T) {
	r, _, _ := setup(t)
	w := do(t, r, http.MethodGet, "/api/does-not-exist", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("want json content-type, got %q", ct)
	}
	if strings.Contains(w.Body.String(), "<html") {
		t.Fatalf("api 404 should not return html: %s", w.Body.String())
	}
}

// /assets/ 下不存在的文件必须 404 —— 回退 index.html 会让浏览器报 MIME 错误而非 404,极难排查。
func TestWebMissingAssetReturns404(t *testing.T) {
	r, _, _ := setup(t)
	w := do(t, r, http.MethodGet, "/assets/nope-not-exist.js", "", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("missing asset want 404, got %d (body=%s)", w.Code, w.Body.String()[:min(80, w.Body.Len())])
	}
}
