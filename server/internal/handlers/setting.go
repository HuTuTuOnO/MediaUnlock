package handlers

// /api/settings:系统设置(键值)。当前已知键:
//   title          —— 站点标题(登录页/侧边栏显示,/api/common/settings 公开读取)
//   token          —— client 模式 Agent 拉取 unlocked 用的全局只读 Token
//   retention_days —— unlocks 保留天数(非负整数;0 = 不自动清理)
// 存储为通用 key/value,PUT 做 upsert,便于后期增键而不改表结构。

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"server/internal/models"
	"server/internal/utils"
)

// 已知配置键(白名单:PUT 只接受这些,避免写入任意脏键)。
// 键名常量统一定义在 models,供种子初始化复用。
const (
	SettingTitle         = models.SettingKeyTitle
	SettingToken         = models.SettingKeyToken
	SettingRetentionDays = models.SettingKeyRetentionDays
)

var knownSettingKeys = map[string]bool{
	SettingTitle:         true,
	SettingToken:         true,
	SettingRetentionDays: true,
}

// settingsMap 读全部设置为 map;供 GetSettings 与 UpdateSettings 复用。
func (h *H) settingsMap() (map[string]string, error) {
	var rows []models.Setting
	if err := h.DB.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, r := range rows {
		out[r.Key] = r.Value
	}
	return out, nil
}

// GET /api/settings → data: { "title", "token", "retention_days" }
// 已知键即使未落库也补空串,方便前端受控输入不出现 undefined。
func (h *H) GetSettings(c *gin.Context) {
	out, err := h.settingsMap()
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}
	for k := range knownSettingKeys {
		if _, ok := out[k]; !ok {
			out[k] = ""
		}
	}
	utils.Success(c, out)
}

// PUT /api/settings  body: { "title": "xxx", "token": "yyy" }
// 只 upsert 传入的、且在白名单内的键;未传的键不动。
func (h *H) UpdateSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}
	for k := range req {
		if !knownSettingKeys[k] {
			utils.BadRequest(c, "unknown setting key: "+k)
			return
		}
	}

	// retention_days 必须是非负整数;0 表示不自动清理,不设上限(填多大都行)。
	// 不校验的话,写个 "abc" 进去会让定时清理任务每次都回退到默认值、静默失效。
	if raw, ok := req[SettingRetentionDays]; ok {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 0 {
			utils.BadRequest(c, "retention_days must be a non-negative integer (0 = never clean up)")
			return
		}
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for k, v := range req {
			s := models.Setting{Key: k, Value: v}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "key"}},
				DoUpdates: clause.AssignmentColumns([]string{"value"}),
			}).Create(&s).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}

	out, err := h.settingsMap()
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Success(c, out)
}

// POST /api/settings/retoken —— 重新生成全局只读 Token(settings.token),返回新值。
func (h *H) RetokenSetting(c *gin.Context) {
	token, err := utils.RandomHex(16)
	if err != nil {
		utils.ServerError(c, "token error")
		return
	}
	s := models.Setting{Key: SettingToken, Value: token}
	if err := h.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value"}),
	}).Create(&s).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Success(c, gin.H{"token": token})
}
