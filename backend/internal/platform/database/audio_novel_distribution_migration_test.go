package database

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// This source-level contract catches an omitted migration even when the
// PostgreSQL integration database is not configured on a developer machine.
func TestAudioNovelDistributionMigrationDefinesSchema(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/023_audio_novel_distribution.sql")
	if err != nil {
		t.Fatalf("read audio novel distribution migration: %v", err)
	}
	sqlText := string(sqlBytes)
	for _, required := range []string{
		"ADD COLUMN audio_duration_seconds integer NOT NULL DEFAULT 0",
		"ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id)",
		"ADD COLUMN playback_seconds integer NOT NULL DEFAULT 0",
		"ADD COLUMN media_consumed_seconds numeric(12,3) NOT NULL DEFAULT 0",
		"ADD COLUMN audio_started_at timestamptz",
		"ADD COLUMN audio_qualified_at timestamptz",
		"ADD COLUMN audio_completed_at timestamptz",
		"short_links_content_binding_check",
		"tiktok_events_content_binding_check",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("migration missing %q", required)
		}
	}
}

func TestAudioNovelVisitSnapshotMigrationDefinesFrozenTitle(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/024_audio_novel_visit_snapshot.sql")
	if err != nil {
		t.Fatalf("read audio visit snapshot migration: %v", err)
	}
	if !strings.Contains(string(sqlBytes), "ADD COLUMN audio_novel_title text NOT NULL DEFAULT ''") {
		t.Fatal("audio visit snapshot migration does not freeze the content title")
	}
}

func TestMetaAudioNovelEventMigrationDefinesFrozenBindingAndLookupIndex(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/025_meta_audio_novel_events.sql")
	if err != nil {
		t.Fatalf("read Meta audio novel event migration: %v", err)
	}
	sqlText := string(sqlBytes)
	for _, required := range []string{
		"ADD COLUMN audio_novel_id bigint REFERENCES audio_novels(id)",
		"UPDATE meta_events e",
		"FROM click_events c",
		"CREATE INDEX meta_events_visit_event_updated",
		"ON meta_events(visit_id,event_name,updated_at DESC)",
		"WHERE NOT is_test",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("Meta audio novel event migration missing %q", required)
		}
	}
}

