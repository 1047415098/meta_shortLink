package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestVisitDetailRequiresLoginAndReturnsSafeSnapshot(t *testing.T) {
	a := setup(t)
	// A normal tracked request supplies device, attribution and anonymous visitor fields for the detail view.
	response := call(a, "GET", "/hello?utm_source=facebook&campaign_id=campaign-a&adset_id=group-a&ad_id=ad-a", "", nil)
	if response.Code != 200 {
		t.Fatalf("tracked visit failed: %d %s", response.Code, response.Body.String())
	}
	var visitID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM click_events ORDER BY occurred_at DESC LIMIT 1").Scan(&visitID); err != nil {
		t.Fatal(err)
	}
	if got := call(a, "GET", "/api/v1/clicks/"+visitID, "", nil).Code; got != 401 {
		t.Fatalf("anonymous detail status=%d, want 401", got)
	}

	admin := login(t, a)
	detailResponse := call(a, "GET", "/api/v1/clicks/"+visitID, "", admin)
	if detailResponse.Code != 200 {
		t.Fatalf("detail status=%d body=%s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	// Structured device values are returned while the complete User-Agent remains intentionally absent.
	if detail["id"] != visitID || detail["device"] != "mobile" || detail["device_model"] != "iPhone" ||
		detail["os"] != "iOS" || detail["os_version"] != "17.0" || detail["browser"] != "Safari" ||
		detail["browser_version"] != "17.0" || detail["campaign_id"] != "campaign-a" ||
		detail["adset_id"] != "group-a" || detail["ad_id"] != "ad-a" {
		t.Fatalf("incomplete detail: %s", detailResponse.Body.String())
	}
	// Raw request diagnostics are available to authenticated operations without exposing request credentials.
	if detail["client_ip"] != "192.0.2.3" ||
		!strings.Contains(detail["user_agent"].(string), "iPhone OS 17_0") ||
		detail["request_url"] != "http://localhost:8080/hello?utm_source=facebook&campaign_id=campaign-a&adset_id=group-a&ad_id=ad-a" {
		t.Fatalf("request diagnostics missing: %s", detailResponse.Body.String())
	}
	if _, ok := detail["client_hints"].(map[string]any); !ok {
		t.Fatalf("client hints missing: %s", detailResponse.Body.String())
	}
	if _, ok := detail["delivery_events"].([]any); !ok {
		t.Fatalf("delivery history missing: %s", detailResponse.Body.String())
	}
	// Encrypted TikTok context and platform payloads must never leave the authenticated API.
	body := detailResponse.Body.String()
	for _, forbidden := range []string{"tiktok_context_cipher", "payload_cipher", "lock_token", "Authorization", "Cookie"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("detail exposed %s: %s", forbidden, body)
		}
	}
	if got := call(a, "GET", "/api/v1/clicks/visit-does-not-exist", "", admin).Code; got != 404 {
		t.Fatalf("missing visit status=%d, want 404", got)
	}
}
