package novel

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const novelColumns = "n.id,n.title,n.slug,n.author,n.category,n.excerpt,n.cover_path,to_char(n.published_at,'YYYY-MM-DD'),n.enabled,n.featured,n.sort_order,(SELECT count(*) FROM novel_chapters c WHERE c.novel_id=n.id AND c.deleted_at IS NULL),n.created_at,n.updated_at,n.deleted_at"
const novelReturnColumns = "id,title,slug,author,category,excerpt,cover_path,to_char(published_at,'YYYY-MM-DD'),enabled,featured,sort_order,0,created_at,updated_at,deleted_at"
const chapterColumns = "id,novel_id,chapter_number,title,body_markdown,enabled,created_at,updated_at,deleted_at"

type Repository struct{ DB *pgxpool.Pool }

func scanNovel(row pgx.Row) (Novel, error) {
	var item Novel
	err := row.Scan(&item.ID, &item.Title, &item.Slug, &item.Author, &item.Category, &item.Excerpt, &item.CoverPath, &item.PublishedAt, &item.Enabled, &item.Featured, &item.SortOrder, &item.ChapterCount, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt)
	return item, err
}

func scanChapter(row pgx.Row) (Chapter, error) {
	var item Chapter
	err := row.Scan(&item.ID, &item.NovelID, &item.ChapterNumber, &item.Title, &item.BodyMarkdown, &item.Enabled, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt)
	return item, err
}

func normalizedPage(page, pageSize, fallback int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = fallback
	}
	return page, pageSize
}

