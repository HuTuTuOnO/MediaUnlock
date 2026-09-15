package router

import (
	"net/http"
	"testing"

	"server/internal/models"
	"server/internal/retention"
)

func TestSettingsGetEmptyThenUpsert(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	// 初始:已知键补空串(title/token/retention_days)
	w := do(t, r, http.MethodGet, "/api/settings", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get want 200, got %d", w.Code)
	}
	var got map[string]string
	decodeData(t, w, &got)
	if got["title"] != "" || got["token"] != "" || got["retention_days"] != "" {
		t.Fatalf("expected empty known settings, got %v", got)
	}

	// upsert 两个键
	w = do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{
		"title": "MyPanel",
		"token": "ro-token-abc",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("put want 200, got %d: %s", w.Code, w.Body.String())
	}
	decodeData(t, w, &got)
	if got["title"] != "MyPanel" || got["token"] != "ro-token-abc" {
		t.Fatalf("upsert result unexpected: %v", got)
	}

	// 再次 upsert 单键,另一个键不变
	w = do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"title": "Renamed"})
	decodeData(t, w, &got)
	if got["title"] != "Renamed" || got["token"] != "ro-token-abc" {
		t.Fatalf("partial upsert should keep token: %v", got)
	}
}

func TestSettingsUnknownKeyRejected(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")
	w := do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"evil": "x"})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("unknown key want 400, got %d", w.Code)
	}
}

// retention_days 只接受非负整数(不设上限);0 合法 = 不自动清理。
// 非法值必须 400 —— 否则定时清理每次都会静默回退,配置形同虚设。
func TestSettingsRetentionDaysValidation(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	for _, bad := range []string{"abc", "-1", "1.5", ""} {
		w := do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"retention_days": bad})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("retention_days=%q want 400, got %d", bad, w.Code)
		}
	}

	// 0 = 不清理,合法
	if w := do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"retention_days": "0"}); w.Code != http.StatusOK {
		t.Fatalf("retention_days=0 want 200, got %d", w.Code)
	}
	// 不设上限:很大的值也接受
	if w := do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"retention_days": "99999"}); w.Code != http.StatusOK {
		t.Fatalf("retention_days=99999 want 200, got %d", w.Code)
	}

	// 落库值正确,GET 能读回
	var s models.Setting
	if err := db.Where("key = ?", models.SettingKeyRetentionDays).First(&s).Error; err != nil {
		t.Fatalf("read retention_days: %v", err)
	}
	if s.Value != "99999" {
		t.Fatalf("stored retention_days = %q, want 99999", s.Value)
	}
	var got map[string]string
	decodeData(t, do(t, r, http.MethodGet, "/api/settings", tok, nil), &got)
	if got["retention_days"] != "99999" {
		t.Fatalf("GET retention_days = %q, want 99999", got["retention_days"])
	}
}

// 通过 API 写入的保留天数,应能被 retention 包直接读到(跨包一致性)。
func TestSettingsRetentionDaysReadByRetention(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	if w := do(t, r, http.MethodPut, "/api/settings", tok, map[string]string{"retention_days": "14"}); w.Code != http.StatusOK {
		t.Fatalf("put want 200, got %d", w.Code)
	}
	if got := retention.Days(db); got != 14 {
		t.Fatalf("retention.Days = %d, want 14", got)
	}
}
