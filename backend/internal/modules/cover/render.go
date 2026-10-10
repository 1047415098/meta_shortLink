package cover

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/novel"
	"whatsapp-analytics/internal/platform/runtime"
)

const languageCookieName = "cover_lang"

type PublicLink struct {
	Code               string `json:"code"`
	TimeSpentThreshold int    `json:"time_spent_threshold"`
}

type Bootstrap struct {
	Link                 *PublicLink `json:"link,omitempty"`
	Ticket               string      `json:"ticket,omitempty"`
	Surface              string      `json:"surface"`
	AdPlatform           string      `json:"ad_platform"`
	MetaMeasurement      bool        `json:"meta_measurement"`
	MetaBrowserPixelID   string      `json:"meta_browser_pixel_id,omitempty"`
	MetaPageViewEventID  string      `json:"meta_pageview_event_id,omitempty"`
	MetaTimeSpentEventID string      `json:"meta_time_spent_event_id,omitempty"`
	TikTokEnabled        bool        `json:"tiktok_enabled"`
	TikTokPixelCode      string      `json:"tiktok_pixel_code,omitempty"`
	TikTokPageViewID     string      `json:"tiktok_pageview_event_id,omitempty"`
	TikTokQualifiedID    string      `json:"tiktok_qualified_event_id,omitempty"`
	Locale               string      `json:"locale"`
	AvailableLocales     []string    `json:"available_locales"`
	Error                *PageError  `json:"error,omitempty"`
}

type PageError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (h *Handler) Render(c *gin.Context, link links.Link, eventID string, recorded bool, country string, allowLanguageCookie bool) {
	platform := link.AdPlatform
	if platform == "" {
		platform = "meta"
	}
	available := novel.SupportedLocaleCodes()
	remembered, _ := c.Cookie(languageCookieName)
	locale := novel.ResolveLocale(c.Query("lang"), remembered, country, available)
	data := Bootstrap{
		Link: &PublicLink{Code: link.Code, TimeSpentThreshold: link.TimeSpentThreshold}, Surface: "cover",
		AdPlatform: platform, Locale: locale, AvailableLocales: available,
	}
	c.Header("Content-Language", locale)
	if allowLanguageCookie && remembered != locale {
		// Cover preferences stay separate from novel translation availability.
		http.SetCookie(c.Writer, &http.Cookie{Name: languageCookieName, Value: locale, Path: "/", MaxAge: 365 * 24 * 60 * 60, Secure: h.Config.SecureCookies, SameSite: http.SameSiteLaxMode})
	}
	if recorded {
		data.Ticket = eventID + "." + h.Sign("contact:cover:"+link.Code+":"+eventID)
		if platform == "tiktok" {
			_ = h.loadTikTok(c, eventID, &data)
		} else {
			_ = h.loadMeta(c, eventID, &data)
		}
	}
	if err := h.render(c, 200, data); err != nil {
		runtime.ServerError(c, err)
	}
}

func (h *Handler) loadMeta(c *gin.Context, eventID string, data *Bootstrap) error {
	var pageViewEnabled, timeSpentEnabled bool
	err := h.DB.QueryRow(c.Request.Context(), `SELECT e.meta_measurement,
		COALESCE(CASE WHEN e.meta_measurement AND p.enabled AND (e.meta_pageview_enabled OR e.time_spent_threshold>0) THEN p.pixel_id ELSE '' END,''),
		e.meta_pageview_enabled,e.time_spent_threshold>0
		FROM click_events e LEFT JOIN meta_pixels p ON p.id=e.meta_pixel_id WHERE e.id=$1`, eventID).
		Scan(&data.MetaMeasurement, &data.MetaBrowserPixelID, &pageViewEnabled, &timeSpentEnabled)
	if err == nil && data.MetaBrowserPixelID != "" && pageViewEnabled {
		data.MetaPageViewEventID = "short_" + eventID + "_view"
	}
	if err == nil && data.MetaBrowserPixelID != "" && timeSpentEnabled {
		data.MetaTimeSpentEventID = "short_" + eventID + "_time_spent"
	}
	return err
}

func (h *Handler) loadTikTok(c *gin.Context, eventID string, data *Bootstrap) error {
	err := h.DB.QueryRow(c.Request.Context(), `SELECT
		COALESCE($2::boolean AND e.ad_platform='tiktok' AND p.enabled AND tc.enabled AND e.tiktok_pixel_code<>'',false),
		COALESCE(CASE WHEN $2::boolean AND e.ad_platform='tiktok' AND p.enabled AND tc.enabled THEN e.tiktok_pixel_code ELSE '' END,'')
		FROM click_events e LEFT JOIN tiktok_pixels p ON p.id=e.tiktok_pixel_id AND p.pixel_code=e.tiktok_pixel_code
		LEFT JOIN tiktok_connections tc ON tc.id=p.connection_id WHERE e.id=$1`, eventID, h.Config.TikTokEnabled).
		Scan(&data.TikTokEnabled, &data.TikTokPixelCode)
	if err == nil && data.TikTokEnabled && data.TikTokPixelCode != "" {
		data.TikTokPageViewID = "short_" + eventID + "_view"
		data.TikTokQualifiedID = "short_" + eventID + "_qualified"
	}
	return err
}

func (h *Handler) Unavailable(c *gin.Context, status int, message string) {
	data := Bootstrap{Surface: "cover", AdPlatform: "meta", Locale: "en", AvailableLocales: novel.SupportedLocaleCodes(), Error: &PageError{Status: status, Message: message}}
	if err := h.render(c, status, data); err != nil {
		c.String(status, message)
	}
}

func (h *Handler) render(c *gin.Context, status int, data Bootstrap) error {
	page, err := os.ReadFile(filepath.Join(h.Config.CoverDir, "index.html"))
	if err != nil {
		return err
	}
	marker := []byte("<!--COVER_H5_BOOTSTRAP-->")
	if bytes.Count(page, marker) != 1 {
		return errors.New("cover H5 index must contain exactly one COVER_H5_BOOTSTRAP placeholder")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	nonce := runtime.Token()
	script := append([]byte(`<script id="cover-h5-data" type="application/json" nonce="`+nonce+`">`), payload...)
	script = append(script, []byte("</script>")...)
	page = bytes.Replace(page, marker, script, 1)
	c.Header("Content-Security-Policy", coverContentSecurityPolicy(nonce))
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == http.MethodHead {
		c.Status(status)
		return nil
	}
	c.Data(status, "text/html; charset=utf-8", page)
	return nil
}

func coverContentSecurityPolicy(nonce string) string {
	return "default-src 'none'; script-src 'self' 'nonce-" + nonce + "' https://connect.facebook.net https://analytics.tiktok.com; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: https://www.facebook.com https://analytics.tiktok.com https://business-api.tiktok.com; " +
		"font-src 'self' data:; connect-src 'self' https://www.facebook.com https://analytics.tiktok.com https://business-api.tiktok.com; base-uri 'none'; frame-ancestors 'none'"
}
