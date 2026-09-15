package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Manual and timer-driven consultations must be independently configurable for each Pixel.
func TestMetaConsultationRulesAreIndependent(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	account := metaConnection(t, a, "12345", "98765", true)
	admin := login(t, a)
	var pixelID int64
	if err := a.DB.QueryRow(ctx, "SELECT id FROM meta_pixels WHERE connection_id=$1 ORDER BY id LIMIT 1", account).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}

	// Deprecated fields are ignored, manual clicks are fixed to Contact, and auto is disabled separately.
	w := call(a, "PATCH", fmt.Sprintf("/api/v1/meta/pixels/%d", pixelID), `{"pageview_enabled":false,"manual_enabled":true,"auto_enabled":false,"manual_event_name":"WhatsAppConsultClick","token_expires_at":"2020-01-01T23:59:59Z"}`, admin)
	if w.Code != 200 {
		t.Fatalf("first rule update %d %s", w.Code, w.Body.String())
	}
	var first map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first["manual_event_name"] != "Contact" || first["auto_enabled"] != false {
		t.Fatalf("unexpected first rule response: %s", w.Body.String())
	}
	if _, exists := first["token_expires_at"]; exists {
		t.Fatalf("removed expiry field returned: %s", w.Body.String())
	}

	metaLanding(t, a, account)
	ticket := metaTicket(t, a)
	metaContact(a, ticket, "auto")
	metaContact(a, ticket, "manual")
	var names string
	if err := a.DB.QueryRow(ctx, "SELECT COALESCE(string_agg(event_name,',' ORDER BY event_name),'') FROM meta_events").Scan(&names); err != nil {
		t.Fatal(err)
	}
	if names != "Contact" {
		t.Fatalf("manual-only events = %q", names)
	}

	// A fresh visit freezes the opposite rule and emits only the automatic event.
	if _, err := a.DB.Exec(ctx, "DELETE FROM meta_events"); err != nil {
		t.Fatal(err)
	}
	w = call(a, "PATCH", fmt.Sprintf("/api/v1/meta/pixels/%d", pixelID), `{"manual_enabled":false,"auto_enabled":true}`, admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"auto_enabled":true`) {
		t.Fatalf("second rule update %d %s", w.Code, w.Body.String())
	}
	ticket = metaTicket(t, a)
	metaContact(a, ticket, "manual")
	metaContact(a, ticket, "auto")
	if err := a.DB.QueryRow(ctx, "SELECT COALESCE(string_agg(event_name,',' ORDER BY event_name),'') FROM meta_events").Scan(&names); err != nil {
		t.Fatal(err)
	}
	if names != "WhatsAppAutoRedirect" {
		t.Fatalf("auto-only events = %q", names)
	}
}
