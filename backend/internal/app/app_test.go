package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oschwald/geoip2-golang"
)

func TestTargetValidation(t *testing.T) {
	for _, s := range []string{"https://wa.me/13365661092", "https://wa.me/13365661092?text=hello"} {
		if !validTarget(s) {
			t.Errorf("valid target rejected: %s", s)
		}
	}
	for _, s := range []string{"http://wa.me/13365661092", "https://wa.me.evil.com/12345678", "https://evil.com", "https://u@wa.me/12345678", "https://wa.me/12345678?redirect=https://evil.com", "https://wa.me:444/13365661092"} {
		if validTarget(s) {
			t.Errorf("unsafe target accepted: %s", s)
		}
	}
}

func setup(t *testing.T) *App {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required for PostgreSQL integration tests")
	}
	p, e := pgxpool.New(context.Background(), dsn)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	_, e = p.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public")
	if e != nil {
		t.Fatal(e)
	}
	c := Config{PublicURL: "http://localhost:8080", AdminUser: "admin", AdminPassword: "test-password-long", Secret: "test-secret-must-be-at-least-32-characters", CookieMode: "all", SecureCookies: false, Timezone: "Asia/Shanghai", RetentionDays: 90}
	c.LandingDir = t.TempDir()
	if e := os.WriteFile(filepath.Join(c.LandingDir, "index.html"), []byte(`<!doctype html><html><head><!--LANDING_BOOTSTRAP--><script type="module" src="/landing-assets/app-test1234.js"></script></head><body><div id="app"></div></body></html>`), 0600); e != nil {
		t.Fatal(e)
	}
	// Integration requests render the independent audio novel shell from its own directory.
	c.AudioNovelDir = t.TempDir()
	if e := os.WriteFile(filepath.Join(c.AudioNovelDir, "index.html"), []byte(`<!doctype html><html><head><title>Audio Novel</title><meta name="description" content="test" /><!--AUDIO_NOVEL_BOOTSTRAP--><script type="module" src="/audio-novel-assets/app-test1234.js"></script></head><body><div id="app"></div></body></html>`), 0600); e != nil {
		t.Fatal(e)
	}
	a, e := New(c, p)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(a.Close)
	return a
}
func call(a *App, method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "192.0.2.3:43123"
	r.Header.Set("User-Agent", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 Version/17.0 Mobile/15E148 Safari/604.1")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-Requested-With", "XMLHttpRequest")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	return w
}
func login(t *testing.T, a *App) *http.Cookie {
	t.Helper()
	w := call(a, "POST", "/api/v1/auth/login", `{"username":"admin","password":"test-password-long"}`, nil)
	if w.Code != 200 {
		t.Fatalf("login %d %s", w.Code, w.Body.String())
	}
	return w.Result().Cookies()[0]
}