func TestMetaAudioNovelEventMigrationBackfillsAndKeepsGenericEventsNullable(t *testing.T) {
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
	// This test rebuilds the schema at the exact pre-migration boundary, so it
	// must never be pointed at a non-test database.
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
		if entry.Name() >= "025_meta_audio_novel_events.sql" {
			break
		}
		sqlBytes, readErr := migrations.ReadFile("migrations/" + entry.Name())
		if readErr != nil {
			t.Fatal(readErr)
		}
		if _, err = db.Exec(ctx, string(sqlBytes)); err != nil {
			t.Fatalf("apply pre-025 migration %s: %v", entry.Name(), err)
		}
	}

	var connectionID, pixelID, audioNovelID, linkID int64
	if err = db.QueryRow(ctx, `INSERT INTO meta_connections(name,account_id) VALUES('Meta event migration','meta-event-migration') RETURNING id`).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO meta_pixels(connection_id,name,pixel_id,capi_token_cipher) VALUES($1,'Migration Pixel','250001','cipher') RETURNING id`, connectionID).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO audio_novels(
		title,slug,category,excerpt,body_markdown,audio_path,audio_duration,audio_duration_seconds,audio_size_bytes,published_at,enabled)
		VALUES('Migration Audio','migration-audio','Drama','Excerpt','# Body','/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3','00:10',10,100,'2026-09-24',true)
		RETURNING id`).Scan(&audioNovelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO short_links(
		code,name,target_url,product_type,audio_novel_id,ad_platform,meta_connection_id,meta_pixel_id)
		VALUES('meta-event-migration','Migration link','','audio_novel',$1,'meta',$2,$3) RETURNING id`, audioNovelID, connectionID, pixelID).Scan(&linkID); err != nil {
		t.Fatal(err)
	}
	const visitID = "meta-audio-migration-visit"
	if _, err = db.Exec(ctx, `INSERT INTO click_events(
		id,link_id,cookie_status,method,target_url,status,device,os,browser,
		country,region,city,source,campaign_id,adset_id,ad_id,referrer,
		classification,reason,event_type,surface,audio_novel_id)
		VALUES($1,$2,'off','GET','',200,'mobile','test','test',
		'test','test','test','facebook','','','ad-1','','normal','','landing','audio_novel',$3)`, visitID, linkID, audioNovelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO meta_events(
		id,connection_id,pixel_record_id,visit_id,event_name,event_time,pixel_id)
		VALUES('meta-audio-migration-event',$1,$2,$3,'StartListening',now(),'250001')`, connectionID, pixelID, visitID); err != nil {
		t.Fatal(err)
	}

	migrationSQL, err := migrations.ReadFile("migrations/025_meta_audio_novel_events.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, string(migrationSQL)); err != nil {
		t.Fatalf("apply Meta audio novel event migration: %v", err)
	}
	var frozenAudioNovelID *int64
	if err = db.QueryRow(ctx, `SELECT audio_novel_id FROM meta_events WHERE id='meta-audio-migration-event'`).Scan(&frozenAudioNovelID); err != nil || frozenAudioNovelID == nil || *frozenAudioNovelID != audioNovelID {
		t.Fatalf("audio binding was not backfilled: value=%v err=%v", frozenAudioNovelID, err)
	}
	// Generic and test events have no audio content owner and must remain valid.
	if _, err = db.Exec(ctx, `INSERT INTO meta_events(
		id,connection_id,pixel_record_id,event_name,event_time,pixel_id,is_test)
		VALUES('generic-meta-test-event',$1,$2,'PageView',now(),'250001',true)`, connectionID, pixelID); err != nil {
		t.Fatalf("nullable audio binding broke generic Meta events: %v", err)
	}
	var indexPredicate string
	if err = db.QueryRow(ctx, `SELECT pg_get_expr(i.indpred,i.indrelid)
		FROM pg_index i JOIN pg_class idx ON idx.oid=i.indexrelid
		WHERE idx.relname='meta_events_visit_event_updated'`).Scan(&indexPredicate); err != nil || !strings.Contains(indexPredicate, "NOT is_test") {
		t.Fatalf("non-test Meta event lookup index missing: predicate=%q err=%v", indexPredicate, err)
	}
}

