package client

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"

	"agent/internal/api"
)

// 本版认不出的节点类型 → 出口退回 direct,不带 server,整份配置照样合法。
func TestRenderSogaFallsBackToDirect(t *testing.T) {
	got, err := renderSoga(assignment{
		Platforms: map[string]platform{"X": {Rules: []string{"domain:x.com"}, Alias: "VM"}},
		Nodes:     map[string]node{"VM": {UnlockedNode: api.UnlockedNode{Type: "vmess", Host: "1.2.3.4", Port: 1080}}},
	})
	if err != nil {
		t.Fatalf("认不出的类型不该报错: %v", err)
	}

	var doc struct {
		Routes []struct {
			Outs []struct {
				Type   string `toml:"type"`
				Server string `toml:"server"`
			} `toml:"outs"`
		} `toml:"routes"`
	}
	if err := toml.Unmarshal(got, &doc); err != nil {
		t.Fatalf("生成的配置不是合法 TOML: %v\n%s", err, got)
	}
	if len(doc.Routes) != 2 {
		t.Fatalf("routes = %d, want 2", len(doc.Routes))
	}
	out := doc.Routes[0].Outs[0]
	if out.Type != "direct" || out.Server != "" {
		t.Errorf("出口应为不带 server 的 direct: %+v", out)
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
  # Disney+
  "domain:disneyplus.com",
  # Netflix
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
	if rules := doc.Routes[0].Rules; len(rules) != 1 || rules[0] != `domain:a"b` {
		t.Errorf("平台名成了注释,不该再占一个规则位: rules = %q", rules)
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
}

// 必须原地覆盖:写完后从"写入前打开的 fd"里能读到新内容。
// 改成"临时文件 + rename"的话,旧 fd 还指着被换掉的 inode,读到的会是旧内容 ——
// 那样 soga 的 inotify watch 会随旧 inode 一起被内核摘掉,之后再也感知不到配置更新。
func TestWriteFileWritesInPlace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "routes.toml")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if err := writeFile(path, []byte("brand new content")); err != nil {
		t.Fatalf("writeFile: %v", err)
	}

	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "brand new content" {
		t.Errorf("旧 fd 读到 %q, want brand new content(说明 inode 被换掉了)", got)
	}
}
