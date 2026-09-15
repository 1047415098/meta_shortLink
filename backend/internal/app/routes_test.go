package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestAdminRouteReload(t *testing.T) {
	a := setup(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<!doctype html><div id=app>admin fixture</div>"), 0600); err != nil {
		t.Fatal(err)
	}
	a.Config.FrontendDir = dir
	for _, path := range []string{"/admin/overview", "/admin/links", "/admin/login"} {
		w := call(a, "GET", path, "", nil)
		if w.Code != 200 || !strings.Contains(w.Body.String(), "admin fixture") {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
	}
	if w := call(a, "GET", "/", "", nil); w.Code != 302 || w.Header().Get("Location") != "/admin/overview" {
		t.Fatalf("root route: %d", w.Code)
	}
	if w := call(a, "GET", "/api/v1/unknown", "", nil); w.Code != 404 {
		t.Fatalf("unknown API: %d", w.Code)
	}
}

func TestVueUnavailableRoutesDoNotCountVisits(t *testing.T) {
	a := setup(t)
	for _, tc := range []struct {
		path   string
		status int
	}{{"/missing", 404}, {"/hello", 410}} {
		if tc.status == 410 {
			if _, err := a.DB.Exec(context.Background(), "UPDATE short_links SET enabled=false WHERE code='hello'"); err != nil {
				t.Fatal(err)
			}
		}
		w := call(a, "GET", tc.path, "", nil)
		if w.Code != tc.status || !strings.Contains(w.Body.String(), `"error":{"status":`+strconv.Itoa(tc.status)) {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
	}
	var count int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events").Scan(&count); err != nil || count != 0 {
		t.Fatal("unavailable links count as visits", count, err)
	}
}

func TestIndependentAssetsAndMigrationRestart(t *testing.T) {
	a := setup(t)
	ctx := context.Background()
	var initialVersions int
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&initialVersions); err != nil || initialVersions < 2 {
		t.Fatal("initial migration versions", initialVersions, err)
	}
	// Recomposition must not overwrite an existing link or reapply versioned migrations.
	if _, err := a.DB.Exec(ctx, "UPDATE short_links SET name='Keep me',landing_delay=17 WHERE code='hello'"); err != nil {
		t.Fatal(err)
	}
	admin, visitor := t.TempDir(), t.TempDir()
	for _, fixture := range []struct{ dir, path, body string }{{admin, "admin-assets/admin-abcdefgh.js", "admin code"}, {visitor, "landing-assets/visitor-abcdefgh.js", "visitor code"}, {visitor, "landing-assets/images/picture.png", "image"}} {
		target := filepath.Join(fixture.dir, fixture.path)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(fixture.body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	cfg := a.Config
	cfg.FrontendDir = admin
	cfg.LandingDir = visitor
	second, err := New(cfg, a.DB)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	for _, tc := range []struct{ path, body string }{{"/admin-assets/admin-abcdefgh.js", "admin code"}, {"/landing-assets/visitor-abcdefgh.js", "visitor code"}, {"/landing-assets/images/picture.png", "image"}} {
		w := call(second, "GET", tc.path, "", nil)
		if w.Code != 200 || w.Body.String() != tc.body {
			t.Fatalf("%s: %d %s", tc.path, w.Code, w.Body.String())
		}
		if strings.HasSuffix(tc.path, ".js") && !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
			t.Fatal("hashed asset cache policy lost")
		}
	}
	for _, path := range []string{"/landing-assets/missing.js", "/admin-assets/missing.js", "/api/v1/missing"} {
		if w := call(second, "GET", path, "", nil); w.Code != 404 {
			t.Fatal(path, w.Code)
		}
	}
	var name string
	var delay, versions, logs int
	if err := a.DB.QueryRow(ctx, "SELECT name,landing_delay FROM short_links WHERE code='hello'").Scan(&name, &delay); err != nil || name != "Keep me" || delay != 17 {
		t.Fatal("link changed on restart", name, delay, err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil || versions != initialVersions {
		t.Fatal("migration versions", versions, err)
	}
	if err := a.DB.QueryRow(ctx, "SELECT count(*) FROM request_logs").Scan(&logs); err != nil || logs != 0 {
		t.Fatal("asset/API requests logged as visitors", logs, err)
	}
}

func TestBrowserMetadataDoesNotCreateVisitorLogs(t *testing.T) {
	a := setup(t)
	for _, path := range []string{"/favicon.ico", "/robots.txt"} {
		call(a, "GET", path, "", nil)
	}
	var count int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM request_logs").Scan(&count); err != nil || count != 0 {
		t.Fatalf("browser metadata created visitor logs: count=%d err=%v", count, err)
	}
}
