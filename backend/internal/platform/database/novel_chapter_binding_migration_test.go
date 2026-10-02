package database

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The migration contract keeps historical links nullable while freezing the
// selected entry chapter for every new visitor.
func TestNovelChapterBindingMigrationDefinesLinkAndVisitSnapshots(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/026_novel_chapter_binding.sql")
	if err != nil {
		t.Fatalf("read novel chapter binding migration: %v", err)
	}
	sqlText := string(sqlBytes)
	for _, required := range []string{
		"ADD COLUMN entry_chapter_id bigint REFERENCES novel_chapters(id)",
		"ADD COLUMN entry_chapter_number integer",
		"ADD COLUMN entry_chapter_title text NOT NULL DEFAULT ''",
		"short_links_entry_chapter_owner_fk",
		"short_links_entry_chapter_product_check",
		"click_events_entry_chapter_owner_fk",
		"short_links_entry_chapter_lookup",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("novel chapter binding migration missing %q", required)
		}
	}
}

func TestNovelChapterSnapshotIntegrityMigrationDefinesAtomicIdentity(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/027_novel_chapter_snapshot_integrity.sql")
	if err != nil {
		t.Fatalf("read novel chapter snapshot integrity migration: %v", err)
	}
	sqlText := string(sqlBytes)
	for _, required := range []string{
		"DROP CONSTRAINT IF EXISTS click_events_entry_chapter_snapshot_check",
		"ADD CONSTRAINT click_events_entry_chapter_snapshot_check",
		"entry_chapter_id IS NULL AND entry_chapter_number IS NULL AND entry_chapter_title = ''",
		"entry_chapter_id IS NOT NULL AND novel_id IS NOT NULL AND entry_chapter_number IS NOT NULL",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("novel chapter snapshot integrity migration missing %q", required)
		}
	}
}

func TestNovelChapterBindingMigrationKeepsHistoricalRowsNullable(t *testing.T) {
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
	// This test rebuilds the exact pre-026 boundary and must never target production.
	if !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing to reset non-test database %q", databaseName)
	}
	if _, err = db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() >= "026_novel_chapter_binding.sql" {
			break
		}
		sqlBytes, readErr := migrations.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = db.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply pre-026 migration %s: %v", entry.Name(), err)
		}
	}

	var connectionID, pixelID, novelID, linkID int64
	if err = db.QueryRow(ctx, `INSERT INTO meta_connections(name,account_id)
		VALUES('Historical Meta','historical-meta') RETURNING id`).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO meta_pixels(connection_id,name,pixel_id,capi_token_cipher)
		VALUES($1,'Historical Pixel','260026','cipher') RETURNING id`, connectionID).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO novels(title,slug,author,category,excerpt,published_at,enabled)
		VALUES('Historical story','historical-story','Tester','Drama','Excerpt','2026-09-25',true) RETURNING id`).Scan(&novelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO short_links(
		code,name,target_url,product_type,novel_id,ad_platform,meta_connection_id,meta_pixel_id)
		VALUES('historical-novel-entry','Historical link','','novel',$1,'meta',$2,$3) RETURNING id`, novelID, connectionID, pixelID).Scan(&linkID); err != nil {
		t.Fatal(err)
	}
	const visitID = "historical-novel-entry-visit"
	if _, err = db.Exec(ctx, `INSERT INTO click_events(
		id,link_id,cookie_status,method,target_url,status,device,os,browser,
		country,region,city,source,campaign_id,adset_id,ad_id,referrer,
		classification,reason,event_type,surface,novel_id)
		VALUES($1,$2,'off','GET','',200,'mobile','test','test',
		'test','test','test','facebook','','','ad-1','','normal','','landing','novel',$3)`, visitID, linkID, novelID); err != nil {
		t.Fatal(err)
	}

	migrationSQL, err := migrations.ReadFile("migrations/026_novel_chapter_binding.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("apply novel chapter binding migration: %v", err)
	}
	var linkChapterID *int64
	if err = db.QueryRow(ctx, "SELECT entry_chapter_id FROM short_links WHERE id=$1", linkID).Scan(&linkChapterID); err != nil || linkChapterID != nil {
		t.Fatalf("historical link binding changed: value=%v err=%v", linkChapterID, err)
	}
	var visitChapterID *int64
	var visitChapterNumber *int
	var visitChapterTitle string
	if err = db.QueryRow(ctx, `SELECT entry_chapter_id,entry_chapter_number,entry_chapter_title
		FROM click_events WHERE id=$1`, visitID).Scan(&visitChapterID, &visitChapterNumber, &visitChapterTitle); err != nil || visitChapterID != nil || visitChapterNumber != nil || visitChapterTitle != "" {
		t.Fatalf("historical visit snapshot changed: id=%v number=%v title=%q err=%v", visitChapterID, visitChapterNumber, visitChapterTitle, err)
	}
}
