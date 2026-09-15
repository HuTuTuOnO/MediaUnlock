package client

// client 模式(落地节点):本地全平台检测 → 拉 unlocked → 分配(解析 + 测速 + 取最优节点)
// → 生成 soga 分流配置。本地检测只为对比,不上报、不写库、不心跳。详见 需求文档.md 4.3。
//
// 本文件只做编排,具体动作分别在 assign.go(分配)与 render.go(生成 + 写入)。

import (
	"context"
	"fmt"
	"log/slog"

	"agent/internal/api"
	"agent/internal/config"
	"agent/internal/detect"
)

type Runner struct {
	cfg *config.Config
	api *api.Client
}

func NewRunner(cfg *config.Config) *Runner {
	return &Runner{cfg: cfg, api: api.New(cfg.API, cfg.Token)}
}

// Run 执行一轮。
func (r *Runner) Run(ctx context.Context) error {
	local := detect.All(ctx)

	data, err := r.api.Unlocked()
	if err != nil {
		return fmt.Errorf("拉取下发数据: %w", err)
	}

	a := assign(local, data, r.cfg.Stack)
	if len(a.Platforms) == 0 {
		// 一条都选不出来时保留原配置:Server 空库、瞬时异常都会让这里为空,
		// 照常覆盖会把正在生效的分流整个清掉。
		slog.Warn("本轮没有可分流平台,保留原配置", "path", r.cfg.Render.Path)
		return nil
	}

	if err := render(r.cfg.Render.Type, r.cfg.Render.Path, a); err != nil {
		return fmt.Errorf("写入配置: %w", err)
	}
	slog.Info("分流配置已生成", "path", r.cfg.Render.Path, "platforms", len(a.Platforms))
	return nil
}
