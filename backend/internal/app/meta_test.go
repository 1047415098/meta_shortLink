package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestMetaWorkerLifecycleDeliversAndStops(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, id)
	metaContact(a, metaTicket(t, a), "manual")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"lifecycle"}`))
	}))
	defer srv.Close()
	a.Meta.Client.BaseURL = srv.URL
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	a.Meta.Start(ctx)
	a.Meta.Start(ctx) // Idempotent application startup must not create another worker set.
	defer a.Meta.Close()
	for {
		var status string
		if err := a.DB.QueryRow(ctx, "SELECT status FROM meta_events LIMIT 1").Scan(&status); err != nil {
			t.Fatal(err)
		}
		if status == "succeeded" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("background worker did not deliver the queued event")
		case <-time.After(20 * time.Millisecond):
		}
	}
	done := make(chan struct{})
	go func() { a.Meta.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not cancel idle background workers")
	}
}

// A missing auth gate or a credential returned to the browser must fail this test.
func TestMetaConnectionsProtectCredentials(t *testing.T) {
	a := setup(t)
	if w := call(a, "GET", "/api/v1/meta/connections", "", nil); w.Code != 401 {
		t.Fatalf("unauthenticated connections: %d", w.Code)
	}
	admin := login(t, a)
	// Deprecated Insights and account-level CAPI fields may still arrive from an
	// older browser, but the reduced account API must never persist or return them.
	w := call(a, "POST", "/api/v1/meta/connections", `{"name":"Company","account_id":"123456","pixel_id":"654321","api_version":"https://evil.invalid","report_time":"impression","backfill_days":28,"capi_enabled":true,"insights_enabled":true,"capi_token":"sample-capi-token-not-real","read_token":"sample-read-token-not-real"}`, admin)
	if w.Code != 200 {
		t.Fatalf("create %d %s", w.Code, w.Body.String())
	}
	var c struct {
		ID         int64  `json:"id"`
		APIVersion string `json:"api_version"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &c); e != nil {
		t.Fatal(e)
	}
	for _, deprecated := range []string{"pixel_id", "capi_enabled", "insights_enabled", "has_capi_token", "has_read_token", "read_token"} {
		if strings.Contains(w.Body.String(), `"`+deprecated+`"`) {
			t.Fatalf("deprecated account field %s returned: %s", deprecated, w.Body.String())
		}
	}
	// The API returns the server-owned version even when an older or forged
	// browser submits a different value.
	if c.ID == 0 || c.APIVersion != "v26.0" || strings.Contains(w.Body.String(), "sample-") {
		t.Fatalf("credential response: %s", w.Body.String())
	}
	var capiStored, readStored, legacyPixel string
	var capiEnabled, insightsEnabled bool
	if e := a.DB.QueryRow(context.Background(), "SELECT capi_token_cipher,read_token_cipher,pixel_id,capi_enabled,insights_enabled FROM meta_connections WHERE id=$1", c.ID).Scan(&capiStored, &readStored, &legacyPixel, &capiEnabled, &insightsEnabled); e != nil {
		t.Fatal(e)
	}
	if capiStored != "" || readStored != "" || legacyPixel != "" || capiEnabled || insightsEnabled {
		t.Fatalf("deprecated account settings persisted: capi=%t read=%t pixel=%q capi_enabled=%t insights_enabled=%t", capiStored != "", readStored != "", legacyPixel, capiEnabled, insightsEnabled)
	}
	w = call(a, "GET", "/api/v1/meta/connections", "", admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), "sample-") || strings.Contains(w.Body.String(), "insights_enabled") {
		t.Fatal("credential exposed in listing")
	}
}

func TestMetaInsightsRoutesAreRemoved(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	connection := metaConnection(t, a, "12345", "98765", true)
	// Every permission-dependent Insights entry point is removed instead of left
	// dormant behind a UI-only feature flag.
	requests := []struct {
		method string
		path   string
		body   string
	}{
		{"POST", fmt.Sprintf("/api/v1/meta/connections/%d/validate", connection), `{}`},
		{"POST", fmt.Sprintf("/api/v1/meta/connections/%d/sync", connection), `{"start":"2026-09-01","end":"2026-09-01"}`},
		{"GET", "/api/v1/meta/sync-jobs", ""},
		{"POST", "/api/v1/meta/sync-jobs/1/retry", `{}`},
		{"GET", fmt.Sprintf("/api/v1/meta/reports?connection_id=%d&start=2026-09-01&end=2026-09-01", connection), ""},
	}
	for _, request := range requests {
		if w := call(a, request.method, request.path, request.body, admin); w.Code != 404 {
			t.Fatalf("Insights route still active: %s %s returned %d %s", request.method, request.path, w.Code, w.Body.String())
		}
	}
}

