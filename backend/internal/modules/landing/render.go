package landing

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

// Bootstrap is the only data contract between the visitor HTTP request and Vue.
// encoding/json escapes HTML-sensitive characters, including closing script tags.
type Bootstrap struct {
	Link                 *links.Link `json:"link"`
	Ticket               string      `json:"ticket"`
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

func (a *Handler) Render(c *gin.Context, l links.Link, eventID string, recorded bool) {
	ticket := ""
	if recorded {
		ticket = eventID + "." + a.Sign("contact:short_link:"+l.Code+":"+eventID)
	}
	measurement := false
	browserPixelID := ""
	pageViewEnabled, manualEnabled, timeSpentEnabled := false, false, false
	if recorded && l.MetaConnectionID != nil {
		// Resolve the public Pixel number from the frozen visit target. Internal
		// record IDs never reach the browser. Manual-only targets still need the
		// client library even when PageView delivery is disabled.
		_ = a.DB.QueryRow(c.Request.Context(), `SELECT e.meta_measurement,
			COALESCE(CASE WHEN e.meta_measurement AND p.enabled AND (e.meta_pageview_enabled OR e.meta_manual_enabled OR e.time_spent_threshold>0) THEN p.pixel_id ELSE '' END,''),
			e.meta_pageview_enabled,e.meta_manual_enabled,e.time_spent_threshold>0
			FROM click_events e LEFT JOIN meta_pixels p ON p.id=e.meta_pixel_id WHERE e.id=$1`, eventID).Scan(&measurement, &browserPixelID, &pageViewEnabled, &manualEnabled, &timeSpentEnabled)
	}
	pageViewEventID, manualEventID := "", ""
	if browserPixelID != "" && pageViewEnabled {
		// This exactly matches the server event identifier built by Meta CAPI so
		// Events Manager can deduplicate browser and server PageView delivery.
		pageViewEventID = "wa_" + eventID + "_view"
	}
	if browserPixelID != "" && manualEnabled {
		// Manual AddToCart uses the same event identifier in browser Pixel and CAPI.
		manualEventID = "wa_" + eventID + "_manual"
	}
	timeSpentEventID := ""
	if browserPixelID != "" && timeSpentEnabled {
		// Browser Pixel and CAPI share this ID for custom-event deduplication.
		timeSpentEventID = "wa_" + eventID + "_time_spent"
	}
	if err := a.render(c, 200, Bootstrap{Link: &l, Ticket: ticket, CookieEnabled: a.Config.CookieMode == "all", MetaMeasurement: measurement, MetaBrowserPixelID: browserPixelID, MetaPageViewEventID: pageViewEventID, MetaManualEventID: manualEventID, MetaTimeSpentEventID: timeSpentEventID}); err != nil {
		landingError(c, err)
	}
}
func (a *Handler) Unavailable(c *gin.Context, status int, message string) {
	if err := a.render(c, status, Bootstrap{CookieEnabled: a.Config.CookieMode == "all", Error: &PageError{Status: status, Message: message}}); err != nil {
		c.String(status, message)
	}
}
func (a *Handler) render(c *gin.Context, status int, data Bootstrap) error {
	page, err := os.ReadFile(filepath.Join(a.Config.LandingDir, "index.html"))
	if err != nil {
		return err
	}
	marker := []byte("<!--LANDING_BOOTSTRAP-->")
	if bytes.Count(page, marker) != 1 {
		return errors.New("landing index must contain exactly one LANDING_BOOTSTRAP placeholder")
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	nonce := runtime.Token()
	script := append([]byte(`<script id="landing-data" type="application/json" nonce="`+nonce+`">`), payload...)
	script = append(script, []byte("</script>")...)
	page = bytes.Replace(page, marker, script, 1)
	if data.Link != nil {
		page = pageMetadata(page, data.Link.LandingTitle, data.Link.LandingDescription)
	}
	if data.MetaBrowserPixelID != "" && data.MetaPageViewEventID != "" {
		// The fallback covers browsers with JavaScript disabled. Vue cannot mount
		// in that case, so no CAPI PageView is generated and no duplicate exists.
		pixel := html.EscapeString(data.MetaBrowserPixelID)
		fallback := []byte(`<noscript><img height="1" width="1" style="display:none" alt="" src="https://www.facebook.com/tr?id=` + pixel + `&amp;ev=PageView&amp;noscript=1" /></noscript>`)
		page = bytes.Replace(page, []byte("</body>"), append(fallback, []byte("</body>")...), 1)
	}
	// Permit the exact Meta and AnyTrack loader/collection hosts while retaining
	// the landing page's deny-by-default policy.
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' 'nonce-"+nonce+"' https://connect.facebook.net https://assets.anytrack.io; style-src 'self' 'unsafe-inline'; img-src 'self' data: https://www.facebook.com; font-src 'self'; connect-src 'self' https://www.facebook.com https://t1.anytrack.io; form-action 'self' https://wa.me https://*.whatsapp.com whatsapp:; base-uri 'none'; frame-ancestors 'none'")
	c.Header("Referrer-Policy", "same-origin")
	c.Header("Content-Type", "text/html; charset=utf-8")
	if c.Request.Method == "HEAD" {
		c.Status(status)
		return nil
	}
	c.Data(status, "text/html; charset=utf-8", page)
	return nil
}

var titleTag = regexp.MustCompile(`(?is)<title>.*?</title>`)
var descriptionTag = regexp.MustCompile(`(?is)<meta\s+name="description"\s+content="[^"]*"\s*/?>`)
var ogTitleTag = regexp.MustCompile(`(?is)<meta\s+property="og:title"\s+content="[^"]*"\s*/?>`)
var ogDescriptionTag = regexp.MustCompile(`(?is)<meta\s+property="og:description"\s+content="[^"]*"\s*/?>`)

func pageMetadata(page []byte, title, description string) []byte {
	replacements := []struct {
		pattern *regexp.Regexp
		value   string
	}{
		{titleTag, "<title>" + html.EscapeString(title) + "</title>"},
		{descriptionTag, `<meta name="description" content="` + html.EscapeString(description) + `" />`},
		{ogTitleTag, `<meta property="og:title" content="` + html.EscapeString(title) + `" />`},
		{ogDescriptionTag, `<meta property="og:description" content="` + html.EscapeString(description) + `" />`},
	}
	for _, replacement := range replacements {
		value := []byte(replacement.value)
		page = replacement.pattern.ReplaceAllFunc(page, func([]byte) []byte { return value })
	}
	return page
}
