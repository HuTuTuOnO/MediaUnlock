package router

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"server/internal/models"
)

// createNode 走 API 建节点并取回一次性 token
func createNode(t *testing.T, r *gin.Engine, adminTok, alias string) (id uint, token string) {
	t.Helper()
	w := do(t, r, http.MethodPost, "/api/nodes", adminTok, map[string]any{
		"name": alias, "alias": alias, "type": "socks5", "host": "1.2.3.4", "port": 1080,
		"value1": "u", "value2": "p",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create node want 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Node  models.Node `json:"node"`
		Token string      `json:"token"`
	}
	decodeData(t, w, &resp)
	return resp.Node.ID, resp.Token
}

func createPlatform(t *testing.T, r *gin.Engine, adminTok, name, rules string) {
	t.Helper()
	w := do(t, r, http.MethodPost, "/api/platforms", adminTok, map[string]any{"name": name, "rules": rules})
	if w.Code != http.StatusCreated {
		t.Fatalf("create platform %s want 201, got %d", name, w.Code)
	}
}

func TestAgentNodeAndReport(t *testing.T) {
	r, db, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, admin, "Netflix", "domain:netflix.com")
	createPlatform(t, r, admin, "Disney+", "domain:disney.com")
	nodeID, ntok := createNode(t, r, admin, "jp1")

	// GET /api/agent/node
	w := doAgent(t, r, http.MethodGet, "/api/agent/node", ntok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("agent node want 200, got %d: %s", w.Code, w.Body.String())
	}
	var ni struct {
		Type   string `json:"type"`
		Port   int    `json:"port"`
		Value1 string `json:"value1"`
	}
	decodeData(t, w, &ni)
	if ni.Type != "socks5" || ni.Port != 1080 || ni.Value1 != "u" {
		t.Fatalf("unexpected node info: %+v", ni)
	}

	// POST /api/agent/report:Netflix 成功、Disney+ 失败、Unknown 匹配不到
	w = doAgent(t, r, http.MethodPost, "/api/agent/report", ntok, map[string]any{
		"results": []map[string]any{
			{"name": "Netflix", "status": 1, "region": "JP"},
			{"name": "Disney+", "status": 5, "err": "blocked"},
			{"name": "NoSuchPlatform", "status": 1},
		},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("report want 200, got %d: %s", w.Code, w.Body.String())
	}

	// unlocks 历史应有 2 条(丢弃的不入库)
	var unlockCount int64
	db.Model(&models.Unlock{}).Count(&unlockCount)
	if unlockCount != 2 {
		t.Fatalf("want 2 unlock rows, got %d", unlockCount)
	}
	// node_platforms 只有 Netflix(status==1)
	var relCount int64
	db.Table("node_platforms").Count(&relCount)
	if relCount != 1 {
		t.Fatalf("want 1 node_platform, got %d", relCount)
	}
	// report_at 已刷新
	var node models.Node
	db.First(&node, nodeID)
	if node.ReportAt == nil {
		t.Fatal("report_at should be set")
	}

	// 再次上报:Netflix 变失败 → 关联表应移除该关系
	doAgent(t, r, http.MethodPost, "/api/agent/report", ntok, map[string]any{
		"results": []map[string]any{{"name": "Netflix", "status": 5}},
	})
	db.Table("node_platforms").Count(&relCount)
	if relCount != 0 {
		t.Fatalf("failed re-report should remove relation, got %d", relCount)
	}
}

func TestAgentNodeBadToken(t *testing.T) {
	r, _, _ := setup(t)
	if w := doAgent(t, r, http.MethodGet, "/api/agent/node", "bogus", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("bad node token want 401, got %d", w.Code)
	}
	if w := doAgent(t, r, http.MethodGet, "/api/agent/node", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing node token want 401, got %d", w.Code)
	}
}

func TestAgentUnlocked(t *testing.T) {
	r, db, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, admin, "Netflix", "domain:netflix.com")
	nodeID, ntok := createNode(t, r, admin, "jp1")

	// 配置全局只读 token
	if w := do(t, r, http.MethodPut, "/api/settings", admin, map[string]string{"token": "ro-xyz"}); w.Code != http.StatusOK {
		t.Fatalf("set token want 200, got %d", w.Code)
	}

	// 上报一条成功
	doAgent(t, r, http.MethodPost, "/api/agent/report", ntok, map[string]any{
		"results": []map[string]any{{"name": "Netflix", "status": 1, "region": "JP"}},
	})

	// 错误 token → 401
	if w := doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "wrong", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("wrong client token want 401, got %d", w.Code)
	}

	// 缺失 token → 401
	if w := doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "", nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("missing client token want 401, got %d", w.Code)
	}

	// 正确 token → 拿到 Netflix + jp1,且都带 status
	w := doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro-xyz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unlocked want 200, got %d: %s", w.Code, w.Body.String())
	}
	var out unlockedPayload
	decodeData(t, w, &out)

	np, ok := out.Platform["Netflix"]
	if !ok {
		t.Fatalf("Netflix missing in platform: %+v", out.Platform)
	}
	if np.Status != models.StatusEnabled {
		t.Fatalf("Netflix status = %d, want enabled(%d)", np.Status, models.StatusEnabled)
	}
	if len(np.Aliases) != 1 || np.Aliases[0] != "jp1" {
		t.Fatalf("Netflix aliases = %v, want [jp1]", np.Aliases)
	}
	if n, ok := out.Node["jp1"]; !ok {
		t.Fatalf("jp1 missing in node: %+v", out.Node)
	} else if n.Status != models.StatusEnabled {
		t.Fatalf("jp1 status = %d, want enabled(%d)", n.Status, models.StatusEnabled)
	}

	// 禁用该节点 → 仍是全量下发,但 status 变成禁用(Agent 自行判断)
	db.Model(&models.Node{}).Where("id = ?", nodeID).Update("status", models.StatusDisabled)
	w = doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro-xyz", nil)
	var out2 unlockedPayload
	decodeData(t, w, &out2)
	n2, ok := out2.Node["jp1"]
	if !ok {
		t.Fatal("disabled node should still be delivered (full delivery)")
	}
	if n2.Status != models.StatusDisabled {
		t.Fatalf("disabled node status = %d, want disabled(%d)", n2.Status, models.StatusDisabled)
	}
}

