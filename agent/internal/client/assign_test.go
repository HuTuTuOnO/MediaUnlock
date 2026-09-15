package client

import (
	"net"
	"testing"
	"time"

	"agent/internal/api"
	"agent/internal/detect"
	"agent/pkg/core"
)

// livePort 起一个本地监听并返回端口,测试结束自动关闭。
func livePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("环境不允许 bind,跳过: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port
}

// closedPort 拿一个当前没人监听的端口(拿到后立刻释放)。
func closedPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("环境不允许 bind,跳过: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port
}

func TestResolveDefaultKeepsHost(t *testing.T) {
	// default 不提前解析:域名、IPv4、IPv6 都原样返回
	for _, host := range []string{"jp.example.com", "1.2.3.4", "2001:db8::1"} {
		got, err := resolve(host, stackDefault)
		if err != nil || got != host {
			t.Errorf("resolve(%q, default) = %q, %v; want 原样返回", host, got, err)
		}
	}
}

func TestResolveLiteralKeepsMatchingStack(t *testing.T) {
	cases := []struct {
		host  string
		stack string
		ok    bool
	}{
		{"1.2.3.4", stackIPv4, true},
		{"2001:db8::1", stackIPv6, true},
		{"2001:db8::1", stackIPv4, false},
		{"1.2.3.4", stackIPv6, false},
	}
	for _, c := range cases {
		got, err := resolve(c.host, c.stack)
		if c.ok && (err != nil || got != c.host) {
			t.Errorf("resolve(%q, %s) = %q, %v; want 原样返回", c.host, c.stack, got, err)
		}
		if !c.ok && err == nil {
			t.Errorf("resolve(%q, %s) 应报错(协议栈不符,该节点要移除)", c.host, c.stack)
		}
	}
}

func TestTcping(t *testing.T) {
	if _, err := tcping("127.0.0.1", livePort(t)); err != nil {
		t.Fatalf("监听中的端口应探测成功: %v", err)
	}
	if _, err := tcping("127.0.0.1", closedPort(t)); err == nil {
		t.Error("没人监听的端口应探测失败")
	}
}

// 所有 node 都测,但解析不出地址 / 探测不通的不进结果。
func TestProbeNodeDropsUnreachable(t *testing.T) {
	live, dead := livePort(t), closedPort(t)
	enabled := api.StatusEnabled

	got := probeNode(map[string]api.UnlockedNode{
		"live": {Type: "socks5", Host: "127.0.0.1", Port: live, Value1: "u", Value2: "p", Status: &enabled},
		"dead": {Type: "socks5", Host: "127.0.0.1", Port: dead, Status: &enabled},
	}, stackDefault)

	n, ok := got["live"]
	if !ok {
		t.Fatalf("探得通的节点应进结果: %+v", got)
	}
	if n.Host != "127.0.0.1" || n.Port != live || n.Value1 != "u" {
		t.Errorf("原节点信息没带上: %+v", n)
	}
	if n.Delay <= 0 {
		t.Errorf("延迟应大于 0, got %v", n.Delay)
	}
	if _, ok := got["dead"]; ok {
		t.Error("探测不通的节点不该进结果")
	}
}

// stack=4/6 下,协议栈不符的 IP 字面量会让整个节点不可用(连拨都不拨)。
func TestProbeNodeDropsMismatchedStack(t *testing.T) {
	port := livePort(t)
	enabled := api.StatusEnabled
	in := map[string]api.UnlockedNode{
		"v4": {Type: "socks5", Host: "127.0.0.1", Port: port, Status: &enabled},
		"v6": {Type: "socks5", Host: "::1", Port: port, Status: &enabled},
	}

	got := probeNode(in, stackIPv4)
	if _, ok := got["v4"]; !ok {
		t.Errorf("IPv4 字面量在 stack=4 下应可用: %+v", got)
	}
	if _, ok := got["v6"]; ok {
		t.Error("IPv6 字面量在 stack=4 下应被移除")
	}

	got = probeNode(in, stackIPv6)
	if _, ok := got["v4"]; ok {
		t.Error("IPv4 字面量在 stack=6 下应被移除")
	}
}

// stack=default 不提前解析域名:host 原样写回。
// 借 localhost 验证 —— 它是域名,本机能解析所以拨得通,但结果里 Host 仍应是 "localhost" 而不是某个 IP。
func TestProbeNodeKeepsDomainOnDefaultStack(t *testing.T) {
	enabled := api.StatusEnabled
	got := probeNode(map[string]api.UnlockedNode{
		"jp": {Type: "socks5", Host: "localhost", Port: livePort(t), Status: &enabled},
	}, stackDefault)

	n, ok := got["jp"]
	if !ok {
		t.Fatalf("localhost 应拨得通: %+v", got)
	}
	if n.Host != "localhost" {
		t.Errorf("default 不该提前解析,Host 应保持 %q, got %q", "localhost", n.Host)
	}
}

