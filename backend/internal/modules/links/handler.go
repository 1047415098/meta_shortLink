package links

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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
	// Dynamic attribution is the required default for every newly created link.
	l := Link{AttributionMode: "dynamic"}
	if c.ShouldBindJSON(&l) != nil {
		runtime.Bad(c, "无效数据")
		return
	}
	if l.Code == "" {
		l.Code = runtime.Token()[:8]
	}
	if !ValidLink(l) || !HasMetaBinding(l) {
		runtime.Bad(c, "请检查链接信息，并选择 Meta Pixel 和广告归因方式")
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
	// Once a link has a Pixel, later edits cannot remove that required binding.
	hadMetaBinding := HasMetaBinding(l)
	if c.ShouldBindJSON(&l) != nil || !ValidLink(l) || (hadMetaBinding && !HasMetaBinding(l)) {
		runtime.Bad(c, "链接配置无效")
		return
	}
	l.ID = id
	a.saveLink(c, l, true)
}

// deleteLinksRequest uses a dedicated bulk contract because JSON bodies on
// DELETE requests are not handled consistently by every reverse proxy.
type deleteLinksRequest struct {
	IDs []int64 `json:"ids"`
}

// DeleteBatch permanently removes selected links and all data owned by their
// visitor journeys after validating the complete selection.
func (a *Handler) DeleteBatch(c *gin.Context) {
	var in deleteLinksRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if c.ContentType() != "application/json" || decoder.Decode(&in) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		runtime.Bad(c, "请使用 JSON 提交要删除的短链接")
		return
	}
	if len(in.IDs) == 0 || len(in.IDs) > 1000 {
		runtime.Bad(c, "请选择 1–1000 条短链接")
		return
	}
	seen := make(map[int64]struct{}, len(in.IDs))
	for _, id := range in.IDs {
		if id < 1 {
			runtime.Bad(c, "短链接编号无效")
			return
		}
		if _, exists := seen[id]; exists {
			runtime.Bad(c, "短链接编号不能重复")
			return
		}
		seen[id] = struct{}{}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	deleted, err := (Repository{DB: a.DB}).DeleteBatch(ctx, in.IDs, a.Config.AdminUser)
	if errors.Is(err, ErrLinksNotFound) {
		c.JSON(404, gin.H{"error": "部分短链接不存在，请刷新后重试"})
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"deleted": deleted})
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