// unlockedPayload 对应 /api/agent/unlocked 的返回结构:node 与 platform 都是以别名为键的 map。
type unlockedPayload struct {
	Node map[string]struct {
		Status int `json:"status"`
	} `json:"node"`
	Platform map[string]struct {
		Aliases []string `json:"alias"`
		Rules   []string `json:"rules"`
		Status  int      `json:"status"`
	} `json:"platform"`
}

// unlockedNodeOut / unlockedPlatOut 供全量下发与 upload_at 用例复用。
type unlockedNodeOut struct {
	UploadAt *string `json:"upload_at"`
	Status   int     `json:"status"`
}

type unlockedPlatOut struct {
	Aliases []string `json:"alias"`
	Rules   []string `json:"rules"`
	Status  int      `json:"status"`
}

type unlockedFullPayload struct {
	Node     map[string]unlockedNodeOut `json:"node"`
	Platform map[string]unlockedPlatOut `json:"platform"`
}

// 全量下发:平台 / 节点被禁用后依然下发,并带 status=0;rules 只按逗号切分并去掉空白项。
func TestAgentUnlockedFullDeliveryAndRules(t *testing.T) {
	r, db, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	// rules 故意带空格和尾随逗号,验证切分 + trim + 跳过空项
	createPlatform(t, r, admin, "Netflix", "domain:netflix.com, domain:nflxso.net ,")
	nodeID, ntok := createNode(t, r, admin, "jp1")

	if w := do(t, r, http.MethodPut, "/api/settings", admin, map[string]string{"token": "ro-xyz"}); w.Code != http.StatusOK {
		t.Fatalf("set token want 200, got %d", w.Code)
	}
	// 上报一条成功,顺带建立 node_platforms 关联
	doAgent(t, r, http.MethodPost, "/api/agent/report", ntok, map[string]any{
		"results": []map[string]any{{"name": "Netflix", "status": 1, "region": "JP"}},
	})

	var out unlockedFullPayload
	decodeData(t, doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro-xyz", nil), &out)

	p := out.Platform["Netflix"]
	if len(p.Rules) != 2 || p.Rules[0] != "domain:netflix.com" || p.Rules[1] != "domain:nflxso.net" {
		t.Fatalf("rules = %v, want 2 trimmed items", p.Rules)
	}
	if n, ok := out.Node["jp1"]; !ok {
		t.Fatalf("jp1 missing: %+v", out.Node)
	} else if n.UploadAt == nil {
		t.Fatal("jp1 upload_at should not be null after a report")
	}

	// 平台 + 节点双双禁用 → 仍全量下发,status 都变 0
	db.Model(&models.Platform{}).Where("name = ?", "Netflix").Update("status", models.StatusDisabled)
	db.Model(&models.Node{}).Where("id = ?", nodeID).Update("status", models.StatusDisabled)

	var out2 unlockedFullPayload
	decodeData(t, doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro-xyz", nil), &out2)
	if p2, ok := out2.Platform["Netflix"]; !ok || p2.Status != models.StatusDisabled {
		t.Fatalf("disabled platform should still be delivered with status=0, got %+v", out2.Platform)
	}
	if n2, ok := out2.Node["jp1"]; !ok || n2.Status != models.StatusDisabled {
		t.Fatalf("disabled node should still be delivered with status=0, got %+v", out2.Node)
	}
}

// 从未上报过的节点,upload_at 应为 null(而不是 "0001-01-01 ..." 这种假时间)。
//
// 注意 node map 的条目来自"被平台引用的节点",所以这里必须手工建立 node_platforms 关联 ——
// 走上报接口会顺带写入 report_at,就测不到 null 了。
func TestAgentUnlockedNullUploadAt(t *testing.T) {
	r, db, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, admin, "Netflix", "domain:netflix.com")
	nodeID, _ := createNode(t, r, admin, "jp1")
	if w := do(t, r, http.MethodPut, "/api/settings", admin, map[string]string{"token": "ro-xyz"}); w.Code != http.StatusOK {
		t.Fatalf("set token want 200, got %d", w.Code)
	}

	var n models.Node
	db.First(&n, nodeID)
	var p models.Platform
	db.Where("name = ?", "Netflix").First(&p)
	if err := db.Model(&n).Association("Platforms").Append(&p); err != nil {
		t.Fatalf("link node_platforms: %v", err)
	}

	var out struct {
		Node map[string]unlockedNodeOut `json:"node"`
	}
	decodeData(t, doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro-xyz", nil), &out)
	nOut, ok := out.Node["jp1"]
	if !ok {
		t.Fatalf("jp1 missing: %+v", out.Node)
	}
	if nOut.UploadAt != nil {
		t.Fatalf("upload_at should be null before any report, got %q", *nOut.UploadAt)
	}
}

// node 字典是**全量下发**:即使节点没有被任何平台关联,也要出现在下发里,
// 由 client 按 alias 自取(平台那侧才按关联关系给出 aliases)。
func TestAgentUnlockedIncludesAllNodes(t *testing.T) {
	r, _, _ := setup(t)
	admin := tokenFor(t, r, "admin", "testpass")

	createPlatform(t, r, admin, "Netflix", "domain:netflix.com")

	_, jpTok := createNode(t, r, admin, "jp1") // 会关联到 Netflix
	createNode(t, r, admin, "lonely")          // 不关联任何平台

	doAgent(t, r, http.MethodPost, "/api/agent/report", jpTok, map[string]any{
		"results": []map[string]any{{"name": "Netflix", "status": 1}},
	})

	if w := do(t, r, http.MethodPut, "/api/settings", admin, map[string]string{"token": "ro"}); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}

	w := doAgent(t, r, http.MethodGet, "/api/agent/unlocked", "ro", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("unlocked want 200, got %d: %s", w.Code, w.Body.String())
	}

	var data struct {
		Node map[string]unlockedNodeOut `json:"node"`
	}
	decodeData(t, w, &data)

	if _, ok := data.Node["jp1"]; !ok {
		t.Fatal("关联了平台的 jp1 应下发")
	}
	if _, ok := data.Node["lonely"]; !ok {
		t.Fatal("未关联任何平台的 lonely 也应下发 —— 节点是全量的,不受平台关联限制")
	}
}
