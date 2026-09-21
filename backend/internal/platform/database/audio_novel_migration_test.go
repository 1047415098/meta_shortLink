package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAudioNovelRenameMigration(t *testing.T) {
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
	if err := db.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil {
		t.Fatalf("read test database name: %v", err)
	}
	// This test recreates public, so an explicit test database name is mandatory.
	if !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing to reset non-test database %q", databaseName)
	}
	if _, err := db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}

	entries, err := migrations.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read migrations: %v", err)
	}
	for _, entry := range entries {
		if entry.Name() > "015_time_spent.sql" {
			break
		}
		sqlBytes, err := migrations.ReadFile("migrations/" + entry.Name())
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		if _, err := db.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply %s: %v", entry.Name(), err)
		}
	}

	// Use one controlled row so row-count and identity preservation are unambiguous.
	if _, err := db.Exec(ctx, "TRUNCATE TABLE novels RESTART IDENTITY"); err != nil {
		t.Fatalf("clear seeded novels: %v", err)
	}
	var preservedID int64
	if err := db.QueryRow(ctx, `
		INSERT INTO novels(title,slug,category,excerpt,body_markdown,cover_path,published_at,enabled,featured)
		VALUES('Preserved story','preserved-story','Fantasy','Excerpt','# Body','/novel-uploads/0123456789abcdef0123456789abcdef.webp','2026-09-21',true,true)
		RETURNING id
	`).Scan(&preservedID); err != nil {
		t.Fatalf("insert preserved novel: %v", err)
	}
	if _, err := db.Exec(ctx, `
		INSERT INTO click_events(
			id,link_id,cookie_status,method,target_url,status,device,os,browser,
			country,region,city,source,campaign_id,adset_id,ad_id,referrer,
			classification,reason,event_type,surface
		)
		SELECT 'audio-novel-rename-visit',id,'off','GET',target_url,200,'desktop','test','test',
			'test','test','test','direct','','','','','normal','','landing','novel'
		FROM short_links ORDER BY id LIMIT 1
	`); err != nil {
		t.Fatalf("insert preserved novel visit: %v", err)
	}

	renameSQL, err := migrations.ReadFile("migrations/016_audio_novel_rename.sql")
	if err != nil {
		t.Fatalf("read audio novel rename migration: %v", err)
	}
	if _, err := db.Exec(ctx, string(renameSQL)); err != nil {
		t.Fatalf("apply audio novel rename migration: %v", err)
	}

	if got := migrationTableCount(t, ctx, db, "audio_novels"); got != 1 {
		t.Fatalf("audio_novels rows = %d, want 1", got)
	}
	if migrationTableExists(t, ctx, db, "novels") {
		t.Fatal("old novels table still exists")
	}
	for _, relation := range []string{
		"audio_novels_id_seq",
		"audio_novels_active_slug_unique",
		"audio_novels_one_effective_featured",
		"audio_novels_public_order",
	} {
		if !migrationTableExists(t, ctx, db, relation) {
			t.Fatalf("renamed relation %q does not exist", relation)
		}
	}

	var surface string
	if err := db.QueryRow(ctx, "SELECT surface FROM click_events WHERE id='audio-novel-rename-visit'").Scan(&surface); err != nil {
		t.Fatalf("read migrated surface: %v", err)
	}
	if surface != "audio_novel" {
		t.Fatalf("surface = %q, want audio_novel", surface)
	}

	coverPathSQL, err := migrations.ReadFile("migrations/017_audio_novel_cover_paths.sql")
	if err != nil {
		t.Fatalf("read audio novel cover-path migration: %v", err)
	}
	if _, err := db.Exec(ctx, string(coverPathSQL)); err != nil {
		t.Fatalf("apply audio novel cover-path migration: %v", err)
	}
	var coverPath string
	if err := db.QueryRow(ctx, "SELECT cover_path FROM audio_novels WHERE id=$1", preservedID).Scan(&coverPath); err != nil {
		t.Fatalf("read migrated cover path: %v", err)
	}
	if coverPath != "/audio-novel-uploads/0123456789abcdef0123456789abcdef.webp" {
		t.Fatalf("cover path = %q", coverPath)
	}

	var nextID int64
	if err := db.QueryRow(ctx, `
		INSERT INTO audio_novels(title,slug,category,excerpt,body_markdown,published_at)
		VALUES('Next story','next-story','Fantasy','Excerpt','# Body','2026-09-22')
		RETURNING id
	`).Scan(&nextID); err != nil {
		t.Fatalf("insert audio novel after migration: %v", err)
	}
	if nextID <= preservedID {
		t.Fatalf("new identity = %d, want greater than preserved id %d", nextID, preservedID)
	}

	// The rebuilt constraint must reject the retired surface value.
	if _, err := db.Exec(ctx, `
		INSERT INTO click_events(
			id,link_id,cookie_status,method,target_url,status,device,os,browser,
			country,region,city,source,campaign_id,adset_id,ad_id,referrer,
			classification,reason,event_type,surface
		)
		SELECT 'retired-novel-surface',id,'off','GET',target_url,200,'desktop','test','test',
			'test','test','test','direct','','','','','normal','','landing','novel'
		FROM short_links ORDER BY id LIMIT 1
	`); err == nil {
		t.Fatal("retired surface novel was accepted")
	}
}

func migrationTableCount(t *testing.T, ctx context.Context, db *pgxpool.Pool, table string) int {
	t.Helper()
	var count int
	if err := db.QueryRow(ctx, fmt.Sprintf("SELECT count(*) FROM %s", table)).Scan(&count); err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return count
}

func migrationTableExists(t *testing.T, ctx context.Context, db *pgxpool.Pool, relation string) bool {
	t.Helper()
	var exists bool
	if err := db.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", "public."+relation).Scan(&exists); err != nil {
		t.Fatalf("check relation %s: %v", relation, err)
	}
	return exists
}