func TestMetaDeliveryRetriesStablePayloadAndClearsMatchingData(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, id)
	ticket := metaTicket(t, a)
	if w := metaContact(a, ticket, "manual"); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	attempts := 0
	var first []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/v26.0/98765/events" || r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fake-capi-token" {
			t.Errorf("bad Graph request %s %s", r.Method, r.URL.Path)
		}
		if strings.Contains(r.URL.String(), "fake-capi-token") || strings.Contains(string(body), "fake-capi-token") {
			t.Error("token in URL or event payload")
		}
		if attempts == 1 {
			first = body
			w.WriteHeader(503)
			w.Write([]byte(`{"error":{"message":"Temporary","code":2,"is_transient":true}}`))
			return
		}
		if string(first) != string(body) {
			t.Error("retry changed event ID/time/matching payload")
		}
		w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"test-trace"}`))
	}))
	defer srv.Close()
	a.Meta.Client.BaseURL = srv.URL
	if worked, e := a.Meta.ProcessEvent(context.Background()); e != nil || !worked {
		t.Fatalf("first attempt %t %v", worked, e)
	}
	var status string
	var retry int
	a.DB.QueryRow(context.Background(), "SELECT status,attempts FROM meta_events LIMIT 1").Scan(&status, &retry)
	if status != "retry" || retry != 1 {
		t.Fatalf("not queued for retry: %s %d", status, retry)
	}
	a.DB.Exec(context.Background(), "UPDATE meta_events SET next_attempt_at=now()-interval '1 minute'")
	if worked, e := a.Meta.ProcessEvent(context.Background()); e != nil || !worked {
		t.Fatalf("second attempt %t %v", worked, e)
	}
	var cipher, eventID string
	a.DB.QueryRow(context.Background(), "SELECT id,status,payload_cipher,attempts FROM meta_events LIMIT 1").Scan(&eventID, &status, &cipher, &retry)
	if status != "succeeded" || cipher != "" || retry != 2 {
		t.Fatalf("delivery result %s %d", status, retry)
	}
	w := call(a, "POST", "/api/v1/meta/events/"+eventID+"/retry", `{}`, login(t, a))
	if w.Code != 409 {
		t.Fatalf("successful event retry %d", w.Code)
	}
	w = call(a, "GET", "/api/v1/meta/events", "", login(t, a))
	if w.Code != 200 || strings.Contains(w.Body.String(), "192.0.2.7") || strings.Contains(w.Body.String(), "fake-capi-token") || strings.Contains(w.Body.String(), "payload_cipher") {
		t.Fatalf("unsafe event listing: %d %s", w.Code, w.Body.String())
	}
}
func TestMetaEventExpiredLeaseCanRecover(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, id)
	ticket := metaTicket(t, a)
	metaContact(a, ticket, "manual")
	a.DB.Exec(context.Background(), "UPDATE meta_events SET status='processing',locked_until=now()-interval '1 minute',lock_token='crashed-worker'")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"recovered"}`))
	}))
	defer srv.Close()
	a.Meta.Client.BaseURL = srv.URL
	if worked, e := a.Meta.ProcessEvent(context.Background()); e != nil || !worked {
		t.Fatalf("lease recovery %t %v", worked, e)
	}
	var status string
	a.DB.QueryRow(context.Background(), "SELECT status FROM meta_events LIMIT 1").Scan(&status)
	if status != "succeeded" {
		t.Fatal(status)
	}
}

