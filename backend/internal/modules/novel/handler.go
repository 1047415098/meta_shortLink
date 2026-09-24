package novel

import (
	"context"
	"crypto/hmac"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Meta         *meta.Service
	TikTok       *tiktok.Service
	Translations *TranslationService
}

// View confirms the one document PageView using the signed visit created by tracking.
func (h *Handler) View(c *gin.Context) {
	if !h.validActionOrigin(c) {
		return
	}
	eventID, link, ok := h.validActionTicket(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	fbc, _ := c.Cookie("_fbc")
	fbp, _ := c.Cookie("_fbp")
	err := (landing.Repository{DB: h.DB, Meta: h.Meta}).MarkView(ctx, eventID, link.ID, "novel", meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp})
	if err == nil {
		// Novel H5 consumes JSON for every signed action, including Meta-only visits.
		c.JSON(200, gin.H{"ok": true})
		return
	}
	if err == pgx.ErrNoRows {
		c.Status(400)
		return
	}
	runtime.ServerError(c, err)
}

// StartReading promotes only chapter-one interaction from the original normal GET visit.
func (h *Handler) StartReading(c *gin.Context) {
	if !h.validActionOrigin(c) {
		return
	}
	eventID, link, ok := h.validActionTicket(c)
	if !ok {
		return
	}
	if c.PostForm("chapter") != "1" {
		runtime.Bad(c, "开始阅读事件只接受第一章")
		return
	}
	ttp, err := actionTikTokTTP(c)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	defer tx.Rollback(ctx)
	var platform string
	var startAt *time.Time
	err = tx.QueryRow(ctx, `SELECT ad_platform,tiktok_start_reading_at FROM click_events
		WHERE id=$1 AND link_id=$2 AND surface='novel' AND event_type='landing' AND method='GET'
		AND classification='normal' AND occurred_at>=now()-interval '2 hours 5 minutes' FOR UPDATE`, eventID, link.ID).
		Scan(&platform, &startAt)
	if errors.Is(err, pgx.ErrNoRows) {
		runtime.Bad(c, "访问票据无效或已过期")
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	if platform != "tiktok" {
		if err = tx.Commit(ctx); err != nil {
			runtime.ServerError(c, err)
			return
		}
		c.JSON(200, gin.H{"ok": true})
		return
	}
	if ttp != "" {
		if _, err = tx.Exec(ctx, `UPDATE click_events SET tiktok_ttp=$3
			WHERE id=$1 AND link_id=$2 AND ad_platform='tiktok' AND tiktok_ttp=''`, eventID, link.ID, ttp); err != nil {
			runtime.ServerError(c, err)
			return
		}
	}
	if startAt == nil {
		var value time.Time
		if err = tx.QueryRow(ctx, `UPDATE click_events SET tiktok_start_reading_at=COALESCE(tiktok_start_reading_at,now())
			WHERE id=$1 AND link_id=$2 RETURNING tiktok_start_reading_at`, eventID, link.ID).Scan(&value); err != nil {
			runtime.ServerError(c, err)
			return
		}
		startAt = &value
	}
	if h.TikTok == nil {
		runtime.ServerError(c, errors.New("TikTok 服务尚未初始化"))
		return
	}
	browserEvent, _, err := h.TikTok.QueueVisitEventTx(ctx, tx, eventID, link.ID, "StartReading", *startAt)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	if err = tx.Commit(ctx); err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"ok": true, "tiktok_event": browserEvent})
}

// TimeSpent reuses the shared, signed stay-event implementation for the novel surface.
func (h *Handler) TimeSpent(c *gin.Context) {
	proxy := landing.Handler{Core: h.Core, Meta: h.Meta}
	proxy.TimeSpentForSurface(c, "novel", "contact:novel:")
}

func (h *Handler) validActionOrigin(c *gin.Context) bool {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != c.Request.Host {
			c.Status(403)
			return false
		}
	}
	return true
}

func (h *Handler) validActionTicket(c *gin.Context) (string, links.Link, bool) {
	parts := strings.Split(c.PostForm("ticket"), ".")
	if len(parts) != 2 || !runtime.VisitorPattern.MatchString(parts[0]) || !hmac.Equal([]byte(parts[1]), []byte(h.Sign("contact:novel:"+c.Param("code")+":"+parts[0]))) {
		c.Status(400)
		return "", links.Link{}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: h.DB}).ByCode(ctx, c.Param("code"))
	if err == pgx.ErrNoRows {
		c.Status(404)
		return "", links.Link{}, false
	}
	if err != nil {
		runtime.ServerError(c, err)
		return "", links.Link{}, false
	}
	if !link.Enabled {
		c.Status(410)
		return "", links.Link{}, false
	}
	if link.ProductType == "novel" && link.NovelID == nil {
		c.Status(410)
		return "", links.Link{}, false
	}
	return parts[0], link, true
}
