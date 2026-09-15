package handlers

// POST /api/auth/login → JWT。

import (
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"

	"server/internal/middleware"
	"server/internal/models"
	"server/internal/utils"
)

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *H) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "invalid request")
		return
	}

	var u models.User
	if err := h.DB.Where("username = ?", req.Username).First(&u).Error; err != nil {
		utils.Unauthorized(c, "invalid credentials")
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
		utils.Unauthorized(c, "invalid credentials")
		return
	}

	token, err := middleware.GenerateToken(h.Cfg.JWT.Secret, h.Cfg.JWT.ExpireHours, &u)
	if err != nil {
		utils.ServerError(c, "token error")
		return
	}
	utils.Success(c, gin.H{
		"token": token,
		"user":  gin.H{"id": u.ID, "username": u.Username},
	})
}

type changePasswordReq struct {
	NewPassword string `json:"new_password" binding:"required"`
}

// POST /api/auth/change-password —— 已登录用户修改自己的密码。
// 约定:登录态下不校验原密码,直接以 new_password 覆盖。
func (h *H) ChangePassword(c *gin.Context) {
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "new_password required")
		return
	}
	if len(req.NewPassword) < 6 {
		utils.BadRequest(c, "password too short (min 6)")
		return
	}

	uid := c.GetUint(middleware.CtxUserID)
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.ServerError(c, "hash error")
		return
	}
	if err := h.DB.Model(&models.User{}).Where("id = ?", uid).
		Update("password_hash", string(hash)).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}
	utils.SuccessMsg(c, "password updated")
}
