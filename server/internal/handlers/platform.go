package handlers

// /api/platforms CRUD。name 唯一(与 MediaUnlockTest 检测名对应);rules 为分流规则,非检测规则。

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"server/internal/models"
	"server/internal/utils"
)

type platformReq struct {
	Name   string `json:"name"`
	Rules  string `json:"rules"`
	Status *int   `json:"status"`
}

// GET /api/platforms?limit=&offset=&search=
func (h *H) ListPlatforms(c *gin.Context) {
	limit, offset := utils.PageParams(c)

	q := h.DB.Model(&models.Platform{})
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("name LIKE ? OR rules LIKE ?", like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	var ps []models.Platform
	if err := q.Preload("Nodes").Order("id").Limit(limit).Offset(offset).Find(&ps).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Page(c, total, limit, offset, ps)
}

// POST /api/platforms
func (h *H) CreatePlatform(c *gin.Context) {
	var req platformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}
	if req.Name == "" {
		utils.BadRequest(c, "name required")
		return
	}
	p := models.Platform{Name: req.Name, Rules: req.Rules, Status: models.StatusEnabled}
	if req.Status != nil {
		p.Status = *req.Status
	}
	if err := h.DB.Create(&p).Error; err != nil {
		utils.Conflict(c, "create failed (name may be duplicated)")
		return
	}
	utils.Created(c, p)
}

// PUT /api/platforms/:id
func (h *H) UpdatePlatform(c *gin.Context) {
	var p models.Platform
	if err := h.DB.First(&p, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "not found")
			return
		}
		utils.ServerError(c, "db error")
		return
	}
	var req platformReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}
	if req.Name == "" {
		utils.BadRequest(c, "name required")
		return
	}
	p.Name, p.Rules = req.Name, req.Rules
	if req.Status != nil {
		p.Status = *req.Status
	}
	if err := h.DB.Save(&p).Error; err != nil {
		utils.Conflict(c, "update failed (name may be duplicated)")
		return
	}
	utils.Success(c, p)
}

// DELETE /api/platforms/:id —— 连带清理该平台的关联(node_platforms)与解锁历史(unlocks)。
func (h *H) DeletePlatform(c *gin.Context) {
	id := c.Param("id")
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("node_platforms").Where("platform_id = ?", id).Delete(&struct{}{}).Error; err != nil {
			return err
		}
		if err := tx.Where("platform_id = ?", id).Delete(&models.Unlock{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Platform{}, id).Error
	})
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.SuccessMsg(c, "deleted")
}
