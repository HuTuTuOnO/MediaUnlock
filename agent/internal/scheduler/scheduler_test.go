package scheduler

import "testing"

func TestNewAcceptsValidExpr(t *testing.T) {
	c, err := New("0 45 * * * *", func() {})
	if err != nil {
		t.Fatalf("合法表达式不应报错: %v", err)
	}
	if c == nil {
		t.Fatal("应返回可用的 *cron.Cron")
	}
	// main.go 会调 Start/Stop
	c.Start()
	c.Stop()
}

func TestNewRejectsInvalidExpr(t *testing.T) {
	for _, expr := range []string{"", "not a cron", "0 45 * * *"} { // 5 段缺秒,不合法
		if _, err := New(expr, func() {}); err == nil {
			t.Errorf("%q: 非法表达式应返回 error", expr)
		}
	}
}
