package node

// node 模式(解锁节点):常驻保持本机代理服务在跑;按 scheduler 定时检测 + 上报。
// 详见 需求文档.md 4.2。

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"agent/internal/api"
	"agent/internal/config"
	"agent/internal/detect"

	coresvc "github.com/go-gost/core/service"
)

// proxyInterval 拉本节点信息 + 保证代理在跑的周期。写死,不跟 scheduler:
// 代理掉线、或后台改了端口密码,不该等一个检测周期(可能一小时)才生效。
const proxyInterval = 30 * time.Second

type Runner struct {
	cfg    *config.Config
	api    *api.Client
	proxy  coresvc.Service // 懒启动,启动成功才记住
	served api.NodeInfo    // r.proxy 是按这份信息起的
	done   <-chan struct{} // r.proxy 的 Serve 返回时关闭
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg, api: api.New(cfg.API, cfg.Token)}
}

// Watch 常驻:按内置周期拉本节点信息,并保证本机代理在跑,直到 ctx 结束。
func (r *Runner) Watch(ctx context.Context) {
	for {
		ni, err := r.api.Node()
		if err != nil {
			slog.Error("拉取本节点信息失败", "err", err)
		} else {
			r.ensureProxy(*ni)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(proxyInterval):
		}
	}
}

// Tick 执行一轮全平台检测并上报。
func (r *Runner) Tick(ctx context.Context) error {
	results := detect.All(ctx)
	// ctx 已取消时每项都是"网络错误",拿这种结果上报会把本节点的解锁关联清空(服务端用本次
	// status==1 的平台整体替换)。放弃本轮。
	if ctx.Err() != nil {
		slog.Warn("检测被取消,跳过本轮上报", "err", ctx.Err())
		return nil
	}

	items := make([]api.ReportItem, 0, len(results))
	unlocked := 0
	for _, res := range results {
		items = append(items, api.ReportItem{
			Name:   res.Name,
			Status: res.Status,
			Region: res.Region,
			Info:   res.Info,
			Err:    res.Err,
		})
		if res.Unlocked() {
			unlocked++
		}
	}

	if err := r.api.Report(items); err != nil {
		return fmt.Errorf("上报检测结果: %w", err)
	}
	slog.Info("node 检测上报完成", "platforms", len(items), "unlocked", unlocked)
	return nil
}

// ensureProxy 保证本机代理服务在跑:服务已退出、或连接信息变了,就重启。
// 启动失败不阻断(代理没起来不影响解锁数据),下一轮再试。
func (r *Runner) ensureProxy(n api.NodeInfo) {
	if r.proxy != nil {
		switch {
		case exited(r.done):
			slog.Warn("本机代理服务已退出,重新启动", "type", n.Type, "port", n.Port)
		case sameConn(r.served, n):
			return
		default:
			slog.Info("节点连接信息已变化,重启本机代理服务", "alias", n.Alias)
			r.proxy.Close()
		}
		r.proxy, r.done = nil, nil
	}

	svc, done, err := startProxy(n)
	if err != nil {
		slog.Error("启动本机代理服务失败", "type", n.Type, "port", n.Port, "err", err)
		return
	}
	r.proxy, r.done, r.served = svc, done, n
	slog.Info("本机代理服务已启动", "type", n.Type, "port", n.Port)
}

// exited 非阻塞检查服务是否已退出。
func exited(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}

// sameConn 只比代理真正用到的字段;alias 之类改名不该踢掉在线连接。
func sameConn(a, b api.NodeInfo) bool {
	return a.Type == b.Type && a.Port == b.Port && a.Value1 == b.Value1 && a.Value2 == b.Value2
}