func metaConnection(t *testing.T, a *App, account, pixel string, _ bool) int64 {
	t.Helper()
	admin := login(t, a)
	w := call(a, "POST", "/api/v1/meta/connections", fmt.Sprintf(`{"name":"Meta test","account_id":"%s","api_version":"v26.0"}`, account), admin)
	if w.Code != 200 {
		t.Fatalf("connection %d %s", w.Code, w.Body.String())
	}
	var c struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &c)
	// Account-level CAPI was removed; every fixture now creates the explicit
	// Pixel credential used by both live and test events.
	w = call(a, "POST", "/api/v1/meta/pixels", fmt.Sprintf(`{"connection_id":%d,"name":"Meta test Pixel","pixel_id":"%s","capi_token":"fake-capi-token","enabled":true,"manual_enabled":true,"manual_event_name":"WhatsAppConsultClick"}`, c.ID, pixel), admin)
	if w.Code != 200 {
		t.Fatalf("pixel %d %s", w.Code, w.Body.String())
	}
	// Omitting the PageView switch must still produce the complete default
	// funnel so API-created Pixels behave like the administration form.
	var created struct {
		PageviewEnabled bool `json:"pageview_enabled"`
	}
	if e := json.Unmarshal(w.Body.Bytes(), &created); e != nil || !created.PageviewEnabled {
		t.Fatalf("PageView default missing: %v %s", e, w.Body.String())
	}
	return c.ID
}
func metaLanding(t *testing.T, a *App, connection int64) {
	t.Helper()
	var pixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1 ORDER BY id LIMIT 1", connection).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	// Bind both foreign keys so every visit freezes the concrete Pixel target.
	w := call(a, "PATCH", "/api/v1/links/1", fmt.Sprintf(`{"mode":"landing","landing_brand":"Research","landing_title":"Products","landing_description":"Product enquiries","landing_delay":3,"meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","legacy_campaign_param":true}`, connection, pixelID), login(t, a))
	if w.Code != 200 {
		t.Fatalf("link %d %s", w.Code, w.Body.String())
	}
}
func metaTicket(t *testing.T, a *App) string {
	t.Helper()
	w := call(a, "GET", "/hello?token=never-save-this&utm_content=1001&adset_id=2001&ad_id=3001&site_source_name=ig&placement=Instagram_Feed&ad_name=Product&fbclid=real-click-identifier", "", nil)
	m := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatalf("ticket %s", w.Body.String())
	}
	return m[1]
}
func metaContact(a *App, ticket, trigger string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {ticket}, "trigger": {trigger}}.Encode()))
	r.RemoteAddr = "192.0.2.7:43123"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("User-Agent", "Mozilla/5.0 iPhone")
	r.AddCookie(&http.Cookie{Name: "_fbp", Value: "fb.1.1789012345000.12345678"})
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	return w
}
func TestMetaRealAdClickQueuesDistinctConsultationEvents(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, id)
	ticket := metaTicket(t, a)
	for _, trigger := range []string{"auto", "auto", "manual", "manual"} {
		if w := metaContact(a, ticket, trigger); w.Code != 303 {
			t.Fatalf("contact %d %s", w.Code, w.Body.String())
		}
	}
	var n int
	var campaign, params string
	var conn int64
	// A qualifying ad visit may produce one manual event and one automatic event,
	// while retries of either browser action remain idempotent.
	if e := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_events WHERE is_test=false").Scan(&n); e != nil || n != 2 {
		t.Fatalf("wanted two consultation events got %d %v", n, e)
	}
	if e := a.DB.QueryRow(context.Background(), "SELECT campaign_id,parameters::text,meta_connection_id FROM click_events LIMIT 1").Scan(&campaign, &params, &conn); e != nil {
		t.Fatal(e)
	}
	if campaign != "1001" || conn != id || !strings.Contains(params, "Instagram_Feed") || strings.Contains(params, "never-save-this") {
		t.Fatalf("incorrect attribution %s %s %d", campaign, params, conn)
	}
	var manual, automatic, encrypted int
	if e := a.DB.QueryRow(context.Background(), `SELECT
		-- Deliberate clicks are reported with Meta's standard Contact event.
		count(*) FILTER(WHERE id LIKE '%_manual' AND event_name='Contact'),
		count(*) FILTER(WHERE id LIKE '%_auto' AND event_name='WhatsAppAutoRedirect'),
		count(*) FILTER(WHERE payload_cipher<>'' AND payload_cipher NOT LIKE '%192.0.2.7%')
		FROM meta_events WHERE is_test=false`).Scan(&manual, &automatic, &encrypted); e != nil {
		t.Fatal(e)
	}
	if manual != 1 || automatic != 1 || encrypted != 2 {
		t.Fatalf("wrong event split manual=%d automatic=%d encrypted=%d", manual, automatic, encrypted)
	}
}

