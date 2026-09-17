package node

// 嵌入 GOST 在本机起 SOCKS5 / HTTP 服务,供落地节点连接。
// 详见 需求文档.md 4.2。节点是裸机,不依赖任何外部代理软件。

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"

	"agent/internal/api"

	clogger "github.com/go-gost/core/logger"
	coresvc "github.com/go-gost/core/service"
	gconfig "github.com/go-gost/x/config"
	gparsing "github.com/go-gost/x/config/parsing"
	gsvcparsing "github.com/go-gost/x/config/parsing/service"
	glogger "github.com/go-gost/x/logger"

	// handler / listener 工厂靠 init 自注册,不导入则 ParseService 找不到类型。
	_ "github.com/go-gost/x/handler/http"
	_ "github.com/go-gost/x/handler/socks/v5"
	_ "github.com/go-gost/x/listener/tcp"
)

// egressProbeAddr 只为让内核按路由表挑一个出口源地址,不真发包,
// 所以这个地址不需要可达,只要默认路由存在。
const egressProbeAddr = "8.8.8.8:53"

// ParseService 依赖 gost 的全局默认 logger 与 TLS 配置,这两项平时由 gost CLI 的
// config loader 设置;直接调 ParseService 就绕过了那步,不补会空指针 panic。
// gost 内部日志用 nop:Agent 自己走 slog。
func init() {
	clogger.SetDefault(glogger.Nop())
	gparsing.SetDefaultTLSConfig(&tls.Config{})
}

// 节点 type → GOST handler type。
func handlerType(nodeType string) (string, error) {
	switch nodeType {
	case "socks5":
		return "socks5", nil
	case "http":
		return "http", nil
	default:
		return "", fmt.Errorf("不支持的节点类型 %q", nodeType)
	}
}

// egressIPv4 取本机默认出口的 IPv4;探不到返回空串。
//
// 用 UDP "connect":UDP 无连接,这步只做本地路由查找,内核据路由表把源地址填进
// LocalAddr,不会真的发包。拿到的一定是本机真实持有、且默认路由选中的那个 IPv4。
//
// 刻意不用"问外部接口拿公网 IP"那种做法:机器在 NAT 后时公网 IP 不属于本机,
// 拿它当 bind 地址会直接失败。
func egressIPv4() string {
	conn, err := net.Dial("udp4", egressProbeAddr)
	if err != nil {
		// 探测失败(无默认路由等)不该拦住代理启动:退回不绑源地址,与改动前一致。
		slog.Warn("探测出口 IPv4 失败,不绑定源地址", "err", err)
		return ""
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || addr.IP.To4() == nil {
		slog.Warn("出口地址不是 IPv4,不绑定源地址", "addr", conn.LocalAddr())
		return ""
	}
	return addr.IP.String()
}

// proxyConfig 组装本机代理服务配置(纯函数,不监听端口)。
func proxyConfig(n api.NodeInfo) (*gconfig.ServiceConfig, error) {
	ht, err := handlerType(n.Type)
	if err != nil {
		return nil, err
	}
	handler := &gconfig.HandlerConfig{Type: ht}
	// 账号为空时 GOST 视为无需认证;空 AuthConfig 反而会绕一圈,所以只在有账号时设。
	if n.Value1 != "" {
		handler.Auth = &gconfig.AuthConfig{Username: n.Value1, Password: n.Value2}
	}
	// 监听地址用 IPv6 通配 "[::]":gost 的 tcp listener 见首字符是 '[' 就走 network="tcp",
	// 由 Go 开成 AF_INET6 + IPV6_V6ONLY=0 的双栈 socket,IPv4 连接以 v4-mapped 地址进来。
	// 写 "0.0.0.0" 会被 gost 的 IsIPv4 判成 v4 而走 "tcp4",v6 客户端就连不上了。
	cfg := &gconfig.ServiceConfig{
		Name:     "agent",
		Addr:     fmt.Sprintf("[::]:%d", n.Port),
		Listener: &gconfig.ListenerConfig{Type: "tcp"},
		Handler:  handler,
	}
	// gost 的 interface 收到 IP(而非网卡名)时只设 net.Dialer.LocalAddr、不绑设备,
	// 正是"以本机这个 IPv4 为源地址出网"的语义。
	if ip := egressIPv4(); ip != "" {
		cfg.Metadata = map[string]any{gparsing.MDKeyInterface: ip}
	}
	return cfg, nil
}

// startProxy 起本机代理服务;返回的 chan 在 Serve 返回(服务退出)时关闭,上层据此决定要不要重起。
func startProxy(n api.NodeInfo) (coresvc.Service, <-chan struct{}, error) {
	cfg, err := proxyConfig(n)
	if err != nil {
		return nil, nil, err
	}
	svc, err := gsvcparsing.ParseService(cfg)
	if err != nil {
		return nil, nil, err
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := svc.Serve(); err != nil {
			slog.Error("本机代理服务退出", "addr", cfg.Addr, "err", err)
		}
	}()
	return svc, done, nil
}