func testLinkMetaBinding(t *testing.T, a *App) (int64, int64) {
	t.Helper()
	// Successful link-creation fixtures follow the production requirement and
	// bind the concrete Pixel created for their Meta account.
	connectionID := metaConnection(t, a, "12345", "98765", true)
	var pixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1 ORDER BY id LIMIT 1", connectionID).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	return connectionID, pixelID
}
func TestRedirectAndDedup(t *testing.T) {
	a := setup(t)
	w := call(a, "GET", "/hello?ad_id=forged", "", nil)
	// Direct-mode visits keep the measured HTTP request, then hand off through
	// one minimal top.location script without rendering an intermediate page.
	if w.Code != 200 || w.Header().Get("Location") != "" || !strings.Contains(w.Body.String(), `top.location = "https://wa.me/13365661092"`) {
		t.Fatalf("direct page: %d %s", w.Code, w.Header().Get("Location"))
	}
	if strings.Contains(w.Body.String(), `direct-data`) || strings.Contains(w.Body.String(), `<body`) {
		t.Fatal("direct mode rendered the removed intermediate page")
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("cacheable redirect")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("visitor cookie missing")
	}
	call(a, "GET", "/hello", "", cookies[0])
	call(a, "HEAD", "/hello", "", cookies[0])
	var n, uv, heads int
	e := a.DB.QueryRow(context.Background(), "SELECT count(*),count(distinct visitor_id),count(*) filter(where method='HEAD') FROM click_events").Scan(&n, &uv, &heads)
	if e != nil || n != 3 || uv != 1 || heads != 1 {
		t.Fatalf("counts %d %d %d %v", n, uv, heads, e)
	}
	admin := login(t, a)
	w = call(a, "GET", "/api/v1/analytics?start=2020-01-01&end=2030-01-01", "", admin)
	if w.Code != 400 {
		t.Fatal("unbounded query allowed")
	}
	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	w = call(a, "GET", "/api/v1/analytics?start="+today+"&end="+today, "", admin)
	var out struct {
		Summary struct {
			Total         int
			Filtered      int
			Unique        int
			Head          int
			LandingViews  int `json:"landing_views"`
			AutoRedirects int `json:"auto_redirects"`
		}
	}
	if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	// Both normal direct requests are visible as user visits and automatic
	// handoffs; the HEAD probe remains outside those business metrics.
	if out.Summary.Total != 3 || out.Summary.Filtered != 2 || out.Summary.Unique != 1 || out.Summary.Head != 1 || out.Summary.LandingViews != 2 || out.Summary.AutoRedirects != 2 {
		t.Fatalf("summary %s", w.Body.String())
	}
}
func TestAuthAndCSRF(t *testing.T) {
	a := setup(t)
	if call(a, "GET", "/api/v1/links", "", nil).Code != 401 {
		t.Fatal("anonymous admin access")
	}
	c := login(t, a)
	r := httptest.NewRequest("POST", "/api/v1/links", strings.NewReader(`{}`))
	r.AddCookie(c)
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("missing CSRF guard")
	}
	if call(a, "POST", "/api/v1/links", `{"code":"bad","name":"bad","target_url":"https://evil.com","enabled":true}`, c).Code != 400 {
		t.Fatal("unsafe destination")
	}
}
func TestBotCookieDisabledAndFailOpen(t *testing.T) {
	a := setup(t)
	a.Config.CookieMode = "off"
	r := httptest.NewRequest("GET", "/hello", nil)
	r.Header.Set("User-Agent", "facebookexternalhit/1.1")
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	if w.Code != 200 || len(w.Result().Cookies()) != 0 {
		t.Fatal("bot page/cookie")
	}
	var class string
	a.DB.QueryRow(context.Background(), "SELECT classification FROM click_events LIMIT 1").Scan(&class)
	if class != "bot" {
		t.Fatal(class)
	}
	_, e := a.DB.Exec(context.Background(), "ALTER TABLE click_events RENAME TO unavailable_events")
	if e != nil {
		t.Fatal(e)
	}
	w = call(a, "GET", "/hello", "", nil)
	if w.Code != 200 || a.WriteFailures.Load() != 1 {
		t.Fatal("must fail open and count failure")
	}
}
func TestClassify(t *testing.T) {
	for _, tc := range []struct{ method, ua, purpose, want string }{{"GET", "facebookexternalhit/1.1", "", "bot"}, {"HEAD", "Mozilla/5.0", "", "head"}, {"GET", "Mozilla/5.0", "prefetch", "prefetch"}, {"GET", "", "", "unclassified"}, {"GET", "Mozilla/5.0 iPhone", "", "normal"}} {
		c, _ := classify(tc.method, tc.ua, tc.purpose, false)
		if c != tc.want {
			t.Errorf("%+v got %s", tc, c)
		}
	}
}

