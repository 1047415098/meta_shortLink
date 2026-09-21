package audionovel

import (
	"context"
	"crypto/hmac"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Meta *meta.Service
}

func (a *Handler) View(c *gin.Context)    { a.action(c, true) }
func (a *Handler) Contact(c *gin.Context) { a.action(c, false) }

// TimeSpent delegates to the shared signed action implementation while retaining the audio novel surface.
func (a *Handler) TimeSpent(c *gin.Context) {
	proxy := landing.Handler{Core: a.Core, Meta: a.Meta}
	proxy.TimeSpentForSurface(c, "audio_novel", "contact:audio_novel:")
}

func (a *Handler) action(c *gin.Context, view bool) {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != c.Request.Host {
			c.String(403, "Please use the enquiry button on the original page.")
			return
		}
	}
	parts := strings.Split(c.PostForm("ticket"), ".")
	if len(parts) != 2 || !runtime.VisitorPattern.MatchString(parts[0]) || !hmac.Equal([]byte(parts[1]), []byte(a.Sign("contact:audio_novel:"+c.Param("code")+":"+parts[0]))) {
		c.String(400, "This enquiry link is invalid. Refresh the page and try again.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
	if err == pgx.ErrNoRows {
		c.Status(404)
		return
	}
	if err != nil {
		audioNovelError(c, err)
		return
	}
	if !link.Enabled {
		c.String(410, "This enquiry link is disabled. Please reopen the original page.")
		return
	}
	trigger := c.PostForm("trigger")
	if trigger != "" && trigger != "manual" {
		runtime.Bad(c, "The literature site supports manual enquiries only.")
		return
	}
	fbc, _ := c.Cookie("_fbc")
	fbp, _ := c.Cookie("_fbp")
	input := meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp}
	repository := landing.Repository{DB: a.DB, Meta: a.Meta}
	if view {
		err = repository.MarkView(ctx, parts[0], link.ID, "audio_novel", input)
		if err == nil {
			c.Status(204)
			return
		}
	} else {
		var target string
		target, err = repository.MarkContact(ctx, parts[0], link.ID, "audio_novel", false, input)
		if err == nil {
			c.JSON(200, gin.H{"target_url": target})
			return
		}
	}
	if err == pgx.ErrNoRows {
		c.String(400, "This page has expired. Refresh the page and try again.")
		return
	}
	a.WriteFailures.Add(1)
	audioNovelError(c, err)
}

func audioNovelError(c *gin.Context, err error) {
	slog.Error("audio novel request failed", "error", err)
	c.String(503, "This service is temporarily unavailable. Please try again later.")
}
