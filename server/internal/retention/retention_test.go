package retention

import (
	"path/filepath"
	"testing"
	"time"

	"gorm.io/gorm"

	"server/internal/database"
	"server/internal/models"
)

// setRetention 直接改 settings.retention_days 的值(可写入非法值以测回退)。
func setRetention(t *testing.T, db *gorm.DB, v string) {
	t.Helper()
	if err := db.Model(&models.Setting{}).
		Where("key = ?", models.SettingKeyRetentionDays).
		Update("value", v).Error; err != nil {
		t.Fatalf("set retention: %v", err)
	}
}

// TestDays 保留天数的读取:种子默认值 / 自定义 / 0(不清理)/ 非法回退 / 不设上限。
func TestDays(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "retention.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := database.SeedSettings(db); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	// 1) 读到的就是种子写进数据库的默认值
	if got := Days(db); got != database.DefaultRetentionDays {
		t.Fatalf("default retention = %d, want %d", got, database.DefaultRetentionDays)
	}

	// 2) 管理员改成 7 天
	setRetention(t, db, "7")
	if got := Days(db); got != 7 {
		t.Fatalf("retention = %d, want 7", got)
	}

	// 3) 改成 0 = 不自动清理(这是合法值,不能回退成默认)
	setRetention(t, db, "0")
	if got := Days(db); got != 0 {
		t.Fatalf("retention = %d, want 0 (不清理)", got)
	}

	// 4) 非法值 / 负数 → 返回 0(不清理):宁可不清理,也不按猜的天数误删数据。
	//    注意这里**不是**回退成默认值 —— 默认值只属于种子(database),retention 只管读。
	setRetention(t, db, "abc")
	if got := Days(db); got != 0 {
		t.Fatalf("invalid retention = %d, want 0 (不清理)", got)
	}
	setRetention(t, db, "-1")
	if got := Days(db); got != 0 {
		t.Fatalf("negative retention = %d, want 0 (不清理)", got)
	}

	// 5) 不设上限:很大的值照原样返回(等价于"永不自动清理")
	setRetention(t, db, "99999")
	if got := Days(db); got != 99999 {
		t.Fatalf("retention = %d, want 99999 (不设上限)", got)
	}
}

// TestCleanup 只删超过保留期的记录;days<=0 表示不清理,一条都不删。
func TestCleanup(t *testing.T) {
	db, err := database.Open(filepath.Join(t.TempDir(), "cleanup.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	node := models.Node{Name: "n1", Alias: "n1", Type: models.NodeTypeSOCKS5}
	plat := models.Platform{Name: "p1", Status: models.StatusEnabled}
	if err := db.Create(&node).Error; err != nil {
		t.Fatalf("create node: %v", err)
	}
	if err := db.Create(&plat).Error; err != nil {
		t.Fatalf("create platform: %v", err)
	}

	now := time.Now()
	mk := func(daysAgo int) models.Unlock {
		return models.Unlock{
			NodeID: node.ID, PlatformID: plat.ID, Status: models.UnlockStatusSuccess,
			CreatedAt: now.Add(-time.Duration(daysAgo) * 24 * time.Hour),
		}
	}
	seed := []models.Unlock{mk(2), mk(10), mk(40)}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed unlocks: %v", err)
	}

	// days=0 → 不清理
	n, err := Cleanup(db, 0)
	if err != nil {
		t.Fatalf("cleanup days=0: %v", err)
	}
	if n != 0 {
		t.Fatalf("days=0 deleted %d, want 0", n)
	}

	// days=30 → 只删 40 天前那条
	n, err = Cleanup(db, 30)
	if err != nil {
		t.Fatalf("cleanup days=30: %v", err)
	}
	if n != 1 {
		t.Fatalf("days=30 deleted %d, want 1", n)
	}

	// days=7 → 再删 10 天前那条
	n, err = Cleanup(db, 7)
	if err != nil {
		t.Fatalf("cleanup days=7: %v", err)
	}
	if n != 1 {
		t.Fatalf("days=7 deleted %d, want 1", n)
	}

	// 只剩 2 天前那条
	var left int64
	if err := db.Model(&models.Unlock{}).Count(&left).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if left != 1 {
		t.Fatalf("remaining = %d, want 1", left)
	}
}
