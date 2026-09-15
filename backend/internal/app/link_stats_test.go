package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLinkStatsRequireVerifiedAdClick catches regressions that mix Meta previews,
// organic post clicks, or incomplete attribution into the per-ad report.
func TestLinkStatsRequireVerifiedAdClick(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	_, err := a.DB.Exec(ctx, `INSERT INTO meta_connections(id,name,account_id,read_credential_status) VALUES(1,'Meta','101','invalid');
	INSERT INTO meta_ad_entities(connection_id,ad_id,ad_name,campaign_id,campaign_name,adset_id,adset_name) VALUES(1,'401','Verified Ad A','','','','');`)
	if err != nil {
		t.Fatal(err)
	}
	type visit struct {
		id, ad, visitor, params, class string
		manual, auto                   bool
	}
	// Literal fixtures mirror the production cases observed in Meta traffic; the
	// first row deliberately retains Meta's trailing spaces.
	visits := []visit{
		{"real-spaced", "401 ", "person-1", `{"fbclid":"click-1","ad_id":"401 ","utm_content":"999","ad_name":"Captured A "}`, "normal", true, true},
		{"real-repeat", "401", "person-1", `{"fbclid":"click-2","ad_id":"401"}`, "normal", false, true},
		{"real-other", "402", "person-2", `{"fbclid":"click-3","ad_id":"402","ad_name":"Ad B"}`, "normal", true, false},
		// A valid utm_content value resolves the advertising ID only when ad_id is absent.
		{"utm-fallback", "", "person-7", `{"fbclid":"click-7","utm_content":"405","ad_name":"Captured UTM"}`, "normal", true, false},
		{"meta-preview", "401", "NULL", `{"ad_id":"401","utm_source":"fb"}`, "bot", true, true},
		{"organic-post", "", "person-3", `{"fbclid":"organic-click"}`, "normal", true, true},
		{"ad-without-click", "403", "person-4", `{"ad_id":"403","utm_source":"fb"}`, "normal", true, true},
		{"placeholder", "{{ad.id}}", "person-5", `{"fbclid":"click-5","ad_id":"{{ad.id}}"}`, "normal", true, true},
		{"utm-placeholder", "", "person-8", `{"fbclid":"click-8","utm_content":"{{ad.id}}"}`, "normal", true, true},
		{"suspicious", "404", "person-6", `{"fbclid":"click-6","ad_id":"404"}`, "suspicious", true, true},
	}
	for _, item := range visits {
		_, err = a.DB.Exec(ctx, `INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,classification,reason,event_type,parameters,meta_connection_id,meta_account_id,occurred_at,whatsapp_clicked_at,auto_redirected_at)
		VALUES($1,1,NULLIF($2,'NULL'),'issued','GET','https://wa.me/12345678','mobile','','','','','','facebook','', '',$3,'',$4,'','landing',$5::jsonb,1,'101','2026-09-02T12:00:00Z',CASE WHEN $6 THEN '2026-09-02T12:01:00Z'::timestamptz ELSE NULL END,CASE WHEN $7 THEN '2026-09-02T12:02:00Z'::timestamptz ELSE NULL END)`, item.id, item.visitor, item.ad, item.class, item.params, item.manual, item.auto)
		if err != nil {
			t.Fatal(err)
		}
	}
	admin := login(t, a)
	w := call(a, "POST", "/api/v1/links/1/stats", `{"start":"2026-09-02","end":"2026-09-02","tz":"Asia/Shanghai"}`, admin)
	if w.Code != 200 {
		t.Fatalf("stats %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Total   int              `json:"total"`
		Summary map[string]int   `json:"summary"`
		Items   []map[string]any `json:"items"`
	}
	if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Total != 3 || body.Summary["visits"] != 4 || body.Summary["unique_visitors"] != 3 || body.Summary["manual_consultations"] != 3 || body.Summary["auto_redirects"] != 2 {
		t.Fatalf("non-ad traffic entered strict stats: %s", w.Body.String())
	}
	if _, exists := body.Summary["meta_visits"]; exists {
		t.Fatalf("removed Meta parameter metric still exposed: %s", w.Body.String())
	}
	utmFallbackFound := false
	for _, row := range body.Items {
		if row["source_kind"] != "ad_id" {
			t.Fatalf("non-ad source row exposed: %#v", row)
		}
		if row["source_value"] == "401" {
			if row["visits"] != float64(2) || row["unique_visitors"] != float64(1) || row["manual_consultations"] != float64(1) || row["auto_redirects"] != float64(2) || row["ad_name"] != "Verified Ad A" {
				t.Fatalf("whitespace-normalized ad row is wrong: %#v", row)
			}
		}
		if row["source_value"] == "405" {
			utmFallbackFound = true
			if row["visits"] != float64(1) || row["unique_visitors"] != float64(1) || row["manual_consultations"] != float64(1) || row["auto_redirects"] != float64(0) || row["ad_name"] != "Captured UTM" {
				t.Fatalf("utm_content fallback row is wrong: %#v", row)
			}
		}
	}
	if !utmFallbackFound {
		t.Fatal("missing utm_content fallback ad group")
	}
}

// Exercise the authenticated API against real PostgreSQL: cross-link leakage,
// summed UVs, or grouping a non-click source as an ad must fail this test.
func TestLinkStatsScopeAttributionAndCookieDedup(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	_, err := a.DB.Exec(ctx, `INSERT INTO meta_connections(id,name,account_id,read_credential_status) VALUES(1,'Unverified','101','invalid'),(2,'Other','102','unverified');
	INSERT INTO short_links(id,code,name,target_url) VALUES(2,'other','Other link','https://wa.me/12345678');
	INSERT INTO meta_ad_entities(connection_id,ad_id,ad_name,campaign_id,campaign_name,adset_id,adset_name) VALUES(1,'401','Verified Ad A','','','','');`)
	if err != nil {
		t.Fatal(err)
	}
	add := func(id, ad, visitor, params, at, class, kind string, link, connection int, account string, manual, auto bool) {
		t.Helper()
		_, err := a.DB.Exec(ctx, `INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,classification,reason,event_type,parameters,meta_connection_id,meta_account_id,occurred_at,whatsapp_clicked_at,auto_redirected_at)
		VALUES($1,$2,NULLIF($3,'NULL'),'issued','GET','https://wa.me/12345678','mobile','','','','','','facebook','','',$4,'',$5,'',$6,$7::jsonb,$8,$9,$10::timestamptz,CASE WHEN $11 THEN '2026-09-03T00:00:00Z'::timestamptz ELSE NULL END,CASE WHEN $12 THEN '2026-09-03T00:00:00Z'::timestamptz ELSE NULL END)`, id, link, visitor, ad, class, kind, params, connection, account, at, manual, auto)
		if err != nil {
			t.Fatal(err)
		}
	}
	add("a1", "401", "p1", `{"fbclid":"click-a1","ad_id":"401","utm_content":"different","ad_name":"Untrusted name"}`, "2026-09-01T16:00:00Z", "normal", "landing", 1, 1, "101", true, true)
	add("a2", "401", "p1", `{"fbclid":"click-a2","ad_id":"401"}`, "2026-09-02T15:59:59Z", "normal", "landing", 1, 1, "101", false, true)
	add("b", "402", "p1", `{"fbclid":"click-b","ad_id":"402","ad_name":"Ad B"}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 1, "101", true, false)
	add("utm", "", "p2", `{"utm_content":"401"}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 1, "101", false, true)
	add("missing1", "", "NULL", `{}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 1, "101", false, false)
	add("missing2", "", "", `{"ad_id":"{{ad.id}}"}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 1, "101", false, false)
	// A resolved click parameter without ad_id is intentionally absent from strict ad statistics.
	add("meta-only", "", "p4", `{"fbclid":"click-only"}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 1, "101", false, false)
	add("other-account", "401", "p3", `{"fbclid":"click-other-account","ad_id":"401"}`, "2026-09-02T12:00:00Z", "normal", "landing", 1, 2, "102", false, false)
	add("other-link", "401", "outsider", `{}`, "2026-09-02T12:00:00Z", "normal", "landing", 2, 1, "101", true, true)
	for _, class := range []string{"bot", "suspicious", "head", "prefetch", "unclassified"} {
		add(class, "401", class, `{}`, "2026-09-02T12:00:00Z", class, "landing", 1, 1, "101", true, true)
	}
	add("redirect", "401", "redirect", `{}`, "2026-09-02T12:00:00Z", "normal", "redirect", 1, 1, "101", true, true)
	add("before", "401", "before", `{}`, "2026-09-01T15:59:59Z", "normal", "landing", 1, 1, "101", true, true)
	add("after", "401", "after", `{}`, "2026-09-02T16:00:00Z", "normal", "landing", 1, 1, "101", true, true)
	path := "/api/v1/links/1/stats"
	filters := `{"start":"2026-09-02","end":"2026-09-02","tz":"Asia/Shanghai"}`
	if w := call(a, "POST", path, filters, nil); w.Code != 401 {
		t.Fatalf("unauthenticated: %d", w.Code)
	}
	admin := login(t, a)
	w := call(a, "POST", path, filters, admin)
	if w.Code != 200 {
		t.Fatalf("stats %d: %s", w.Code, w.Body.String())
	}
	var total int
	// JSON names are decoded through a generic map to keep the test independent
	// from implementation structs and verify the actual public field names.
	var body map[string]json.RawMessage
	if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(body["total"], &total)
	var sum map[string]int
	json.Unmarshal(body["summary"], &sum)
	if total != 3 || sum["visits"] != 4 || sum["unique_visitors"] != 2 || sum["manual_consultations"] != 2 || sum["auto_redirects"] != 2 || sum["no_cookie"] != 0 {
		t.Fatalf("wrong totals: %s", w.Body.String())
	}
	var items []map[string]any
	json.Unmarshal(body["items"], &items)
	found := false
	for _, row := range items {
		if row["source_kind"] == "ad_id" && row["source_value"] == "401" && row["connection_id"] == float64(1) {
			found = true
			if row["visits"] != float64(2) || row["unique_visitors"] != float64(1) || row["manual_consultations"] != float64(1) || row["auto_redirects"] != float64(2) || row["ad_name"] != "Verified Ad A" {
				t.Fatalf("wrong ad group: %#v", row)
			}
		}
		if row["source_kind"] != "ad_id" {
			t.Fatalf("strict report exposed a non-ad group: %#v", row)
		}
		if _, ok := row["spend"]; ok {
			t.Fatal("cost field must not be exposed")
		}
	}
	if !found {
		t.Fatal("missing ad group")
	}
	w = call(a, "POST", path+"?ad_id=ignored&start=invalid", strings.TrimSuffix(filters, "}")+`,"ad_id":"401"}`, admin)
	json.Unmarshal(w.Body.Bytes(), &body)
	json.Unmarshal(body["summary"], &sum)
	if w.Code != 200 || sum["visits"] != 3 {
		t.Fatalf("identifier filter: %d %s", w.Code, w.Body.String())
	}
	// Invalid JSON/types and filters must be rejected; URL parameters must not override the JSON body.
	for _, payload := range []string{``, `{`, `null`, `[]`, `{} {}`, `{"page":"1"}`, `{"page":0}`, `{"page":100001}`, `{"sort":"unsafe"}`, `{"order":"unsafe"}`, `{"tz":"Invalid/Zone"}`, `{"start":"invalid"}`, `{"end":"invalid"}`, `{"start":"2026-09-03","end":"2026-09-02"}`} {
		if w := call(a, "POST", path, payload, admin); w.Code != 400 {
			t.Fatalf("invalid body %s: %d", payload, w.Code)
		}
	}
	for url, status := range map[string]int{"/api/v1/links/bad/stats": 400, "/api/v1/links/999/stats": 404} {
		if w := call(a, "POST", url, filters, admin); w.Code != status {
			t.Fatalf("%s: %d", url, w.Code)
		}
	}
	if w := call(a, "GET", path, "", admin); w.Code == 200 {
		t.Fatal("legacy GET still serves statistics")
	}
	// POST retains the existing request-header check in addition to the login session.
	r := httptest.NewRequest("POST", path, strings.NewReader(filters))
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(admin)
	w = httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatalf("missing request header: %d", w.Code)
	}

}

// Facebook's common fbcli typo is accepted only as a canonical fbclid entry
// parameter, and URL-builder whitespace cannot become part of attribution IDs.
func TestTrackingCanonicalizesFbcliAlias(t *testing.T) {
	a := setup(t)
	if w := call(a, "GET", "/hello?fbcli=%20legacy-click%20&utm_campaign=%20Campaign-A%20&ad_id=%20401%20", "", nil); w.Code != 302 {
		t.Fatalf("entry request failed: %d", w.Code)
	}
	var fbclid, campaign, ad string
	if err := a.DB.QueryRow(context.Background(), "SELECT parameters->>'fbclid',parameters->>'utm_campaign',ad_id FROM click_events LIMIT 1").Scan(&fbclid, &campaign, &ad); err != nil || fbclid != "legacy-click" || campaign != "Campaign-A" || ad != "401" {
		t.Fatalf("canonical parameters missing: %q %q %q %v", fbclid, campaign, ad, err)
	}
}

// More than one page must retain the full summary and stable server-side sorting.
func TestLinkStatsPagination(t *testing.T) {
	a := setup(t)
	// Every generated row is a complete real-ad click so pagination cannot be
	// accidentally satisfied by incomplete legacy attribution.
	_, err := a.DB.Exec(context.Background(), `INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,classification,reason,event_type,parameters,occurred_at)
	SELECT 'v-'||n,1,'same-person','issued','GET','https://wa.me/12345678','','','','','','','facebook','','',n::text,'','normal','','landing',jsonb_build_object('fbclid','click-'||n,'ad_id',n::text),'2026-09-02T12:00:00Z' FROM generate_series(1,55) n`)
	if err != nil {
		t.Fatal(err)
	}
	admin := login(t, a)
	for page, want := range map[int]int{1: 50, 2: 5, 3: 0} {
		w := call(a, "POST", "/api/v1/links/1/stats", fmt.Sprintf(`{"start":"2026-09-02","end":"2026-09-02","page":%d}`, page), admin)
		var body struct {
			Total   int            `json:"total"`
			Items   []any          `json:"items"`
			Summary map[string]int `json:"summary"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if w.Code != 200 || body.Total != 55 || len(body.Items) != want || body.Summary["unique_visitors"] != 1 || body.Summary["visits"] != 55 {
			t.Fatalf("page %d: %d %s", page, w.Code, w.Body.String())
		}
	}
}
