package config

// 配置解析:缺省值填充 + 必填项校验(jwt.secret 为空必须拒绝启动)。

import (
	"os"
	"path/filepath"
	"testing"
)

// write 写一份临时 config.yml 并返回路径。
func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return p
}

func TestLoadDefaults(t *testing.T) {
	c, err := Load(write(t, "jwt:\n  secret: s3cret\n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Server.Addr != ":8080" {
		t.Fatalf("addr = %q, want :8080", c.Server.Addr)
	}
	if c.Database.Path != "./data/database.db" {
		t.Fatalf("db path = %q, want ./data/database.db", c.Database.Path)
	}
	if c.JWT.ExpireHours != 72 {
		t.Fatalf("expire_hours = %d, want 72", c.JWT.ExpireHours)
	}
	if c.JWT.Secret != "s3cret" {
		t.Fatalf("secret = %q, want s3cret", c.JWT.Secret)
	}
}

func TestLoadRejectsEmptyJWTSecret(t *testing.T) {
	// 显式空串
	if _, err := Load(write(t, "jwt:\n  secret: \"\"\n")); err == nil {
		t.Fatal("empty jwt.secret should be rejected")
	}
	// 整个 jwt 段缺失
	if _, err := Load(write(t, "server:\n  addr: \":9090\"\n")); err == nil {
		t.Fatal("missing jwt.secret should be rejected")
	}
}

func TestLoadMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yml")); err == nil {
		t.Fatal("missing config file should error")
	}
}
