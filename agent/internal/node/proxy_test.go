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
		// [::] 才是双栈;写成 0.0.0.0 会被 gost 判成 v4 而只监听 IPV4。
		if cfg.Addr != "[::]:1080" {
			t.Errorf("%s: addr = %q, want [::]:1080", c.nodeType, cfg.Addr)
		}
	}
}

func TestProxyConfigRejectsUnknownType(t *testing.T) {
	if _, err := proxyConfig(api.NodeInfo{Type: "vmess", Port: 1080}); err == nil {
		t.Fatal("未知 type 应返回 error")
	}
}

// 出口 IPv4 要写进 metadata.interface —— gost 收到 IP 时只设 LocalAddr 不绑设备。
// 探不到出口时(无默认路由)整个键都不给:空串会让 gost 走 ParseInterfaceAddr("") 的分支。
func TestProxyConfigBindsEgress(t *testing.T) {
	want := egressIPv4() // 同一次探测:测试和被测代码看到同一个出口地址

	cfg, err := proxyConfig(api.NodeInfo{Type: "socks5", Port: 1080})
	if err != nil {
		t.Fatal(err)
	}
	got, ok := cfg.Metadata["interface"]
	switch {
	case want == "" && ok:
		t.Fatalf("探不到出口时不该设 interface: %+v", cfg.Metadata)
	case want != "" && !ok:
		t.Fatalf("探到出口 %s 却没写进 metadata", want)
	case want != "" && got != want:
		t.Fatalf("metadata.interface = %v, want %s", got, want)
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

			addr := "127.0.0.1:" + waitProxyPort(t, svc)
			conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				t.Fatalf("服务未在 %s 上接受连接: %v", addr, err)
			}
			conn.Close()
		})
	}
}

// 真起服务,验证 [::] 监听确实双栈:IPv4 和 IPv6 都要连得上。
// 这是本次改动最容易悄悄回退的地方 —— 退回 "0.0.0.0" 时 IPv4 那条照样通过,
// 只有 IPv6 这条会挂,所以两条都必须测。
func TestStartProxyListensDualStack(t *testing.T) {
	svc, _, err := startProxy(api.NodeInfo{Type: "socks5", Port: 0, Value1: "u", Value2: "p"})
	if err != nil {
		t.Fatalf("startProxy 失败: %v", err)
	}
	defer svc.Close()

	port := waitProxyPort(t, svc)
	for _, host := range []string{"127.0.0.1", "::1"} {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 2*time.Second)
		if err != nil {
			t.Errorf("无法通过 %s 连接: %v", host, err)
			continue
		}
		conn.Close()
	}
}

// 绑了出口 IPv4 后,本机自连仍要通 —— 确认 interface 传 IP 不会把监听带坏。
func TestStartProxyWithEgressBind(t *testing.T) {
	ip := egressIPv4()
	if ip == "" {
		t.Skip("本机探不到出口 IPv4,跳过")
	}
	t.Logf("绑定出口 IPv4 = %s", ip)

	svc, _, err := startProxy(api.NodeInfo{Type: "socks5", Port: 0, Value1: "u", Value2: "p"})
	if err != nil {
		t.Fatalf("startProxy(绑定 %s)失败: %v", ip, err)
	}
	defer svc.Close()

	port := waitProxyPort(t, svc)
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(ip, port), 2*time.Second)
	if err != nil {
		t.Fatalf("无法连到绑定的 %s: %v", ip, err)
	}
	conn.Close()
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

// waitProxyPort 等监听就绪并返回端口。[::] 监听的 Addr() 形如 "[::]:1234",
// 不能直接当拨号目标(通配地址连不上),双栈测试要拿端口自己拼 host。
func waitProxyPort(t *testing.T, svc coresvc.Service) string {
	t.Helper()
	addr := waitProxyAddr(t, svc)
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatalf("解析监听地址 %q: %v", addr, err)
	}
	return port
}
