package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"server/internal/models"
)

// createPlatformID 建平台并返回其 id
func createPlatformID(t *testing.T, r *gin.Engine, adminTok, name string) uint {
	t.Helper()
	w := do(t, r, http.MethodPost, "/api/platforms", adminTok, map[string]any{"name": name})
	if w.Code != http.StatusCreated {
		t.Fatalf("create platform %s want 201, got %d", name, w.Code)
	}
	var p models.Platform
	decodeData(t, w, &p)
	return p.ID
}

func TestCommonSettingsPublic(t *testing.T) {
	r, _, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	// 未鉴权也能读(默认空标题)
	w := do(t, r, http.MethodGet, "/api/common/settings", "", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("public settings want 200, got %d", w.Code)
	}
	var s struct {
		Title string `json:"title"`
	}
	decodeData(t, w, &s)
	if s.Title != "" {
		t.Fatalf("want empty title, got %q", s.Title)
	}

	// 设置标题后再读
	if w := do(t, r, http.MethodPut, "/api/settings", admin, map[string]string{"title": "面板X"}); w.Code != http.StatusOK {
		t.Fatalf("set title want 200, got %d", w.Code)
	}
	w = do(t, r, http.MethodGet, "/api/common/settings", "", nil)
	decodeData(t, w, &s)
	if s.Title != "面板X" {
		t.Fatalf("want title 面板X, got %q", s.Title)
	}
}

func TestCommonStats(t *testing.T) {
	r, _, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, admin, "Netflix", "domain:netflix.com")
	createNode(t, r, admin, "jp1")
	createNode(t, r, admin, "jp2")

	// 未登录 → 401
	if w := do(t, r, http.MethodGet, "/api/common/stats", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("stats no token want 401, got %d", w.Code)
	}

	w := do(t, r, http.MethodGet, "/api/common/stats", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("stats want 200, got %d", w.Code)
	}
	var st struct {
		NodeCount        int64 `json:"node_count"`
		PlatformCount    int64 `json:"platform_count"`
		AssociationCount int64 `json:"association_count"`
		ActiveNodeCount  int64 `json:"active_node_count"`
	}
	decodeData(t, w, &st)
	if st.NodeCount != 2 || st.PlatformCount != 1 || st.ActiveNodeCount != 2 || st.AssociationCount != 0 {
		t.Fatalf("unexpected stats: %+v", st)
	}
}

func TestSettingRetoken(t *testing.T) {
	r, _, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	w := do(t, r, http.MethodPost, "/api/settings/retoken", admin, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("retoken want 200, got %d", w.Code)
	}
	var out struct {
		Token string `json:"token"`
	}
	decodeData(t, w, &out)
	if out.Token == "" {
		t.Fatal("retoken should return a token")
	}

	// 该 token 应能通过 client agent 鉴权(Agent 用 "Token:" 头)
	if w := doAgent(t, r, http.MethodGet, "/api/agent/unlocked", out.Token, nil); w.Code != http.StatusOK {
		t.Fatalf("new token should authorize client agent, got %d", w.Code)
	}
}

func TestChangePasswordNoOldCheck(t *testing.T) {
	r, _, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	// 无需原密码,直接改
	w := do(t, r, http.MethodPost, "/api/auth/change-password", admin,
		map[string]string{"new_password": "newpass123"})
	if w.Code != http.StatusOK {
		t.Fatalf("change password want 200, got %d: %s", w.Code, w.Body.String())
	}

	// 旧密码登录失败
	if w := login(t, r, "admin", "testpass"); w.Code != http.StatusUnauthorized {
		t.Fatalf("old password should fail, got %d", w.Code)
	}
	// 新密码登录成功
	if w := login(t, r, "admin", "newpass123"); w.Code != http.StatusOK {
		t.Fatalf("new password should succeed, got %d", w.Code)
	}

	// 太短 → 400
	if w := do(t, r, http.MethodPost, "/api/auth/change-password", admin,
		map[string]string{"new_password": "123"}); w.Code != http.StatusBadRequest {
		t.Fatalf("short password want 400, got %d", w.Code)
	}
}
