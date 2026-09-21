package tracking

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/mileusna/useragent"

	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/platform/geoip"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Landing        *landing.Handler
	AudioNovelPage interface {
		Render(*gin.Context, links.Link, string, bool)
		Unavailable(*gin.Context, int, string)
	}
}

func (a *Handler) Redirect(c *gin.Context) {
	a.track(c, "short_link")
}

// AudioNovel records a standalone literature visit while reusing the same link,
// attribution and visitor-classification contract as the original short link.
func (a *Handler) AudioNovel(c *gin.Context) {
	a.track(c, "audio_novel")
}

func (a *Handler) track(c *gin.Context, surface string) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 1500*time.Millisecond)
	defer cancel()
	l, e := (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
	if e == pgx.ErrNoRows {
		a.unavailable(c, surface, 404, "This link was not found.")
		return
	}
	if e != nil {
		c.String(503, "This link is temporarily unavailable. Please try again later.")
		return
	}
	// Operators explicitly control availability through the enabled flag.
	if !l.Enabled {
		a.unavailable(c, surface, 410, "This link is disabled.")
		return
	}
	ip := net.ParseIP(c.ClientIP())
	ua := runtime.Bounded(c.GetHeader("User-Agent"), 1024)
	high := a.Exceed("click:"+a.Sign(c.ClientIP()), 120, time.Minute)
	class, reason := Classify(c.Request.Method, ua, c.GetHeader("Purpose")+" "+c.GetHeader("Sec-Purpose"), high)
	visitor, cookieStatus := a.visitor(c, class == "normal" || class == "suspicious")
	parsed := useragent.Parse(ua)
	device := "unknown"
	if parsed.Mobile {
		device = "mobile"
	} else if parsed.Tablet {
		device = "tablet"
	} else if parsed.Desktop {
		device = "desktop"
	}
	browser := parsed.Name
	if strings.Contains(ua, "FBAN") || strings.Contains(ua, "FBAV") {
		browser = "Facebook 内置浏览器"
	} else if strings.Contains(ua, "Instagram") {
		browser = "Instagram 内置浏览器"
	}
	if browser == "" {
		browser = "unknown"
	}
	osName := parsed.OS
	if osName == "" {
		osName = "unknown"
	}
	country, region, city := geoip.Lookup(a.Geo, ip)
	params := map[string]string{}
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term", "campaign_id", "adset_id", "ad_id", "fbclid", "campaign_name", "adset_name", "ad_name", "placement", "site_source_name"} {
		// Meta's URL builder can preserve spaces around ampersands; normalize every
		// attribution value before persistence so ad-level statistics join reliably.
		v := strings.TrimSpace(c.Query(key))
		// Canonicalize the common fbcli typo without persisting a second attribution field.
		if key == "fbclid" && v == "" {
			v = strings.TrimSpace(c.Query("fbcli"))
		}
		if v != "" {
			if key == "fbclid" {
				if len(v) <= 2048 && !strings.ContainsAny(v, "\r\n") {
					params[key] = v
				}
				continue
			}
			params[key] = runtime.Bounded(v, 512)
		}
	}
	conflict := false
	resolve := func(bound, key string) string {
		v := params[key]
		if strings.Contains(v, "{{") || strings.Contains(v, "}}") {
			v = ""
		}
		if l.AttributionMode == "dynamic" {
			return v
		}
		if bound != "" {
			if v != "" && v != bound {
				conflict = true
			}
			return bound
		}
		return v
	}
	campaign, adset, ad := resolve(l.CampaignID, "campaign_id"), resolve(l.AdsetID, "adset_id"), resolve(l.AdID, "ad_id")
	ref := ""
	if u, e := url.Parse(c.GetHeader("Referer")); e == nil {
		ref = runtime.Bounded(u.Hostname(), 255)
	}
	source := platformSource(l.Channel)
	if l.AttributionMode == "dynamic" {
		for _, key := range []string{"site_source_name", "utm_source"} {
			if v := strings.TrimSpace(params[key]); v != "" && !strings.Contains(v, "{{") && !strings.Contains(v, "}}") {
				source = platformSource(v)
				break
			}
		}
	}
	if source == "" {
		source = platformSource(params["utm_source"])
	}
	if source == "" {
		source = ref
	}
	if source == "" {
		source = "unknown"
	}
	b, _ := json.Marshal(params)
	var vid any
	if visitor != "" {
		vid = visitor
	}
	eventID := runtime.Token()
	// Direct mode returns a measured handoff page, so the stored status matches
	// the 200 response while the event type continues to identify the link mode.
	eventType, status := "redirect", 200
	if l.Mode == "landing" || surface == "audio_novel" {
		eventType, status = "landing", 200
	}
	// Store before emitting the Redirect. No raw IP, full URL query or raw User-Agent is persisted.
	e = (Repository{DB: a.DB}).Record(ctx, Event{ID: eventID, LinkID: l.ID, VisitorID: vid, CookieStatus: cookieStatus, Method: c.Request.Method, TargetURL: l.TargetURL, Device: device, OS: osName, Browser: browser, Country: country, Region: region, City: city, Source: source, CampaignID: campaign, AdsetID: adset, AdID: ad, Referrer: ref, Parameters: b, AttributionConflict: conflict, Classification: class, Reason: reason, Type: eventType, Surface: surface, Status: status, MetaConnectionID: l.MetaConnectionID, MetaPixelID: l.MetaPixelID, TimeSpentThreshold: l.TimeSpentThreshold})
	if e != nil {
		a.WriteFailures.Add(1)
		slog.Error("CLICK_WRITE_FAILED: redirect continues; analytics gap", "link_id", l.ID, "error", e)
	}
	if surface == "audio_novel" {
		a.AudioNovelPage.Render(c, l, eventID, e == nil)
		return
	}
	if l.Mode == "landing" {
		a.Landing.Render(c, l, eventID, e == nil)
		return
	}
	if e == nil && c.Request.Method == "GET" {
		// Direct mode has no rendered landing page or second browser callback, so
		// record only the automatic AddToCart handoff before navigation.
		fbc, _ := c.Cookie("_fbc")
		fbp, _ := c.Cookie("_fbp")
		actionCtx, actionCancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		actionErr := a.Landing.RecordDirect(actionCtx, eventID, l.ID, meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp})
		actionCancel()
		if actionErr != nil {
			a.WriteFailures.Add(1)
			slog.Error("DIRECT_ACTION_WRITE_FAILED: redirect continues; analytics gap", "link_id", l.ID, "error", actionErr)
		}
	}
	// Direct mode has already persisted the visit above; its response only
	// contains the configured top.location handoff.
	a.Landing.RenderDirect(c, l)
}

func (a *Handler) unavailable(c *gin.Context, surface string, status int, message string) {
	if surface == "audio_novel" && a.AudioNovelPage != nil {
		a.AudioNovelPage.Unavailable(c, status, message)
		return
	}
	a.Landing.Unavailable(c, status, message)
}

func platformSource(s string) string {
	switch strings.ToLower(s) {
	case "fb", "facebook":
		return "facebook"
	case "ig", "instagram":
		return "instagram"
	case "msg", "messenger":
		return "messenger"
	case "an", "audience_network":
		return "audience_network"
	}
	return strings.ToLower(s)
}
