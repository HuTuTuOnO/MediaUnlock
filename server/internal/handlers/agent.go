package handlers

// Agent 专用接口(不走 JWT):
//   GET  /api/agent/node      —— node 模式,节点 token 认证,回本节点连接信息
//   POST /api/agent/report    —— node 模式,节点 token 认证,落库检测结果
//   GET  /api/agent/unlocked  —— client 模式,全局只读 token 认证,回下发数据
// 认证由 middleware 完成:node 接口在 ctx 里放了 node_id。

import (
	"log/slog"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"server/internal/middleware"
	"server/internal/models"
	"server/internal/utils"
)

// GET /api/agent/node —— node Agent 拉本节点连接信息以启动本机 socks/http 服务
func (h *H) AgentNode(c *gin.Context) {
	id := c.GetUint(middleware.CtxNodeID)
	var node models.Node
	if err := h.DB.First(&node, id).Error; err != nil {
		utils.NotFound(c, "node not found")
		return
	}
	utils.Success(c, gin.H{
		"alias":  node.Alias,
		"type":   node.Type,
		"port":   node.Port,
		"value1": node.Value1,
		"value2": node.Value2,
		"value3": node.Value3,
		"value4": node.Value4,
		"value5": node.Value5,
		"value6": node.Value6,
	})
}

// ---- report ----

type reportItem struct {
	Name   string `json:"name"` // 平台名,与 platforms.name 匹配
	Status int    `json:"status"`
	Region string `json:"region"`
	Info   string `json:"info"`
	Err    string `json:"err"`
}

type reportReq struct {
	Results []reportItem `json:"results"`
}

// POST /api/agent/report —— 每次检测结果追加写入 unlocks;
// 收集本次 status==1 的平台后,一次性替换 node_platforms 关联。
// 平台名匹配不到 → 静默丢弃并记录日志,最后刷新 node.report_at。
func (h *H) AgentReport(c *gin.Context) {
	nodeID := c.GetUint(middleware.CtxNodeID)
	var req reportReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}

	var platforms []models.Platform
	if err := h.DB.Find(&platforms).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	nameToID := make(map[string]uint, len(platforms))
	for _, platform := range platforms {
		nameToID[platform.Name] = platform.ID
	}

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		unlocks := make([]models.Unlock, 0, len(req.Results))
		unlockedPlatforms := make([]models.Platform, 0, len(req.Results))
		unlockedPlatformIDs := make(map[uint]struct{}, len(req.Results))
		for _, item := range req.Results {
			platformID, ok := nameToID[item.Name]
			if !ok {
				slog.Warn("上报平台名未匹配,已丢弃", "node_id", nodeID, "name", item.Name)
				continue
			}

			unlocks = append(unlocks, models.Unlock{
				NodeID: nodeID, PlatformID: platformID,
				Status: item.Status, Region: item.Region, Info: item.Info, Err: item.Err,
			})

			if item.Status == models.UnlockStatusSuccess {
				if _, exists := unlockedPlatformIDs[platformID]; !exists {
					unlockedPlatforms = append(unlockedPlatforms, models.Platform{ID: platformID})
					unlockedPlatformIDs[platformID] = struct{}{}
				}
			}
		}

		if len(unlocks) > 0 {
			if err := tx.Create(&unlocks).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&models.Node{ID: nodeID}).
			Association("Platforms").Replace(&unlockedPlatforms); err != nil {
			return err
		}

		now := time.Now()
		return tx.Model(&models.Node{}).Where("id = ?", nodeID).
			Update("report_at", &now).Error
	})
	if err != nil {
		utils.ServerError(c, "db error")
		return
	}

	utils.Success(c, gin.H{"reported": len(req.Results)})
}

// ---- unlocked ----

// unlockedNode 下发的单个节点:连接信息 + 启用状态。
// Status:1=启用 / 0=禁用 —— 全量下发时禁用节点也会带出来,由 Agent 自行判断。
type unlockedNode struct {
	Host     string     `json:"host"`
	Port     int        `json:"port"`
	Type     string     `json:"type"`
	Value1   string     `json:"value1"`
	Value2   string     `json:"value2"`
	Value3   string     `json:"value3"`
	Value4   string     `json:"value4"`
	Value5   string     `json:"value5"`
	Value6   string     `json:"value6"`
	Status   int        `json:"status"`
	UploadAt *time.Time `json:"upload_at"`
}

// unlockedPlatform 下发的单个平台:分流规则 + 能解它的节点别名列表。
// Status:1=启用 / 0=禁用 —— 禁用平台也会下发,由 Agent 自行判断。
type unlockedPlatform struct {
	Aliases []string `json:"alias"`
	Rules   []string `json:"rules"`
	Status  int      `json:"status"`
}

// GET /api/agent/unlocked —— client Agent 拉下发数据:按平台分组,含 rules + 可解锁节点连接信息。
// 全量下发:禁用的平台 / 节点也一并返回,并各自携带 status,由 Agent 自行决定是否使用。
func (h *H) AgentUnlocked(c *gin.Context) {
	// 平台:全量(禁用的也下发,带 status),并预加载关联节点以得出 aliases
	var platforms []models.Platform
	if err := h.DB.
		Preload("Nodes").
		Find(&platforms).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}

	// 节点:直接查 nodes 表**全量下发**,不受平台关联限制 ——
	// 与平台是否关联、是否启用无关,由 client 按 alias 自取。
	var nodes []models.Node
	if err := h.DB.Find(&nodes).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}

	nodesOut := make(map[string]unlockedNode, len(nodes))
	for _, n := range nodes {
		nodesOut[n.Alias] = unlockedNode{
			Host: n.Host, Port: n.Port, Type: n.Type,
			UploadAt: n.ReportAt,
			Value1:   n.Value1, Value2: n.Value2, Value3: n.Value3,
			Value4: n.Value4, Value5: n.Value5, Value6: n.Value6,
			Status: n.Status,
		}
	}

	platformsOut := make(map[string]unlockedPlatform, len(platforms))
	for _, p := range platforms {
		aliases := make([]string, 0, len(p.Nodes))
		for _, n := range p.Nodes {
			aliases = append(aliases, n.Alias)
		}
		rules := []string{}
		for _, part := range strings.Split(p.Rules, ",") {
			if s := strings.TrimSpace(part); s != "" {
				rules = append(rules, s)
			}
		}
		platformsOut[p.Name] = unlockedPlatform{Aliases: aliases, Rules: rules, Status: p.Status}
	}

	utils.Success(c, gin.H{"node": nodesOut, "platform": platformsOut})
}
