package audionovel

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
	handler.Render(c, links.Link{Code: "hello", TargetURL: "https://wa.me/123", Name: "private"}, strings.Repeat("a", 48), false)
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
