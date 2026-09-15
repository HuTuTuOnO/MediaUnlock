package handlers

// /api/nodes CRUD + /nodes/:id/retoken。
// token 由 models.Node 的 json:"token" 直接下发(该接口需 JWT,仅管理员可见),
// 前端列表页不渲染它,只在「查看详情」弹窗里展示;创建 / 重置时也会一次性返回明文。

import (
	"errors"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"server/internal/models"
	"server/internal/utils"
)

// nodeReq 新建/更新节点的入参(token 不在其中,单独由 retoken 管理)
type nodeReq struct {
	Name   string `json:"name"`
	Alias  string `json:"alias"`
	Type   string `json:"type"`
	Status *int   `json:"status"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Value1 string `json:"value1"`
	Value2 string `json:"value2"`
	Value3 string `json:"value3"`
	Value4 string `json:"value4"`
	Value5 string `json:"value5"`
	Value6 string `json:"value6"`
}

// validNodeType 节点类型白名单:socks5 / http。
func validNodeType(t string) bool {
	return t == models.NodeTypeSOCKS5 || t == models.NodeTypeHTTP
}

// GET /api/nodes?limit=&offset=&search= search 为空则全量;非空时对 name/alias/host 做模糊匹配。
func (h *H) ListNodes(c *gin.Context) {
	limit, offset := utils.PageParams(c)

	q := h.DB.Model(&models.Node{})
	if s := c.Query("search"); s != "" {
		like := "%" + s + "%"
		q = q.Where("name LIKE ? OR alias LIKE ? OR host LIKE ?", like, like, like)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	var nodes []models.Node
	if err := q.Preload("Platforms").Order("id").Limit(limit).Offset(offset).Find(&nodes).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Page(c, total, limit, offset, nodes)
}

// POST /api/nodes —— 创建并返回一次性 token
func (h *H) CreateNode(c *gin.Context) {
	var req nodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}
	if req.Alias == "" {
		utils.BadRequest(c, "alias required")
		return
	}
	if !validNodeType(req.Type) {
		utils.BadRequest(c, "invalid type, expect socks5 or http")
		return
	}
	token, err := utils.RandomHex(16)
	if err != nil {
		utils.ServerError(c, "token error")
		return
	}
	n := models.Node{
		Name: req.Name, Alias: req.Alias, Type: req.Type,
		Status: models.StatusEnabled, Host: req.Host, Port: req.Port,
		Value1: req.Value1, Value2: req.Value2, Value3: req.Value3,
		Value4: req.Value4, Value5: req.Value5, Value6: req.Value6,
		Token: token,
	}
	if req.Status != nil {
		n.Status = *req.Status
	}
	if err := h.DB.Create(&n).Error; err != nil {
		utils.Conflict(c, "create failed (alias may be duplicated)")
		return
	}
	// 一次性把明文 token 返回给管理员
	utils.Created(c, gin.H{"node": n, "token": token})
}

// PUT /api/nodes/:id
func (h *H) UpdateNode(c *gin.Context) {
	var n models.Node
	if err := h.DB.First(&n, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "not found")
			return
		}
		utils.ServerError(c, "db error")
		return
	}
	var req nodeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}
	if req.Alias == "" {
		utils.BadRequest(c, "alias required")
		return
	}
	if !validNodeType(req.Type) {
		utils.BadRequest(c, "invalid type, expect socks5 or http")
		return
	}
	n.Name, n.Alias, n.Type = req.Name, req.Alias, req.Type
	n.Host, n.Port = req.Host, req.Port
	n.Value1, n.Value2, n.Value3 = req.Value1, req.Value2, req.Value3
	n.Value4, n.Value5, n.Value6 = req.Value4, req.Value5, req.Value6
	if req.Status != nil {
		n.Status = *req.Status
	}
	if err := h.DB.Save(&n).Error; err != nil {
		utils.Conflict(c, "update failed (alias may be duplicated)")
		return
	}
	utils.Success(c, n)
}

// DELETE /api/nodes/:id —— 连带清理该节点的关联(node_platforms)与解锁历史(unlocks)。
func (h *H) DeleteNode(c *gin.Context) {
	id := c.Param("id")
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("node_platforms").Where("node_id = ?", id).Delete(&struct{}{}).Error; err != nil {
			return err
		}
		if err := tx.Where("node_id = ?", id).Delete(&models.Unlock{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Node{}, id).Error
	})
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.SuccessMsg(c, "deleted")
}

// POST /api/nodes/:id/retoken —— 重置并一次性返回新 token
func (h *H) RetokenNode(c *gin.Context) {
	var n models.Node
	if err := h.DB.First(&n, c.Param("id")).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "not found")
			return
		}
		utils.ServerError(c, "db error")
		return
	}
	token, err := utils.RandomHex(16)
	if err != nil {
		utils.ServerError(c, "token error")
		return
	}
	if err := h.DB.Model(&n).Update("token", token).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.Success(c, gin.H{"token": token})
}
