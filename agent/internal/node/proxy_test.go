package node

import (
	"net"
	"strings"
	"testing"
	"time"

	"agent/internal/api"

	coresvc "github.com/go-gost/core/service"
)

func TestProxyConfigMapsType(t *testing.T) {
	cases := []struct {
		nodeType string
		want     string
	}{
		{"socks5", "socks5"},
		{"http", "http"},
	}
	for _, c := range cases {
		cfg, err := proxyConfig(api.NodeInfo{Type: c.nodeType, Port: 1080})
		if err != nil {
			t.Fatalf("%s: %v", c.nodeType, err)
		}
		if cfg.Handler.Type != c.want {
			t.Errorf("%s: handler type = %q, want %q", c.nodeType, cfg.Handler.Type, c.want)
		}
		if cfg.Listener.Type != "tcp" {
			t.Errorf("%s: listener type = %q, want tcp", c.nodeType, cfg.Listener.Type)
		}
		if cfg.Addr != "0.0.0.0:1080" {
			t.Errorf("%s: addr = %q", c.nodeType, cfg.Addr)
		}
	}
}

func TestProxyConfigRejectsUnknownType(t *testing.T) {
	if _, err := proxyConfig(api.NodeInfo{Type: "vmess", Port: 1080}); err == nil {
		t.Fatal("未知 type 应返回 error")
	}
}

func TestProxyConfigAuthOnlyWithAccount(t *testing.T) {
	cfg, err := proxyConfig(api.NodeInfo{Type: "socks5", Port: 1})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Handler.Auth != nil {
		t.Fatalf("无账号时不应带 Auth, got %+v", cfg.Handler.Auth)
	}

	cfg, err = proxyConfig(api.NodeInfo{Type: "http", Port: 1, Value1: "u", Value2: "p"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Handler.Auth == nil || cfg.Handler.Auth.Username != "u" || cfg.Handler.Auth.Password != "p" {
		t.Fatalf("账号未透传: %+v", cfg.Handler.Auth)
	}
}

// 保护 proxy.go 的 init():缺了那段全局 logger/TLS 补丁,ParseService 会空指针 panic,
// 而 proxyConfig 这类纯函数测试完全发现不了。这里真起服务并连一次。
func TestStartProxyBindsAndAccepts(t *testing.T) {
	for _, nt := range []string{"socks5", "http"} {
		t.Run(nt, func(t *testing.T) {
			svc, _, err := startProxy(api.NodeInfo{Type: nt, Port: 0, Value1: "u", Value2: "p"})
			if err != nil {
				t.Fatalf("startProxy(%s) 失败: %v", nt, err)
			}
			defer svc.Close()

			addr := waitProxyAddr(t, svc)
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				t.Fatalf("服务未在 %s 上接受连接: %v", addr, err)
			}
			conn.Close()
		})
	}
}

func waitProxyAddr(t *testing.T, svc coresvc.Service) string {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if a := svc.Addr(); a != nil && !strings.HasSuffix(a.String(), ":0") {
			return a.String()
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("服务未在超时内监听, addr=%v", svc.Addr())
	return ""
}
