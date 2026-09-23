package novel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrTranslationNotConfigured = errors.New("翻译服务尚未配置")

// 先以服务商文档范围内的 450 字节发送，仅在特定段落被拒绝时自适应缩小。
const translationChunkBytes = 450

const (
	// 多个长篇任务共享限速时会超过 30 分钟，保留充足时间使整本原子发布。
	translationJobTimeout    = 90 * time.Minute
	translationRecoveryDelay = 95 * time.Minute
)

type TextTranslator interface {
	Configured() bool
	Translate(context.Context, string, string) (string, error)
}

type TranslationStatus struct {
	Locale                  string `json:"locale"`
	Name                    string `json:"name"`
	Status                  string `json:"status"`
	Enabled                 bool   `json:"enabled"`
	Published               bool   `json:"published"`
	SourceRevision          int64  `json:"source_revision"`
	PublishedSourceRevision int64  `json:"published_source_revision"`
	TotalItems              int    `json:"total_items"`
	CompletedItems          int    `json:"completed_items"`
	ErrorMessage            string `json:"error_message"`
	UpdatedAt               string `json:"updated_at,omitempty"`
}

type translationRecord struct {
	ID, SourceRevision    int64
	Locale, Status, Error string
	Enabled               bool
	Total, Completed      int
	CreatedAt, FinishedAt time.Time
	PublishedAt           *time.Time
}

type TranslationService struct {
	db         *pgxpool.Pool
	translator TextTranslator
	jobs       chan int64
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	pending    sync.Map
}

func NormalizeTargetLocales(locales []string) ([]string, error) {
	if len(locales) == 0 {
		return nil, errors.New("请至少选择一种目标语言")
	}
	seen, result := map[string]bool{}, []string{}
	for _, locale := range locales {
		locale = strings.TrimSpace(locale)
		if _, ok := TranslationType(locale); !ok {
			return nil, fmt.Errorf("不支持的目标语言 %s", locale)
		}
		if !seen[locale] {
			seen[locale] = true
			result = append(result, locale)
		}
	}
	return result, nil
}

func NewTranslationService(db *pgxpool.Pool, translator TextTranslator, workers int) *TranslationService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &TranslationService{db: db, translator: translator, jobs: make(chan int64, 128), ctx: ctx, cancel: cancel}
	if service.Configured() {
		if workers < 1 {
			workers = 1
		}
		if workers > 8 {
			// 请求起点由 TranslationClient 全局限速，worker 只用于覆盖上游响应和重试等待时间。
			workers = 8
		}
		for i := 0; i < workers; i++ {
			service.wg.Add(1)
			go service.worker()
		}
	}
	service.recoverExpired(ctx)
	service.wg.Add(1)
	go service.recoveryLoop()
	return service
}

func (service *TranslationService) Close() {
	if service == nil {
		return
	}
	service.cancel()
	service.wg.Wait()
}

func (service *TranslationService) Configured() bool {
	return service != nil && service.translator != nil && service.translator.Configured()
}

func (service *TranslationService) enqueue(id int64) bool {
	if _, loaded := service.pending.LoadOrStore(id, struct{}{}); loaded {
		return true
	}
	select {
	case service.jobs <- id:
		return true
	case <-service.ctx.Done():
		service.pending.Delete(id)
		return false
	default:
		// 队列满时任务仍保留在数据库，后台调度器下一轮继续补队。
		service.pending.Delete(id)
		return false
	}
}

func (service *TranslationService) worker() {
	defer service.wg.Done()
	for {
		if service.ctx.Err() != nil {
			return
		}
		select {
		case <-service.ctx.Done():
			return
		case id := <-service.jobs:
			service.pending.Delete(id)
			if service.ctx.Err() != nil {
				return
			}
			service.process(id)
		}
	}
}

