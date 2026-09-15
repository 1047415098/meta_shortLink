package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestMetaPixelRoutingAndFrozenRules(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	account := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, account)
	cookie := login(t, a)
	w := call(a, "POST", "/api/v1/meta/pixels", fmt.Sprintf(`{"connection_id":%d,"name":"Second","pixel_id":"87654","capi_token":"second-private-token","enabled":true,"pageview_enabled":true,"manual_enabled":true,"manual_event_name":"Contact"}`, account), cookie)
	if w.Code != 200 {
		t.Fatalf("pixel create %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "second-private-token") {
		t.Fatal("credential exposed")
	}
	var p struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &p)
	w = call(a, "PATCH", "/api/v1/links/1", fmt.Sprintf(`{"meta_pixel_id":%d}`, p.ID), cookie)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	ticket := metaTicket(t, a)
	w = call(a, "PATCH", fmt.Sprintf("/api/v1/meta/pixels/%d", p.ID), `{"manual_event_name":"WhatsAppConsultClick","pageview_enabled":false}`, cookie)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for i := 0; i < 2; i++ {
		r := httptest.NewRequest("POST", "/hello/view", strings.NewReader(url.Values{"ticket": {ticket}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		out := httptest.NewRecorder()
		a.Router.ServeHTTP(out, r)
		if out.Code != 204 {
			t.Fatalf("view %d %s", out.Code, out.Body.String())
		}
	}
	for _, trigger := range []string{"auto", "manual", "manual"} {
		if w := metaContact(a, ticket, trigger); w.Code != 303 {
			t.Fatalf("contact %d", w.Code)
		}
	}
	var count int
	var names, pixels string
	if e := a.DB.QueryRow(ctx, `SELECT count(*),string_agg(event_name,',' ORDER BY event_name),string_agg(DISTINCT pixel_id,',') FROM meta_events`).Scan(&count, &names, &pixels); e != nil {
		t.Fatal(e)
	}
	// The visit keeps its original PageView/manual rule snapshot, while the timer
	// action is emitted under the dedicated automatic event name.
	if count != 3 || names != "Contact,PageView,WhatsAppAutoRedirect" || pixels != "87654" {
		t.Fatalf("routing/dedupe: %d %s %s", count, names, pixels)
	}
	other := metaConnection(t, a, "22222", "33333", false)
	w = call(a, "PATCH", "/api/v1/links/1", fmt.Sprintf(`{"meta_connection_id":%d,"meta_pixel_id":%d}`, other, p.ID), cookie)
	if w.Code != 400 {
		t.Fatalf("cross-account accepted %d", w.Code)
	}
}
func TestMetaAccountWithoutPixelAndSourceInspection(t *testing.T) {
	a := setup(t)
	cookie := login(t, a)
	w := call(a, "POST", "/api/v1/meta/connections", `{"name":"Account only","account_id":"44444","read_token":"private-read-token"}`, cookie)
	if w.Code != 200 {
		t.Fatalf("account-only %d %s", w.Code, w.Body.String())
	}
	for _, path := range []string{"/api/v1/meta/pixels", "/api/v1/meta/credentials", "/api/v1/meta/audit"} {
		if w := call(a, "GET", path, "", nil); w.Code != 401 {
			t.Fatalf("auth missing %s %d", path, w.Code)
		}
	}
	w = call(a, "POST", "/api/v1/meta/source/inspect", `{"url":"https://example.com/hello?campaign_id=123&adset_id=456&ad_id=789&site_source_name=fb&token=do-not-echo"}`, cookie)
	if w.Code != 200 || strings.Contains(w.Body.String(), "do-not-echo") {
		t.Fatalf("unsafe source inspection %s", w.Body.String())
	}
	var source struct {
		Valid  bool     `json:"valid"`
		Source string   `json:"source"`
		Issues []string `json:"issues"`
	}
	json.Unmarshal(w.Body.Bytes(), &source)
	if source.Valid || source.Source != "facebook" || len(source.Issues) == 0 {
		t.Fatalf("unsafe URL accepted: %s", w.Body.String())
	}
	w = call(a, "GET", "/api/v1/meta/credentials", "", cookie)
	// An account without Pixels has no active credential inventory; deprecated
	// account read tokens must never create a visible credential row.
	if w.Code != 200 || strings.Contains(w.Body.String(), "private-read-token") || strings.Contains(w.Body.String(), `"kind":"read"`) || !strings.Contains(w.Body.String(), `"items":[]`) {
		t.Fatal("credentials endpoint invalid")
	}
	w = call(a, "GET", "/api/v1/meta/audit", "", cookie)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "meta.connection.save") || !strings.Contains(w.Body.String(), "created_at") || strings.Contains(w.Body.String(), "private-read-token") {
		t.Fatalf("audit endpoint invalid: %d %s", w.Code, w.Body.String())
	}
}

func TestDynamicSourceParametersTakePriorityOverLinkChannel(t *testing.T) {
	a := setup(t)
	cookie := login(t, a)
	if w := call(a, "PATCH", "/api/v1/links/1", `{"attribution_mode":"dynamic","channel":"facebook"}`, cookie); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	for _, fixture := range []struct{ query, want string }{
		{"utm_source=instagram", "instagram"},
		{"utm_source=facebook&site_source_name=ig", "instagram"},
		{"utm_source=instagram&site_source_name=%7B%7Bsite_source_name%7D%7D", "instagram"},
		{"", "facebook"},
	} {
		call(a, "GET", "/hello?"+fixture.query, "", nil)
		var source string
		if e := a.DB.QueryRow(context.Background(), `SELECT source FROM click_events ORDER BY occurred_at DESC LIMIT 1`).Scan(&source); e != nil || source != fixture.want {
			t.Fatalf("source %q want %q: %v", source, fixture.want, e)
		}
	}
}
