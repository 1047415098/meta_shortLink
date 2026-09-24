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
	data := Bootstrap{AdPlatform: "tiktok", TikTokEnabled: true, TikTokPixelCode: "C0ABC123", TikTokStartEventID: "novel_v1_start", TikTokQualifiedID: "novel_v1_qualified"}
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"access_token", "test_event_code", "ttclid", "_ttp", "user_agent", "payload_cipher"} {
		if strings.Contains(string(raw), secret) {
			t.Fatalf("bootstrap exposed %q: %s", secret, raw)
		}
	}
	csp := novelContentSecurityPolicy("nonce-value")
	for _, required := range []string{"'nonce-nonce-value'", "https://connect.facebook.net", "https://analytics.tiktok.com", "https://business-api.tiktok.com"} {
		if !strings.Contains(csp, required) {
			t.Fatalf("CSP missing %q: %s", required, csp)
		}
	}
	if strings.Contains(csp, "*.tiktok.com") {
		t.Fatalf("wildcard TikTok host is not allowed: %s", csp)
	}
}
