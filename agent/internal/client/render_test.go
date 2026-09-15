package client

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"agent/internal/api"
)

func TestOutType(t *testing.T) {
	// soga 只认 "socks",写 socks5 会报 unknown out type
	for nodeType, want := range map[string]string{"socks5": "socks", "http": "http"} {
		got, err := outType(nodeType)
		if err != nil || got != want {
			t.Errorf("outType(%q) = %q, %v; want %q", nodeType, got, err, want)
		}
	}
	if _, err := outType("vmess"); err == nil {
		t.Error("未知类型应返回 error")
	}
}

func TestRenderSoga(t *testing.T) {
	got, err := renderSoga(assignment{
		Platforms: map[string]platform{
			"Netflix": {Rules: []string{"domain:netflix.com", "domain:netflix.net"}, Alias: "JPAK1"},
			"Disney+": {Rules: []string{"domain:disneyplus.com"}, Alias: "JPAK1"},
		},
		Nodes: map[string]node{
			"JPAK1": {UnlockedNode: api.UnlockedNode{Type: "socks5", Host: "1.2.3.4", Port: 1080, Value1: "stream", Value2: "pw"}},
		},
	})
	if err != nil {
		t.Fatalf("renderSoga: %v", err)
	}

	want := `enable=true

# 路由 JPAK1
[[routes]]
rules=[
  "# Disney+",
  "domain:disneyplus.com",
  "# Netflix",
  "domain:netflix.com",
  "domain:netflix.net",
]

[[routes.Outs]]
type="socks"
server="1.2.3.4"
port=1080
username="stream"
password="pw"

[[routes]]
rules=["*"]

[[routes.Outs]]
type="direct"
`
	if string(got) != want {
		t.Errorf("输出不符:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// 多个节点各占一个块,按 alias 排序,输出稳定。
func TestRenderSogaGroupsByNode(t *testing.T) {
	got, err := renderSoga(assignment{
		Platforms: map[string]platform{
			"Netflix": {Rules: []string{"domain:netflix.com"}, Alias: "USA1"},
			"Hulu":    {Rules: []string{"domain:hulu.com"}, Alias: "JPAK1"},
		},
		Nodes: map[string]node{
			"USA1":  {UnlockedNode: api.UnlockedNode{Type: "socks5", Host: "5.6.7.8", Port: 1080}},
			"JPAK1": {UnlockedNode: api.UnlockedNode{Type: "http", Host: "1.2.3.4", Port: 8080}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	// JPAK1 排在 USA1 前面
	if i, j := strings.Index(string(got), "# 路由 JPAK1"), strings.Index(string(got), "# 路由 USA1"); i < 0 || j < 0 || i > j {
		t.Fatalf("块未按 alias 排序:\n%s", got)
	}
	if !strings.Contains(string(got), `type="http"`) {
		t.Errorf("http 节点应生成 http 出口:\n%s", got)
	}
}

// 无账号的节点不该写出空的 username/password。
func TestRenderSogaOmitsEmptyAuth(t *testing.T) {
	got, err := renderSoga(assignment{
		Platforms: map[string]platform{"X": {Rules: []string{"domain:x.com"}, Alias: "JP"}},
		Nodes:     map[string]node{"JP": {UnlockedNode: api.UnlockedNode{Type: "http", Host: "1.2.3.4", Port: 8080}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "username") || strings.Contains(string(got), "password") {
		t.Errorf("空账号不该写出 username/password:\n%s", got)
	}
}

// 平台指向的节点必须在探测结果里,否则会写出空 server 的配置 —— 直接报错。
func TestRenderSogaRejectsUnknownNode(t *testing.T) {
	_, err := renderSoga(assignment{
		Platforms: map[string]platform{"X": {Rules: []string{"domain:x.com"}, Alias: "GHOST"}},
		Nodes:     map[string]node{},
	})
	if err == nil {
		t.Fatal("节点不在探测结果里应返回 error")
	}
}

// 生成的文本必须能被 TOML 解析器读进去,否则 soga 直接加载失败。
// 顺带覆盖含引号/反斜杠的字段:转义错了这里就过不去。
func TestRenderSogaIsValidTOML(t *testing.T) {
	got, err := renderSoga(assignment{
		Platforms: map[string]platform{
			`X"Y`: {Rules: []string{`domain:a"b`}, Alias: `a"b\c`},
		},
		Nodes: map[string]node{
			`a"b\c`: {UnlockedNode: api.UnlockedNode{Type: "socks5", Host: "1.2.3.4", Port: 1080, Value1: `u"1`, Value2: `p\q`}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var doc struct {
		Enable bool `toml:"enable"`
		Routes []struct {
			Rules []string `toml:"rules"`
			Outs  []struct {
				Type     string `toml:"type"`
				Server   string `toml:"server"`
				Port     int    `toml:"port"`
				Username string `toml:"username"`
				Password string `toml:"password"`
			} `toml:"outs"`
		} `toml:"routes"`
	}
	if err := toml.Unmarshal(got, &doc); err != nil {
		t.Fatalf("生成的配置不是合法 TOML: %v\n%s", err, got)
	}

	if !doc.Enable || len(doc.Routes) != 2 {
		t.Fatalf("enable=%v routes=%d, want true / 2(1 条分流 + 1 条兜底)", doc.Enable, len(doc.Routes))
	}
	if rules := doc.Routes[0].Rules; len(rules) != 2 || rules[0] != `# X"Y` || rules[1] != `domain:a"b` {
		t.Errorf("rules = %q", rules)
	}
	out := doc.Routes[0].Outs[0]
	if out.Type != "socks" || out.Server != "1.2.3.4" || out.Port != 1080 {
		t.Errorf("out = %+v", out)
	}
	if out.Username != `u"1` || out.Password != `p\q` {
		t.Errorf("账号转义后不对: %+v", out)
	}
	if last := doc.Routes[1]; len(last.Rules) != 1 || last.Rules[0] != "*" || last.Outs[0].Type != "direct" {
		t.Errorf("兜底路由不对: %+v", last)
	}
}

func TestWriteFileKeepsMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.toml")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(path, []byte("new")); err != nil {
		t.Fatalf("writeFile: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("内容 = %q, want new", got)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Errorf("权限 = %v, want 0600(应沿用原文件)", fi.Mode().Perm())
	}
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Error("临时文件应已 rename,不该留下")
	}
}
