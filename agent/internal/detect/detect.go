package detect

// 调用 pkg/providers 的全平台检测,归一成 Result。
// 详见 需求文档.md 4.5 MediaUnlockTest 接入方式。

import (
	"context"
	"fmt"
	"sync"

	"agent/pkg/core"
	"agent/pkg/providers"
)

// maxConcurrent 同时进行的检测数;检测项全是网络等待,12 路足够。
const maxConcurrent = 12

// ipVersion 检测固定走 IPv4,与 client 的 stack 无关。
const ipVersion = 4

// Result 单个平台的检测结果(Err 已由 error 转成 string,便于上报与日志)。
type Result struct {
	Name   string
	Status int
	Region string
	Info   string
	Err    string
}

// Unlocked 本机是否解锁该平台。
func (r Result) Unlocked() bool { return r.Status == core.StatusOK }

// allItems 合并各地区切片,按 Name 去重并保留首次出现,跳过 Func 为 nil 的地区占位项。
func allItems() []providers.TestItem {
	lists := [][]providers.TestItem{
		providers.GlobeTests,
		providers.HongKongTests,
		providers.TaiwanTests,
		providers.JapanTests,
		providers.KoreaTests,
		providers.NorthAmericaTests,
		providers.SouthAmericaTests,
		providers.EuropeTests,
		providers.AfricaTests,
		providers.SouthEastAsiaTests,
		providers.OceaniaTests,
		providers.AITests,
	}
	var out []providers.TestItem
	seen := make(map[string]bool)
	for _, list := range lists {
		for _, item := range list {
			if item.Func == nil || seen[item.Name] {
				continue
			}
			seen[item.Name] = true
			out = append(out, item)
		}
	}
	return out
}

// All 跑一轮全平台检测,结果顺序与 allItems 一致。
func All(ctx context.Context) []Result {
	items := allItems()
	results := make([]Result, len(items))

	sem := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	for i, item := range items {
		// 按下标写回,保证输出顺序与 items 一致
		wg.Add(1)
		go func(i int, item providers.TestItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			// 必须在 wg.Done 之后注册:LIFO 保证先写回结果再 Done,否则主协程可能读到零值。
			// provider panic 会带走整个常驻进程,这里兜成一条 StatusFailed。
			defer func() {
				if r := recover(); r != nil {
					results[i] = Result{Name: item.Name, Status: core.StatusFailed, Err: fmt.Sprintf("panic: %v", r)}
				}
			}()
			if err := ctx.Err(); err != nil {
				results[i] = Result{Name: item.Name, Status: core.StatusNetworkErr, Err: err.Error()}
				return
			}
			// 每项各建一个 client:providers 里有几个会改 client 自身状态
			// (SetCookieJar / SetFollowRedirect),共享会被并发写坏,也会跨平台串味。
			client := core.NewHttpClient(ipVersion)
			raw := item.Func(client)

			one := Result{Name: item.Name, Status: raw.Status, Region: raw.Region, Info: raw.Info}
			// error 必须先转 string,直接 JSON 出去是 {}
			if raw.Err != nil {
				one.Err = raw.Err.Error()
			}
			results[i] = one
		}(i, item)
	}
	wg.Wait()
	return results
}
