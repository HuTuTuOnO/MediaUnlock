package retention

// 数据保留策略:unlocks 只保留最近 N 天,超期记录由后台 goroutine 定时删除。
//
// 单独成包而不放 database:database 只负责建库 / AutoMigrate / 首启种子;
// 保留策略是独立的业务规则,放一起会让 database 包职责越来越杂。
//
// 保留天数读 settings.retention_days(由 database.SeedSettings 写入默认值;
// 0 = 不自动清理;不设上限)。管理员在 Web「系统配置」里改,不用重启。

import (
	"log/slog"
	"strconv"
	"time"

	"gorm.io/gorm"

	"server/internal/models"
)

// DefaultInterval 定时清理的默认执行间隔。
const DefaultInterval = 6 * time.Hour

// Days 读取数据库中配置的保留天数(settings.retention_days)。
//
// 本包**不持有默认值**:默认值由 database.SeedSettings 写进数据库,这里只负责读当前值。
// 键缺失或值非法(非数字 / 负数)时返回 0 = 不自动清理 ——
// 宁可不清理,也不按猜出来的天数误删数据。
func Days(db *gorm.DB) int {
	var s models.Setting
	// 用 Limit(1).Find 而非 First:键可能不存在,避免打 record not found 噪音日志。
	// Setting 以 Key 为主键(没有 ID 字段),所以未命中时看 Key 是否为空。
	if err := db.Where("key = ?", models.SettingKeyRetentionDays).Limit(1).Find(&s).Error; err != nil || s.Key == "" {
		return 0
	}
	n, err := strconv.Atoi(s.Value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

// Cleanup 删除 created_at 早于 now-days 的 unlocks 记录,返回删除条数。
// days <= 0 表示不清理,直接返回 0。
func Cleanup(db *gorm.DB, days int) (int64, error) {
	if days <= 0 {
		return 0, nil
	}
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	res := db.Where("created_at < ?", cutoff).Delete(&models.Unlock{})
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

// Start 启动后台定期清理:先立即执行一次,之后每 DefaultInterval 重复执行。
// 间隔由本包自己决定,调用方不用关心;每次执行都重新读配置,
// 所以管理员改了保留天数,下次清理即生效,不用重启。
//
// 不返回"停止函数":当前项目没有平滑退出路径(r.Run 一直阻塞,进程退出 goroutine 随之消失),
// 留一个没人调用的 stop 只是死代码。将来真要接 SIGTERM 平滑退出时,再把停止能力加回来。
func Start(db *gorm.DB) {
	run := func() {
		days := Days(db)
		n, err := Cleanup(db, days)
		if err != nil {
			slog.Error("清理过期解锁记录失败", "days", days, "err", err)
			return
		}
		if n > 0 {
			slog.Info("已清理过期解锁记录", "days", days, "deleted", n)
		}
	}

	go func() {
		run() // 启动时先清一次
		ticker := time.NewTicker(DefaultInterval)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
}
