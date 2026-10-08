package database

import (
	"strings"
	"testing"
)

func TestFreeNovelMigrationDefinesIndependentContentAndSurface(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/018_free_novels.sql")
	if err != nil {
		t.Fatalf("read free novel migration: %v", err)
	}
	sql := string(sqlBytes)
	for _, required := range []string{
		"CREATE TABLE novels",
		"CREATE TABLE novel_chapters",
		"novel_chapters_active_number_unique",
		"'short_link', 'audio_novel', 'novel'",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("migration does not contain %q", required)
		}
	}
}

func TestNovelDistributionMigrationFreezesLinkAndReadingMetrics(t *testing.T) {
	// 投放链接、访问归属和可见时长必须由数据库列约束，避免只靠前端约定。
	sqlBytes, err := migrations.ReadFile("migrations/019_novel_distribution.sql")
	if err != nil {
		t.Fatalf("read novel distribution migration: %v", err)
	}
	sql := string(sqlBytes)
	for _, required := range []string{
		"product_type",
		"novel_id bigint REFERENCES novels(id)",
		"first_visited_at timestamptz",
		"visible_seconds integer NOT NULL DEFAULT 0",
		"clicks_novel_link_time",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("distribution migration does not contain %q", required)
		}
	}
}

func TestNovelLinkStartupTailMigrationUsesSafeDefaultAndBounds(t *testing.T) {
	// The database remains the final guard when an admin request bypasses browser validation.
	sqlBytes, err := migrations.ReadFile("migrations/029_novel_link_startup_tail_seconds.sql")
	if err != nil {
		t.Fatalf("read startup-tail migration: %v", err)
	}
	sql := string(sqlBytes)
	for _, required := range []string{
		"startup_tail_seconds integer NOT NULL DEFAULT 5",
		"short_links_startup_tail_seconds_check",
		"startup_tail_seconds BETWEEN 1 AND 60",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("startup-tail migration does not contain %q", required)
		}
	}
}
