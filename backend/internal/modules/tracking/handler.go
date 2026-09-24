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
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/geoip"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	TikTok         *tiktok.Service
	Landing        *landing.Handler
	AudioNovelPage interface {
		Render(*gin.Context, links.Link, string, bool, bool)
		Unavailable(*gin.Context, int, string)
	}
	NovelPage interface {
		Render(*gin.Context, links.Link, string, bool, string, bool)
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

func (a *Handler) Novel(c *gin.Context) { a.track(c, "novel") }

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
	// Project-specific links may only enter their own frontend; legacy links remain compatible.
	if l.ProductType == "novel" && surface != "novel" {
		a.unavailable(c, surface, 404, "This link was not found.")
		return
	}
	if l.ProductType == "audio_novel" && surface != "audio_novel" {
		a.unavailable(c, surface, 404, "This link was not found.")
		return
	}
	ip := net.ParseIP(c.ClientIP())
	ua := runtime.Bounded(c.GetHeader("User-Agent"), 1024)
	high := a.Exceed("click:"+a.Sign(c.ClientIP()), 120, time.Minute)
	class, reason := Classify(c.Request.Method, ua, c.GetHeader("Purpose")+" "+c.GetHeader("Sec-Purpose"), high)
	if surface == "novel" && l.ProductType == "novel" {
		// Freeze the binding before reading the final link snapshot so a concurrent
		// pre-visit edit cannot attribute the first real reader to a stale setup.
		// HEAD, bot and suspicious traffic stay observable without locking the link.
		if c.Request.Method == "GET" && class == "normal" {
			if freezeErr := (Repository{DB: a.DB}).FreezeNovelLink(ctx, l.ID); freezeErr != nil {
				if freezeErr == pgx.ErrNoRows {
					a.unavailable(c, surface, 410, "This story is unavailable.")
					return
				}
				c.String(503, "This link is temporarily unavailable. Please try again later.")
				return
			}
			l, e = (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
			if e != nil {
				c.String(503, "This link is temporarily unavailable. Please try again later.")
				return
			}
		}
		if !l.Enabled || l.NovelID == nil {
			a.unavailable(c, surface, 410, "This story is unavailable.")
			return
		}
		var available bool
		if a.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM novels WHERE id=$1 AND enabled AND deleted_at IS NULL)", l.NovelID).Scan(&available) != nil {
			c.String(503, "This link is temporarily unavailable. Please try again later.")
			return
		}
		if !available {
			a.unavailable(c, surface, 410, "This story is unavailable.")
			return
		}
	}
	if surface == "audio_novel" && l.ProductType == "audio_novel" {
		// Freeze before taking the visit snapshot so the first reader and a
		// concurrent admin edit cannot attribute different content or Pixels.
		if c.Request.Method == "GET" && class == "normal" {
			if freezeErr := (Repository{DB: a.DB}).FreezeAudioNovelLink(ctx, l.ID); freezeErr != nil {
				if freezeErr == pgx.ErrNoRows {
					a.unavailable(c, surface, 410, "This audio story is unavailable.")
					return
				}
				c.String(503, "This link is temporarily unavailable. Please try again later.")
				return
			}
			l, e = (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
			if e != nil {
				c.String(503, "This link is temporarily unavailable. Please try again later.")
				return
			}
		}
		if !l.Enabled || l.AudioNovelID == nil {
			a.unavailable(c, surface, 410, "This audio story is unavailable.")
			return
		}
		var playable bool
		// Historical MP3 rows may have unknown duration metadata. They remain
		// playable; only the 90% completion metric requires a known duration.
		if a.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audio_novels
			WHERE id=$1 AND enabled AND deleted_at IS NULL AND audio_path<>'')`, l.AudioNovelID).Scan(&playable) != nil {
			c.String(503, "This link is temporarily unavailable. Please try again later.")
			return
		}
		if !playable {
			a.unavailable(c, surface, 410, "This audio story is unavailable.")
			return
		}
	}
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
			if v = resolvedAttributionValue(v, 512); v != "" {
				params[key] = v
			}
		}
	}
	// TikTok click and ad hierarchy identifiers are stored in dedicated frozen
	// columns; ttclid is intentionally excluded from the generic parameters JSON.
	ttclid := resolvedAttributionValue(c.Query("ttclid"), 2048)
	tiktokAdgroupID := resolvedAttributionValue(c.Query("adgroup_id"), 512)
	tiktokCreativeID := resolvedAttributionValue(c.Query("creative_id"), 512)
	tiktokAdIDV2 := resolvedAttributionValue(c.Query("ad_id_v2"), 512)
	tiktokPlacement := resolvedAttributionValue(c.Query("placement"), 512)
	conflict := false
	resolve := func(bound, key string) string {
		v := resolvedAttributionValue(params[key], 512)
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
	adPlatform := l.AdPlatform
	if adPlatform == "" {
		adPlatform = "meta"
	}
	var tiktokPixelID *int64
	tiktokTTP, tiktokContextCipher := "", ""
	if (surface == "novel" || surface == "audio_novel") && c.Request.Method == "GET" && class == "normal" && adPlatform == "tiktok" {
		tiktokPixelID = l.TikTokPixelID
		if cookie, cookieErr := c.Cookie("_ttp"); cookieErr == nil {
			tiktokTTP = safeCookieValue(cookie, 512)
		}
		if a.TikTok != nil {
			pageURL := strings.TrimRight(a.Config.PublicURL, "/") + c.Request.URL.RequestURI()
			contextCipher, cipherErr := a.TikTok.SealVisitContext(tiktok.VisitContext{
				IP: c.ClientIP(), UserAgent: ua, PageURL: pageURL, Referrer: c.GetHeader("Referer"),
			}, eventID)
			if cipherErr != nil {
				// Reading and internal analytics remain available if private context
				// encryption fails; delivery diagnostics expose no request secrets.
				slog.Error("TIKTOK_VISIT_CONTEXT_ENCRYPT_FAILED", "link_id", l.ID, "error", cipherErr)
			} else {
				tiktokContextCipher = contextCipher
			}
		}
	} else {
		// Non-reader traffic never receives a reusable TikTok attribution snapshot.
		ttclid, tiktokAdgroupID, tiktokCreativeID, tiktokAdIDV2, tiktokPlacement = "", "", "", "", ""
	}
	// Direct mode returns a measured handoff page, so the stored status matches
	// the 200 response while the event type continues to identify the link mode.
	eventType, status := "redirect", 200
	if l.Mode == "landing" || surface == "audio_novel" || surface == "novel" {
		eventType, status = "landing", 200
	}
	// Store before emitting the Redirect. No raw IP, full URL query or raw User-Agent is persisted.
	e = (Repository{DB: a.DB}).Record(ctx, Event{ID: eventID, LinkID: l.ID, NovelID: l.NovelID, AudioNovelID: l.AudioNovelID, VisitorID: vid, CookieStatus: cookieStatus, Method: c.Request.Method, TargetURL: l.TargetURL, Device: device, OS: osName, Browser: browser, Country: country, Region: region, City: city, Source: source, CampaignID: campaign, AdsetID: adset, AdID: ad, Referrer: ref, Parameters: b, AttributionConflict: conflict, Classification: class, Reason: reason, Type: eventType, Surface: surface, Status: status, MetaConnectionID: l.MetaConnectionID, MetaPixelID: l.MetaPixelID, TikTokPixelID: tiktokPixelID, AdPlatform: adPlatform, TikTokTTCLID: ttclid, TikTokTTP: tiktokTTP, TikTokAdgroupID: tiktokAdgroupID, TikTokCreativeID: tiktokCreativeID, TikTokAdIDV2: tiktokAdIDV2, TikTokPlacement: tiktokPlacement, TikTokContextCipher: tiktokContextCipher, TimeSpentThreshold: l.TimeSpentThreshold})
	if e != nil {
		a.WriteFailures.Add(1)
		slog.Error("CLICK_WRITE_FAILED: redirect continues; analytics gap", "link_id", l.ID, "error", e)
	}
	if surface == "audio_novel" {
		playbackEligible := e == nil && c.Request.Method == "GET" && class == "normal" && l.ProductType == "audio_novel"
		a.AudioNovelPage.Render(c, l, eventID, e == nil, playbackEligible)
		return
	}
	if surface == "novel" {
		// Language preferences follow the same visitor eligibility as anonymous
		// analytics cookies, so crawler and preview requests remain cookie-free.
		allowLanguageCookie := c.Request.Method == "GET" && (class == "normal" || class == "suspicious")
		a.NovelPage.Render(c, l, eventID, e == nil, country, allowLanguageCookie)
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
	if surface == "novel" && a.NovelPage != nil {
		a.NovelPage.Unavailable(c, status, message)
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

func resolvedAttributionValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsAny(value, "\r\n") ||
		strings.Contains(value, "{{") || strings.Contains(value, "}}") ||
		(len(value) > 4 && strings.HasPrefix(value, "__") && strings.HasSuffix(value, "__")) {
		return ""
	}
	return value
}

func safeCookieValue(value string, limit int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}
