package database

import (
	"strings"
	"testing"
)

func TestNovelTranslationMigrationDefinesVersionedPublishing(t *testing.T) {
	sqlBytes, err := migrations.ReadFile("migrations/021_novel_translations.sql")
	if err != nil {
		t.Fatalf("read novel translation migration: %v", err)
	}
	sql := string(sqlBytes)
	for _, required := range []string{
		"source_revision bigint NOT NULL DEFAULT 1",
		"CREATE TABLE novel_translation_versions",
		"CREATE TABLE novel_chapter_translations",
		"novel_translation_one_published",
		"novel_translation_one_active",
		"queued', 'running', 'failed', 'published', 'superseded",
	} {
		if !strings.Contains(sql, required) {
			t.Fatalf("translation migration does not contain %q", required)
		}
	}
}
