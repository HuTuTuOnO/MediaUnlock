package node

// 嵌入 GOST 在本机起 SOCKS5 / HTTP 服务,供落地节点连接。
// 详见 需求文档.md 4.2。节点是裸机,不依赖任何外部代理软件。

import (
	"crypto/tls"
	"fmt"
	"log/slog"

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
	return &gconfig.ServiceConfig{
		Name:     "agent",
		Addr:     fmt.Sprintf("0.0.0.0:%d", n.Port),
		Listener: &gconfig.ListenerConfig{Type: "tcp"},
		Handler:  handler,
	}, nil
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
