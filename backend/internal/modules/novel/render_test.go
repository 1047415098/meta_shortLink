package novel

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelMetadataReplacesGenericTags(t *testing.T) {
	page := []byte(`<html><head><title>Generic</title><meta name="description" content="generic" /></head></html>`)
	got := novelMetadata(page)
	if bytes.Contains(got, []byte("Generic")) || !bytes.Contains(got, []byte("Free Stories")) {
		t.Fatalf("metadata = %s", got)
	}
}

func TestNovelTikTokBootstrapAndCSPExposeOnlyPublicFields(t *testing.T) {
	data := Bootstrap{AdPlatform: "tiktok", TikTokEnabled: true, TikTokPixelCode: "C0ABC123", TikTokStartEventID: "novel_v1_start", TikTokQualifiedID: "novel_v1_qualified", StartupCoverPath: "/novel-uploads/0123456789abcdef0123456789abcdef.webp"}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	// The startup cover is a public, same-origin upload path and contains no visitor data.
	if !strings.Contains(string(raw), `"startup_cover_path":"/novel-uploads/0123456789abcdef0123456789abcdef.webp"`) {
		t.Fatalf("startup cover missing from public bootstrap: %s", raw)
	}
	for _, secret := range []string{"access_token", "test_event_code", "ttclid", "_ttp", "user_agent", "payload_cipher"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("bootstrap exposed %q: %s", secret, raw)
		}
	}
	csp := novelContentSecurityPolicy("nonce-value")
	for _, required := range []string{"'nonce-nonce-value'", "https://connect.facebook.net", "https://analytics.tiktok.com", "https://business-api.tiktok.com", "https://cdn.overseas-new-media.com", "frame-src 'self'"} {
		if !strings.Contains(csp, required) {
			t.Fatalf("CSP missing %q: %s", required, csp)
		}
	}
	if strings.Contains(csp, "*.tiktok.com") {
		t.Fatalf("wildcard TikTok host is not allowed: %s", csp)
	}
}

func TestStartupCoverAllowsOnlyLocalUploadsOrTheHistoricalCDN(t *testing.T) {
	// The legacy CDN remains narrowly scoped so older published novels can keep their existing cover.
	if !legacyStartupCoverPattern.MatchString("https://cdn.overseas-new-media.com/xiaoyao-writer/prod/content/cover/EJJjSSQy89w9JEe7tWjCoXwWPmbcL6KQ.jpg") {
		t.Fatal("historical cover CDN was unexpectedly rejected")
	}
	if legacyStartupCoverPattern.MatchString("https://example.com/xiaoyao-writer/prod/content/cover/cover.jpg") {
		t.Fatal("untrusted cover host was accepted")
	}
}

func TestStartupExcerptRemovesMarkupAndLimitsTheVisibleText(t *testing.T) {
	if got := startupExcerpt("<p>First &amp; second <strong>chapter</strong>.</p>"); got != "First & second chapter." {
		t.Fatalf("startup excerpt = %q", got)
	}
	long := strings.Repeat("文", startupPreviewMaxRunes+1)
	if got := startupExcerpt("<p>" + long + "</p>"); len([]rune(got)) != startupPreviewMaxRunes+1 || !strings.HasSuffix(got, "…") {
		t.Fatalf("startup excerpt limit = %q", got)
	}
}
