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
	// POST query keeps event filters out of the address bar and request URL.
	g.POST("/events/query", h.listEvents)
	g.POST("/events/:id/retry", h.retryEvent)
}

type eventQueryInput struct {
	Page          int    `json:"page"`
	ConnectionID  int64  `json:"connection_id"`
	Status        string `json:"status"`
	PixelRecordID int64  `json:"pixel_record_id"`
	EventName     string `json:"event_name"`
}

func (h *Handler) listEvents(c *gin.Context) {
	input := eventQueryInput{Page: 1}
	if c.Request.Method == "POST" {
		if c.ShouldBindJSON(&input) != nil {
			runtime.Bad(c, "事件筛选条件格式无效")
			return
		}
	} else {
		input.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
		input.Status = c.Query("status")
		input.EventName = c.Query("event_name")
		input.ConnectionID, _ = strconv.ParseInt(c.DefaultQuery("connection_id", "0"), 10, 64)
		input.PixelRecordID, _ = strconv.ParseInt(c.DefaultQuery("pixel_record_id", "0"), 10, 64)
	}
	if input.Page < 1 || input.Page > 100000 || input.ConnectionID < 0 || input.PixelRecordID < 0 {
		runtime.Bad(c, "页码无效")
		return
	}
	valid := map[string]bool{"": true, "pending": true, "processing": true, "succeeded": true, "retry": true, "failed": true, "expired": true, "skipped": true}
	if !valid[input.Status] {
		runtime.Bad(c, "事件状态无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	// Automatic redirects are a separate custom event in both tests and event logs.
	// Current standard events and immutable historical names are both valid log filters.
	if input.EventName != "" && input.EventName != "PageView" && input.EventName != "StartListening" && input.EventName != "ViewContent" && input.EventName != EventName && input.EventName != LegacyManualEventName && input.EventName != LegacyAutoRedirectEventName {
		runtime.Bad(c, "事件名称无效")
		return
	}
	args := []any{input.ConnectionID, input.Status, input.PixelRecordID, input.EventName}
	where := " WHERE ($1::bigint=0 OR e.connection_id=$1) AND ($2::text='' OR e.status=$2) AND ($3::bigint=0 OR e.pixel_record_id=$3) AND ($4::text='' OR e.event_name=$4)"
	var total int
	err := h.Service.Core.DB.QueryRow(ctx, "SELECT count(*) FROM meta_events e"+where, args...).Scan(&total)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	rows, err := h.Service.Core.DB.Query(ctx, `SELECT e.id,e.connection_id,c.name,e.visit_id,e.event_name,e.event_time,e.is_test,e.status,e.attempts,e.last_error,e.events_received,e.fbtrace_id,e.created_at,e.updated_at,e.pixel_id,e.pixel_record_id,e.audio_novel_id FROM meta_events e JOIN meta_connections c ON c.id=e.connection_id`+where+" ORDER BY e.created_at DESC,e.id DESC LIMIT 50 OFFSET $5", append(args, (input.Page-1)*50)...)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	defer rows.Close()
	items := []EventRecord{}
	for rows.Next() {
		var v EventRecord
		err = rows.Scan(&v.ID, &v.ConnectionID, &v.ConnectionName, &v.VisitID, &v.EventName, &v.EventTime, &v.IsTest, &v.Status, &v.Attempts, &v.LastError, &v.EventsReceived, &v.Trace, &v.CreatedAt, &v.UpdatedAt, &v.PixelID, &v.PixelRecordID, &v.AudioNovelID)
		if err != nil {
			runtime.ServerError(c, err)
			return
		}
		items = append(items, v)
	}
	if err = rows.Err(); err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": input.Page, "page_size": 50})
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
