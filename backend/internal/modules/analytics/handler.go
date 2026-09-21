package analytics

import (
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct{ *runtime.Core }

func (a *Handler) Overview(c *gin.Context) {
	f, e := ParseFilter(c, a.Config.Timezone)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	data, e := (Repository{DB: a.DB}).Overview(ctx, f)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, gin.H{"summary": data.Summary, "trends": data.Trends, "devices": data.Devices, "countries": data.Countries, "sources": data.Sources, "ads": data.Ads, "health": gin.H{"write_failures": a.WriteFailures.Load(), "geo_enabled": a.Geo != nil, "cookie_mode": a.Config.CookieMode}, "timezone": f.TZ, "retention_days": a.Config.RetentionDays})

}

func (a *Handler) Clicks(c *gin.Context) {
	f, e := ParseFilter(c, a.Config.Timezone)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	page, e := strconv.Atoi(c.DefaultQuery("page", "1"))
	if e != nil || page < 1 || page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	var total int64
	total, e = (Repository{DB: a.DB}).Count(ctx, f)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	items, e := (Repository{DB: a.DB}).Events(ctx, f, 50, (page-1)*50)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.JSON(200, gin.H{"items": items, "page": page, "total": total})
}

func (a *Handler) Export(c *gin.Context) {
	f, e := ParseFilter(c, a.Config.Timezone)
	if e != nil {
		runtime.Bad(c, e.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	items, e := (Repository{DB: a.DB}).Events(ctx, f, 100001, 0)
	if e != nil {
		runtime.ServerError(c, e)
		return
	}
	if len(items) > 100000 {
		runtime.Bad(c, "单次导出最多 100000 条，请缩小日期范围")
		return
	}
	if e = a.Audit(ctx, "clicks.export", gin.H{"count": len(items), "start": f.Start, "end": f.End}); e != nil {
		runtime.ServerError(c, e)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="clicks.csv"`)
	c.Writer.Write([]byte{0xef, 0xbb, 0xbf})
	w := csv.NewWriter(c.Writer)
	w.Write([]string{"event_id", "time", "code", "visitor_id", "cookie_status", "method", "device", "os", "browser", "country", "region", "city", "source", "ad_id", "classification", "reason", "referrer", "attribution_conflict", "event_type", "surface", "whatsapp_clicked_at", "auto_redirected_at"})
	for _, v := range items {
		automatic := ""
		if v.AutoRedirectedAt != nil {
			automatic = v.AutoRedirectedAt.In(config.Location(f.TZ)).Format(time.RFC3339)
		}
		clicked := ""
		if v.WhatsAppClickedAt != nil {
			clicked = v.WhatsAppClickedAt.In(config.Location(f.TZ)).Format(time.RFC3339)
		}
		r := []string{v.ID, v.OccurredAt.In(config.Location(f.TZ)).Format(time.RFC3339), v.Code, v.VisitorID, v.CookieStatus, v.Method, v.Device, v.OS, v.Browser, v.Country, v.Region, v.City, v.Source, v.AdID, v.Classification, v.Reason, v.Referrer, fmt.Sprint(v.AttributionConflict), v.EventType, v.Surface, clicked, automatic}
		for i := range r {
			r[i] = CSVSafe(r[i])
		}
		w.Write(r)
	}
	w.Flush()
}