func TestAudioNovelDistributionMigrationAppliesAndEnforcesBindings(t *testing.T) {
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
	// The test resets public, so never allow it to point at a production database.
	if !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing to reset non-test database %q", databaseName)
	}
	if _, err = db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err = Migrate(ctx, db); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	for table, column := range map[string]string{
		"short_links":   "audio_novel_id",
		"audio_novels":  "audio_duration_seconds",
		"click_events":  "playback_seconds",
		"tiktok_events": "audio_novel_id",
	} {
		var exists bool
		if err = db.QueryRow(ctx, `
			SELECT EXISTS(
				SELECT 1 FROM information_schema.columns
				WHERE table_schema='public' AND table_name=$1 AND column_name=$2
			)
		`, table, column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		if !exists {
			t.Fatalf("expected %s.%s after migrations", table, column)
		}
	}
	var frozenTitleExists bool
	if err = db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns
		WHERE table_schema='public' AND table_name='click_events' AND column_name='audio_novel_title')`).Scan(&frozenTitleExists); err != nil || !frozenTitleExists {
		t.Fatalf("expected click_events.audio_novel_title after migrations: exists=%v err=%v", frozenTitleExists, err)
	}

	if _, err = db.Exec(ctx, `
		INSERT INTO short_links(code,name,target_url,product_type,ad_platform)
		VALUES('legacy-audio-schema','legacy audio','https://example.com','legacy','meta')
	`); err != nil {
		t.Fatalf("legacy link without content binding must remain valid: %v", err)
	}

	var metaConnectionID, metaPixelID, audioNovelID, novelID int64
	if err = db.QueryRow(ctx, `INSERT INTO meta_connections(name,account_id) VALUES('Meta audio test','123456') RETURNING id`).Scan(&metaConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO meta_pixels(connection_id,name,pixel_id,capi_token_cipher) VALUES($1,'Audio Pixel','987654','cipher') RETURNING id`, metaConnectionID).Scan(&metaPixelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `
		INSERT INTO audio_novels(title,slug,category,excerpt,body_markdown,audio_path,audio_duration,audio_duration_seconds,audio_size_bytes,published_at,enabled)
		VALUES('Audio story','audio-story','Drama','Excerpt','# Body','/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3','00:10',10,100,'2026-09-24',true)
		RETURNING id
	`).Scan(&audioNovelID); err != nil {
		t.Fatal(err)
	}
	// Fresh databases intentionally have no free-novel seed after the audio
	// archive split, so the fixture creates the second binding explicitly.
	if err = db.QueryRow(ctx, `
		INSERT INTO novels(title,slug,author,category,excerpt,published_at,enabled)
		VALUES('Text story','text-story','Tester','Drama','Excerpt','2026-09-24',true)
		RETURNING id
	`).Scan(&novelID); err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec(ctx, `
		INSERT INTO short_links(code,name,target_url,product_type,ad_platform,meta_connection_id,meta_pixel_id)
		VALUES('audio-missing-binding','bad','','audio_novel','meta',$1,$2)
	`, metaConnectionID, metaPixelID); err == nil {
		t.Fatal("audio novel link accepted a missing audio_novel_id")
	}
	if _, err = db.Exec(ctx, `
		INSERT INTO short_links(code,name,target_url,product_type,novel_id,audio_novel_id,ad_platform,meta_connection_id,meta_pixel_id)
		VALUES('audio-double-binding','bad','','audio_novel',$1,$2,'meta',$3,$4)
	`, novelID, audioNovelID, metaConnectionID, metaPixelID); err == nil {
		t.Fatal("audio novel link accepted both content bindings")
	}

	var tiktokConnectionID, tiktokPixelID int64
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_connections(name,access_token_cipher) VALUES('TikTok audio test','cipher') RETURNING id`).Scan(&tiktokConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_pixels(connection_id,name,pixel_code) VALUES($1,'TikTok Audio Pixel','C0AUDIO123') RETURNING id`, tiktokConnectionID).Scan(&tiktokPixelID); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `
		INSERT INTO tiktok_events(
			id,visit_id,link_id,novel_id,audio_novel_id,connection_id,pixel_record_id,
			pixel_code,event_name,event_id,event_time,payload_cipher
		) VALUES(
			'audio-double-event','visit',1,$1,$2,$3,$4,
			'C0AUDIO123','ViewContent','event',now(),'cipher'
		)
	`, novelID, audioNovelID, tiktokConnectionID, tiktokPixelID); err == nil {
		t.Fatal("TikTok event accepted both content bindings")
	}

	if _, err = db.Exec(ctx, `
		INSERT INTO click_events(
			id,link_id,cookie_status,method,target_url,status,device,os,browser,
			country,region,city,source,campaign_id,adset_id,ad_id,referrer,
			classification,reason,event_type,surface,playback_seconds
		)
		SELECT 'negative-audio-playback',id,'off','GET',target_url,200,'desktop','test','test',
			'test','test','test','direct','','','','','normal','','landing','audio_novel',-1
		FROM short_links WHERE code='hello'
	`); err == nil {
		t.Fatal("click event accepted negative playback_seconds")
	}
	if _, err = db.Exec(ctx, `
		INSERT INTO click_events(
			id,link_id,cookie_status,method,target_url,status,device,os,browser,
			country,region,city,source,campaign_id,adset_id,ad_id,referrer,
			classification,reason,event_type,surface,media_consumed_seconds
		)
		SELECT 'negative-media-playback',id,'off','GET',target_url,200,'desktop','test','test',
			'test','test','test','direct','','','','','normal','','landing','audio_novel',-0.1
		FROM short_links WHERE code='hello'
	`); err == nil {
		t.Fatal("click event accepted negative media_consumed_seconds")
	}
}
