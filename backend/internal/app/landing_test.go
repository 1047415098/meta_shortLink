package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDirectModeRecordsVisitAndReturnsMinimalTopLocationScript(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	// The configured target remains authoritative, including its prefilled text.
	if w := call(a, "PATCH", "/api/v1/links/1", `{"target_url":"https://wa.me/13365661092?text=hello+world"}`, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}

	page := call(a, "GET", "/hello?utm_source=facebook&ad_id=direct-ad", "", nil)
	if page.Code != 200 || page.Header().Get("Location") != "" {
		t.Fatalf("direct page response: %d %s", page.Code, page.Header().Get("Location"))
	}
	if !strings.Contains(page.Body.String(), `top.location = "https://wa.me/13365661092?text=hello+world"`) {
		t.Fatalf("configured target missing from direct script: %s", page.Body.String())
	}
	for _, removed := range []string{`<!doctype html>`, `<body`, `direct-data`, `data-manual-contact`, `sendBeacon`, `whatsapp://`} {
		if strings.Contains(page.Body.String(), removed) {
			t.Fatalf("removed direct-page behavior %q is still present", removed)
		}
	}
	var visits, manual, automatic, viewed int
	var eventID, eventType string
	if err := a.DB.QueryRow(context.Background(), `SELECT count(*) OVER(),count(whatsapp_clicked_at) OVER(),count(auto_redirected_at) OVER(),count(pageview_reported_at) OVER(),id,event_type FROM click_events LIMIT 1`).Scan(&visits, &manual, &automatic, &viewed, &eventID, &eventType); err != nil {
		t.Fatal(err)
	}
	// Direct handoffs never render the landing page, so they record an automatic
	// AddToCart action without manufacturing a PageView timestamp.
	if visits != 1 || manual != 0 || automatic != 1 || viewed != 0 || eventType != "redirect" {
		t.Fatalf("direct counters: visits=%d manual=%d auto=%d view=%d type=%s", visits, manual, automatic, viewed, eventType)
	}
	// Direct mode records its automatic action on the server and never exposes a
	// browser ticket that could manufacture a manual consultation.
	ticket := eventID + "." + a.Sign("contact:short_link:hello:"+eventID)
	r := httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {ticket}, "trigger": {"manual"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	result := httptest.NewRecorder()
	a.Router.ServeHTTP(result, r)
	if result.Code != 410 {
		t.Fatalf("direct consultation endpoint remained enabled: %d", result.Code)
	}
}

func TestLandingVisitAndContact(t *testing.T) {
	a := setup(t)
	a.Config.CookieMode = "off"
	admin := login(t, a)
	w := call(a, "PATCH", "/api/v1/links/1", `{"mode":"landing","landing_brand":"Example","landing_title":"Ask us","landing_description":"Details <script>alert(1)</script>"}`, admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(a, "GET", "/hello?utm_source=facebook&ad_id=ad-42", "", nil)
	if w.Code != 200 || w.Header().Get("Location") != "" {
		t.Fatalf("landing must render without redirect: %d", w.Code)
	}
	if strings.Contains(w.Body.String(), "<script>alert(1)</script>") || !strings.Contains(w.Body.String(), "Ask us") {
		t.Fatal("unsafe or missing content")
	}
	m := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatal("contact ticket missing")
	}
	post := func(ticket string) int {
		r := httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {ticket}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "null")
		out := httptest.NewRecorder()
		a.Router.ServeHTTP(out, r)
		if out.Code == 303 && out.Header().Get("Location") != "https://wa.me/13365661092" {
			t.Fatal("wrong contact target")
		}
		return out.Code
	}
	if post(m[1]+"tampered") != 400 {
		t.Fatal("forged ticket accepted")
	}
	if post(m[1]) != 303 || post(m[1]) != 303 {
		t.Fatal("contact / retry failed")
	}
	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	w = call(a, "GET", "/api/v1/analytics?start="+today+"&end="+today, "", admin)
	var data struct {
		Summary map[string]int
		Trends  []map[string]any
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Summary["total"] != 1 || data.Summary["landing_views"] != 1 || data.Summary["whatsapp_clicks"] != 1 {
		t.Fatalf("funnel counts/retry: %s", w.Body.String())
	}
	if len(data.Trends) != 1 || data.Trends[0]["landing_views"] != float64(1) || data.Trends[0]["whatsapp_clicks"] != float64(1) {
		t.Fatalf("two-stage trend mismatch: %s", w.Body.String())
	}
	call(a, "PATCH", "/api/v1/links/1", `{"enabled":false}`, admin)
	if post(m[1]) != 410 {
		t.Fatal("disabled link still accepts contact")
	}
}

// JavaScript consultations receive the validated WhatsApp target as JSON; native forms retain the redirect contract.
func TestLandingContactJSONResponse(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	if w := call(a, "PATCH", "/api/v1/links/1", `{"mode":"landing","landing_brand":"Example","landing_title":"Ask us","landing_description":"Contact our team"}`, admin); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	page := call(a, "GET", "/hello?ad_id=3303", "", nil)
	match := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(match) != 2 {
		t.Fatal("contact ticket missing")
	}
	r := httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {match[1]}, "trigger": {"manual"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Accept", "application/json")
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	var response struct {
		TargetURL string `json:"target_url"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil || w.Code != 200 || response.TargetURL != "https://wa.me/13365661092" || w.Header().Get("Location") != "" {
		t.Fatalf("JSON consultation response: %d %s %v", w.Code, w.Body.String(), err)
	}
}

func TestLandingBrowserPixelUsesBoundPixelAndSharedPageViewID(t *testing.T) {
	a := setup(t)
	connectionID := metaConnection(t, a, "12345", "1066571352827370", true)
	metaLanding(t, a, connectionID)

	page := call(a, "GET", "/hello?fbclid=browser-pixel-click&ad_id=3303", "", nil)
	ticketMatch := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(ticketMatch) != 2 {
		t.Fatalf("browser Pixel page has no visit ticket: %s", page.Body.String())
	}
	visitID := strings.SplitN(ticketMatch[1], ".", 2)[0]
	body := page.Body.String()
	// The browser contract exposes the public Meta Pixel ID, never the internal
	// database record ID, and shares the CAPI event ID for Meta deduplication.
	if !strings.Contains(body, `"meta_browser_pixel_id":"1066571352827370"`) {
		t.Fatalf("bound browser Pixel missing: %s", body)
	}
	if !strings.Contains(body, `"meta_pageview_event_id":"wa_`+visitID+`_view"`) {
		t.Fatalf("shared browser PageView event ID missing: %s", body)
	}
	if !strings.Contains(body, `"meta_manual_event_id":"wa_`+visitID+`_manual"`) {
		t.Fatalf("shared browser manual event ID missing: %s", body)
	}
	if !strings.Contains(body, `https://www.facebook.com/tr?id=1066571352827370&amp;ev=PageView&amp;noscript=1`) {
		t.Fatalf("dynamic Meta noscript fallback missing: %s", body)
	}

	// A disabled Pixel remains bound for administration but must not execute in
	// a new visitor document.
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_pixels SET enabled=false WHERE connection_id=$1", connectionID); err != nil {
		t.Fatal(err)
	}
	disabled := call(a, "GET", "/hello?fbclid=disabled-pixel&ad_id=3303", "", nil)
	if strings.Contains(disabled.Body.String(), `"meta_browser_pixel_id"`) || strings.Contains(disabled.Body.String(), `facebook.com/tr?id=`) {
		t.Fatalf("disabled Pixel leaked into landing page: %s", disabled.Body.String())
	}

	// A manual-only configuration must initialize the browser Pixel without
	// emitting a PageView or its no-script fallback.
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_pixels SET enabled=true,pageview_enabled=false,manual_enabled=true WHERE connection_id=$1", connectionID); err != nil {
		t.Fatal(err)
	}
	manualOnly := call(a, "GET", "/hello?fbclid=manual-only-click&ad_id=3303", "", nil)
	manualTicket := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(manualOnly.Body.String())
	if len(manualTicket) != 2 {
		t.Fatalf("manual-only browser Pixel page has no visit ticket: %s", manualOnly.Body.String())
	}
	manualVisitID := strings.SplitN(manualTicket[1], ".", 2)[0]
	manualBody := manualOnly.Body.String()
	if !strings.Contains(manualBody, `"meta_browser_pixel_id":"1066571352827370"`) || !strings.Contains(manualBody, `"meta_manual_event_id":"wa_`+manualVisitID+`_manual"`) {
		t.Fatalf("manual-only browser Pixel contract missing: %s", manualBody)
	}
	if strings.Contains(manualBody, `"meta_pageview_event_id"`) || strings.Contains(manualBody, `facebook.com/tr?id=`) {
		t.Fatalf("manual-only browser Pixel emitted PageView data: %s", manualBody)
	}
}

