package meta

import (
	"context"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/platform/runtime"
)

func (h *Handler) registerEvents(g *gin.RouterGroup) {
	g.GET("/events", h.listEvents)
	g.POST("/events/:id/retry", h.retryEvent)
}
func (h *Handler) listEvents(c *gin.Context) {
	page, e := strconv.Atoi(c.DefaultQuery("page", "1"))
	if e != nil || page < 1 || page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	connection := int64(0)
	if v := c.Query("connection_id"); v != "" {
		connection, e = strconv.ParseInt(v, 10, 64)
		if e != nil || connection < 0 {
			runtime.Bad(c, "接入配置编号无效")
			return
		}
	}
	status := c.Query("status")
	valid := map[string]bool{"": true, "pending": true, "processing": true, "succeeded": true, "retry": true, "failed": true, "expired": true, "skipped": true}
	if !valid[status] {
		runtime.Bad(c, "事件状态无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	pixel, e := strconv.ParseInt(c.DefaultQuery("pixel_record_id", "0"), 10, 64)
	if e != nil || pixel < 0 {
		runtime.Bad(c, "Pixel 编号无效")
		return
	}
	name := c.Query("event_name")
	// Automatic redirects are a separate custom event in both tests and event logs.
	if name != "" && name != "PageView" && name != "Contact" && name != EventName && name != AutoRedirectEventName {
		runtime.Bad(c, "事件名称无效")
		return
	}
	args := []any{connection, status, pixel, name}
	where := " WHERE ($1::bigint=0 OR e.connection_id=$1) AND ($2::text='' OR e.status=$2) AND ($3::bigint=0 OR e.pixel_record_id=$3) AND ($4::text='' OR e.event_name=$4)"
	var total int
	e = h.Service.Core.DB.QueryRow(ctx, "SELECT count(*) FROM meta_events e"+where, args...).Scan(&total)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	rows, e := h.Service.Core.DB.Query(ctx, `SELECT e.id,e.connection_id,c.name,e.visit_id,e.event_name,e.event_time,e.is_test,e.status,e.attempts,e.last_error,e.events_received,e.fbtrace_id,e.created_at,e.updated_at,e.pixel_id,e.pixel_record_id FROM meta_events e JOIN meta_connections c ON c.id=e.connection_id`+where+" ORDER BY e.created_at DESC,e.id DESC LIMIT 50 OFFSET $5", append(args, (page-1)*50)...)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	defer rows.Close()
	items := []EventRecord{}
	for rows.Next() {
		var v EventRecord
		e = rows.Scan(&v.ID, &v.ConnectionID, &v.ConnectionName, &v.VisitID, &v.EventName, &v.EventTime, &v.IsTest, &v.Status, &v.Attempts, &v.LastError, &v.EventsReceived, &v.Trace, &v.CreatedAt, &v.UpdatedAt, &v.PixelID, &v.PixelRecordID)
		if e != nil {
			runtime.ServerError(c, e)
			return
		}
		items = append(items, v)
	}
	if e = rows.Err(); e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": 50})
}
func (h *Handler) retryEvent(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if e := h.Service.RetryEvent(ctx, c.Param("id")); e != nil {
		c.JSON(409, gin.H{"error": e.Error()})
		return
	}
	c.JSON(200, gin.H{"ok": true})
}
