package router

// GET /api/unlocks 的契约(与 handlers/unlock.go 对应):
//   - 直接返回记录数组,不分页(不传 limit / offset)
//   - 支持 node_id / platform_ids / range 筛选,按 id 倒序
//   - range / platform_ids 非法时直接 400(不是静默忽略)

import (
	"net/http"
	"testing"
	"time"

	"server/internal/models"
)

func TestUnlocksQueryAndFilter(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	// 直接写库造历史:node1 平台1 成功、node1 平台2 失败、node2 平台1 成功
	rows := []models.Unlock{
		{NodeID: 1, PlatformID: 1, Status: 1, Region: "JP"},
		{NodeID: 1, PlatformID: 2, Status: 5, Err: "connection refused"},
		{NodeID: 2, PlatformID: 1, Status: 1, Region: "US"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed unlocks: %v", err)
	}

	// 不传筛选 → 全部记录,按 id 倒序
	w := do(t, r, http.MethodGet, "/api/unlocks", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d: %s", w.Code, w.Body.String())
	}
	var all []models.Unlock
	decodeData(t, w, &all)
	if len(all) != 3 {
		t.Fatalf("want 3 rows, got %d", len(all))
	}
	if all[0].ID < all[len(all)-1].ID {
		t.Fatal("expected id desc order")
	}

	// 按 node_id 过滤
	w = do(t, r, http.MethodGet, "/api/unlocks?node_id=1", tok, nil)
	var byNode []models.Unlock
	decodeData(t, w, &byNode)
	if len(byNode) != 2 {
		t.Fatalf("node_id=1 want 2, got %d", len(byNode))
	}

	// 按 platform_ids 过滤(逗号分隔,可多个)
	w = do(t, r, http.MethodGet, "/api/unlocks?platform_ids=2", tok, nil)
	var byPlat []models.Unlock
	decodeData(t, w, &byPlat)
	if len(byPlat) != 1 || byPlat[0].PlatformID != 2 {
		t.Fatalf("platform_ids=2 want 1 row of platform 2, got %+v", byPlat)
	}

	// 组合筛选
	w = do(t, r, http.MethodGet, "/api/unlocks?node_id=1&platform_ids=1", tok, nil)
	var combo []models.Unlock
	decodeData(t, w, &combo)
	if len(combo) != 1 {
		t.Fatalf("node_id=1&platform_ids=1 want 1, got %d", len(combo))
	}
}

// 非法 range / platform_ids 必须 400 —— 静默忽略会让调用方拿到远超预期的数据
func TestUnlocksInvalidParams(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	if w := do(t, r, http.MethodGet, "/api/unlocks?range=abc", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid range want 400, got %d", w.Code)
	}
	if w := do(t, r, http.MethodGet, "/api/unlocks?platform_ids=abc", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid platform_ids want 400, got %d", w.Code)
	}
	// node_id 非法也必须 400(不能静默忽略成"不过滤")
	if w := do(t, r, http.MethodGet, "/api/unlocks?node_id=abc", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid node_id want 400, got %d", w.Code)
	}
	if w := do(t, r, http.MethodGet, "/api/unlocks?node_id=0", tok, nil); w.Code != http.StatusBadRequest {
		t.Fatalf("node_id=0 want 400, got %d", w.Code)
	}
}

func TestUnlocksRequiresAuth(t *testing.T) {
	r, _, _ := setup(t)
	if w := do(t, r, http.MethodGet, "/api/unlocks", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("no token want 401, got %d", w.Code)
	}
}

// range 合法值:只返回窗口内的记录(按 created_at 过滤)。
func TestUnlocksRangeFilter(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	rows := []models.Unlock{
		{NodeID: 1, PlatformID: 1, Status: 1, CreatedAt: time.Now().Add(-10 * 24 * time.Hour)},
		{NodeID: 1, PlatformID: 1, Status: 1, CreatedAt: time.Now().Add(-1 * time.Hour)},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	var got []models.Unlock
	decodeData(t, do(t, r, http.MethodGet, "/api/unlocks?range=24h", tok, nil), &got)
	if len(got) != 1 {
		t.Fatalf("range=24h want 1 (only the fresh row), got %d", len(got))
	}

	decodeData(t, do(t, r, http.MethodGet, "/api/unlocks?range=7d", tok, nil), &got)
	if len(got) != 1 {
		t.Fatalf("range=7d want 1, got %d", len(got))
	}

	decodeData(t, do(t, r, http.MethodGet, "/api/unlocks?range=30d", tok, nil), &got)
	if len(got) != 2 {
		t.Fatalf("range=30d want 2, got %d", len(got))
	}
}

// platform_ids 的空片段应被跳过:"1,,2," 等价于 "1,2"。
func TestUnlocksPlatformIDsTolerant(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	rows := []models.Unlock{
		{NodeID: 1, PlatformID: 1, Status: 1},
		{NodeID: 1, PlatformID: 2, Status: 1},
		{NodeID: 1, PlatformID: 3, Status: 1},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	var got []models.Unlock
	decodeData(t, do(t, r, http.MethodGet, "/api/unlocks?platform_ids=1,,2,", tok, nil), &got)
	if len(got) != 2 {
		t.Fatalf("platform_ids=1,,2, want 2 rows, got %d", len(got))
	}
}
