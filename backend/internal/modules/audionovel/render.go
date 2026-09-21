package audionovel

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"os"
	"path/filepath"
	"regexp"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/platform/runtime"
)

type PublicLink struct {
	Code      string `json:"code"`
	TargetURL string `json:"target_url"`
	// The audio novel app reads only the shared timer setting, never private landing copy.
	TimeSpentThreshold int `json:"time_spent_threshold"`
}

type Bootstrap struct {
	Link                 *PublicLink `json:"link"`
	Ticket               string      `json:"ticket"`
	Surface              string      `json:"surface"`
	CookieEnabled        bool        `json:"cookie_enabled"`
	MetaMeasurement      bool        `json:"meta_measurement"`
	MetaBrowserPixelID   string      `json:"meta_browser_pixel_id,omitempty"`
	MetaPageViewEventID  string      `json:"meta_pageview_event_id,omitempty"`
	MetaManualEventID    string      `json:"meta_manual_event_id,omitempty"`
	MetaTimeSpentEventID string      `json:"meta_time_spent_event_id,omitempty"`
	Error                *PageError  `json:"error,omitempty"`
}

type PageError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (a *Handler) Render(c *gin.Context, link links.Link, eventID string, recorded bool) {
	data := Bootstrap{Link: &PublicLink{Code: link.Code, TargetURL: link.TargetURL, TimeSpentThreshold: link.TimeSpentThreshold}, Surface: "audio_novel", CookieEnabled: a.Config.CookieMode == "all"}
	if recorded {
		data.Ticket = eventID + "." + a.Sign("contact:audio_novel:"+link.Code+":"+eventID)
		_ = a.loadMeta(c, eventID, &data)
	}
	if err := a.render(c, 200, data); err != nil {
		audioNovelError(c, err)
	}
}

func (a *Handler) Unavailable(c *gin.Context, status int, message string) {
	data := Bootstrap{Surface: "audio_novel", CookieEnabled: a.Config.CookieMode == "all", Error: &PageError{Status: status, Message: message}}
	if err := a.render(c, status, data); err != nil {
		c.String(status, message)
	}
}

func (a *Handler) loadMeta(c *gin.Context, eventID string, data *Bootstrap) error {
	var pageViewEnabled, manualEnabled, timeSpentEnabled bool
	err := a.DB.QueryRow(c.Request.Context(), `SELECT e.meta_measurement,
		COALESCE(CASE WHEN e.meta_measurement AND p.enabled AND (e.meta_pageview_enabled OR e.meta_manual_enabled OR e.time_spent_threshold>0) THEN p.pixel_id ELSE '' END,''),
		e.meta_pageview_enabled,e.meta_manual_enabled,e.time_spent_threshold>0
		FROM click_events e LEFT JOIN meta_pixels p ON p.id=e.meta_pixel_id WHERE e.id=$1`, eventID).Scan(&data.MetaMeasurement, &data.MetaBrowserPixelID, &pageViewEnabled, &manualEnabled, &timeSpentEnabled)
	if err != nil {
		return err
	}
	if data.MetaBrowserPixelID != "" && pageViewEnabled {
		data.MetaPageViewEventID = "wa_" + eventID + "_view"
	}
	if data.MetaBrowserPixelID != "" && manualEnabled {
		data.MetaManualEventID = "wa_" + eventID + "_manual"
	}
	if data.MetaBrowserPixelID != "" && timeSpentEnabled {
		data.MetaTimeSpentEventID = "wa_" + eventID + "_time_spent"
	}
	return nil
}

func (a *Handler) render(c *gin.Context, status int, data Bootstrap) error {
	page, err := os.ReadFile(filepath.Join(a.Config.AudioNovelDir, "index.html"))
	if err != nil {
		return err
	}
	marker := []byte("<!--AUDIO_NOVEL_BOOTSTRAP-->")
	if bytes.Count(page, marker) != 1 {
		return errors.New("audio novel index must contain exactly one AUDIO_NOVEL_BOOTSTRAP placeholder")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	nonce := runtime.Token()
	script := append([]byte(`<script id="audio-novel-data" type="application/json" nonce="`+nonce+`">`), payload...)
	script = append(script, []byte("</script>")...)
	page = bytes.Replace(page, marker, script, 1)
	page = audioNovelMetadata(page)
	// 语音小说站只加载自身资源、Meta Pixel 与 WhatsApp；正文不允许注入任意 HTML。
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' 'nonce-"+nonce+"' https://connect.facebook.net; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://www.facebook.com; font-src 'self'; connect-src 'self' https://www.facebook.com; form-action 'self' https://wa.me https://*.whatsapp.com whatsapp:; base-uri 'none'; frame-ancestors 'none'")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(status)
		return nil
	}
	c.Data(status, "text/html; charset=utf-8", page)
	return nil
}

var audioNovelTitleTag = regexp.MustCompile(`(?is)<title>.*?</title>`)
var audioNovelDescriptionTag = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="[^"]*"\s*/?>`)

func audioNovelMetadata(page []byte) []byte {
	title := html.EscapeString("The Lantern Archive — Fantasy Fiction")
	description := html.EscapeString("Original tales of strange kingdoms, old magic, and impossible journeys.")
	page = audioNovelTitleTag.ReplaceAll(page, []byte("<title>"+title+"</title>"))
	return audioNovelDescriptionTag.ReplaceAll(page, []byte(`<meta name="description" content="`+description+`" />`))
}