func (service *TranslationService) recoveryLoop() {
	defer service.wg.Done()
	service.dispatchQueued(service.ctx)
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-service.ctx.Done():
			return
		case <-ticker.C:
			service.recoverExpired(service.ctx)
			service.dispatchQueued(service.ctx)
		}
	}
}

func (service *TranslationService) dispatchQueued(ctx context.Context) {
	if !service.Configured() {
		return
	}
	rows, err := service.db.Query(ctx, "SELECT id FROM novel_translation_versions WHERE status='queued' ORDER BY created_at,id")
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if rows.Scan(&id) != nil || !service.enqueue(id) {
			return
		}
	}
}

func (service *TranslationService) recoverExpired(ctx context.Context) {
	// 额外留 5 分钟，避免和仍在退出的旧实例争抢同一任务。
	rows, err := service.db.Query(ctx, "UPDATE novel_translation_versions SET status='queued',started_at=NULL WHERE status='running' AND (started_at IS NULL OR started_at < now()-make_interval(secs => $1)) RETURNING id", translationRecoveryDelay.Seconds())
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil && service.Configured() {
			_ = service.enqueue(id)
		}
	}
}

func (service *TranslationService) Queue(ctx context.Context, novelID int64, requested []string, actor string) ([]TranslationStatus, error) {
	if !service.Configured() {
		return nil, ErrTranslationNotConfigured
	}
	locales, err := NormalizeTargetLocales(requested)
	if err != nil {
		return nil, err
	}
	tx, err := service.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	var revision int64
	var chapters int
	if err = tx.QueryRow(ctx, `SELECT source_revision,(SELECT count(*) FROM novel_chapters WHERE novel_id=novels.id AND enabled AND deleted_at IS NULL) FROM novels WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, novelID).Scan(&revision, &chapters); err != nil {
		return nil, err
	}
	ids := []int64{}
	for _, locale := range locales {
		var existing bool
		if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM novel_translation_versions WHERE novel_id=$1 AND locale=$2 AND status IN ('queued','running'))", novelID, locale).Scan(&existing); err != nil {
			return nil, err
		}
		if existing {
			continue
		}
		var id int64
		if err = tx.QueryRow(ctx, "INSERT INTO novel_translation_versions(novel_id,locale,source_revision,total_items) VALUES($1,$2,$3,$4) RETURNING id", novelID, locale, revision, chapters+1).Scan(&id); err != nil {
			return nil, err
		}
		if err = audit(ctx, tx, actor, "novel.translation.generate", id, fmt.Sprintf("%d:%s", novelID, locale)); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	for _, id := range ids {
		service.enqueue(id)
	}
	return service.List(ctx, novelID)
}

func (service *TranslationService) List(ctx context.Context, novelID int64) ([]TranslationStatus, error) {
	var sourceRevision int64
	if err := service.db.QueryRow(ctx, "SELECT source_revision FROM novels WHERE id=$1 AND deleted_at IS NULL", novelID).Scan(&sourceRevision); err != nil {
		return nil, err
	}
	rows, err := service.db.Query(ctx, `SELECT id,source_revision,locale,status,enabled,total_items,completed_items,error_message,created_at,COALESCE(finished_at,created_at),published_at FROM novel_translation_versions WHERE novel_id=$1 ORDER BY id`, novelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	records := map[string][]translationRecord{}
	for rows.Next() {
		var item translationRecord
		if err = rows.Scan(&item.ID, &item.SourceRevision, &item.Locale, &item.Status, &item.Enabled, &item.Total, &item.Completed, &item.Error, &item.CreatedAt, &item.FinishedAt, &item.PublishedAt); err != nil {
			return nil, err
		}
		records[item.Locale] = append(records[item.Locale], item)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	result := make([]TranslationStatus, 0, len(TargetLocales))
	for _, option := range TargetLocales {
		status := TranslationStatus{Locale: option.Code, Name: option.Name, Status: "not_generated", Enabled: true, SourceRevision: sourceRevision}
		var published *translationRecord
		items := records[option.Code]
		for i := range items {
			if items[i].Status == "published" {
				published = &items[i]
			}
		}
		if published != nil {
			status.Published, status.Enabled, status.PublishedSourceRevision = true, published.Enabled, published.SourceRevision
			status.Status = "published"
			status.UpdatedAt = published.FinishedAt.Format(time.RFC3339)
			if published.SourceRevision < sourceRevision {
				status.Status = "stale"
			}
		}
		if len(items) > 0 {
			latest := items[len(items)-1]
			if latest.Status == "queued" || latest.Status == "running" || latest.Status == "failed" {
				status.Status, status.TotalItems, status.CompletedItems, status.ErrorMessage = latest.Status, latest.Total, latest.Completed, latest.Error
				status.UpdatedAt = latest.FinishedAt.Format(time.RFC3339)
			} else if latest.Status == "published" {
				status.TotalItems, status.CompletedItems = latest.Total, latest.Completed
			}
		}
		result = append(result, status)
	}
	return result, nil
}

func (service *TranslationService) SetEnabled(ctx context.Context, novelID int64, locale string, enabled bool, actor string) error {
	if _, ok := TranslationType(locale); !ok {
		return errors.New("不支持的目标语言")
	}
	tx, err := service.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var id int64
	if err = tx.QueryRow(ctx, "UPDATE novel_translation_versions SET enabled=$3 WHERE novel_id=$1 AND locale=$2 AND status='published' RETURNING id", novelID, locale, enabled).Scan(&id); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "novel.translation.status", id, fmt.Sprintf("%d:%s:%t", novelID, locale, enabled)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *TranslationService) process(id int64) {
	ctx, cancel := context.WithTimeout(service.ctx, translationJobTimeout)
	defer cancel()
	job, chapters, err := service.claim(ctx, id)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			service.failQueued(id, err)
		}
		return
	}
	title, err := service.translateText(ctx, job.Locale, job.Title)
	if err == nil {
		job.TranslatedTitle = title
		job.TranslatedAuthor, err = service.translateText(ctx, job.Locale, job.Author)
	}
	if err == nil {
		job.TranslatedCategory, err = service.translateText(ctx, job.Locale, job.Category)
	}
	if err == nil {
		job.TranslatedExcerpt, err = service.translateText(ctx, job.Locale, job.Excerpt)
	}
	if err != nil {
		service.fail(id, err)
		return
	}
	if _, err = service.db.Exec(ctx, "UPDATE novel_translation_versions SET title=$2,author=$3,category=$4,excerpt=$5,completed_items=1 WHERE id=$1 AND status='running'", id, job.TranslatedTitle, job.TranslatedAuthor, job.TranslatedCategory, job.TranslatedExcerpt); err != nil {
		service.fail(id, err)
		return
	}
	for _, chapter := range chapters {
		translatedTitle, translateErr := service.translateText(ctx, job.Locale, chapter.Title)
		if translateErr != nil {
			service.fail(id, translateErr)
			return
		}
		translatedBody, translateErr := service.translateMarkdown(ctx, job.Locale, chapter.BodyMarkdown)
		if translateErr != nil {
			service.fail(id, translateErr)
			return
		}
		if _, err = service.db.Exec(ctx, `INSERT INTO novel_chapter_translations(version_id,chapter_id,chapter_number,title,body_markdown) VALUES($1,$2,$3,$4,$5) ON CONFLICT(version_id,chapter_id) DO UPDATE SET chapter_number=excluded.chapter_number,title=excluded.title,body_markdown=excluded.body_markdown`, id, chapter.ID, chapter.ChapterNumber, translatedTitle, translatedBody); err != nil {
			service.fail(id, err)
			return
		}
		if _, err = service.db.Exec(ctx, "UPDATE novel_translation_versions SET completed_items=completed_items+1 WHERE id=$1 AND status='running'", id); err != nil {
			service.fail(id, err)
			return
		}
	}
	if err = service.publish(ctx, id, job.NovelID, job.SourceRevision); err != nil {
		service.fail(id, err)
	}
}

type translationJob struct {
	NovelID, SourceRevision                                                  int64
	Locale, Title, Author, Category, Excerpt                                 string
	TranslatedTitle, TranslatedAuthor, TranslatedCategory, TranslatedExcerpt string
}

func (service *TranslationService) claim(ctx context.Context, id int64) (translationJob, []Chapter, error) {
	tx, err := service.db.Begin(ctx)
	if err != nil {
		return translationJob{}, nil, err
	}
	defer tx.Rollback(ctx)
	var job translationJob
	err = tx.QueryRow(ctx, `UPDATE novel_translation_versions v SET status='running',started_at=now(),finished_at=NULL,error_message='',completed_items=0 WHERE v.id=$1 AND v.status='queued' RETURNING v.novel_id,v.source_revision,v.locale`, id).Scan(&job.NovelID, &job.SourceRevision, &job.Locale)
	if err != nil {
		return translationJob{}, nil, err
	}
	if _, err = tx.Exec(ctx, "DELETE FROM novel_chapter_translations WHERE version_id=$1", id); err != nil {
		return translationJob{}, nil, err
	}
	var currentRevision int64
	if err = tx.QueryRow(ctx, "SELECT title,author,category,excerpt,source_revision FROM novels WHERE id=$1 AND deleted_at IS NULL", job.NovelID).Scan(&job.Title, &job.Author, &job.Category, &job.Excerpt, &currentRevision); err != nil {
		return translationJob{}, nil, err
	}
	if currentRevision != job.SourceRevision {
		return translationJob{}, nil, errors.New("英文原文已更新，请重新生成")
	}
	rows, err := tx.Query(ctx, "SELECT "+chapterColumns+" FROM novel_chapters WHERE novel_id=$1 AND enabled AND deleted_at IS NULL ORDER BY chapter_number", job.NovelID)
	if err != nil {
		return translationJob{}, nil, err
	}
	chapters := []Chapter{}
	for rows.Next() {
		chapter, scanErr := scanChapter(rows)
		if scanErr != nil {
			rows.Close()
			return translationJob{}, nil, scanErr
		}
		chapters = append(chapters, chapter)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return translationJob{}, nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return translationJob{}, nil, err
	}
	return job, chapters, nil
}

func (service *TranslationService) translateText(ctx context.Context, locale, source string) (string, error) {
	if strings.TrimSpace(source) == "" {
		return source, nil
	}
	var translateChunk func(string) (string, error)
	translateChunk = func(chunk string) (string, error) {
		result, err := service.translator.Translate(ctx, locale, chunk)
		if err == nil {
			return result, nil
		}
		var providerErr translationProviderError
		if !errors.As(err, &providerErr) || !providerErr.splittable() || len([]byte(chunk)) <= 48 {
			return "", err
		}
		// APIHZ 会偶发拒绝某些复杂长句，仅对该错误递归对半切分并保持原文顺序。
		pieces := SplitTranslationText(chunk, len([]byte(chunk))/2)
		translated := strings.Builder{}
		for _, piece := range pieces {
			value, pieceErr := translateChunk(piece)
			if pieceErr != nil {
				return "", pieceErr
			}
			translated.WriteString(value)
		}
		return translated.String(), nil
	}
	translated := strings.Builder{}
	for _, chunk := range SplitTranslationText(source, translationChunkBytes) {
		result, err := translateChunk(chunk)
		if err != nil {
			return "", err
		}
		translated.WriteString(result)
	}
	return translated.String(), nil
}

func (service *TranslationService) translateMarkdown(ctx context.Context, locale, source string) (string, error) {
	var output, prose strings.Builder
	var fenceCharacter byte
	fenceLength := 0
	flush := func() error {
		if prose.Len() == 0 {
			return nil
		}
		translated, err := service.translateText(ctx, locale, prose.String())
		if err != nil {
			return err
		}
		output.WriteString(translated)
		prose.Reset()
		return nil
	}
	for _, line := range strings.SplitAfter(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if fenceLength > 0 {
			output.WriteString(line)
			if closesMarkdownFence(trimmed, fenceCharacter, fenceLength) {
				fenceCharacter, fenceLength = 0, 0
			}
			continue
		}
		marker, markerLength := markdownFence(trimmed)
		standaloneURL := (strings.HasPrefix(trimmed, "https://") || strings.HasPrefix(trimmed, "http://")) && !strings.ContainsAny(trimmed, " \t")
		if markerLength > 0 || standaloneURL || trimmed == "" {
			if err := flush(); err != nil {
				return "", err
			}
			output.WriteString(line)
			fenceCharacter, fenceLength = marker, markerLength
			continue
		}
		prose.WriteString(line)
	}
	if err := flush(); err != nil {
		return "", err
	}
	return output.String(), nil
}

func markdownFence(line string) (byte, int) {
	if len(line) < 3 || (line[0] != '`' && line[0] != '~') {
		return 0, 0
	}
	length := 1
	for length < len(line) && line[length] == line[0] {
		length++
	}
	if length < 3 {
		return 0, 0
	}
	return line[0], length
}

