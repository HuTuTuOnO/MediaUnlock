package database

// GORM 初始化 + AutoMigrate + 首启种子(admin 与 settings 初始值)。
// 使用 glebarez/sqlite(纯 Go,无需 CGO),便于交叉编译成单二进制。

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/glebarez/sqlite"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"server/internal/models"
	"server/internal/utils"
)

// Open 连接 SQLite 并执行 AutoMigrate。
func Open(dbPath string) (*gorm.DB, error) {
	if dir := filepath.Dir(dbPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := db.AutoMigrate(models.AllModels()...); err != nil {
		return nil, err
	}
	return db, nil
}

// AdminUsername 管理员账号名,固定为 admin。
const AdminUsername = "admin"

// SeedAdmin 首次启动若无任何用户,创建 admin 并随机生成密码打印到日志。
// 账号固定 admin;密码不硬编码、每次随机生成(首次建号时)。
func SeedAdmin(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	pw, err := utils.RandomURLSafe(12)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := models.User{
		Username:     AdminUsername,
		PasswordHash: string(hash),
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}

	slog.Warn("初始管理员已创建,请尽快登录并修改密码",
		"username", AdminUsername, "password", pw)
	return nil
}

// DefaultSiteTitle settings.title 的初始默认值(与前端内置回退一致)。
const DefaultSiteTitle = "MediaUnlock"

// DefaultRetentionDays settings.retention_days 的种子默认值:unlocks 保留 7 天。
// 只在这里(种子)使用 —— retention 包不引用它,它只负责从数据库读当前配置。
// 0 表示不自动清理;不设上限,填多大都行。
const DefaultRetentionDays = 7

// SeedSettings 首次启动(或键缺失时)写入 settings 初始值:
//
//	title          —— 默认站点标题
//	token          —— client 模式全局只读 Token(随机生成)
//	retention_days —— unlocks 保留天数(默认 DefaultRetentionDays;0 = 不自动清理)
//
// 已存在的键不覆盖(OnConflict DoNothing),避免重启冲掉管理员在 Web 的修改。
func SeedSettings(db *gorm.DB) error {
	token, err := utils.RandomHex(16)
	if err != nil {
		return err
	}
	defaults := []models.Setting{
		{Key: models.SettingKeyTitle, Value: DefaultSiteTitle},
		{Key: models.SettingKeyToken, Value: token},
		{Key: models.SettingKeyRetentionDays, Value: strconv.Itoa(DefaultRetentionDays)},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error
}
