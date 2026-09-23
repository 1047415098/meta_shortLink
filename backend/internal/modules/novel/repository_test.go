package novel

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whatsapp-analytics/internal/platform/database"
)

func TestRepositoryCreatesIndependentChaptersAndChecksOwnership(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is required for repository integration tests")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	defer db.Close()
	var name string
	if err = db.QueryRow(ctx, "SELECT current_database()").Scan(&name); err != nil || !strings.Contains(name, "_test") {
		t.Fatalf("refusing non-test database %q: %v", name, err)
	}
	// Migration tests intentionally leave partial schemas behind, so each destructive integration test starts clean.
	if _, err = db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatalf("reset test schema: %v", err)
	}
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if _, err = db.Exec(ctx, "TRUNCATE novel_chapters,novels RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("reset novel data: %v", err)
	}
	repo := Repository{DB: db}
	first, err := repo.Create(ctx, NovelInput{Title: "One", Slug: "story-one", Excerpt: "Excerpt", PublishedAt: "2026-09-21", Enabled: true}, "test")
	if err != nil {
		t.Fatalf("create novel: %v", err)
	}
	second, err := repo.Create(ctx, NovelInput{Title: "Two", Slug: "story-two", Excerpt: "Excerpt", PublishedAt: "2026-09-21", Enabled: true}, "test")
	if err != nil {
		t.Fatalf("create second novel: %v", err)
	}
	chapter, err := repo.CreateChapter(ctx, first.ID, ChapterInput{ChapterNumber: 1, Title: "Start", BodyMarkdown: "Body", Enabled: true}, "test")
	if err != nil {
		t.Fatalf("create chapter: %v", err)
	}
	if _, err = repo.CreateChapter(ctx, first.ID, ChapterInput{ChapterNumber: 1, BodyMarkdown: "Again", Enabled: true}, "test"); err == nil {
		t.Fatal("duplicate chapter number accepted")
	}
	if _, err = repo.UpdateChapter(ctx, second.ID, chapter.ID, ChapterInput{ChapterNumber: 2, BodyMarkdown: "Wrong", Enabled: true}, "test"); err != pgx.ErrNoRows {
		t.Fatalf("wrong owner error = %v, want no rows", err)
	}
}
