package landing

import (
	"context"
	"crypto/hmac"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Meta *meta.Service
}

// Contact accepts a manual or timer-triggered form submission; these are stored separately. Signed visit tickets bind attribution
// to the landing request; retries update the same row, even without cookies.
func (a *Handler) View(c *gin.Context)    { a.contact(c, true) }
func (a *Handler) Contact(c *gin.Context) { a.contact(c, false) }
func (a *Handler) contact(c *gin.Context, view bool) {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != c.Request.Host {
			c.String(403, "Please use the enquiry button on the original page.")
			return
		}
	}
	parts := strings.Split(c.PostForm("ticket"), ".")
	if len(parts) != 2 || !runtime.VisitorPattern.MatchString(parts[0]) || !hmac.Equal([]byte(parts[1]), []byte(a.Sign("contact:"+c.Param("code")+":"+parts[0]))) {
		c.String(400, "This enquiry link is invalid. Refresh the original page and try again.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	l, err := (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
	if err == pgx.ErrNoRows {
		c.Status(404)
		return
	}
	if err != nil {
		landingError(c, err)
		return
	}
	if !l.Enabled || l.Mode != "landing" || (l.ExpiresAt != nil && !time.Now().Before(*l.ExpiresAt)) {
		c.String(410, "This enquiry link has expired or changed. Please reopen the original link.")
		return
	}
	trigger := c.PostForm("trigger")
	if trigger != "" && trigger != "manual" && trigger != "auto" {
		runtime.Bad(c, "Invalid redirect type.")
		return
	}
	if trigger == "auto" {
		if l.LandingDelay == 0 {
			runtime.Bad(c, "Automatic redirect is disabled. Please return to the page and use the enquiry button.")
			return
		}
	}
	var target string
	fbc, _ := c.Cookie("_fbc")
	fbp, _ := c.Cookie("_fbp")
	if view {
		err = (Repository{DB: a.DB, Meta: a.Meta}).MarkView(ctx, parts[0], l.ID, meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp})
		if err != nil {
			if err == pgx.ErrNoRows {
				c.Status(400)
			} else {
				landingError(c, err)
			}
			return
		}
		c.Status(204)
		return
	}
	target, err = (Repository{DB: a.DB, Meta: a.Meta}).MarkContact(ctx, parts[0], l.ID, trigger == "auto", meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp})
	if err == pgx.ErrNoRows {
		c.String(400, "This page has expired. Refresh the original page and try again.")
		return
	}
	if err != nil {
		a.WriteFailures.Add(1)
		landingError(c, err)
		return
	}
	// Fetch clients navigate only after receiving confirmation that the signed visit was updated.
	if strings.Contains(c.GetHeader("Accept"), "application/json") {
		c.JSON(200, gin.H{"target_url": target})
		return
	}
	// Native forms remain a no-JavaScript fallback and preserve the historical redirect contract.
	c.Redirect(303, target)
}

func landingError(c *gin.Context, err error) {
	slog.Error("landing request failed", "error", err)
	c.String(503, "This service is temporarily unavailable. Please try again later.")
}
