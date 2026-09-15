package meta

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"strconv"
	"time"
	"whatsapp-analytics/internal/platform/runtime"
)

func (h *Handler) registerPixels(g *gin.RouterGroup) {
	g.GET("/pixels", h.listPixels)
	g.POST("/pixels", h.savePixel)
	g.PATCH("/pixels/:id", h.savePixel)
	g.POST("/pixels/:id/test-event", h.testPixel)
}
func (h *Handler) listPixels(c *gin.Context) {
	id, e := strconv.ParseInt(c.DefaultQuery("connection_id", "0"), 10, 64)
	if e != nil || id < 0 {
		runtime.Bad(c, "账户编号无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	out, e := h.Service.Pixels(ctx, id)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, out)
}
func (h *Handler) savePixel(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	// API clients that omit optional switches receive the complete default
	// landing funnel: one qualified PageView plus consultation events.
	in := PixelInput{Pixel: Pixel{PageviewEnabled: true, ManualEnabled: true, ManualEventName: EventName}}
	var id int64
	if c.Request.Method == "PATCH" {
		var ok bool
		id, ok = requestID(c)
		if !ok {
			return
		}
		p, e := h.Service.Pixel(ctx, id)
		if e != nil {
			if IsNotFound(e) {
				c.Status(404)
			} else {
				runtime.ServerError(c, e)
			}
			return
		}
		in.Pixel = p
	}
	if c.ShouldBindJSON(&in) != nil {
		runtime.Bad(c, "Pixel 配置无效")
		return
	}
	p, e := h.Service.SavePixel(ctx, in, id)
	if e != nil {
		var pg *pgconn.PgError
		if errors.As(e, &pg) {
			if pg.Code == "23505" {
				c.JSON(409, gin.H{"error": "该账户已配置此 Pixel"})
				return
			}
			runtime.ServerError(c, e)
			return
		}
		runtime.Bad(c, e.Error())
		return
	}
	c.JSON(200, p)
}
func (h *Handler) testPixel(c *gin.Context) {
	id, ok := requestID(c)
	if !ok {
		return
	}
	var in struct {
		Code string `json:"test_event_code"`
		Name string `json:"event_name"`
	}
	if c.ShouldBindJSON(&in) != nil {
		runtime.Bad(c, "测试参数无效")
		return
	}
	if in.Name == "" {
		in.Name = EventName
	}
	if h.Service.Core.Exceed("meta-pixel-test:"+strconv.FormatInt(id, 10), 5, time.Minute) {
		c.Status(429)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	p, e := h.Service.Pixel(ctx, id)
	if e != nil {
		c.Status(404)
		return
	}
	out, e := h.Service.QueuePixelTest(ctx, p, in.Code, in.Name, ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	c.JSON(200, out)
}
