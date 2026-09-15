package handlers

// GET /api/unlocks:解锁历史查询。按 node_id / platform_ids / range 筛选,按时间倒序。
// 不分页 —— 直接返回符合条件的全部记录(数组)。
// 仅供后台查看,不参与下发。

import (
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"server/internal/models"
	"server/internal/utils"
)

// GET /api/unlocks?node_id=&platform_ids=&range=
//
//	platform_ids:逗号分隔的多个平台 id(如 1,2,3),前端一次拉一批平台的记录;
//	             空片段跳过,任一段非法直接 400
//	range:24h / 7d / 30d(支持任意 <数字>h / <数字>d),只返回该时间窗内的记录;
//	      不传则不过滤时间;非法值直接 400
func (h *H) ListUnlocks(c *gin.Context) {
	q := h.DB.Model(&models.Unlock{})

	// node_id 非法同样直接 400,与下面的 platform_ids / range 保持一致 ——
	// 静默忽略会让调用方以为过滤生效、实际拿到全量数据。
	if v := c.Query("node_id"); v != "" {
		id, err := strconv.Atoi(v)
		if err != nil || id <= 0 {
			utils.BadRequest(c, "invalid node_id, expect a positive integer")
			return
		}
		q = q.Where("node_id = ?", id)
	}

	// platform_ids 逗号分隔多平台:解析逻辑内联在此,空片段跳过,任一段非法 → 400
	if v := c.Query("platform_ids"); v != "" {
		ids := make([]uint, 0, strings.Count(v, ",")+1)
		bad := false
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			n, err := strconv.ParseUint(p, 10, 64)
			if err != nil || n == 0 {
				bad = true
				break
			}
			ids = append(ids, uint(n))
		}
		if bad {
			utils.BadRequest(c, "invalid platform_ids, expect e.g. 1,2,3")
			return
		}
		if len(ids) > 0 {
			q = q.Where("platform_id IN ?", ids)
		}
	}

	// range 时间窗:支持 <数字>h / <数字>d(如 24h / 7d / 30d),大小写不敏感。
	if v := c.Query("range"); v != "" {
		cutoff, ok := time.Time{}, false
		if len(v) >= 2 {
			if n, err := strconv.Atoi(v[:len(v)-1]); err == nil && n > 0 {
				switch v[len(v)-1] {
				case 'h', 'H':
					cutoff, ok = time.Now().Add(-time.Duration(n)*time.Hour), true
				case 'd', 'D':
					cutoff, ok = time.Now().Add(-time.Duration(n)*24*time.Hour), true
				}
			}
		}
		if !ok {
			utils.BadRequest(c, "invalid range, expect e.g. 24h / 7d / 30d")
			return
		}
		q = q.Where("created_at >= ?", cutoff)
	}

	var items []models.Unlock
	if err := q.Order("id desc").Find(&items).Error; err != nil {
		utils.ServerError(c, "db error")
		return
	}

	utils.Success(c, items)
}
