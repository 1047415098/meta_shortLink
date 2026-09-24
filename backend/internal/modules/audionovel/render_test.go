package audionovel

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/platform/runtime"
)

func testRenderer(t *testing.T) *Handler {
	t.Helper()
	dir := t.TempDir()
	page := `<!doctype html><html><head><title>Audio Novel</title><meta name="description" content="generic" /></head><body><!--AUDIO_NOVEL_BOOTSTRAP--></body></html>`
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0600); err != nil {
		t.Fatal(err)
	}
	return &Handler{Core: runtime.New(config.Config{AudioNovelDir: dir, Secret: strings.Repeat("n", 32), CookieMode: "all"}, nil)}
}

func TestRenderInjectsPublicAudioNovelBootstrap(t *testing.T) {
	handler := testRenderer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/audio-novel/hello", nil)
	handler.Render(c, links.Link{Code: "hello", TargetURL: "https://wa.me/123", Name: "private"}, strings.Repeat("a", 48), false, false)
	match := regexp.MustCompile(`<script id="audio-novel-data"[^>]*>(.*?)</script>`).FindStringSubmatch(w.Body.String())
	if len(match) != 2 {
		t.Fatal("audio novel bootstrap script missing")
	}
	var data Bootstrap
	if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
		t.Fatal(err)
	}
	if data.Surface != "audio_novel" || data.Link.Code != "hello" || strings.Contains(match[1], "private") {
		t.Fatalf("unexpected public bootstrap: %s", match[1])
	}
}

func TestUnavailableHeadHasNoBody(t *testing.T) {
	handler := testRenderer(t)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("HEAD", "/audio-novel/missing", nil)
	handler.Unavailable(c, 404, "Missing")
	c.Writer.WriteHeaderNow()
	if w.Code != 404 || w.Body.Len() != 0 {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}

func TestPlaybackTicketBindsVisitLinkContentAndExpiry(t *testing.T) {
	handler := testRenderer(t)
	now := time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)
	visitID := strings.Repeat("a", 48)
	ticket := issuePlaybackTicket(handler.Core, visitID, 12, 34, now)
	claims, err := parsePlaybackTicket(handler.Core, ticket, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if claims.VisitID != visitID || claims.LinkID != 12 || claims.AudioNovelID != 34 || !claims.IssuedAt.Equal(now) || !claims.ExpiresAt.Equal(now.Add(playbackTicketLifetime)) {
		t.Fatalf("unexpected playback claims: %+v", claims)
	}
	// The admin accepts recordings up to 24 hours, so a legitimate long-running
	// player must keep its original signed ticket beyond the old two-hour limit.
	if _, err = parsePlaybackTicket(handler.Core, ticket, now.Add(12*time.Hour)); err != nil {
		t.Fatalf("long audio ticket expired while playback was still valid: %v", err)
	}
	if _, err = parsePlaybackTicket(handler.Core, strings.Replace(ticket, ".12.", ".13.", 1), now.Add(time.Minute)); err == nil {
		t.Fatal("tampered link binding was accepted")
	}
	if _, err = parsePlaybackTicket(handler.Core, ticket, now.Add(playbackTicketLifetime)); err == nil {
		t.Fatal("expired playback ticket was accepted")
	}
}

func TestAudioNovelCSPAllowsOnlyOfficialPixelHosts(t *testing.T) {
	policy := audioNovelContentSecurityPolicy("nonce-value")
	for _, expected := range []string{"https://connect.facebook.net", "https://analytics.tiktok.com", "https://business-api.tiktok.com", "media-src 'self'"} {
		if !strings.Contains(policy, expected) {
			t.Fatalf("CSP missing %q: %s", expected, policy)
		}
	}
	if strings.Contains(policy, "https://*.tiktok.com") {
		t.Fatalf("CSP widened TikTok access with a wildcard: %s", policy)
	}
}
