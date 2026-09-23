package novel

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"whatsapp-analytics/internal/platform/database"
)

type fakeTextTranslator struct {
	mu       sync.Mutex
	failText string
	started  chan struct{}
	release  chan struct{}
}

type disabledTextTranslator struct{}

func (disabledTextTranslator) Configured() bool { return false }
func (disabledTextTranslator) Translate(context.Context, string, string) (string, error) {
	return "", errors.New("disabled")
}

type blockingTextTranslator struct{ release <-chan struct{} }

type sizeLimitedTextTranslator struct{ maxBytes int }

func (translator sizeLimitedTextTranslator) Configured() bool { return true }
func (translator sizeLimitedTextTranslator) Translate(_ context.Context, _ string, text string) (string, error) {
	if len([]byte(text)) > translator.maxBytes {
		return "", translationProviderError{code: 400, message: "失败，请重试！"}
	}
	return text, nil
}

func (blockingTextTranslator) Configured() bool { return true }
func (translator blockingTextTranslator) Translate(ctx context.Context, _, text string) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-translator.release:
		return text, nil
	}
}

func (f *fakeTextTranslator) Configured() bool { return true }
func (f *fakeTextTranslator) Translate(ctx context.Context, locale, text string) (string, error) {
	if f.started != nil {
		f.mu.Lock()
		started := f.started
		f.started = nil
		f.mu.Unlock()
		if started != nil {
			close(started)
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-f.release:
			}
		}
	}
	if f.failText != "" && strings.Contains(text, f.failText) {
		return "", errors.New("forced translation failure")
	}
	return "[" + locale + "]" + text, nil
}

func TestNormalizeTargetLocalesRejectsUnsupportedAndDeduplicates(t *testing.T) {
	got, err := NormalizeTargetLocales([]string{"ja", "ko", "ja"})
	if err != nil || strings.Join(got, ",") != "ja,ko" {
		t.Fatalf("normalized locales = %v, %v", got, err)
	}
	for _, input := range [][]string{nil, {}, {"en"}, {"ja", "xx"}} {
		if _, err := NormalizeTargetLocales(input); err == nil {
			t.Fatalf("invalid locales accepted: %v", input)
		}
	}
}

func TestTranslateMarkdownPreservesCodeFencesAndStandaloneURLs(t *testing.T) {
	translator := &fakeTextTranslator{}
	service := &TranslationService{translator: translator}
	source := "Opening paragraph.\n\n````md\n```\nthis line remains code\n````\nhttps://outside.example/path?q=1\n\nClosing paragraph."
	translated, err := service.translateMarkdown(context.Background(), "ja", source)
	if err != nil {
		t.Fatal(err)
	}
	for _, protected := range []string{"````md\n```\nthis line remains code\n````", "https://outside.example/path?q=1"} {
		if !strings.Contains(translated, protected) {
			t.Fatalf("protected Markdown changed: %q", translated)
		}
	}
	if !strings.Contains(translated, "[ja]Opening paragraph.") || !strings.Contains(translated, "[ja]Closing paragraph.") {
		t.Fatalf("prose was not translated around protected blocks: %q", translated)
	}
}

func TestTranslationServiceAdaptsWhenProviderRejectsRecommendedChunkSize(t *testing.T) {
	// 初始请求保持较大分段；若服务商对某个段落只接受 180 字节，应自动缩小后完成。
	service := &TranslationService{translator: sizeLimitedTextTranslator{maxBytes: 180}}
	source := strings.Repeat("A readable English sentence with natural boundaries. ", 50)
	translated, err := service.translateText(context.Background(), "ja", source)
	if err != nil {
		t.Fatal(err)
	}
	if translated != source {
		t.Fatal("safe chunking changed the source sequence")
	}
}

func TestTranslationServiceSplitsRejectedChunkAndRetriesSmallerPieces(t *testing.T) {
	// 上游偶发拒绝某些复杂段落时，继续缩小分段，避免整本小说因单个长句失败。
	service := &TranslationService{translator: sizeLimitedTextTranslator{maxBytes: 90}}
	source := strings.Repeat("A sentence with a stable word boundary. ", 16)
	translated, err := service.translateText(context.Background(), "pt", source)
	if err != nil {
		t.Fatal(err)
	}
	if translated != source {
		t.Fatal("adaptive chunking changed the source sequence")
	}
}

func translationTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for translation integration tests")
	}
	db, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	var databaseName string
	if err = db.QueryRow(context.Background(), "SELECT current_database()").Scan(&databaseName); err != nil || !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing non-test database %q: %v", databaseName, err)
	}
	// Translation cases publish multiple versions, so isolate every case from earlier migration fixtures.
	if _, err = db.Exec(context.Background(), "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(context.Background(), "TRUNCATE novel_translation_versions,novel_chapters,novels RESTART IDENTITY CASCADE"); err != nil {
		t.Fatal(err)
	}
	return db
}

func waitTranslationStatus(t *testing.T, service *TranslationService, novelID int64, locale, status string) TranslationStatus {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		items, err := service.List(context.Background(), novelID)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range items {
			if item.Locale == locale && item.Status == status {
				return item
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("locale %s did not reach %s", locale, status)
	return TranslationStatus{}
}

func TestTranslationServicePublishesCompleteVersionAndKeepsOldUntilReplacement(t *testing.T) {
	db := translationTestDB(t)
	repo := Repository{DB: db}
	novel, err := repo.Create(context.Background(), NovelInput{Title: "Story", Slug: "story", Author: "Nine", Category: "Drama", Excerpt: "Excerpt", PublishedAt: "2026-09-22", Enabled: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	chapter, err := repo.CreateChapter(context.Background(), novel.ID, ChapterInput{ChapterNumber: 1, Title: "Start", BodyMarkdown: "Body", Enabled: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	service := NewTranslationService(db, &fakeTextTranslator{}, 1)
	t.Cleanup(service.Close)
	if _, err = service.Queue(context.Background(), novel.ID, []string{"ja"}, "test"); err != nil {
		t.Fatal(err)
	}
	first := waitTranslationStatus(t, service, novel.ID, "ja", "published")
	if !first.Enabled || first.CompletedItems != first.TotalItems {
		t.Fatalf("first translation = %#v", first)
	}
	if _, err = repo.UpdateChapter(context.Background(), novel.ID, chapter.ID, ChapterInput{ChapterNumber: 1, Title: "Start", BodyMarkdown: "Changed", Enabled: true}, "test"); err != nil {
		t.Fatal(err)
	}
	stale := waitTranslationStatus(t, service, novel.ID, "ja", "stale")
	if stale.PublishedSourceRevision >= stale.SourceRevision {
		t.Fatalf("stale revisions = %#v", stale)
	}
	if _, err = service.Queue(context.Background(), novel.ID, []string{"ja"}, "test"); err != nil {
		t.Fatal(err)
	}
	waitTranslationStatus(t, service, novel.ID, "ja", "published")
	var published, superseded int
	if err = db.QueryRow(context.Background(), "SELECT count(*) FILTER (WHERE status='published'),count(*) FILTER (WHERE status='superseded') FROM novel_translation_versions WHERE novel_id=$1 AND locale='ja'", novel.ID).Scan(&published, &superseded); err != nil || published != 1 || superseded != 1 {
		t.Fatalf("versions published=%d superseded=%d err=%v", published, superseded, err)
	}
}

func TestTranslationServiceNeverPublishesFailureOrRacingSource(t *testing.T) {
	db := translationTestDB(t)
	repo := Repository{DB: db}
	novel, err := repo.Create(context.Background(), NovelInput{Title: "Story", Slug: "story", Excerpt: "Excerpt", PublishedAt: "2026-09-22", Enabled: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = repo.CreateChapter(context.Background(), novel.ID, ChapterInput{ChapterNumber: 1, Title: "Start", BodyMarkdown: "Body", Enabled: true}, "test"); err != nil {
		t.Fatal(err)
	}
	failing := NewTranslationService(db, &fakeTextTranslator{failText: "Body"}, 1)
	if _, err = failing.Queue(context.Background(), novel.ID, []string{"ko"}, "test"); err != nil {
		t.Fatal(err)
	}
	waitTranslationStatus(t, failing, novel.ID, "ko", "failed")
	failing.Close()

	started, release := make(chan struct{}), make(chan struct{})
	racing := NewTranslationService(db, &fakeTextTranslator{started: started, release: release}, 1)
	t.Cleanup(racing.Close)
	if _, err = racing.Queue(context.Background(), novel.ID, []string{"th"}, "test"); err != nil {
		t.Fatal(err)
	}
	<-started
	current, err := repo.ByID(context.Background(), novel.ID)
	if err != nil {
		t.Fatal(err)
	}
	current.Title = "Changed while translating"
	if _, err = repo.Update(context.Background(), novel.ID, NovelInput{Title: current.Title, Slug: current.Slug, Author: current.Author, Category: current.Category, Excerpt: current.Excerpt, CoverPath: current.CoverPath, PublishedAt: current.PublishedAt, Enabled: current.Enabled, Featured: current.Featured, SortOrder: current.SortOrder}, "test"); err != nil {
		t.Fatal(err)
	}
	close(release)
	waitTranslationStatus(t, racing, novel.ID, "th", "failed")
	var published int
	if err = db.QueryRow(context.Background(), "SELECT count(*) FROM novel_translation_versions WHERE novel_id=$1 AND status='published'", novel.ID).Scan(&published); err != nil || published != 0 {
		t.Fatalf("racing source published=%d err=%v", published, err)
	}
}

func TestTranslationRecoveryOnlyRequeuesExpiredRunningJobsWithoutCredentials(t *testing.T) {
	db := translationTestDB(t)
	repo := Repository{DB: db}
	novel, err := repo.Create(context.Background(), NovelInput{Title: "Story", Slug: "recovery-story", Excerpt: "Excerpt", PublishedAt: "2026-09-22", Enabled: true}, "test")
	if err != nil {
		t.Fatal(err)
	}
	for locale, started := range map[string]string{"ja": "now() - interval '2 hours'", "ko": "now()"} {
		if _, err = db.Exec(context.Background(), "INSERT INTO novel_translation_versions(novel_id,locale,source_revision,status,total_items,started_at) VALUES($1,$2,$3,'running',1,"+started+")", novel.ID, locale, novel.SourceRevision); err != nil {
			t.Fatal(err)
		}
	}
	service := NewTranslationService(db, disabledTextTranslator{}, 1)
	service.Close()
	var expiredStatus, freshStatus string
	if err = db.QueryRow(context.Background(), "SELECT status FROM novel_translation_versions WHERE novel_id=$1 AND locale='ja'", novel.ID).Scan(&expiredStatus); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(context.Background(), "SELECT status FROM novel_translation_versions WHERE novel_id=$1 AND locale='ko'", novel.ID).Scan(&freshStatus); err != nil {
		t.Fatal(err)
	}
	if expiredStatus != "queued" || freshStatus != "running" {
		t.Fatalf("recovery statuses expired=%s fresh=%s", expiredStatus, freshStatus)
	}
	if _, err = db.Exec(context.Background(), "UPDATE novel_translation_versions SET started_at=now()-interval '2 hours' WHERE novel_id=$1 AND locale='ko'", novel.ID); err != nil {
		t.Fatal(err)
	}
	service.recoverExpired(context.Background())
	if err = db.QueryRow(context.Background(), "SELECT status FROM novel_translation_versions WHERE novel_id=$1 AND locale='ko'", novel.ID).Scan(&freshStatus); err != nil || freshStatus != "queued" {
		t.Fatalf("periodic recovery status=%s err=%v", freshStatus, err)
	}
}

func TestTranslationServiceStartupDoesNotBlockOnQueuedBacklog(t *testing.T) {
	db := translationTestDB(t)
	if _, err := db.Exec(context.Background(), `
INSERT INTO novels(title,slug,excerpt,published_at)
SELECT 'Story '||n,'backlog-story-'||n,'Excerpt','2026-09-22' FROM generate_series(1,140) n;
INSERT INTO novel_translation_versions(novel_id,locale,source_revision,total_items)
SELECT id,'ja',source_revision,1 FROM novels`); err != nil {
		t.Fatal(err)
	}
	release := make(chan struct{})
	returned := make(chan *TranslationService, 1)
	go func() { returned <- NewTranslationService(db, blockingTextTranslator{release: release}, 2) }()
	select {
	case service := <-returned:
		close(release)
		service.Close()
	case <-time.After(300 * time.Millisecond):
		close(release)
		service := <-returned
		service.Close()
		t.Fatal("service startup blocked while the in-memory job queue was full")
	}
}
