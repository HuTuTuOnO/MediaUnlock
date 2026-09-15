package router

// 路由装配:把各 handler 挂到 Gin 引擎上。main 与测试共用。

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"server/internal/config"
	"server/internal/handlers"
	"server/internal/middleware"
	"server/internal/static"
	"server/internal/utils"
)

func New(db *gorm.DB, cfg *config.Config) *gin.Engine {
	h := handlers.New(db, cfg)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api")
	{
		api.POST("/auth/login", h.Login)

		// 公开:站点标题(登录页需要,无需鉴权)
		api.GET("/common/settings", h.CommonSettings)

		// 需要 JWT 的接口(系统仅单一管理员,登录即全权)
		auth := api.Group("")
		auth.Use(middleware.JWTAuth(cfg.JWT.Secret))
		auth.GET("/userinfo", func(c *gin.Context) {
			utils.Success(c, gin.H{"user_id": c.GetUint(middleware.CtxUserID)})
		})
		auth.POST("/auth/change-password", h.ChangePassword)
		auth.GET("/nodes", h.ListNodes)
		auth.GET("/platforms", h.ListPlatforms)
		auth.GET("/unlocks", h.ListUnlocks)
		auth.GET("/settings", h.GetSettings)
		auth.GET("/common/stats", h.CommonStats)
		auth.POST("/nodes", h.CreateNode)
		auth.PUT("/nodes/:id", h.UpdateNode)
		auth.DELETE("/nodes/:id", h.DeleteNode)
		auth.POST("/nodes/:id/retoken", h.RetokenNode)
		auth.POST("/platforms", h.CreatePlatform)
		auth.PUT("/platforms/:id", h.UpdatePlatform)
		auth.DELETE("/platforms/:id", h.DeletePlatform)
		auth.PUT("/settings", h.UpdateSettings)
		auth.POST("/settings/retoken", h.RetokenSetting)

		// ===== Agent 专用(不走 JWT)=====
		// node 模式:节点 token
		agentNode := api.Group("/agent")
		agentNode.Use(middleware.NodeTokenAuth(db))
		agentNode.GET("/node", h.AgentNode)
		agentNode.POST("/report", h.AgentReport)

		// client 模式:全局只读 token
		agentClient := api.Group("/agent")
		agentClient.Use(middleware.ClientTokenAuth(db))
		agentClient.GET("/unlocked", h.AgentUnlocked)
	}

	// 前端静态资源(嵌入 static)+ SPA 回退。放在最后,只接管未命中的路由。
	static.Register(r)
	return r
}
