package tiktok

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct{ Service *Service }

func (h *Handler) Register(api *gin.RouterGroup) {
	api.GET("/tiktok-connections", h.listConnections)
	api.POST("/tiktok-connections", h.createConnection)
	api.PATCH("/tiktok-connections/:id", h.updateConnection)
	api.DELETE("/tiktok-connections/:id", h.deleteConnection)
	api.GET("/tiktok-pixels", h.listPixels)
	api.POST("/tiktok-pixels", h.createPixel)
	api.PATCH("/tiktok-pixels/:id", h.updatePixel)
	api.DELETE("/tiktok-pixels/:id", h.deletePixel)
	api.POST("/tiktok-pixels/:id/test", h.testPixel)
	api.GET("/tiktok-events", h.listEvents)
	api.POST("/tiktok-events/:id/retry", h.retryEvent)
}

func requestID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		runtime.Bad(c, "TikTok 配置编号无效")
		return 0, false
	}
	return id, true
}

func (h *Handler) listConnections(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := h.Service.Connections(ctx)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, items)
}

func (h *Handler) createConnection(c *gin.Context) { h.saveConnection(c, 0) }
func (h *Handler) updateConnection(c *gin.Context) {
	id, ok := requestID(c)
	if ok {
		h.saveConnection(c, id)
	}
}

func (h *Handler) saveConnection(c *gin.Context, id int64) {
	var input ConnectionInput
	if c.ShouldBindJSON(&input) != nil {
		runtime.Bad(c, "TikTok 凭证格式无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := h.Service.SaveConnection(ctx, input, id)
	if err != nil {
		h.writeConfigError(c, err, "TikTok 凭证不存在", "TikTok 凭证名称已存在")
		return
	}
	c.JSON(200, item)
}

func (h *Handler) deleteConnection(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Service.DeleteConnection(ctx, id); err != nil {
		h.writeConfigError(c, err, "TikTok 凭证不存在", "TikTok 凭证无法删除")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (h *Handler) listPixels(c *gin.Context) {
	connectionID, err := strconv.ParseInt(c.DefaultQuery("connection_id", "0"), 10, 64)
	if err != nil || connectionID < 0 {
		runtime.Bad(c, "TikTok 凭证编号无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := h.Service.Pixels(ctx, connectionID)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, items)
}

func (h *Handler) createPixel(c *gin.Context) { h.savePixel(c, 0) }
func (h *Handler) updatePixel(c *gin.Context) {
	id, ok := requestID(c)
	if ok {
		h.savePixel(c, id)
	}
}

func (h *Handler) savePixel(c *gin.Context, id int64) {
	input := PixelInput{Enabled: true}
	if c.ShouldBindJSON(&input) != nil {
		runtime.Bad(c, "TikTok Pixel 格式无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := h.Service.SavePixel(ctx, input, id)
	if err != nil {
		h.writeConfigError(c, err, "TikTok Pixel 不存在", "TikTok Pixel Code 已存在")
		return
	}
	c.JSON(200, item)
}

func (h *Handler) deletePixel(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Service.DeletePixel(ctx, id); err != nil {
		h.writeConfigError(c, err, "TikTok Pixel 不存在", "TikTok Pixel 无法删除")
		return
	}
	c.JSON(200, gin.H{"ok": true})
}

func (h *Handler) testPixel(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	if h.Service.Core.Exceed("tiktok-pixel-test:"+strconv.FormatInt(id, 10), 5, time.Minute) {
		c.Status(429)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	response, err := h.Service.TestPixel(ctx, id)
	if err != nil {
		if IsNotFound(err) {
			c.Status(404)
		} else {
			runtime.Bad(c, cleanMessage(err.Error()))
		}
		return
	}
	c.JSON(200, gin.H{"accepted": true, "request_id": response.RequestID})
}

func (h *Handler) writeConfigError(c *gin.Context, err error, notFound, duplicate string) {
	var inUse *ConfigInUseError
	var postgresError *pgconn.PgError
	switch {
	case IsNotFound(err):
		c.JSON(404, gin.H{"error": notFound})
	case errors.As(err, &inUse):
		c.JSON(409, gin.H{"error": inUse.Error()})
	case errors.As(err, &postgresError) && postgresError.Code == "23505":
		c.JSON(409, gin.H{"error": duplicate})
	case errors.As(err, &postgresError):
		runtime.ServerError(c, err)
	default:
		runtime.Bad(c, err.Error())
	}
}

func (h *Handler) listEvents(c *gin.Context) {
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil || page < 1 || page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	filters := EventFilters{Page: page, Status: c.Query("status"), EventName: c.Query("event_name")}
	validStatus := map[string]bool{"": true, "pending": true, "sending": true, "accepted": true, "retry": true, "failed": true}
	validName := map[string]bool{"": true, "StartReading": true, "StartListening": true, "ViewContent": true, "PageView": true}
	if !validStatus[filters.Status] || !validName[filters.EventName] {
		runtime.Bad(c, "TikTok 事件筛选条件无效")
		return
	}
	filters.PixelRecordID, err = strconv.ParseInt(c.DefaultQuery("pixel_record_id", "0"), 10, 64)
	if err != nil || filters.PixelRecordID < 0 {
		runtime.Bad(c, "TikTok Pixel 编号无效")
		return
	}
	filters.LinkID, err = strconv.ParseInt(c.DefaultQuery("link_id", "0"), 10, 64)
	if err != nil || filters.LinkID < 0 {
		runtime.Bad(c, "投放链接编号无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, total, err := h.Service.ListEvents(ctx, filters)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": 50})
}

func (h *Handler) retryEvent(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Service.RetryEvent(ctx, c.Param("id")); err != nil {
		c.JSON(409, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
