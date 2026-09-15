package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadNode(t *testing.T) {
	c, err := Load(writeCfg(t, `
api: https://panel.example.com/
mode: node
token: abc
scheduler: "0 45 * * * *"
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.API != "https://panel.example.com" { // 末尾斜杠被裁掉
		t.Fatalf("api not trimmed: %q", c.API)
	}
	if c.Mode != ModeNode || c.Token != "abc" {
		t.Fatalf("unexpected: %+v", c)
	}
}

func TestLoadClientDefaultsStack(t *testing.T) {
	c, err := Load(writeCfg(t, `
api: https://p
mode: client
token: t
scheduler: "0 0 * * * *"
render:
  type: soga
  path: /etc/soga/routes.toml
`))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Stack != "default" {
		t.Fatalf("stack should default to 'default', got %q", c.Stack)
	}
}

// stack 取值是 default | 4 | 6。YAML 里 4/6 是数字标量,不写引号也要能读进来。
func TestLoadClientStackValues(t *testing.T) {
	for _, stack := range []string{"default", "4", "6"} {
		c, err := Load(writeCfg(t, `
api: https://p
mode: client
token: t
scheduler: "0 0 * * * *"
stack: `+stack+`
render:
  type: soga
  path: /etc/soga/routes.toml
`))
		if err != nil {
			t.Fatalf("stack %q: %v", stack, err)
		}
		if c.Stack != stack {
			t.Errorf("stack = %q, want %q", c.Stack, stack)
		}
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]string{
		"missing panel":         "mode: node\ntoken: t\nscheduler: \"* * * * * *\"",
		"bad mode":              "api: p\nmode: nope\ntoken: t\nscheduler: \"* * * * * *\"",
		"missing token":         "api: p\nmode: node\nscheduler: \"* * * * * *\"",
		"missing scheduler":     "api: p\nmode: node\ntoken: t",
		"client missing render": "api: p\nmode: client\ntoken: t\nscheduler: \"* * * * * *\"",
		"client bad stack":      "api: p\nmode: client\ntoken: t\nscheduler: \"* * * * * *\"\nstack: 5\nrender:\n  type: soga\n  path: /x",
		"client old stack name": "api: p\nmode: client\ntoken: t\nscheduler: \"* * * * * *\"\nstack: ipv4\nrender:\n  type: soga\n  path: /x",
		"client bad target":     "api: p\nmode: client\ntoken: t\nscheduler: \"* * * * * *\"\nrender:\n  type: nginx\n  path: /x",
	}
	for name, body := range cases {
		if _, err := Load(writeCfg(t, body)); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}
