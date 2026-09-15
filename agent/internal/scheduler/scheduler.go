package scheduler

// cron(6 段含秒)调度封装。详见 需求文档.md 4. Agent。

import (
	"log/slog"
	"sync/atomic"

	"github.com/robfig/cron/v3"
)

// New 按 cron 表达式(6 段含秒)创建调度器;表达式非法返回 error。
// 上一轮未跑完时跳过本轮,避免检测/上报任务堆叠。
func New(expr string, fn func()) (*cron.Cron, error) {
	c := cron.New(cron.WithSeconds())
	var running atomic.Bool
	if _, err := c.AddFunc(expr, func() {
		if !running.CompareAndSwap(false, true) {
			slog.Warn("上一轮尚未结束,跳过本次调度", "scheduler", expr)
			return
		}
		defer running.Store(false)
		fn()
	}); err != nil {
		return nil, err
	}
	return c, nil
}
