package database

// 验证启动种子逻辑:固定 admin 账号(随机密码)、settings 默认值、以及幂等(不覆盖已有值)。

import (
	"path/filepath"
	"strconv"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"server/internal/models"
)

func TestSeedInitialData(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "seed.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	// 1) 首次种子:创建固定账号 admin,密码为随机 bcrypt 哈希。
	if err := SeedAdmin(db); err != nil {
		t.Fatalf("seed admin: %v", err)
	}
	if err := SeedSettings(db); err != nil {
		t.Fatalf("seed settings: %v", err)
	}

	var users []models.User
	if err := db.Find(&users).Error; err != nil {
		t.Fatalf("find users: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("user count = %d, want 1", len(users))
	}
	if users[0].Username != AdminUsername {
		t.Fatalf("username = %q, want %q", users[0].Username, AdminUsername)
	}
	if cost, err := bcrypt.Cost([]byte(users[0].PasswordHash)); err != nil || cost != bcrypt.DefaultCost {
		t.Fatalf("password hash invalid: cost=%d err=%v", cost, err)
	}

	var s models.Setting
	if err := db.Where("key = ?", models.SettingKeyTitle).First(&s).Error; err != nil {
		t.Fatalf("find title: %v", err)
	}
	if s.Value != DefaultSiteTitle {
		t.Fatalf("title = %q, want %q", s.Value, DefaultSiteTitle)
	}

	var tk models.Setting
	if err := db.Where("key = ?", models.SettingKeyToken).First(&tk).Error; err != nil {
		t.Fatalf("find token: %v", err)
	}
	if len(tk.Value) != 32 {
		t.Fatalf("token length = %d, want 32", len(tk.Value))
	}

	var rd models.Setting
	if err := db.Where("key = ?", models.SettingKeyRetentionDays).First(&rd).Error; err != nil {
		t.Fatalf("find retention_days: %v", err)
	}
	if rd.Value != strconv.Itoa(DefaultRetentionDays) {
		t.Fatalf("retention_days = %q, want %d", rd.Value, DefaultRetentionDays)
	}

	// 2) 幂等:重启再次种子不得覆盖管理员改过的 title,也不重复建号。
	if err := db.Model(&models.Setting{}).Where("key = ?", models.SettingKeyTitle).
		Update("value", "Custom").Error; err != nil {
		t.Fatalf("update title: %v", err)
	}
	if err := SeedSettings(db); err != nil {
		t.Fatalf("reseed settings: %v", err)
	}
	db.Where("key = ?", models.SettingKeyTitle).First(&s)
	if s.Value != "Custom" {
		t.Fatalf("SeedSettings overwrote existing title: %q", s.Value)
	}

	before := users[0].PasswordHash
	if err := SeedAdmin(db); err != nil {
		t.Fatalf("reseed admin: %v", err)
	}
	var after []models.User
	db.Find(&after)
	if len(after) != 1 {
		t.Fatalf("SeedAdmin created extra user: count=%d", len(after))
	}
	if after[0].PasswordHash != before {
		t.Fatal("SeedAdmin should not overwrite existing admin password")
	}
}
