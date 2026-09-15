package models

// 5 张表 struct:User / Node / Platform / Unlock / Setting。
// (多对多关联表 node_platforms 由 GORM 按 many2many 自动维护,没有对应 struct。)
// 字段定义以 需求文档.md 3.3 为唯一来源。

import "time"

// 状态枚举(节点 / 平台通用)
const (
	StatusDisabled = 0 // 禁用
	StatusEnabled  = 1 // 启用
)

// 节点类型
const (
	NodeTypeSOCKS5 = "socks5"
	NodeTypeHTTP   = "http"
)

// 解锁检测状态。数值与 agent 侧 pkg/core/result.go 的 Status* 常量一一对应,
// 前端 Unlocks.tsx 的 STATUS_META 也按这套值渲染标签与配色 —— 三处必须保持一致。
const (
	UnlockStatusSuccess    = 1  // 成功解锁
	UnlockStatusRestricted = 2  // 受限(只能看部分内容,如仅自制剧)
	UnlockStatusNo         = 3  // 不支持(该地区没有此服务)
	UnlockStatusBanned     = 4  // 封禁(IP 被服务商封禁)
	UnlockStatusFailed     = 5  // 失败(检测未通过)
	UnlockStatusUnexpected = 6  // 异常(返回了非预期的结果)
	UnlockStatusNetworkErr = -1 // 网络错误(连不上目标)
	UnlockStatusErr        = -2 // 错误(其他错误)
)

// settings 表已知配置键(单一来源:种子初始化与 handler 白名单共用)
const (
	SettingKeyTitle         = "title"          // 站点标题
	SettingKeyToken         = "token"          // client 模式全局只读 Token
	SettingKeyRetentionDays = "retention_days" // unlocks 保留天数(0 = 不自动清理)
)

// User 管理员
type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Node 解锁节点
type Node struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `json:"name"`                              // 展示名称
	Alias  string `gorm:"uniqueIndex;not null" json:"alias"` // 唯一,业务引用键
	Type   string `json:"type"`                              // socks5 / http
	Status int    `gorm:"default:1" json:"status"`           // 1=启用 0=禁用
	Host   string `json:"host"`                              // 域名或 IP,管理员手填
	Port   int    `json:"port"`
	Value1 string `json:"value1"` // 账号(可空)
	Value2 string `json:"value2"` // 密码(可空)
	Value3 string `json:"value3"` // 预留
	Value4 string `json:"value4"` // 预留
	Value5 string `json:"value5"` // 预留
	Value6 string `json:"value6"` // 预留
	// Agent 认证,每节点独立。列表接口直接下发(该接口需 JWT,仅管理员可见)。
	Token     string     `gorm:"index" json:"token"`
	ReportAt  *time.Time `json:"report_at"` // 最近上报时间
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// Platforms 关联平台(GORM 原生 many2many);
	// 列表接口用 Preload 自动加载,前端自行取 name 展示。
	Platforms []Platform `gorm:"many2many:node_platforms;" json:"platforms"`
}

// Platform 平台
type Platform struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"uniqueIndex;not null" json:"name"` // 与 MediaUnlockTest 检测名一一对应
	Rules     string    `json:"rules"`                            // 分流域名/路由(json/string),非检测规则
	Status    int       `gorm:"default:1" json:"status"`          // 1=启用 0=禁用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Nodes 关联节点(GORM 原生 many2many);
	// 列表接口用 Preload 自动加载,前端自行取 name/alias 展示。
	Nodes []Node `gorm:"many2many:node_platforms;" json:"nodes"`
}

// Unlock 解锁历史(仅供查看,不参与下发)
type Unlock struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	NodeID     uint   `gorm:"index" json:"node_id"`
	PlatformID uint   `gorm:"index" json:"platform_id"`
	Status     int    `json:"status"` // MediaUnlockTest 原始数值:1成功/2受限/3不支持/4封禁/5失败/6异常/-1网络错误/-2错误
	Region     string `json:"region"`
	Info       string `json:"info"`
	Err        string `json:"err"` // 存 string 不存 error
	// 普通索引(允许重复):清理任务按 created_at < ? 删超期记录、解锁历史按 ?range= 过滤都用这个字段。
	// 没索引时是全表扫描,百万级数据下 DELETE 要 800ms+ 且会阻塞 SQLite 的单写者。
	CreatedAt time.Time `gorm:"index" json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Setting 系统设置(键值)
type Setting struct {
	Key   string `gorm:"primaryKey" json:"key"`
	Value string `json:"value"`
}

// AllModels 供 AutoMigrate 使用
func AllModels() []any {
	return []any{
		&User{}, &Node{}, &Platform{}, &Unlock{}, &Setting{},
	}
}
