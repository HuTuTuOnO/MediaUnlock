package handlers

// 处理器共享依赖:DB + 配置。各资源处理器为 *H 的方法,按资源分文件。

import (
	"gorm.io/gorm"

	"server/internal/config"
)

type H struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func New(db *gorm.DB, cfg *config.Config) *H {
	return &H{DB: db, Cfg: cfg}
}
