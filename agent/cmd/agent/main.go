package main

// Agent 入口:读 config.yml → 按 mode 分发(node / client)→ 起 cron(scheduler)定时执行。
// 详见 需求文档.md 4. Agent。

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"agent/internal/client"
	"agent/internal/config"
	"agent/internal/node"
	"agent/internal/scheduler"
)

func main() {
	cfgPath := flag.String("config", "config.yml", "path to config.yml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("加载配置失败", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Mode {
	case config.ModeNode:
		runNode(ctx, cfg)
	case config.ModeClient:
		runClient(ctx, cfg)
	}
}

func runNode(ctx context.Context, cfg *config.Config) {
	r := node.NewRunner(cfg) // 检测走 detect.All(fork 自 MediaUnlockTest pkg/,全平台)

	cron, err := scheduler.New(cfg.Scheduler, func() {
		if err := r.Tick(ctx); err != nil {
			slog.Error("node 运行失败", "err", err)
		}
	})
	if err != nil {
		slog.Error("解析 scheduler 失败", "err", err)
		os.Exit(1)
	}

	slog.Info("agent node 模式启动", "api", cfg.API, "scheduler", cfg.Scheduler)

	// 代理保活走内置周期(见 node.proxyInterval),和检测上报的 cron 分开
	go r.Watch(ctx)

	// 启动即先跑一次,不必等第一个 cron 周期
	if err := r.Tick(ctx); err != nil {
		slog.Error("node 运行失败", "err", err)
	}
	cron.Start()

	<-ctx.Done()
	cron.Stop()
	slog.Info("agent 退出")
}

func runClient(ctx context.Context, cfg *config.Config) {
	r := client.NewRunner(cfg) // 与 node 同一套全平台检测(固定 IPv4)

	cron, err := scheduler.New(cfg.Scheduler, func() {
		if err := r.Run(ctx); err != nil {
			slog.Error("client 运行失败", "err", err)
		}
	})
	if err != nil {
		slog.Error("解析 scheduler 失败", "err", err)
		os.Exit(1)
	}

	slog.Info("agent client 模式启动", "api", cfg.API, "scheduler", cfg.Scheduler,
		"stack", cfg.Stack, "render", cfg.Render.Path)
	// 启动即先跑一次,不必等第一个 cron 周期
	if err := r.Run(ctx); err != nil {
		slog.Error("client 运行失败", "err", err)
	}
	cron.Start()

	<-ctx.Done()
	cron.Stop()
	slog.Info("agent 退出")
}