// 取延迟最优的那个,并排除禁用的、类型生成不了出口的、以及探测失败的。
func TestAssignPlatformsPicksFastestAndSkips(t *testing.T) {
	enabled, disabled := api.StatusEnabled, api.StatusDisabled
	nodes := map[string]node{
		"jp1": {UnlockedNode: api.UnlockedNode{Type: "socks5", Status: &enabled}, Delay: 30 * time.Millisecond},
		"jp2": {UnlockedNode: api.UnlockedNode{Type: "socks5", Status: &enabled}, Delay: 10 * time.Millisecond},
		"us1": {UnlockedNode: api.UnlockedNode{Type: "socks5", Status: &disabled}, Delay: time.Millisecond},
		"bad": {UnlockedNode: api.UnlockedNode{Type: "vmess", Status: &enabled}, Delay: 2 * time.Millisecond},
	}
	platforms := map[string]api.UnlockedPlatform{
		// us1 延迟最低、bad 次低,但一个被禁用、一个类型生成不了出口;ghost 不在探测结果里 → 取 jp2
		"Hulu": {Aliases: []string{"jp1", "jp2", "us1", "bad", "ghost"}, Rules: []string{"domain:hulu.com"}, Status: &enabled},
		// 候选全是不能用的 → 整个平台跳过
		"Max": {Aliases: []string{"us1", "bad", "ghost"}, Rules: []string{"domain:max.com"}, Status: &enabled},
	}

	got := assignPlatforms(nil, platforms, nodes)
	if len(got) != 1 {
		t.Fatalf("只有 Hulu 有可用节点: %+v", got)
	}
	p, ok := got["Hulu"]
	if !ok || p.Alias != "jp2" || len(p.Rules) != 1 || p.Rules[0] != "domain:hulu.com" {
		t.Errorf("Hulu 应取 jp2 并带上 rules: %+v", p)
	}
}

// 平台禁用 / 本机已解锁 / 没有 rules / 没有可用节点,都不参与分流。
func TestAssignPlatformsSkips(t *testing.T) {
	enabled, disabled := api.StatusEnabled, api.StatusDisabled
	nodes := map[string]node{
		"jp1": {UnlockedNode: api.UnlockedNode{Type: "socks5", Status: &enabled}, Delay: time.Millisecond},
	}
	platforms := map[string]api.UnlockedPlatform{
		"Hulu":    {Aliases: []string{"jp1"}, Rules: []string{"domain:hulu.com"}, Status: &enabled},
		"Max":     {Aliases: []string{"jp1"}, Rules: []string{"domain:max.com"}, Status: &disabled},
		"ZDF":     {Aliases: []string{"jp1"}, Rules: nil, Status: &enabled},
		"AMC+":    {Aliases: []string{"ghost"}, Rules: []string{"domain:amc.com"}, Status: &enabled},
		"Netflix": {Aliases: []string{"jp1"}, Rules: []string{"domain:netflix.com"}, Status: &enabled},
	}
	local := []detect.Result{{Name: "Netflix", Status: core.StatusOK}} // 本机已解锁

	got := assignPlatforms(local, platforms, nodes)
	if len(got) != 1 {
		t.Fatalf("只剩 Hulu 该参与分流: %+v", got)
	}
	p, ok := got["Hulu"]
	if !ok || p.Alias != "jp1" || len(p.Rules) != 1 || p.Rules[0] != "domain:hulu.com" {
		t.Errorf("Hulu 分配不对: %+v", p)
	}
}

// 走完整条链路:探得通的节点进节点层,平台只指向探得通的节点。
func TestAssignEndToEnd(t *testing.T) {
	live, dead := livePort(t), closedPort(t)
	enabled := api.StatusEnabled

	a := assign(nil, &api.UnlockedData{
		Nodes: map[string]api.UnlockedNode{
			"JP": {Type: "socks5", Host: "127.0.0.1", Port: live, Value1: "u", Status: &enabled},
			"US": {Type: "socks5", Host: "127.0.0.1", Port: dead, Status: &enabled},
		},
		Platforms: map[string]api.UnlockedPlatform{
			"Hulu":  {Aliases: []string{"JP"}, Rules: []string{"domain:hulu.com"}, Status: &enabled},
			"FXNOW": {Aliases: []string{"US"}, Rules: []string{"domain:fxnow.com"}, Status: &enabled},
		},
	}, stackDefault)

	if len(a.Nodes) != 1 {
		t.Fatalf("节点层只该剩探得通的: %+v", a.Nodes)
	}
	if len(a.Platforms) != 1 || a.Platforms["Hulu"].Alias != "JP" {
		t.Fatalf("只有 Hulu 能分到可用节点: %+v", a.Platforms)
	}
}