func TestMetaConsultationsSkipTrafficWithoutRealAdAttribution(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", true)
	metaLanding(t, a, id)
	// Each visit misses one part of the real-ad-click contract used by the ad report.
	queries := []string{
		"?ad_id=3001",
		"?fbclid=real-click-identifier",
		"?fbclid=real-click-identifier&ad_id={{ad.id}}",
	}
	for _, query := range queries {
		w := call(a, "GET", "/hello"+query, "", nil)
		match := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(w.Body.String())
		if len(match) != 2 {
			t.Fatalf("ticket for %s: %s", query, w.Body.String())
		}
		for _, trigger := range []string{"manual", "auto"} {
			if w = metaContact(a, match[1], trigger); w.Code != 303 {
				t.Fatalf("%s %s: %d %s", query, trigger, w.Code, w.Body.String())
			}
		}
	}
	var pending, skipped int
	if e := a.DB.QueryRow(context.Background(), `SELECT
		count(*) FILTER(WHERE status='pending'),count(*) FILTER(WHERE status='skipped')
		FROM meta_events WHERE is_test=false`).Scan(&pending, &skipped); e != nil {
		t.Fatal(e)
	}
	if pending != 0 || skipped != 6 {
		t.Fatalf("incomplete attribution entered delivery queue: pending=%d skipped=%d", pending, skipped)
	}
}
func TestMetaPixelControlsDeliveryAndKeepsVisitBinding(t *testing.T) {
	a := setup(t)
	// The account-level CAPI switch is a legacy setting. An enabled Pixel with
	// its own token remains the complete delivery target.
	first := metaConnection(t, a, "12345", "98765", false)
	second := metaConnection(t, a, "23456", "87654", true)
	metaLanding(t, a, first)
	ticket := metaTicket(t, a)
	metaLanding(t, a, second)
	if w := metaContact(a, ticket, "manual"); w.Code != 303 {
		t.Fatal(w.Body.String())
	}
	var n int
	var queuedConnection int64
	if e := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_events").Scan(&n); e != nil {
		t.Fatal(e)
	}
	if n != 1 {
		t.Fatalf("enabled Pixel did not queue exactly one event: count=%d", n)
	}
	if e := a.DB.QueryRow(context.Background(), "SELECT connection_id FROM meta_events LIMIT 1").Scan(&queuedConnection); e != nil || queuedConnection != first {
		t.Fatalf("Pixel event was not frozen to the original account: connection=%d error=%v", queuedConnection, e)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v26.0/98765/events" {
			t.Errorf("event routed to %s", r.URL.Path)
		}
		w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"pixel-owned"}`))
	}))
	defer srv.Close()
	a.Meta.Client.BaseURL = srv.URL
	if worked, e := a.Meta.ProcessEvent(context.Background()); e != nil || !worked {
		t.Fatalf("Pixel-owned delivery %t %v", worked, e)
	}
	var status string
	if e := a.DB.QueryRow(context.Background(), "SELECT status FROM meta_events LIMIT 1").Scan(&status); e != nil || status != "succeeded" {
		t.Fatalf("Pixel-owned event status=%s error=%v", status, e)
	}
}

func TestMetaConfigRejectsUnsafeInput(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	for _, body := range []string{
		`{"name":"Bad","account_id":"../123"}`,
		`{"name":"","account_id":"123456","api_version":"v26.0"}`,
	} {
		w := call(a, "POST", "/api/v1/meta/connections", body, admin)
		if w.Code != 400 {
			t.Fatalf("bad config accepted: %d %s", w.Code, w.Body.String())
		}
	}
}

func TestMetaAccountIgnoresDeprecatedCredentials(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", false)
	admin := login(t, a)
	// Old browser fields are ignored because account rows no longer own secrets.
	w := call(a, "PATCH", fmt.Sprintf("/api/v1/meta/connections/%d", id), `{"insights_enabled":true,"validated_at":"2026-09-10T00:00:00Z","currency":"USD","timezone":"UTC"}`, admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), "insights_enabled") {
		t.Fatalf("deprecated account fields leaked: %d %s", w.Code, w.Body.String())
	}
	w = call(a, "PATCH", fmt.Sprintf("/api/v1/meta/connections/%d", id), `{"name":"Renamed","capi_token":"","read_token":""}`, admin)
	if w.Code != 200 || strings.Contains(w.Body.String(), "read_token") {
		t.Fatalf("deprecated credentials returned: %s", w.Body.String())
	}
	w = call(a, "PATCH", "/api/v1/links/1", `{"meta_connection_id":999999}`, admin)
	if w.Code != 400 {
		t.Fatalf("missing connection needs client error, got %d", w.Code)
	}
}

func TestMetaTestEventIsSeparateAndCanRunWithLiveDisabled(t *testing.T) {
	a := setup(t)
	id := metaConnection(t, a, "12345", "98765", false)
	admin := login(t, a)
	var pixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1", id).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	// Tests target one explicit Pixel, matching the administration workflow.
	w := call(a, "POST", fmt.Sprintf("/api/v1/meta/pixels/%d/test-event", pixelID), `{"test_event_code":"TEST95428","event_name":"WhatsAppConsultClick"}`, admin)
	if w.Code != 200 {
		t.Fatalf("test enqueue %d %s", w.Code, w.Body.String())
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			TestCode string           `json:"test_event_code"`
			Data     []map[string]any `json:"data"`
		}
		if e := json.NewDecoder(r.Body).Decode(&data); e != nil {
			t.Error(e)
		}
		if data.TestCode != "TEST95428" || len(data.Data) != 1 {
			t.Error("test code not on top-level payload")
		}
		w.Write([]byte(`{"events_received":1,"messages":[],"fbtrace_id":"local-test-only"}`))
	}))
	defer srv.Close()
	a.Meta.Client.BaseURL = srv.URL
	if worked, e := a.Meta.ProcessEvent(context.Background()); e != nil || !worked {
		t.Fatalf("test delivery %t %v", worked, e)
	}
	var n int
	a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events").Scan(&n)
	if n != 0 {
		t.Fatal("test changed visitor counters")
	}
}