func TestCrossDayUniqueAndAttribution(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	connectionID, pixelID := testLinkMetaBinding(t, a)
	// This case deliberately selects fixed attribution to keep verifying that a
	// forged URL ad ID conflicts with the link's explicitly bound ad ID.
	w := call(a, "POST", "/api/v1/links", fmt.Sprintf(`{"code":"paid","name":"Paid","target_url":"https://wa.me/13365661092","enabled":true,"ad_id":"ad-A","channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"bound"}`, connectionID, pixelID), admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(a, "GET", "/paid?ad_id=ad-B&utm_source=other", "", nil)
	cookie := w.Result().Cookies()[0]
	call(a, "GET", "/paid", "", cookie)
	_, e := a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at=CASE WHEN id=(SELECT id FROM click_events ORDER BY occurred_at LIMIT 1) THEN '2026-09-01 12:00:00+00'::timestamptz ELSE '2026-09-02 12:00:00+00'::timestamptz END`)
	if e != nil {
		t.Fatal(e)
	}
	w = call(a, "GET", "/api/v1/analytics?start=2026-09-01&end=2026-09-02&tz=UTC", "", admin)
	var out struct {
		Summary Summary
		Trends  []Trend
		Ads     []Ad
	}
	if e = json.Unmarshal(w.Body.Bytes(), &out); e != nil {
		t.Fatal(e)
	}
	if out.Summary.Unique != 1 || len(out.Trends) != 2 || out.Trends[0].Unique != 1 || out.Trends[1].Unique != 1 {
		t.Fatalf("cross-day %s", w.Body.String())
	}
	var conflicts int
	var ad, source string
	e = a.DB.QueryRow(context.Background(), `SELECT count(*) FILTER(WHERE attribution_conflict),min(ad_id),min(source) FROM click_events`).Scan(&conflicts, &ad, &source)
	if e != nil || conflicts != 1 || ad != "ad-A" || source != "facebook" {
		t.Fatalf("attribution %d %s %s %v", conflicts, ad, source, e)
	}
}
func TestTamperedCookieAndNoCookieUV(t *testing.T) {
	a := setup(t)
	w := call(a, "GET", "/hello", "", &http.Cookie{Name: "wa_sid_dev", Value: strings.Repeat("a", 48) + ".forged"})
	if len(w.Result().Cookies()) != 1 {
		t.Fatal("must replace forged cookie")
	}
	a.Config.CookieMode = "off"
	w = call(a, "GET", "/hello", "", w.Result().Cookies()[0])
	if len(w.Result().Cookies()) != 0 {
		t.Fatal("disabled cookie set")
	}
	var n int
	a.DB.QueryRow(context.Background(), `SELECT count(*) FROM click_events WHERE visitor_id IS NULL`).Scan(&n)
	if n != 1 {
		t.Fatal("disabled mode must ignore existing visitor cookies")
	}
}
func TestRetentionRuns(t *testing.T) {
	a := setup(t)
	ctx, cancel := context.WithCancel(context.Background())
	go a.Retention(ctx)
	time.Sleep(100 * time.Millisecond)
	cancel()
	var n int
	e := a.DB.QueryRow(context.Background(), `SELECT count(*) FROM daily_totals`).Scan(&n)
	if e != nil {
		t.Fatal(e)
	}
}

func TestSpendRequiresAllDates(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	_, e := a.DB.Exec(context.Background(), `INSERT INTO ad_spend_daily(date,ad_id,amount,currency,time_zone) VALUES('2026-09-01','ad-A',10,'USD','UTC')`)
	if e != nil {
		t.Fatal(e)
	}
	w := call(a, "GET", "/api/v1/analytics?start=2026-09-01&end=2026-09-02&tz=UTC", "", admin)
	var out struct{ Ads []Ad }
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Ads) != 1 || out.Ads[0].Cost != nil {
		t.Fatalf("incomplete spend must not produce cost: %s", w.Body.String())
	}
}

func TestSpendImportAndExport(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	upload := func(csvText string) *httptest.ResponseRecorder {
		var b bytes.Buffer
		mw := multipart.NewWriter(&b)
		f, _ := mw.CreateFormFile("file", "spend.csv")
		f.Write([]byte(csvText))
		mw.Close()
		r := httptest.NewRequest("POST", "/api/v1/ad-spend/import", &b)
		r.Header.Set("Content-Type", mw.FormDataContentType())
		r.Header.Set("X-Requested-With", "XMLHttpRequest")
		r.AddCookie(admin)
		w := httptest.NewRecorder()
		a.Router.ServeHTTP(w, r)
		return w
	}
	w := upload("date,ad_id,amount,currency,time_zone\n2026-09-01,ad-A,10.25,USD,UTC\n")
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = upload("date,ad_id,amount,currency,time_zone\n2026-09-01,ad-A,-2,USD,UTC\n")
	if w.Code != 400 {
		t.Fatal("negative spend accepted")
	}
	var cost string
	a.DB.QueryRow(context.Background(), "SELECT amount::text FROM ad_spend_daily").Scan(&cost)
	if cost != "10.2500" {
		t.Fatal(cost)
	}
	call(a, "GET", "/hello", "", nil)
	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	w = call(a, "GET", "/api/v1/exports/clicks?start="+today+"&end="+today, "", admin)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "hello") {
		t.Fatalf("export %s", w.Body.String())
	}
	if csvSafe("=1+1") != "'=1+1" {
		t.Fatal("CSV formula not escaped")
	}
}

func TestDisabledLink(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	connectionID, pixelID := testLinkMetaBinding(t, a)
	// Availability now follows only the explicit enabled switch.
	w := call(a, "POST", "/api/v1/links", fmt.Sprintf(`{"code":"disabled","name":"Disabled","target_url":"https://wa.me/13365661092","enabled":false,"meta_connection_id":%d,"meta_pixel_id":%d}`, connectionID, pixelID), admin)
	if w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	w = call(a, "GET", "/disabled", "", nil)
	if w.Code != 410 {
		t.Fatal("disabled link must return 410")
	}
}

func TestGeoLookup(t *testing.T) {
	a := setup(t)
	path := os.Getenv("TEST_GEO_DB")
	if path == "" {
		t.Skip("TEST_GEO_DB needed")
	}
	var e error
	a.Geo, e = geoip2.Open(path)
	if e != nil {
		t.Fatal(e)
	}
	r := httptest.NewRequest("GET", "/hello", nil)
	r.RemoteAddr = "8.8.8.8:43210"
	r.Header.Set("User-Agent", "Mozilla/5.0")
	w := httptest.NewRecorder()
	a.Router.ServeHTTP(w, r)
	var country string
	e = a.DB.QueryRow(context.Background(), "SELECT country FROM click_events LIMIT 1").Scan(&country)
	if e != nil || country == "unknown" || country == "" {
		t.Fatalf("geo lookup %s %v", country, e)
	}
}
