package landing

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/platform/runtime"
)

func renderer(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	html := `<!doctype html><html><head><title>Research enquiries</title><meta name="description" content="generic" /><meta property="og:title" content="generic"/><meta property="og:description" content="generic"/><!--LANDING_BOOTSTRAP--><script type="module" src="/landing-assets/main-abcdefgh.js"></script></head><body><div id="app"></div></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(html), 0600); err != nil {
		t.Fatal(err)
	}
	return &Handler{Core: runtime.New(config.Config{LandingDir: dir, Secret: strings.Repeat("s", 32), CookieMode: "all"}, nil)}
}
func TestVueBootstrapEscapingAndMetadata(t *testing.T) {
	h := renderer(t)
	l := links.Link{Code: "hello", LandingTitle: `Ask </title><script>alert(1)</script>`, LandingDescription: `Text " </script><script>alert(2)</script>`, LandingDelay: 3}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/hello", nil)
	h.Render(c, l, strings.Repeat("a", 48), true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body := w.Body.String()
	if strings.Contains(body, "<script>alert(") {
		t.Fatal("unescaped script injection")
	}
	if !strings.Contains(body, "<title>Ask &lt;/title&gt;") {
		t.Fatal("per-link crawler title missing")
	}
	m := regexp.MustCompile(`<script id="landing-data" type="application/json" nonce="[^"]+">(.*?)</script>`).FindStringSubmatch(body)
	if len(m) != 2 {
		t.Fatal("bootstrap script missing")
	}
	var data Bootstrap
	if err := json.Unmarshal([]byte(m[1]), &data); err != nil {
		t.Fatal(err)
	}
	if data.Link.LandingDescription != l.LandingDescription || data.Ticket == "" || !data.CookieEnabled {
		t.Fatalf("bootstrap lost information: %+v", data)
	}
	if strings.Contains(body, "<!--LANDING_BOOTSTRAP-->") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "script-src 'self' 'nonce-") {
		t.Fatal("bootstrap marker or CSP invalid")
	}
	// Browser Pixel delivery requires only Meta's script and collection hosts;
	// no broader third-party origin should be admitted by the landing CSP.
	csp := w.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "https://connect.facebook.net") || !strings.Contains(csp, "https://www.facebook.com") {
		t.Fatalf("Meta Pixel hosts missing from CSP: %s", csp)
	}
}
func TestVueUnavailableAndHead(t *testing.T) {
	h := renderer(t)
	for _, method := range []string{"GET", "HEAD"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(method, "/missing", nil)
		h.Unavailable(c, 410, "Expired")
		c.Writer.WriteHeaderNow()
		if w.Code != 410 {
			t.Fatal(w.Code)
		}
		if method == "HEAD" && w.Body.Len() != 0 {
			t.Fatal("HEAD emitted body")
		}
		if method == "GET" && !strings.Contains(w.Body.String(), `"error":{"status":410,"message":"Expired"}`) {
			t.Fatal("error bootstrap missing")
		}
	}
}

func TestManualOnlyBrowserPixelDoesNotEmitPageViewFallback(t *testing.T) {
	h := renderer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/manual-only", nil)

	// Loading fbevents.js for manual consultation must not manufacture a
	// no-script PageView when the operator disabled PageView delivery.
	if err := h.render(c, 200, Bootstrap{MetaBrowserPixelID: "1066571352827370"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(w.Body.String(), "facebook.com/tr?id=") {
		t.Fatalf("manual-only Pixel emitted a PageView fallback: %s", w.Body.String())
	}
}
