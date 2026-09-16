package meta

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
	g := api.Group("/meta")
	g.GET("/connections", h.listConnections)
	g.POST("/connections", h.saveConnection)
	g.PATCH("/connections/:id", h.saveConnection)
	g.DELETE("/connections/:id", h.deleteConnection)
	h.registerPixels(g)
	h.registerCredentials(g)
	g.POST("/source/inspect", h.inspectSource)
	h.registerEvents(g)
}

func (h *Handler) deleteConnection(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Service.DeleteConnection(ctx, id); err != nil {
		var inUse *ConfigInUseError
		switch {
		case IsNotFound(err):
			c.JSON(404, gin.H{"error": "Meta 帐号不存在"})
		case errors.As(err, &inUse):
			c.JSON(409, gin.H{"error": inUse.Error()})
		default:
			runtime.ServerError(c, err)
		}
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
func requestID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		runtime.Bad(c, "无效接入编号")
		return 0, false
	}
	return id, true
}
func (h *Handler) listConnections(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	rows, e := h.Service.Connections(ctx)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, rows)
}
func (h *Handler) saveConnection(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	var in ConnectionInput
	var id int64
	if c.Request.Method == "PATCH" {
		var ok bool
		id, ok = requestID(c)
		if !ok {
			return
		}
		old, e := h.Service.Connection(ctx, id)
		if IsNotFound(e) {
			c.Status(404)
			return
		}
		if e != nil {
			runtime.ServerError(c, e)
			return
		}
		in.Connection = old
	}
	if e := c.ShouldBindJSON(&in); e != nil {
		runtime.Bad(c, "接入配置格式无效")
		return
	}
	saved, e := h.Service.SaveConnection(ctx, in, id)
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) {
			if pg.Code == "23505" {
				c.JSON(409, gin.H{"error": "该广告账户已配置，请编辑已有配置"})
				return
			}
			runtime.ServerError(c, e)
			return
		}
		runtime.Bad(c, e.Error())
		return
	}
	c.JSON(200, saved)
}
