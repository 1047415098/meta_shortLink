package novel

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/runtime"
)

type Bootstrap struct {
	Link                 *PublicLink `json:"link,omitempty"`
	Ticket               string      `json:"ticket,omitempty"`
	Surface              string      `json:"surface"`
	CookieEnabled        bool        `json:"cookie_enabled"`
	MetaMeasurement      bool        `json:"meta_measurement"`
	MetaBrowserPixelID   string      `json:"meta_browser_pixel_id,omitempty"`
	MetaPageViewEventID  string      `json:"meta_pageview_event_id,omitempty"`
	MetaTimeSpentEventID string      `json:"meta_time_spent_event_id,omitempty"`
	AdPlatform           string      `json:"ad_platform"`
	TikTokEnabled        bool        `json:"tiktok_enabled"`
	TikTokPixelCode      string      `json:"tiktok_pixel_code,omitempty"`
	TikTokStartEventID   string      `json:"tiktok_start_event_id,omitempty"`
	TikTokQualifiedID    string      `json:"tiktok_qualified_event_id,omitempty"`
	Locale               string      `json:"locale"`
	AvailableLocales     []string    `json:"available_locales"`
	Error                *PageError  `json:"error,omitempty"`
}
type PublicLink struct {
	Code               string `json:"code"`
	EntryStorySlug     string `json:"entry_story_slug,omitempty"`
	TimeSpentThreshold int    `json:"time_spent_threshold"`
}
type PageError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (h *Handler) Render(c *gin.Context, link links.Link, eventID string, recorded bool, country string, allowLanguageCookie bool) {
	publicLink := &PublicLink{Code: link.Code, TimeSpentThreshold: link.TimeSpentThreshold}
	available := []string{"en"}
	if link.NovelID != nil {
		// The bootstrap carries the bound slug so the SPA can switch routes without a second entry request.
		if item, err := (Repository{DB: h.DB}).ByID(c.Request.Context(), *link.NovelID); err == nil {
			publicLink.EntryStorySlug = item.Slug
		}
		if locales, err := (Repository{DB: h.DB}).PublishedLocales(c.Request.Context(), *link.NovelID); err == nil {
			available = locales
		}
	}
	remembered, _ := c.Cookie(LanguageCookieName)
	locale := ResolveLocale(c.Query("lang"), remembered, country, available)
	platform := link.AdPlatform
	if platform == "" {
		platform = "meta"
	}
	data := Bootstrap{Link: publicLink, Surface: "novel", AdPlatform: platform, CookieEnabled: h.Config.CookieMode == "all", Locale: locale, AvailableLocales: available}
	c.Header("Content-Language", locale)
	if allowLanguageCookie && remembered != locale {
		// 只为真实读者记住语言选择；爬虫和平台预览请求必须保持无 Cookie。
		http.SetCookie(c.Writer, &http.Cookie{Name: LanguageCookieName, Value: locale, Path: "/", MaxAge: 365 * 24 * 60 * 60, Secure: h.Config.SecureCookies, SameSite: http.SameSiteLaxMode})
	}
	if recorded {
		data.Ticket = eventID + "." + h.Sign("contact:novel:"+link.Code+":"+eventID)
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

func (h *Handler) loadTikTok(c *gin.Context, eventID string, data *Bootstrap) error {
	err := h.DB.QueryRow(c.Request.Context(), `SELECT e.ad_platform,
		COALESCE($2::boolean AND e.ad_platform='tiktok' AND p.enabled AND c.enabled AND e.tiktok_pixel_code<>'',false) AS enabled,
		COALESCE(CASE WHEN $2::boolean AND e.ad_platform='tiktok' AND p.enabled AND c.enabled THEN e.tiktok_pixel_code ELSE '' END,'')
		FROM click_events e LEFT JOIN tiktok_pixels p ON p.id=e.tiktok_pixel_id AND p.pixel_code=e.tiktok_pixel_code
		LEFT JOIN tiktok_connections c ON c.id=p.connection_id WHERE e.id=$1`, eventID, h.Config.TikTokEnabled).
		Scan(&data.AdPlatform, &data.TikTokEnabled, &data.TikTokPixelCode)
	if err == nil && data.TikTokEnabled && data.TikTokPixelCode != "" {
		// IDs are public deduplication keys only; attribution and matching data remain server-side.
		data.TikTokStartEventID = tiktok.VisitEventID(eventID, "StartReading")
		data.TikTokQualifiedID = tiktok.VisitEventID(eventID, "ViewContent")
	}
	return err
}
func (h *Handler) Unavailable(c *gin.Context, status int, message string) {
	if err := h.render(c, status, Bootstrap{Surface: "novel", CookieEnabled: h.Config.CookieMode == "all", Locale: "en", AvailableLocales: []string{"en"}, Error: &PageError{Status: status, Message: message}}); err != nil {
		c.String(status, message)
	}
}
func (h *Handler) loadMeta(c *gin.Context, eventID string, data *Bootstrap) error {
	var pageViewEnabled, timeSpentEnabled bool
	err := h.DB.QueryRow(c.Request.Context(), `SELECT e.meta_measurement,COALESCE(CASE WHEN e.meta_measurement AND p.enabled AND (e.meta_pageview_enabled OR e.time_spent_threshold>0) THEN p.pixel_id ELSE '' END,''),e.meta_pageview_enabled,e.time_spent_threshold>0 FROM click_events e LEFT JOIN meta_pixels p ON p.id=e.meta_pixel_id WHERE e.id=$1`, eventID).Scan(&data.MetaMeasurement, &data.MetaBrowserPixelID, &pageViewEnabled, &timeSpentEnabled)
	if err == nil && data.MetaBrowserPixelID != "" && pageViewEnabled {
		data.MetaPageViewEventID = "wa_" + eventID + "_view"
	}
	if err == nil && data.MetaBrowserPixelID != "" && timeSpentEnabled {
		data.MetaTimeSpentEventID = "wa_" + eventID + "_time_spent"
	}
	return err
}
func (h *Handler) render(c *gin.Context, status int, data Bootstrap) error {
	page, err := os.ReadFile(filepath.Join(h.Config.NovelDir, "index.html"))
	if err != nil {
		return err
	}
	marker := []byte("<!--NOVEL_H5_BOOTSTRAP-->")
	if bytes.Count(page, marker) != 1 {
		return errors.New("novel H5 index must contain exactly one NOVEL_H5_BOOTSTRAP placeholder")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	nonce := runtime.Token()
	script := append([]byte(`<script id="novel-h5-data" type="application/json" nonce="`+nonce+`">`), payload...)
	script = append(script, []byte("</script>")...)
	page = bytes.Replace(page, marker, script, 1)
	page = novelMetadata(page)
	// 小说正文来自白名单 Markdown；广告脚本和请求只允许两个平台的官方精确域名。
	c.Header("Content-Security-Policy", novelContentSecurityPolicy(nonce))
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(status)
		return nil
	}
	c.Data(status, "text/html; charset=utf-8", page)
	return nil
}

func novelContentSecurityPolicy(nonce string) string {
	return "default-src 'none'; script-src 'self' 'nonce-" + nonce + "' https://connect.facebook.net https://analytics.tiktok.com; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data: https://www.facebook.com https://analytics.tiktok.com https://business-api.tiktok.com; " +
		"font-src 'self'; connect-src 'self' https://www.facebook.com https://analytics.tiktok.com https://business-api.tiktok.com; base-uri 'none'; frame-ancestors 'none'"
}

var titleTag = regexp.MustCompile(`(?is)<title>.*?</title>`)
var descriptionTag = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="[^"]*"\s*/?>`)

func novelMetadata(page []byte) []byte {
	title := html.EscapeString("Free Stories — Read Online")
	description := html.EscapeString("Discover free stories and continue reading chapter by chapter.")
	page = titleTag.ReplaceAll(page, []byte("<title>"+title+"</title>"))
	return descriptionTag.ReplaceAll(page, []byte(`<meta name="description" content="`+description+`" />`))
}
