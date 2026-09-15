package detect

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agent/pkg/core"
	"agent/pkg/providers"
)

// 201 项 → 177 个唯一名 → 去掉 14 个 Func==nil 的地区占位 → 163。
// (lists.go 里注释掉的 "Now TV" 不算;有效条目共 201 条)
func TestAllItemsDedup(t *testing.T) {
	list := allItems()
	if len(list) != 163 {
		t.Fatalf("allItems() = %d 项, want 163", len(list))
	}
	seen := make(map[string]bool, len(list))
	for _, it := range list {
		if it.Func == nil {
			t.Fatalf("%q: Func 为 nil 的占位项不应保留", it.Name)
		}
		if seen[it.Name] {
			t.Fatalf("%q 重复出现", it.Name)
		}
		seen[it.Name] = true
	}
	// 保留首次出现:Netflix 来自 GlobeTests,Bahamut Anime 来自 HongKongTests
	for _, want := range []string{"Netflix", "Bahamut Anime", "Max"} {
		if !seen[want] {
			t.Errorf("缺少检测项 %q", want)
		}
	}
	if seen["GB"] || seen["IN"] {
		t.Error("地区占位项(GB/IN)不应保留")
	}
}

// onlyItems 把检测集换成指定的几项,退出时还原。
// 借的是 providers 的包级切片,所以本包测试不能并行跑。
func onlyItems(t *testing.T, items ...providers.TestItem) {
	t.Helper()
	lists := []*[]providers.TestItem{
		&providers.GlobeTests,
		&providers.HongKongTests,
		&providers.TaiwanTests,
		&providers.JapanTests,
		&providers.KoreaTests,
		&providers.NorthAmericaTests,
		&providers.SouthAmericaTests,
		&providers.EuropeTests,
		&providers.AfricaTests,
		&providers.SouthEastAsiaTests,
		&providers.OceaniaTests,
		&providers.AITests,
	}
	saved := make([][]providers.TestItem, len(lists))
	for i, p := range lists {
		saved[i] = *p
	}
	t.Cleanup(func() {
		for i, p := range lists {
			*p = saved[i]
		}
	})
	for i, p := range lists {
		if i == 0 {
			*p = items
			continue
		}
		*p = nil
	}
}

func TestAllMapsResult(t *testing.T) {
	onlyItems(t,
		providers.TestItem{Name: "Netflix", Func: func(core.HttpClient) core.Result {
			return core.Result{Status: core.StatusOK, Region: "JP", Info: "原生"}
		}},
		providers.TestItem{Name: "Disney+", Func: func(core.HttpClient) core.Result {
			return core.Result{Status: core.StatusNetworkErr, Err: errors.New("connection refused")}
		}},
	)

	got := All(context.Background())
	if len(got) != 2 {
		t.Fatalf("All() = %d 项, want 2", len(got))
	}
	if got[0].Name != "Netflix" || got[0].Status != core.StatusOK || got[0].Region != "JP" || got[0].Info != "原生" || got[0].Err != "" {
		t.Errorf("unexpected: %+v", got[0])
	}
	if !got[0].Unlocked() {
		t.Error("StatusOK 应判定为已解锁")
	}
	if got[1].Status != core.StatusNetworkErr || got[1].Err != "connection refused" {
		t.Errorf("error 未转成 string: %+v", got[1])
	}
	if got[1].Unlocked() {
		t.Error("网络错误不应判定为已解锁")
	}
}

func TestAllCanceledCtx(t *testing.T) {
	onlyItems(t, providers.TestItem{Name: "Netflix", Func: func(core.HttpClient) core.Result {
		t.Error("ctx 已取消,不应再发起检测")
		return core.Result{}
	}})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	got := All(ctx)
	if len(got) != 1 || got[0].Status != core.StatusNetworkErr || got[0].Err == "" {
		t.Fatalf("取消分支未覆盖: %+v", got)
	}
}

// 一个 provider panic 不能带走整个进程,也不能卡住同批其它项。
// (信号量没释放的话,第二项会在这里死住直到 go test 超时。)
func TestAllRecoversPanic(t *testing.T) {
	onlyItems(t,
		providers.TestItem{Name: "Boom", Func: func(core.HttpClient) core.Result {
			panic("provider 炸了")
		}},
		providers.TestItem{Name: "Netflix", Func: func(core.HttpClient) core.Result {
			return core.Result{Status: core.StatusOK}
		}},
	)

	got := All(context.Background())
	if len(got) != 2 {
		t.Fatalf("All() = %d 项, want 2", len(got))
	}
	if got[0].Name != "Boom" || got[0].Status != core.StatusFailed {
		t.Errorf("panic 未兜成 StatusFailed: %+v", got[0])
	}
	if !strings.Contains(got[0].Err, "provider 炸了") {
		t.Errorf("panic 内容没进 Err: %+v", got[0])
	}
	if got[1].Status != core.StatusOK {
		t.Errorf("同批其它项应照常完成: %+v", got[1])
	}
}