func TestLandingValidation(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	for _, body := range []string{`{"mode":"unknown"}`, `{"mode":"landing","landing_title":"","landing_description":"test"}`} {
		if call(a, "PATCH", "/api/v1/links/1", body, admin).Code != 400 {
			t.Fatalf("invalid landing accepted: %s", body)
		}
	}
}

func TestLandingBotsAndExpiredTicket(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	call(a, "PATCH", "/api/v1/links/1", `{"mode":"landing","landing_brand":"Example","landing_title":"Ask us","landing_description":"Contact our team"}`, admin)
	r := httptest.NewRequest("GET", "/hello", nil)
	r.Header.Set("User-Agent", "facebookexternalhit/1.1")
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "Ask us") {
		t.Fatal("bot must see the same page")
	}
	m := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatal("missing ticket")
	}
	_, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '2 hours'")
	if err != nil {
		t.Fatal(err)
	}
	r = httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {m[1]}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "null")
	w = httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("stale visit accepted")
	}
}

func TestTimedLandingDoesNotCountAsManualConsultation(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	w := call(a, "PATCH", "/api/v1/links/1", `{"mode":"landing","landing_brand":"Example","landing_title":"Product","landing_description":"Details","landing_delay":3}`, admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(a, "GET", "/hello", "", nil)
	if !strings.Contains(w.Body.String(), `"landing_delay":3`) {
		t.Fatal("countdown missing")
	}
	m := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(w.Body.String())
	if len(m) != 2 {
		t.Fatal("ticket missing")
	}
	post := func(trigger string) int {
		r := httptest.NewRequest("POST", "/hello/contact", strings.NewReader(url.Values{"ticket": {m[1]}, "trigger": {trigger}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		out := httptest.NewRecorder()
		a.Router.ServeHTTP(out, r)
		return out.Code
	}
	if post("auto") != 303 || post("auto") != 303 {
		t.Fatal("automatic redirect failed")
	}
	var manual, automatic int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(whatsapp_clicked_at),count(auto_redirected_at) FROM click_events").Scan(&manual, &automatic); err != nil || manual != 0 || automatic != 1 {
		t.Fatal("automatic counted as manual / duplicate", manual, automatic, err)
	}
	if post("manual") != 303 {
		t.Fatal("manual consultation failed")
	}

	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	result := call(a, "GET", "/api/v1/analytics?start="+today+"&end="+today, "", admin)
	var report struct {
		Summary map[string]int
		Trends  []map[string]any
	}
	if err := json.Unmarshal(result.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary["whatsapp_clicks"] != 1 || report.Summary["auto_redirects"] != 1 || len(report.Trends) != 1 || report.Trends[0]["whatsapp_clicks"] != float64(1) || report.Trends[0]["auto_redirects"] != float64(1) {
		t.Fatalf("separate manual/automatic reporting: %s", result.Body.String())
	}
	if post("unknown") != 400 {
		t.Fatal("invalid trigger accepted")
	}
	for _, body := range []string{`{"landing_delay":-1}`, `{"landing_delay":301}`, `{"landing_delay":1.5}`} {
		if call(a, "PATCH", "/api/v1/links/1", body, admin).Code != 400 {
			t.Fatal("invalid delay accepted")
		}
	}
	call(a, "PATCH", "/api/v1/links/1", `{"landing_delay":0}`, admin)
	if post("auto") != 400 {
		t.Fatal("disabled timer accepted")
	}
	w = call(a, "GET", "/hello", "", nil)
	if !strings.Contains(w.Body.String(), `"landing_delay":0`) {
		t.Fatal("disabled page has incorrect timer bootstrap")
	}
}
