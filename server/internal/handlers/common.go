package handlers

// /api/common/*:前端通用接口。
//   GET /api/common/settings —— 公开:仅返回站点标题,供登录页 / 侧边栏展示。
//   GET /api/common/stats    —— 需登录:仪表盘统计计数。

import (
	"github.com/gin-gonic/gin"

	"server/internal/models"
	"server/internal/utils"
)

// GET /api/common/settings → { title }(公开,无需鉴权)
func (h *H) CommonSettings(c *gin.Context) {
	// 用 Find 而非 First:键缺失属正常情况,不应产生 "record not found" 日志。
	// (启动时 SeedSettings 已写入默认值,此处仅作兜底 —— 键缺失就返回空串,
	//  前端有内置回退,不影响页面;但 DB 真的出错要如实报 500,不能伪装成"标题为空"。)
	var s models.Setting
	if err := h.DB.Where("key = ?", SettingTitle).Limit(1).Find(&s).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Success(c, gin.H{"title": s.Value})
}

// GET /api/common/stats → { node_count, platform_count, association_count, active_node_count }
func (h *H) CommonStats(c *gin.Context) {
	var nodeCount, platformCount, assocCount, activeNodeCount int64
	// 逐个检查错误:出错时返回 500,而不是静默显示成 0(否则仪表盘会给出"0 个节点"的假象)
	if err := h.DB.Model(&models.Node{}).Count(&nodeCount).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	if err := h.DB.Model(&models.Platform{}).Count(&platformCount).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	if err := h.DB.Table("node_platforms").Count(&assocCount).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	if err := h.DB.Model(&models.Node{}).Where("status = ?", models.StatusEnabled).
		Count(&activeNodeCount).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}

	utils.Success(c, gin.H{
		"node_count":        nodeCount,
		"platform_count":    platformCount,
		"association_count": assocCount,
		"active_node_count": activeNodeCount,
	})
}
