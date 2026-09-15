package main

// Server 入口:读 config.yml → 初始化 DB(AutoMigrate + 首启种子)→ 挂后台任务 → 装配路由 → 启动。
// 详见 需求文档.md 3. Server。
//
// 这里只做"把各部分接起来",不放任何业务细节:
// 过期数据清理的策略、间隔、goroutine 都在 internal/retention 包内,main 只调一行。

import (
	"flag"
	"log/slog"
	"os"

	"server/internal/config"
	"server/internal/database"
	"server/internal/retention"
	"server/internal/router"
)

func main() {
	cfgPath := flag.String("config", "config.yml", "path to config.yml")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		slog.Error("加载配置失败", "path", *cfgPath, "err", err)
		os.Exit(1)
	}

	db, err := database.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("初始化数据库失败", "err", err)
		os.Exit(1)
	}
	if err := database.SeedAdmin(db); err != nil {
		slog.Error("种子管理员失败", "err", err)
		os.Exit(1)
	}
	if err := database.SeedSettings(db); err != nil {
		slog.Error("初始化系统设置失败", "err", err)
		os.Exit(1)
	}

	// 后台任务:过期解锁记录清理(启动立即执行一次,之后按固定间隔重复;间隔在 retention 包内)
	retention.Start(db)

	r := router.New(db, cfg)

	slog.Info("Server 启动", "addr", cfg.Server.Addr)
	if err := r.Run(cfg.Server.Addr); err != nil {
		slog.Error("HTTP 服务退出", "err", err)
		os.Exit(1)
	}
}
