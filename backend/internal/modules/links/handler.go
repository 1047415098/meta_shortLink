package links

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct{ *runtime.Core }

func (a *Handler) List(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	out, e := (Repository{DB: a.DB}).List(ctx)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, out)
}

func (a *Handler) Create(c *gin.Context) {
	var l Link
	if c.ShouldBindJSON(&l) != nil {
		runtime.Bad(c, "无效数据")
		return
	}
	if l.Code == "" {
		l.Code = runtime.Token()[:8]
	}
	if !ValidLink(l) {
		runtime.Bad(c, "请检查名称、短码和 WhatsApp 目标地址")
		return
	}
	a.saveLink(c, l, false)
}

func (a *Handler) Update(c *gin.Context) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		runtime.Bad(c, "无效链接编号")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	l, e := (Repository{DB: a.DB}).ByID(ctx, id)
	if e == pgx.ErrNoRows {
		c.Status(404)
		return
	}
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	// Decode over the existing record so PATCH supports enabling without replacing metadata.
	if c.ShouldBindJSON(&l) != nil || !ValidLink(l) {
		runtime.Bad(c, "链接配置无效")
		return
	}
	l.ID = id
	a.saveLink(c, l, true)
}

func (a *Handler) saveLink(c *gin.Context, l Link, update bool) {
	if l.Mode == "" {
		l.Mode = "redirect"
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	saved, e := (Repository{DB: a.DB}).Save(ctx, l, update, a.Config.AdminUser)
	if e != nil {
		if strings.Contains(e.Error(), "23503") || strings.Contains(e.Error(), "23514") {
			runtime.Bad(c, "所选 Pixel 必须属于当前广告账户，且配置需要存在")
			return
		}
		if strings.Contains(e.Error(), "23505") {
			c.JSON(409, gin.H{"error": "短码已存在"})
			return
		}
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, saved)
}