func (r Repository) ListAdmin(ctx context.Context, filter ListFilter) (NovelList, error) {
	filter.Page, filter.PageSize = normalizedPage(filter.Page, filter.PageSize, 20)
	where, args := []string{"n.deleted_at IS NULL"}, []any{}
	if query := strings.TrimSpace(filter.Query); query != "" {
		args = append(args, "%"+query+"%")
		where = append(where, fmt.Sprintf("(n.title ILIKE $%d OR n.slug ILIKE $%d OR n.author ILIKE $%d)", len(args), len(args), len(args)))
	}
	if filter.Status == "enabled" || filter.Status == "disabled" {
		args = append(args, filter.Status == "enabled")
		where = append(where, fmt.Sprintf("n.enabled=$%d", len(args)))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := r.DB.QueryRow(ctx, "SELECT count(*) FROM novels n WHERE "+clause, args...).Scan(&total); err != nil {
		return NovelList{}, err
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	rows, err := r.DB.Query(ctx, "SELECT "+novelColumns+" FROM novels n WHERE "+clause+fmt.Sprintf(" ORDER BY n.sort_order DESC,n.published_at DESC,n.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return NovelList{}, err
	}
	defer rows.Close()
	items := []Novel{}
	for rows.Next() {
		item, e := scanNovel(rows)
		if e != nil {
			return NovelList{}, e
		}
		items = append(items, item)
	}
	if err = rows.Err(); err != nil {
		return NovelList{}, err
	}
	return NovelList{Items: items, Page: filter.Page, PageSize: filter.PageSize, Total: total, Pages: int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))}, nil
}

func (r Repository) ByID(ctx context.Context, id int64) (Novel, error) {
	return scanNovel(r.DB.QueryRow(ctx, "SELECT "+novelColumns+" FROM novels n WHERE n.id=$1 AND n.deleted_at IS NULL", id))
}

func (r Repository) Create(ctx context.Context, input NovelInput, actor string) (Novel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Novel{}, err
	}
	defer tx.Rollback(ctx)
	if input.Featured && input.Enabled {
		if _, err = tx.Exec(ctx, "UPDATE novels SET featured=false,updated_at=now() WHERE featured AND enabled AND deleted_at IS NULL"); err != nil {
			return Novel{}, err
		}
	}
	item, err := scanNovel(tx.QueryRow(ctx, "INSERT INTO novels(title,slug,author,category,excerpt,cover_path,published_at,enabled,featured,sort_order) VALUES($1,$2,$3,$4,$5,$6,$7::date,$8,$9,$10) RETURNING "+novelReturnColumns, strings.TrimSpace(input.Title), input.Slug, strings.TrimSpace(input.Author), strings.TrimSpace(input.Category), strings.TrimSpace(input.Excerpt), input.CoverPath, input.PublishedAt, input.Enabled, input.Featured && input.Enabled, input.SortOrder))
	if err != nil {
		return Novel{}, err
	}
	if err = audit(ctx, tx, actor, "novel.create", item.ID, item.Slug); err != nil {
		return Novel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) Update(ctx context.Context, id int64, input NovelInput, actor string) (Novel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Novel{}, err
	}
	defer tx.Rollback(ctx)
	if input.Featured && input.Enabled {
		if _, err = tx.Exec(ctx, "UPDATE novels SET featured=false,updated_at=now() WHERE id<>$1 AND featured AND enabled AND deleted_at IS NULL", id); err != nil {
			return Novel{}, err
		}
	}
	item, err := scanNovel(tx.QueryRow(ctx, "UPDATE novels SET title=$2,slug=$3,author=$4,category=$5,excerpt=$6,cover_path=$7,published_at=$8::date,enabled=$9,featured=$10,sort_order=$11,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+novelReturnColumns, id, strings.TrimSpace(input.Title), input.Slug, strings.TrimSpace(input.Author), strings.TrimSpace(input.Category), strings.TrimSpace(input.Excerpt), input.CoverPath, input.PublishedAt, input.Enabled, input.Featured && input.Enabled, input.SortOrder))
	if err != nil {
		return Novel{}, err
	}
	if err = audit(ctx, tx, actor, "novel.update", item.ID, item.Slug); err != nil {
		return Novel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) SetEnabled(ctx context.Context, id int64, enabled bool, actor string) (Novel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Novel{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanNovel(tx.QueryRow(ctx, "UPDATE novels SET enabled=$2,featured=CASE WHEN $2 THEN featured ELSE false END,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+novelReturnColumns, id, enabled))
	if err != nil {
		return Novel{}, err
	}
	if err = audit(ctx, tx, actor, "novel.status", item.ID, item.Slug); err != nil {
		return Novel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) SetFeatured(ctx context.Context, id int64, featured bool, actor string) (Novel, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Novel{}, err
	}
	defer tx.Rollback(ctx)
	// 同一事务锁串行化推荐切换，数据库唯一索引再提供最终保护。
	if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(91827364)"); err != nil {
		return Novel{}, err
	}
	if featured {
		if _, err = tx.Exec(ctx, "UPDATE novels SET featured=false,updated_at=now() WHERE featured AND enabled AND deleted_at IS NULL"); err != nil {
			return Novel{}, err
		}
	}
	item, err := scanNovel(tx.QueryRow(ctx, "UPDATE novels SET featured=$2,updated_at=now() WHERE id=$1 AND enabled AND deleted_at IS NULL RETURNING "+novelReturnColumns, id, featured))
	if err != nil {
		return Novel{}, err
	}
	if err = audit(ctx, tx, actor, "novel.featured", item.ID, item.Slug); err != nil {
		return Novel{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) SoftDelete(ctx context.Context, id int64, actor string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanNovel(tx.QueryRow(ctx, "UPDATE novels SET deleted_at=now(),enabled=false,featured=false,updated_at=now() WHERE id=$1 AND deleted_at IS NULL RETURNING "+novelReturnColumns, id))
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "UPDATE novel_chapters SET deleted_at=now(),enabled=false,updated_at=now() WHERE novel_id=$1 AND deleted_at IS NULL", id); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "novel.delete", item.ID, item.Slug); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) ListChapters(ctx context.Context, novelID int64, publicOnly bool) ([]Chapter, error) {
	clause := "novel_id=$1 AND deleted_at IS NULL"
	if publicOnly {
		clause += " AND enabled"
	}
	rows, err := r.DB.Query(ctx, "SELECT "+chapterColumns+" FROM novel_chapters WHERE "+clause+" ORDER BY chapter_number", novelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Chapter{}
	for rows.Next() {
		item, e := scanChapter(rows)
		if e != nil {
			return nil, e
		}
		if publicOnly {
			item.BodyMarkdown = ""
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateChapter(ctx context.Context, novelID int64, input ChapterInput, actor string) (Chapter, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Chapter{}, err
	}
	defer tx.Rollback(ctx)
	var exists bool
	if err = tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM novels WHERE id=$1 AND deleted_at IS NULL)", novelID).Scan(&exists); err != nil || !exists {
		if err == nil {
			err = pgx.ErrNoRows
		}
		return Chapter{}, err
	}
	item, err := scanChapter(tx.QueryRow(ctx, "INSERT INTO novel_chapters(novel_id,chapter_number,title,body_markdown,enabled) VALUES($1,$2,$3,$4,$5) RETURNING "+chapterColumns, novelID, input.ChapterNumber, strings.TrimSpace(input.Title), strings.TrimSpace(input.BodyMarkdown), input.Enabled))
	if err != nil {
		return Chapter{}, err
	}
	if err = audit(ctx, tx, actor, "novel.chapter.create", item.ID, fmt.Sprint(novelID)); err != nil {
		return Chapter{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) UpdateChapter(ctx context.Context, novelID, chapterID int64, input ChapterInput, actor string) (Chapter, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Chapter{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanChapter(tx.QueryRow(ctx, "UPDATE novel_chapters SET chapter_number=$3,title=$4,body_markdown=$5,enabled=$6,updated_at=now() WHERE id=$1 AND novel_id=$2 AND deleted_at IS NULL RETURNING "+chapterColumns, chapterID, novelID, input.ChapterNumber, strings.TrimSpace(input.Title), strings.TrimSpace(input.BodyMarkdown), input.Enabled))
	if err != nil {
		return Chapter{}, err
	}
	if err = audit(ctx, tx, actor, "novel.chapter.update", item.ID, fmt.Sprint(novelID)); err != nil {
		return Chapter{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) DeleteChapter(ctx context.Context, novelID, chapterID int64, actor string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanChapter(tx.QueryRow(ctx, "UPDATE novel_chapters SET deleted_at=now(),enabled=false,updated_at=now() WHERE id=$1 AND novel_id=$2 AND deleted_at IS NULL RETURNING "+chapterColumns, chapterID, novelID))
	if err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "novel.chapter.delete", item.ID, fmt.Sprint(novelID)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r Repository) PublicHome(ctx context.Context) (Novel, error) {
	return scanNovel(r.DB.QueryRow(ctx, "SELECT "+novelColumns+" FROM novels n WHERE n.enabled AND n.deleted_at IS NULL ORDER BY n.featured DESC,n.sort_order DESC,n.published_at DESC,n.id DESC LIMIT 1"))
}

func (r Repository) PublicList(ctx context.Context, query string, page, pageSize int) (NovelList, error) {
	page, pageSize = normalizedPage(page, pageSize, 20)
	where, args := []string{"n.enabled", "n.deleted_at IS NULL"}, []any{}
	if query = strings.TrimSpace(query); query != "" {
		args = append(args, "%"+query+"%")
		where = append(where, fmt.Sprintf("(n.title ILIKE $%d OR n.author ILIKE $%d OR n.excerpt ILIKE $%d)", len(args), len(args), len(args)))
	}
	clause := strings.Join(where, " AND ")
	var total int64
	if err := r.DB.QueryRow(ctx, "SELECT count(*) FROM novels n WHERE "+clause, args...).Scan(&total); err != nil {
		return NovelList{}, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := r.DB.Query(ctx, "SELECT "+novelColumns+" FROM novels n WHERE "+clause+fmt.Sprintf(" ORDER BY n.sort_order DESC,n.published_at DESC,n.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return NovelList{}, err
	}
	defer rows.Close()
	items := []Novel{}
	for rows.Next() {
		item, e := scanNovel(rows)
		if e != nil {
			return NovelList{}, e
		}
		items = append(items, item)
	}
	return NovelList{Items: items, Page: page, PageSize: pageSize, Total: total, Pages: int((total + int64(pageSize) - 1) / int64(pageSize))}, rows.Err()
}

func (r Repository) PublicBySlug(ctx context.Context, slug string) (Novel, error) {
	return scanNovel(r.DB.QueryRow(ctx, "SELECT "+novelColumns+" FROM novels n WHERE n.slug=$1 AND n.enabled AND n.deleted_at IS NULL", slug))
}

func (r Repository) PublicChapter(ctx context.Context, novelID int64, number int) (Chapter, error) {
	item, err := scanChapter(r.DB.QueryRow(ctx, "SELECT "+chapterColumns+" FROM novel_chapters WHERE novel_id=$1 AND chapter_number=$2 AND enabled AND deleted_at IS NULL", novelID, number))
	if err == nil {
		item.BodyHTML = RenderMarkdown(item.BodyMarkdown)
		item.BodyMarkdown = ""
	}
	return item, err
}

func (r Repository) Related(ctx context.Context, item Novel, limit int) ([]Novel, error) {
	rows, err := r.DB.Query(ctx, "SELECT "+novelColumns+" FROM novels n WHERE n.id<>$1 AND n.enabled AND n.deleted_at IS NULL ORDER BY (n.category=$2) DESC,n.sort_order DESC,n.published_at DESC,n.id DESC LIMIT $3", item.ID, item.Category, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Novel{}
	for rows.Next() {
		next, e := scanNovel(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, next)
	}
	return out, rows.Err()
}

func audit(ctx context.Context, tx pgx.Tx, actor, action string, id int64, label string) error {
	detail, _ := json.Marshal(map[string]any{"id": id, "label": label})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
