package utils

// 列表接口统一分页。PageParams 读参;Paged 是列表数据体;Page 直接写出统一响应壳。
// 最终形态:{ code, msg, data: { total, limit, offset, items } }。

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// Paged 列表数据体(放进 Response.Data)。
type Paged struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
	Items  any   `json:"items"`
}

// PageParams 从 query 读 limit/offset 并做上下限收敛。
func PageParams(c *gin.Context) (limit, offset int) {
	limit = DefaultPageLimit
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	if v := c.Query("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}
	return
}

// Page 写出分页成功响应(200 + Paged 数据体)。
func Page(c *gin.Context, total int64, limit, offset int, items any) {
	Success(c, Paged{Total: total, Limit: limit, Offset: offset, Items: items})
}