func closesMarkdownFence(line string, character byte, minimum int) bool {
	if len(line) < minimum {
		return false
	}
	for index := 0; index < len(line); index++ {
		if line[index] != character {
			return false
		}
	}
	return true
}

func (service *TranslationService) publish(ctx context.Context, id, novelID, revision int64) error {
	tx, err := service.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var currentRevision int64
	if err = tx.QueryRow(ctx, "SELECT source_revision FROM novels WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", novelID).Scan(&currentRevision); err != nil {
		return err
	}
	if currentRevision != revision {
		return errors.New("英文原文已更新，请重新生成")
	}
	var locale string
	var completed, total int
	if err = tx.QueryRow(ctx, "SELECT locale,completed_items,total_items FROM novel_translation_versions WHERE id=$1 AND status='running' FOR UPDATE", id).Scan(&locale, &completed, &total); err != nil {
		return err
	}
	if completed != total {
		return errors.New("翻译内容尚未全部完成")
	}
	// 先下线旧版本再发布新版本，事务提交前访客始终只能看到完整旧译文。
	if _, err = tx.Exec(ctx, "UPDATE novel_translation_versions SET status='superseded',enabled=false WHERE novel_id=$1 AND locale=$2 AND status='published'", novelID, locale); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE novel_translation_versions SET status='published',enabled=true,finished_at=now(),published_at=now(),error_message='' WHERE id=$1", id); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (service *TranslationService) fail(id int64, err error) {
	if service.ctx.Err() != nil {
		// 正常停机把本实例已领取的任务放回队列，让新实例立即续跑。
		_, _ = service.db.Exec(context.Background(), "UPDATE novel_translation_versions SET status='queued',started_at=NULL WHERE id=$1 AND status='running'", id)
		return
	}
	message := translationFailureMessage(err)
	_, _ = service.db.Exec(context.Background(), "UPDATE novel_translation_versions SET status='failed',error_message=$2,finished_at=now() WHERE id=$1 AND status='running'", id, message)
}

func (service *TranslationService) failQueued(id int64, err error) {
	if service.ctx.Err() != nil {
		return
	}
	message := translationFailureMessage(err)
	_, _ = service.db.Exec(context.Background(), "UPDATE novel_translation_versions SET status='failed',error_message=$2,finished_at=now() WHERE id=$1 AND status='queued'", id, message)
}

func translationFailureMessage(err error) string {
	message := "翻译失败"
	if err != nil {
		message = err.Error()
	}
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500])
	}
	return message
}
