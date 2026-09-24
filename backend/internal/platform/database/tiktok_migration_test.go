package database

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This test catches a missing or incomplete additive migration before the
// application tries to query TikTok attribution columns at runtime.
func TestTikTokAttributionMigrationDefinesSchema(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/022_tiktok_novel_attribution.sql")
	if err != nil {
		t.Fatalf("read TikTok attribution migration: %v", err)
	}
	sqlText := string(sqlBytes)
	for _, required := range []string{
		"CREATE TABLE tiktok_connections",
		"CREATE TABLE tiktok_pixels",
		"CREATE TABLE tiktok_events",
		"ADD COLUMN ad_platform text NOT NULL DEFAULT 'meta'",
		"ADD COLUMN tiktok_pixel_id bigint REFERENCES tiktok_pixels(id)",
		"UNIQUE(pixel_code,event_name,event_id)",
		"status IN ('pending','sending','accepted','retry','failed')",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestTikTokAttributionMigrationAppliesAndEnforcesPlatformBinding(t *testing.T) {
	testDatabaseURL := os.Getenv("TEST_DATABASE_URL")
	if testDatabaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for the PostgreSQL migration test")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, testDatabaseURL)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()

	var databaseName string
	if err = db.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	// The test applies every migration from scratch and must never target production.
	if !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing to reset non-test database %q", databaseName)
	}
	if _, err = db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for _, table := range []string{"tiktok_connections", "tiktok_pixels", "tiktok_events"} {
		if !migrationTableExists(t, ctx, db, table) {
			t.Fatalf("expected migrated table %q", table)
		}
	}
	var defaultExpression string
	if err = db.QueryRow(ctx, `SELECT column_default FROM information_schema.columns WHERE table_schema='public' AND table_name='short_links' AND column_name='ad_platform'`).Scan(&defaultExpression); err != nil {
		t.Fatal(err)
	}
	if defaultExpression != "'meta'::text" {
		t.Fatalf("historical links default = %q, want meta", defaultExpression)
	}

	var legacyPlatform string
	if err = db.QueryRow(ctx, "SELECT ad_platform FROM short_links WHERE code='hello'").Scan(&legacyPlatform); err != nil {
		t.Fatalf("read historical link: %v", err)
	}
	if legacyPlatform != "meta" {
		t.Fatalf("historical link platform = %q, want meta", legacyPlatform)
	}

	var connectionID, metaPixelID, novelID, tiktokConnectionID, tiktokPixelID int64
	if err = db.QueryRow(ctx, `INSERT INTO meta_connections(name,account_id) VALUES('Meta test','12345') RETURNING id`).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO meta_pixels(connection_id,name,pixel_id,capi_token_cipher) VALUES($1,'Meta Pixel','98765','cipher') RETURNING id`, connectionID).Scan(&metaPixelID); err != nil {
		t.Fatal(err)
	}
	// Fresh databases intentionally contain no free-novel seed after the
	// audio/free-novel split, so this migration test owns its complete fixture.
	if err = db.QueryRow(ctx, `INSERT INTO novels(title,slug,author,category,excerpt,published_at,enabled)
		VALUES('TikTok text story','tiktok-text-story','Tester','Drama','Excerpt','2026-09-24',true)
		RETURNING id`).Scan(&novelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_connections(name,access_token_cipher) VALUES('TikTok test','cipher') RETURNING id`).Scan(&tiktokConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_pixels(connection_id,name,pixel_code) VALUES($1,'TikTok Pixel','C0ABC123') RETURNING id`, tiktokConnectionID).Scan(&tiktokPixelID); err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec(ctx, `INSERT INTO short_links(code,name,target_url,product_type,novel_id,ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id) VALUES('invalid-tiktok','bad','', 'novel',$1,'tiktok',$2,$3,$4)`, novelID, connectionID, metaPixelID, tiktokPixelID); err == nil {
		t.Fatal("TikTok novel link accepted a Meta Pixel binding")
	}
	if _, err = db.Exec(ctx, `INSERT INTO short_links(code,name,target_url,product_type,novel_id,ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id) VALUES('invalid-meta','bad','', 'novel',$1,'meta',$2,$3,$4)`, novelID, connectionID, metaPixelID, tiktokPixelID); err == nil {
		t.Fatal("Meta novel link accepted a TikTok Pixel binding")
	}
	if _, err = db.Exec(ctx, `INSERT INTO short_links(code,name,target_url,product_type,ad_platform) VALUES('legacy-ok','legacy','https://example.com','legacy','meta')`); err != nil {
		t.Fatalf("legacy link without a Pixel must remain valid: %v", err)
	}
}
