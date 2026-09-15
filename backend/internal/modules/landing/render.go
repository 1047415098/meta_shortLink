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
	Link            *links.Link `json:"link"`
	Ticket          string      `json:"ticket"`
	CookieEnabled   bool        `json:"cookie_enabled"`
	MetaMeasurement bool        `json:"meta_measurement"`
	Error           *PageError  `json:"error,omitempty"`
}
type PageError struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

func (a *Handler) Render(c *gin.Context, l links.Link, eventID string, recorded bool) {
	ticket := ""
	if recorded {
		ticket = eventID + "." + a.Sign("contact:"+l.Code+":"+eventID)
	}
	measurement := false
	if recorded && l.MetaConnectionID != nil {
		_ = a.DB.QueryRow(c.Request.Context(), "SELECT meta_measurement FROM click_events WHERE id=$1", eventID).Scan(&measurement)
	}
	if err := a.render(c, 200, Bootstrap{Link: &l, Ticket: ticket, CookieEnabled: a.Config.CookieMode == "all", MetaMeasurement: measurement}); err != nil {
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
	c.Header("Content-Security-Policy", "default-src 'none'; script-src 'self' 'nonce-"+nonce+"'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'; form-action 'self' https://wa.me https://*.whatsapp.com whatsapp:; base-uri 'none'; frame-ancestors 'none'")
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
