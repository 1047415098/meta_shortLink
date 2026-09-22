package audionovel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const audioNovelColumns = "id,title,slug,category,excerpt,body_markdown,cover_path,audio_path,audio_duration,audio_size_bytes,to_char(published_at,'YYYY-MM-DD'),enabled,featured,created_at,updated_at,deleted_at"
const audioNovelSummaryColumns = "id,title,slug,category,excerpt,'' AS body_markdown,cover_path,audio_path,audio_duration,audio_size_bytes,to_char(published_at,'YYYY-MM-DD'),enabled,featured,created_at,updated_at,deleted_at"

type Repository struct{ DB *pgxpool.Pool }

func scanAudioNovel(row pgx.Row) (AudioNovel, error) {
	var item AudioNovel
	err := row.Scan(&item.ID, &item.Title, &item.Slug, &item.Category, &item.Excerpt, &item.BodyMarkdown, &item.CoverPath, &item.AudioPath, &item.AudioDuration, &item.AudioSizeBytes, &item.PublishedAt, &item.Enabled, &item.Featured, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt)
	return item, err
}

func (r Repository) ListAdmin(ctx context.Context, filter ListFilter) (AudioNovelList, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 || filter.PageSize > 100 {
		filter.PageSize = 20
	}
	where := []string{"deleted_at IS NULL"}
	args := []any{}
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, "%"+query+"%")
		where = append(where, fmt.Sprintf("(title ILIKE $%d OR slug ILIKE $%d OR category ILIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.Status == "enabled" || filter.Status == "disabled" {
		args = append(args, filter.Status == "enabled")
		where = append(where, fmt.Sprintf("enabled=$%d", len(args)))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := r.DB.QueryRow(ctx, "SELECT count(*) FROM audio_novels WHERE "+clause, args...).Scan(&total); err != nil {
		return AudioNovelList{}, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	// 列表不返回大段 Markdown，编辑页再按 ID 单独读取正文。
	rows, err := r.DB.Query(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE "+clause+fmt.Sprintf(" ORDER BY published_at DESC,id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return AudioNovelList{}, err
	}
	defer rows.Close()
	items := []AudioNovel{}
	for rows.Next() {
		item, err := scanAudioNovel(rows)
		if err != nil {
			return AudioNovelList{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return AudioNovelList{}, err
	}
	pages := int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	return AudioNovelList{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total, Pages: pages}, nil
}

func (r Repository) ByID(ctx context.Context, id int64) (AudioNovel, error) {
	return scanAudioNovel(r.DB.QueryRow(ctx, "SELECT "+audioNovelColumns+" FROM audio_novels WHERE id=$1 AND deleted_at IS NULL", id))
}

func (r Repository) Create(ctx context.Context, input AudioNovelInput, actor string) (AudioNovel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return AudioNovel{}, err
	}
	defer tx.Rollback(ctx)
	if input.Featured {
		if _, err = tx.Exec(ctx, "UPDATE audio_novels SET featured=false,updated_at=now() WHERE featured AND enabled AND deleted_at IS NULL"); err != nil {
			return AudioNovel{}, err
		}
	}
	item, err := scanAudioNovel(tx.QueryRow(ctx, "INSERT INTO audio_novels(title,slug,category,excerpt,body_markdown,cover_path,audio_path,audio_duration,audio_size_bytes,published_at,enabled,featured) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date,$11,$12) RETURNING "+audioNovelColumns, input.Title, input.Slug, input.Category, input.Excerpt, input.BodyMarkdown, input.CoverPath, input.AudioPath, input.AudioDuration, input.AudioSizeBytes, input.PublishedAt, input.Enabled, input.Featured && input.Enabled))
	if err != nil {
		return AudioNovel{}, err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.create", item); err != nil {
		return AudioNovel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) Update(ctx context.Context, id int64, input AudioNovelInput, actor string) (AudioNovel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return AudioNovel{}, err
	}
	defer tx.Rollback(ctx)
	if input.Featured && input.Enabled {
		if _, err = tx.Exec(ctx, "UPDATE audio_novels SET featured=false,updated_at=now() WHERE id<>$1 AND featured AND enabled AND deleted_at IS NULL", id); err != nil {
			return AudioNovel{}, err
		}
	}
	item, err := scanAudioNovel(tx.QueryRow(ctx, "UPDATE audio_novels SET title=$2,slug=$3,category=$4,excerpt=$5,body_markdown=$6,cover_path=$7,audio_path=$8,audio_duration=$9,audio_size_bytes=$10,published_at=$11::date,enabled=$12,featured=$13,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+audioNovelColumns, id, input.Title, input.Slug, input.Category, input.Excerpt, input.BodyMarkdown, input.CoverPath, input.AudioPath, input.AudioDuration, input.AudioSizeBytes, input.PublishedAt, input.Enabled, input.Featured && input.Enabled))
	if err != nil {
		return AudioNovel{}, err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.update", item); err != nil {
		return AudioNovel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) SetEnabled(ctx context.Context, id int64, enabled bool, actor string) (AudioNovel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return AudioNovel{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanAudioNovel(tx.QueryRow(ctx, "UPDATE audio_novels SET enabled=$2,featured=CASE WHEN $2 THEN featured ELSE false END,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+audioNovelColumns, id, enabled))
	if err != nil {
		return AudioNovel{}, err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.status", item); err != nil {
		return AudioNovel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) SetFeatured(ctx context.Context, id int64, featured bool, actor string) (AudioNovel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return AudioNovel{}, err
	}
	defer tx.Rollback(ctx)
	// 事务级锁让两个并发推荐请求按顺序执行，最终只保留一个推荐。
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(73261902)"); err != nil {
		return AudioNovel{}, err
	}
	if featured {
		if _, err = tx.Exec(ctx, "UPDATE audio_novels SET featured=false,updated_at=now() WHERE featured AND enabled AND deleted_at IS NULL"); err != nil {
			return AudioNovel{}, err
		}
	}
	item, err := scanAudioNovel(tx.QueryRow(ctx, "UPDATE audio_novels SET featured=$2,updated_at=now() WHERE id=$1 AND enabled AND deleted_at IS NULL RETURNING "+audioNovelColumns, id, featured))
	if err != nil {
		return AudioNovel{}, err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.featured", item); err != nil {
		return AudioNovel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) ClearAudio(ctx context.Context, id int64, actor string) (string, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	var previousPath string
	if err = tx.QueryRow(ctx, "SELECT audio_path FROM audio_novels WHERE id=$1 AND deleted_at IS NULL FOR UPDATE", id).Scan(&previousPath); err != nil {
		return "", err
	}
	item, err := scanAudioNovel(tx.QueryRow(ctx, "UPDATE audio_novels SET audio_path='',audio_duration='',audio_size_bytes=0,updated_at=now() WHERE id=$1 RETURNING "+audioNovelColumns, id))
	if err != nil {
		return "", err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.audio_remove", item); err != nil {
		return "", err
	}
	return previousPath, tx.Commit(ctx)
}

func (r Repository) SoftDelete(ctx context.Context, id int64, actor string) (string, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)
	item, err := scanAudioNovel(tx.QueryRow(ctx, "UPDATE audio_novels SET deleted_at=now(),featured=false,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+audioNovelColumns, id))
	if err != nil {
		return "", err
	}
	if err = auditAudioNovel(ctx, tx, actor, "audio_novel.delete", item); err != nil {
		return "", err
	}
	return item.AudioPath, tx.Commit(ctx)
}

func (r Repository) PublicHome(ctx context.Context) (AudioNovel, error) {
	return scanAudioNovel(r.DB.QueryRow(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE enabled AND deleted_at IS NULL ORDER BY featured DESC,published_at DESC,id DESC LIMIT 1"))
}

func (r Repository) PublicList(ctx context.Context, page, pageSize int) (AudioNovelList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 24 {
		pageSize = 6
	}
	var total int64
	if err := r.DB.QueryRow(ctx, "SELECT count(*) FROM audio_novels WHERE enabled AND deleted_at IS NULL").Scan(&total); err != nil {
		return AudioNovelList{}, err
	}
	rows, err := r.DB.Query(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE enabled AND deleted_at IS NULL ORDER BY published_at DESC,id DESC LIMIT $1 OFFSET $2", pageSize, (page-1)*pageSize)
	if err != nil {
		return AudioNovelList{}, err
	}
	defer rows.Close()
	items := []AudioNovel{}
	for rows.Next() {
		item, e := scanAudioNovel(rows)
		if e != nil {
			return AudioNovelList{}, e
		}
		item.BodyMarkdown = ""
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return AudioNovelList{}, err
	}
	return AudioNovelList{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (r Repository) PublicBySlug(ctx context.Context, slug string) (AudioNovel, error) {
	item, err := scanAudioNovel(r.DB.QueryRow(ctx, "SELECT "+audioNovelColumns+" FROM audio_novels WHERE slug=$1 AND enabled AND deleted_at IS NULL", slug))
	if err == nil {
		item.BodyHTML = RenderMarkdown(item.BodyMarkdown)
		item.BodyMarkdown = ""
	}
	return item, err
}

func (r Repository) PublicAudioList(ctx context.Context, page, pageSize int) (AudioNovelList, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 24 {
		pageSize = 6
	}
	where := "enabled AND deleted_at IS NULL AND audio_path <> ''"
	var total int64
	if err := r.DB.QueryRow(ctx, "SELECT count(*) FROM audio_novels WHERE "+where).Scan(&total); err != nil {
		return AudioNovelList{}, err
	}
	rows, err := r.DB.Query(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE "+where+" ORDER BY published_at DESC,id DESC LIMIT $1 OFFSET $2", pageSize, (page-1)*pageSize)
	if err != nil {
		return AudioNovelList{}, err
	}
	defer rows.Close()
	items := []AudioNovel{}
	for rows.Next() {
		item, scanErr := scanAudioNovel(rows)
		if scanErr != nil {
			return AudioNovelList{}, scanErr
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return AudioNovelList{}, err
	}
	return AudioNovelList{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: int((total + int64(pageSize) - 1) / int64(pageSize))}, nil
}

func (r Repository) PublicAudioBySlug(ctx context.Context, slug string) (AudioNovel, error) {
	// Podcast 详情只返回展示与播放元数据，不读取或渲染正文。
	return scanAudioNovel(r.DB.QueryRow(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE slug=$1 AND enabled AND deleted_at IS NULL AND audio_path <> ''", slug))
}

func (r Repository) Related(ctx context.Context, item AudioNovel, limit int) ([]AudioNovel, error) {
	rows, err := r.DB.Query(ctx, "SELECT "+audioNovelSummaryColumns+" FROM audio_novels WHERE id<>$1 AND enabled AND deleted_at IS NULL ORDER BY (category=$2) DESC,published_at DESC,id DESC LIMIT $3", item.ID, item.Category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AudioNovel{}
	for rows.Next() {
		next, e := scanAudioNovel(rows)
		if e != nil {
			return nil, e
		}
		next.BodyMarkdown = ""
		out = append(out, next)
	}
	return out, rows.Err()
}

func auditAudioNovel(ctx context.Context, tx pgx.Tx, actor, action string, item AudioNovel) error {
	detail, _ := json.Marshal(map[string]any{"id": item.ID, "slug": item.Slug})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
