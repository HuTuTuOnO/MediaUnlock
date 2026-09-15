package router

import (
	"net/http"
	"strconv"
	"strings"
	"testing"

	"server/internal/models"
)

func TestNodeCRUDAndRetoken(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	// create
	w := do(t, r, http.MethodPost, "/api/nodes", tok, map[string]any{
		"name": "JP1", "alias": "jp1", "type": "socks5", "host": "1.2.3.4", "port": 1080,
		"value1": "u", "value2": "p",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Node  models.Node `json:"node"`
		Token string      `json:"token"`
	}
	decodeData(t, w, &created)
	if created.Token == "" {
		t.Fatal("create should return one-time token")
	}
	if created.Node.Status != models.StatusEnabled {
		t.Fatalf("new node should default enabled, got %d", created.Node.Status)
	}
	id := created.Node.ID

	// list — 有意放开:列表响应里带上 token,供「查看详情」弹窗展示(该接口需 JWT,仅管理员可见)。
	// 列表页面本身不渲染 token 列,只在详情弹窗里显示。
	w = do(t, r, http.MethodGet, "/api/nodes", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list want 200, got %d", w.Code)
	}
	if got := w.Body.String(); !strings.Contains(got, created.Token) {
		t.Fatal("list should expose token: detail dialog reads it from the list response")
	}

	// update — disable it
	disabled := models.StatusDisabled
	w = do(t, r, http.MethodPut, "/api/nodes/"+strconv.Itoa(int(id)), tok, map[string]any{
		"name": "JP1", "alias": "jp1", "type": "socks5", "host": "1.2.3.4", "port": 1080,
		"status": disabled,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated models.Node
	decodeData(t, w, &updated)
	if updated.Status != models.StatusDisabled {
		t.Fatalf("update should set status=0, got %d", updated.Status)
	}

	// retoken — new token differs from create token
	w = do(t, r, http.MethodPost, "/api/nodes/"+strconv.Itoa(int(id))+"/retoken", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("retoken want 200, got %d", w.Code)
	}
	var rt struct {
		Token string `json:"token"`
	}
	decodeData(t, w, &rt)
	if rt.Token == "" || rt.Token == created.Token {
		t.Fatalf("retoken should return a new different token")
	}

	// delete
	w = do(t, r, http.MethodDelete, "/api/nodes/"+strconv.Itoa(int(id)), tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d", w.Code)
	}
}

func TestNodeListPagination(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")
	for i := 0; i < 3; i++ {
		do(t, r, http.MethodPost, "/api/nodes", tok, map[string]any{"alias": "n" + strconv.Itoa(i), "type": "http"})
	}
	var resp struct {
		Total  int64         `json:"total"`
		Limit  int           `json:"limit"`
		Offset int           `json:"offset"`
		Items  []models.Node `json:"items"`
	}
	w := do(t, r, http.MethodGet, "/api/nodes?limit=2", tok, nil)
	decodeData(t, w, &resp)
	if resp.Total != 3 || resp.Limit != 2 || len(resp.Items) != 2 {
		t.Fatalf("want total=3 limit=2 items=2, got total=%d limit=%d items=%d", resp.Total, resp.Limit, len(resp.Items))
	}
}

// 删除节点应连带清理 node_platforms 关联与 unlocks 历史。
func TestNodeDeleteCascades(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	// 建节点 + 平台 + 绑定关系
	var created struct {
		Node models.Node `json:"node"`
	}
	decodeData(t, do(t, r, http.MethodPost, "/api/nodes", tok,
		map[string]any{"alias": "del1", "type": "http"}), &created)
	nodeID := created.Node.ID

	var plat models.Platform
	decodeData(t, do(t, r, http.MethodPost, "/api/platforms", tok,
		map[string]any{"name": "Netflix"}), &plat)

	if err := db.Model(&models.Node{ID: nodeID}).Association("Platforms").Append(&plat); err != nil {
		t.Fatalf("bind relation: %v", err)
	}
	// 直接塞一条该节点的 unlock 历史
	db.Create(&models.Unlock{NodeID: nodeID, PlatformID: plat.ID, Status: 1})

	// 删除节点
	if w := do(t, r, http.MethodDelete, "/api/nodes/"+strconv.Itoa(int(nodeID)), tok, nil); w.Code != http.StatusOK {
		t.Fatalf("delete want 200, got %d", w.Code)
	}

	var rel, unlocks int64
	db.Table("node_platforms").Where("node_id = ?", nodeID).Count(&rel)
	db.Model(&models.Unlock{}).Where("node_id = ?", nodeID).Count(&unlocks)
	if rel != 0 || unlocks != 0 {
		t.Fatalf("cascade failed: node_platforms=%d unlocks=%d (want 0/0)", rel, unlocks)
	}
	// 平台本身应保留
	var platCount int64
	db.Model(&models.Platform{}).Where("id = ?", plat.ID).Count(&platCount)
	if platCount != 1 {
		t.Fatalf("platform should survive node delete, got %d", platCount)
	}
}

func TestNodeDuplicateAlias(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")
	body := map[string]any{"alias": "dup", "type": "http"}
	if w := do(t, r, http.MethodPost, "/api/nodes", tok, body); w.Code != http.StatusCreated {
		t.Fatalf("first create want 201, got %d", w.Code)
	}
	if w := do(t, r, http.MethodPost, "/api/nodes", tok, body); w.Code != http.StatusConflict {
		t.Fatalf("duplicate alias want 409, got %d", w.Code)
	}
}

// 编辑节点不应清空它的 node_platforms 关联。
// UpdateNode 用的是 db.Save(&n),而 GORM 的 Save 会连带处理关联字段 —— 这里做回归保护。
func TestNodeUpdateKeepsPlatformLinks(t *testing.T) {
	r, db, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, tok, "Netflix", "domain:netflix.com")
	nodeID, ntok := createNode(t, r, tok, "jp1")
	// 走上报建立 node_platforms 关联
	doAgent(t, r, http.MethodPost, "/api/agent/report", ntok, map[string]any{
		"results": []map[string]any{{"name": "Netflix", "status": 1}},
	})

	var before int64
	db.Table("node_platforms").Where("node_id = ?", nodeID).Count(&before)
	if before != 1 {
		t.Fatalf("setup: want 1 link, got %d", before)
	}

	// 只改名字
	w := do(t, r, http.MethodPut, "/api/nodes/"+strconv.Itoa(int(nodeID)), tok, map[string]any{
		"name": "JP1-renamed", "alias": "jp1", "type": "socks5", "host": "1.2.3.4", "port": 1080,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update want 200, got %d: %s", w.Code, w.Body.String())
	}

	var after int64
	db.Table("node_platforms").Where("node_id = ?", nodeID).Count(&after)
	if after != 1 {
		t.Fatalf("update wiped node_platforms: before=%d after=%d", before, after)
	}
}

// 编辑节点时 alias 不允许清空(CreateNode 校验了,UpdateNode 也应校验)。
func TestNodeUpdateRejectsEmptyAlias(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	id, _ := createNode(t, r, tok, "jp1")
	w := do(t, r, http.MethodPut, "/api/nodes/"+strconv.Itoa(int(id)), tok, map[string]any{
		"name": "JP1", "alias": "", "type": "socks5", "host": "1.2.3.4", "port": 1080,
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("empty alias on update want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// 列表 search:对 name / alias / host 做模糊匹配;空 search 返回全量。
func TestNodeListSearch(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	for _, n := range []map[string]any{
		{"name": "日本 01", "alias": "jp1", "type": "socks5", "host": "1.1.1.1"},
		{"name": "美国 01", "alias": "us1", "type": "socks5", "host": "2.2.2.2"},
	} {
		if w := do(t, r, http.MethodPost, "/api/nodes", tok, n); w.Code != http.StatusCreated {
			t.Fatalf("create node want 201, got %d: %s", w.Code, w.Body.String())
		}
	}

	type paged struct {
		Total int64         `json:"total"`
		Items []models.Node `json:"items"`
	}
	var got paged

	// 按 alias 命中
	decodeData(t, do(t, r, http.MethodGet, "/api/nodes?search=jp1", tok, nil), &got)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Alias != "jp1" {
		t.Fatalf("search=jp1 want only jp1, got total=%d items=%+v", got.Total, got.Items)
	}

	// 按 host 命中
	decodeData(t, do(t, r, http.MethodGet, "/api/nodes?search=2.2.2.2", tok, nil), &got)
	if got.Total != 1 || len(got.Items) != 1 || got.Items[0].Alias != "us1" {
		t.Fatalf("search by host want us1, got %+v", got.Items)
	}

	// 不匹配 → 0 条
	decodeData(t, do(t, r, http.MethodGet, "/api/nodes?search=nope", tok, nil), &got)
	if got.Total != 0 || len(got.Items) != 0 {
		t.Fatalf("search=nope want 0, got %d", got.Total)
	}

	// 空 search → 全量
	decodeData(t, do(t, r, http.MethodGet, "/api/nodes", tok, nil), &got)
	if got.Total != 2 {
		t.Fatalf("no search want 2, got %d", got.Total)
	}
}

// type 白名单:只接受 socks5 / http(小写)—— 其他值 agent 起代理服务会直接报错。
func TestNodeTypeWhitelist(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	for _, bad := range []string{"ss", "trojan", "SOCKS5", "HTTP", "ftp", ""} {
		w := do(t, r, http.MethodPost, "/api/nodes", tok, map[string]any{
			"alias": "n-" + bad, "type": bad,
		})
		if w.Code != http.StatusBadRequest {
			t.Fatalf("create type=%q want 400, got %d", bad, w.Code)
		}
	}

	// 更新时同样要拦
	id, _ := createNode(t, r, tok, "jp1")
	w := do(t, r, http.MethodPut, "/api/nodes/"+strconv.Itoa(int(id)), tok, map[string]any{
		"name": "JP1", "alias": "jp1", "type": "trojan",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("update to invalid type want 400, got %d", w.Code)
	}
}

// 创建时显式传 status=0 要落库为 0。
// 模型上带 gorm:"default:1" 时 GORM 会把零值替换成默认值,「关闭」就建不出来。
func TestCreateNodeDisabled(t *testing.T) {
	r, _, _ := setup(t)
	tok := tokenFor(t, r, "admin", "testpass")

	w := do(t, r, http.MethodPost, "/api/nodes", tok, map[string]any{
		"name": "JP1", "alias": "jp1", "type": "socks5", "host": "1.2.3.4", "port": 1080,
		"status": models.StatusDisabled,
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create want 201, got %d: %s", w.Code, w.Body.String())
	}
	var created struct {
		Node models.Node `json:"node"`
	}
	decodeData(t, w, &created)
	if created.Node.Status != models.StatusDisabled {
		t.Fatalf("create with status=0 should stay disabled, got %d", created.Node.Status)
	}
}
