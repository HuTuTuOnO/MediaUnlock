package router

import (
	"net/http"
	"strconv"
	"testing"

	"server/internal/models"
)

func TestPlatformCRUD(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	// create
	w := do(t, r, http.MethodPost, "/api/platforms", tok, map[string]any{
		"name": "Netflix", "rules": "domain:netflix.com",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d: %s", w.Code, w.Body.String())
	}
	var p models.Platform
	decodeData(t, w, &p)
	if p.ID == 0 || p.Status != models.StatusEnabled {
		t.Fatalf("unexpected created platform: %+v", p)
	}

	// list
	if w := do(t, r, http.MethodGet, "/api/platforms", tok, nil); w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", w.Code)
	}

	// update — disable + change rules
	disabled := models.StatusDisabled
	w = do(t, r, http.MethodPut, "/api/platforms/"+strconv.Itoa(int(p.ID)), tok, map[string]any{
		"name": "Netflix", "rules": "domain:netflix.com,domain:nflxvideo.net", "status": disabled,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d: %s", w.Code, w.Body.String())
	}
	var up models.Platform
	decodeData(t, w, &up)
	if up.Status != models.StatusDisabled {
		t.Fatalf("update should disable, got status=%d", up.Status)
	}

	// delete
	if w := do(t, r, http.MethodDelete, "/api/platforms/"+strconv.Itoa(int(p.ID)), tok, nil); w.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d", w.Code)
	}
}

func TestPlatformDuplicateName(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")
	body := map[string]any{"name": "Disney+"}
	if w := do(t, r, http.MethodPost, "/api/platforms", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d", w.Code)
	}
	if w := do(t, r, http.MethodPost, "/api/platforms", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("duplicate name want 409, got %d", w.Code)
	}
}

// 编辑平台时 name 不允许清空(CreatePlatform 校验了,UpdatePlatform 也应校验)。
func TestPlatformUpdateRejectsEmptyName(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	id := createPlatformID(t, r, tok, "Netflix")
	w := do(t, r, http.MethodPut, "/api/platforms/"+strconv.Itoa(int(id)), tok, map[string]any{
		"name": "", "rules": "domain:netflix.com",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty name on update want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// 列表 search:对 name / rules 做模糊匹配;空 search 返回全量。
func TestPlatformListSearch(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	for _, p := range []map[string]any{
		{"name": "Netflix", "rules": "domain:netflix.com"},
		{"name": "Disney+", "rules": "domain:disney.com"},
	} {
		if w := do(t, r, http.MethodPost, "/api/platforms", tok, p); w.Code != http.StatusCreated {
			t.Fatalf("create platform want 201, got %d: %s", w.Code, w.Body.String())
		}
	}

	type paged struct {
		Total int64             `json:"total"`
		Items []models.Platform `json:"items"`
	}
	var got paged

	// 按 name 命中
	decodeData(t, do(t, r, http.MethodGet, "/api/platforms?search=Netflix", tok, nil), &got)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Name != "Netflix" {
		t.Fatalf("search=Netflix want 1, got total=%d items=%+v", got.Total, got.Items)
	}

	// 按 rules 命中
	decodeData(t, do(t, r, http.MethodGet, "/api/platforms?search=disney", tok, nil), &got)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Name != "Disney+" {
		t.Fatalf("search by rules want Disney+, got %+v", got.Items)
	}

	// 空 search → 全量
	decodeData(t, do(t, r, http.MethodGet, "/api/platforms", tok, nil), &got)
	if got.Total != 2 {
		t.Fatalf("no search want 2, got %d", got.Total)
	}
}

// 创建时显式传 status=0 要落库为 0。
// 模型上带 gorm:"default:1" 时 GORM 会把零值替换成默认值,「关闭」就建不出来。
func TestCreatePlatformDisabled(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	w := do(t, r, http.MethodPost, "/api/platforms", tok, map[string]any{
		"name": "Netflix", "rules": "domain:netflix.com", "status": models.StatusDisabled,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d: %s", w.Code, w.Body.String())
	}
	var p models.Platform
	decodeData(t, w, &p)
	if p.Status != models.StatusDisabled {
		t.Fatalf("create with status=0 should stay disabled, got %d", p.Status)
	}
}
