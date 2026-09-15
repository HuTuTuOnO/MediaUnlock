package middleware

// 三套鉴权中间件:
//   JWTAuth         —— 管理端 Web,Authorization: Bearer <JWT>
//   NodeTokenAuth   —— Agent node 模式,Token: <nodes.token>
//   ClientTokenAuth —— Agent client 模式,Token: <settings.token>

import (
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"server/internal/models"
	"server/internal/utils"
)

const (
	CtxUserID = "user_id"
	CtxNodeID = "node_id"
)

// Claims JWT 载荷
type Claims struct {
	UserID uint `json:"uid"`
	jwt.RegisteredClaims
}

// GenerateToken 为登录用户签发 JWT
func GenerateToken(secret string, expireHours int, u *models.User) (string, error) {
	claims := Claims{
		UserID: u.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHours) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString([]byte(secret))
}

// NodeTokenAuth 校验 node 模式 Agent 的节点专属 token(在 nodes 表)。
// token 从 "Token: <token>" 请求头读取(Agent 统一用这个头,不用 Authorization: Bearer)。
// 通过后把 node_id 写入上下文,供 /api/agent/node、/api/agent/report 使用。
func NodeTokenAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Token")
		if raw == "" {
			utils.AbortUnauthorized(c, "missing token")
			return
		}
		var node models.Node
		if err := db.Where("token = ?", raw).First(&node).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				utils.AbortUnauthorized(c, "invalid node token")
				return
			}
			utils.AbortServerError(c, "db error")
			return
		}
		c.Set(CtxNodeID, node.ID)
		c.Next()
	}
}

// ClientTokenAuth 校验 client 模式 Agent 的全局只读 Token(存 settings.token)。
// 仅放行 /api/agent/unlocked。token 从 "Token: <token>" 请求头读取(与 node 模式一致);
// 不再支持 ?token= 查询参数(避免 token 出现在 URL 与访问日志里)。
// 未配置该 token 时一律拒绝。
func ClientTokenAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader("Token")
		if raw == "" {
			utils.AbortUnauthorized(c, "missing token")
			return
		}
		var s models.Setting
		if err := db.Where("key = ?", models.SettingKeyToken).First(&s).Error; err != nil {
			utils.AbortUnauthorized(c, "client token not configured")
			return
		}
		if s.Value == "" || s.Value != raw {
			utils.AbortUnauthorized(c, "invalid client token")
			return
		}
		c.Next()
	}
}

// JWTAuth 校验 Authorization: Bearer <token>,写入 user_id 到上下文。
// 管理端专用;Agent 走的是 "Token:" 头。
func JWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 管理端用 Authorization: Bearer <JWT>
		raw := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if raw == "" {
			utils.AbortUnauthorized(c, "missing token")
			return
		}
		claims := &Claims{}
		tok, err := jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (any, error) {
			return []byte(secret), nil
		})
		if err != nil || !tok.Valid {
			utils.AbortUnauthorized(c, "invalid token")
			return
		}
		c.Set(CtxUserID, claims.UserID)
		c.Next()
	}
}
