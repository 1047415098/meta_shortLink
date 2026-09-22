package novel

import (
	"context"
	"crypto/hmac"
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
		c.Status(204)
		return
	}
	if err == pgx.ErrNoRows {
		c.Status(400)
		return
	}
	runtime.ServerError(c, err)
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
