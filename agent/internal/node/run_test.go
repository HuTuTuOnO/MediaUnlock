package node

import (
	"net"
	"testing"
	"time"

	"agent/internal/api"
)

func TestSameConn(t *testing.T) {
	base := api.NodeInfo{Type: "socks5", Port: 1080, Value1: "u", Value2: "p"}
	cases := []struct {
		name   string
		mutate func(api.NodeInfo) api.NodeInfo
		want   bool
	}{
		{"完全相同", func(n api.NodeInfo) api.NodeInfo { return n }, true},
		{"只改 alias", func(n api.NodeInfo) api.NodeInfo { n.Alias = "jp2"; return n }, true},
		{"只改 value3", func(n api.NodeInfo) api.NodeInfo { n.Value3 = "x"; return n }, true},
		{"改 type", func(n api.NodeInfo) api.NodeInfo { n.Type = "http"; return n }, false},
		{"改 port", func(n api.NodeInfo) api.NodeInfo { n.Port = 1081; return n }, false},
		{"改账号", func(n api.NodeInfo) api.NodeInfo { n.Value1 = "u2"; return n }, false},
		{"改密码", func(n api.NodeInfo) api.NodeInfo { n.Value2 = "p2"; return n }, false},
	}
	for _, c := range cases {
		if got := sameConn(base, c.mutate(base)); got != c.want {
			t.Errorf("%s: sameConn = %v, want %v", c.name, got, c.want)
		}
	}
}

// freePort 拿一个空闲端口(拿到后立刻释放,给 gost 用)。
func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("环境不允许 bind,跳过: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestEnsureProxyRestartsOnChange(t *testing.T) {
	r := &Runner{}
	port := freePort(t)
	a := api.NodeInfo{Type: "socks5", Port: port, Value1: "u", Value2: "p1"}

	r.ensureProxy(a)
	if r.proxy == nil {
		t.Fatal("首次应启动成功")
	}
	first := r.proxy
	defer func() {
		if r.proxy != nil {
			r.proxy.Close()
		}
	}()

	// 同样信息再来一轮:不该重启
	r.ensureProxy(a)
	if r.proxy != first {
		t.Fatal("连接信息未变时不该重启")
	}

	// 改密码、端口不变:必须释放旧监听再重新绑定同一端口
	b := a
	b.Value2 = "p2"
	r.ensureProxy(b)
	if r.proxy == nil {
		t.Fatal("密码变了应重启成功(同端口需先释放旧监听)")
	}
	if r.proxy == first {
		t.Fatal("密码变了却没重启")
	}
	t.Logf("重启后监听在 %v", r.proxy.Addr())
}

// 服务自己退出(Serve 返回)后,即使连接信息没变,也必须重起 —— 之前这种情况不会重起。
func TestEnsureProxyRestartsAfterExit(t *testing.T) {
	r := &Runner{}
	a := api.NodeInfo{Type: "socks5", Port: freePort(t), Value1: "u", Value2: "p"}

	r.ensureProxy(a)
	if r.proxy == nil {
		t.Fatal("首次应启动成功")
	}
	defer func() {
		if r.proxy != nil {
			r.proxy.Close()
		}
	}()

	// 关掉服务,模拟"运行中退出";Serve 返回后 done 应被关闭
	dead := r.proxy
	dead.Close()
	select {
	case <-r.done:
	case <-time.After(3 * time.Second):
		t.Fatal("服务退出后 done 应被关闭")
	}

	// 连接信息没变,但服务已经退出 → 必须换一个新的起来
	r.ensureProxy(a)
	if r.proxy == nil {
		t.Fatal("服务退出后应重新启动")
	}
	if r.proxy == dead {
		t.Fatal("应该换成一个新的服务,而不是留着已退出的那个")
	}
}
