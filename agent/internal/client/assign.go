package client

// 分配:先给下发里的所有 node 按 stack 解析地址 + 测一次 TCP 握手延迟(每个 node 只测一次),
// 再为每个平台取出延迟最优的那个节点。render 拿这两层直接生成配置。
// 详见 需求文档.md 4.3。

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"agent/internal/api"
	"agent/internal/detect"
)

// stack 取值(与 config 的校验一致)。
const (
	stackDefault = "default"
	stackIPv4    = "4"
	stackIPv6    = "6"
)

// dnsTimeout 单次 DNS 查询超时;probeTimeout 单个节点 TCP 探测超时。
const (
	dnsTimeout   = 5 * time.Second
	probeTimeout = 3 * time.Second
)

// publicDNS 解析节点域名固定走这两个公共 DNS,不用本机 resolver。
var publicDNS = []string{"8.8.8.8:53", "1.1.1.1:53"}

type assignment struct {
	Platforms map[string]platform
	Nodes     map[string]node
}

type platform struct {
	Rules []string
	Alias string // 能解锁它、且延迟最优的节点
}

type node struct {
	api.UnlockedNode
	Delay time.Duration
}

func assign(local []detect.Result, data *api.UnlockedData, stack string) assignment {
	nodes := probeNode(data.Nodes, stack)
	return assignment{
		Platforms: assignPlatforms(local, data.Platforms, nodes),
		Nodes:     nodes,
	}
}

func probeNode(in map[string]api.UnlockedNode, stack string) map[string]node {
	out := make(map[string]node, len(in))
	for alias, n := range in {
		host, err := resolve(n.Host, stack)
		if err != nil {
			slog.Warn("节点地址解析失败,不可用", "alias", alias, "err", err)
			continue
		}
		delay, err := tcping(host, n.Port)
		if err != nil {
			slog.Warn("节点探测不通,不可用", "alias", alias, "err", err)
			continue
		}
		n.Host = host // 换成解析后的地址:后面写进配置、拿去做判断的都是它
		out[alias] = node{UnlockedNode: n, Delay: delay}
	}
	slog.Info("节点探测完成", "total", len(in), "available", len(out))
	return out
}

// resolve 按 stack 解析节点地址,返回最终写进配置的 host。
//
//	default: 原样返回 —— 域名不提前解析,由 soga 所在系统自己决定走 v4 还是 v6
//	4/6: 强制对应协议栈;原始 host 是另一栈的 IP、或域名查不到该栈记录 → error(该节点被移除)
func resolve(host, stack string) (string, error) {
	if stack == stackDefault {
		return host, nil
	}
	want4 := stack == stackIPv4
	if ip := net.ParseIP(host); ip != nil {
		if (ip.To4() != nil) == want4 {
			return host, nil
		}
		return "", fmt.Errorf("%s 不是 %s 地址", host, stack)
	}

	network := "ip6"
	if want4 {
		network = "ip4"
	}
	ips, err := lookupPublicIP(network, host)
	if err != nil {
		return "", fmt.Errorf("解析 %s: %w", host, err)
	}
	if len(ips) == 0 {
		return "", fmt.Errorf("%s 查不到 %s 记录", host, stack)
	}
	return ips[0].String(), nil
}

// lookupPublicIP 用公共 DNS 查 A / AAAA 记录(network 为 ip4 / ip6),不走本机 resolver。
func lookupPublicIP(network, host string) ([]net.IP, error) {
	r := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, netw, _ string) (net.Conn, error) {
			var lastErr error
			for _, addr := range publicDNS {
				conn, err := (&net.Dialer{Timeout: dnsTimeout}).DialContext(ctx, netw, addr)
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			return nil, lastErr
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()
	return r.LookupIP(ctx, network, host)
}

// tcping 对 host:port 做一次 TCP 连接,返回握手耗时。
func tcping(host string, port int) (time.Duration, error) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), probeTimeout)
	if err != nil {
		return 0, err
	}
	conn.Close()
	return time.Since(start), nil
}

func assignPlatforms(local []detect.Result, platforms map[string]api.UnlockedPlatform, nodes map[string]node) map[string]platform {
	unlocked := make(map[string]bool, len(local))
	for _, res := range local {
		if res.Unlocked() {
			unlocked[res.Name] = true
		}
	}

	out := make(map[string]platform, len(platforms))
	for name, p := range platforms {
		switch {
		case p.Status != nil && *p.Status == api.StatusDisabled:
			slog.Debug("平台已禁用,跳过", "platform", name)
			continue
		case unlocked[name]:
			continue // 本机已解锁,直接走本地出口,省一跳
		case len(p.Rules) == 0:
			slog.Warn("平台没有分流规则,跳过", "platform", name)
			continue
		}

		best, ok := bestAlias(p.Aliases, nodes)
		if !ok {
			slog.Warn("平台没有可用解锁节点,跳过", "platform", name)
			continue
		}
		out[name] = platform{Rules: p.Rules, Alias: best}
	}
	return out
}

// bestAlias 在候选里取延迟最优的一个。
func bestAlias(aliases []string, nodes map[string]node) (string, bool) {
	best := ""
	var bestDelay time.Duration
	for _, alias := range aliases {
		n, ok := nodes[alias]
		if !ok {
			continue
		}
		if n.Status != nil && *n.Status == api.StatusDisabled {
			continue
		}
		if _, err := outType(n.Type); err != nil {
			continue
		}
		if best == "" || n.Delay < bestDelay {
			best, bestDelay = alias, n.Delay
		}
	}
	return best, best != ""
}
