package cover

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBindingLocked = errors.New("advertising binding is locked after the first visit")
	ErrLinksNotFound = errors.New("one or more cover links do not exist")
	ErrPixelInvalid  = errors.New("advertising pixel is invalid or disabled")
)

type Repository struct{ DB *pgxpool.Pool }

const linkColumns = `l.id,l.code,l.name,l.enabled,l.product_type,l.meta_connection_id,l.meta_pixel_id,l.tiktok_pixel_id,
	COALESCE(tp.pixel_code,''),COALESCE(tp.name,''),l.ad_platform,l.attribution_mode,l.time_spent_threshold,
	(SELECT count(*) FROM click_events e WHERE e.link_id=l.id),l.created_at,l.first_visited_at`

func scanLink(row pgx.Row) (Link, error) {
	var item Link
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Enabled, &item.ProductType,
		&item.MetaConnectionID, &item.MetaPixelID, &item.TikTokPixelID, &item.TikTokPixelCode,
		&item.TikTokPixelName, &item.AdPlatform, &item.AttributionMode, &item.TimeSpentThreshold,
		&item.VisitCount, &item.CreatedAt, &item.FirstVisitedAt)
	return item, err
}

func (r Repository) List(ctx context.Context) ([]Link, error) {
	// Archived links stay in the database for reports but no longer appear in the active management list.
	rows, err := r.DB.Query(ctx, "SELECT "+linkColumns+" FROM short_links l LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.product_type='cover' AND l.archived_at IS NULL ORDER BY l.id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Link{}
	for rows.Next() {
		item, scanErr := scanLink(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) Create(ctx context.Context, input LinkInput, actor string) (Link, error) {
	normalizeInput(&input)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Link{}, err
	}
	defer tx.Rollback(ctx)
	if err = validatePixel(ctx, tx, input, false); err != nil {
		return Link{}, err
	}
	item, err := scanLink(tx.QueryRow(ctx, `INSERT INTO short_links
		(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,mode,meta_connection_id,attribution_mode,
		meta_pixel_id,tiktok_pixel_id,ad_platform,time_spent_threshold,startup_tail_seconds,startup_theme,product_type)
		VALUES($1,$2,'',$3,'','','',$4,'',$5,'dynamic',$6,$7,$8,$9,5,'cover_wall','cover')
		RETURNING id,code,name,enabled,product_type,meta_connection_id,meta_pixel_id,tiktok_pixel_id,
		COALESCE((SELECT pixel_code FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),
		COALESCE((SELECT name FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),ad_platform,attribution_mode,time_spent_threshold,
		(SELECT count(*) FROM click_events WHERE link_id=short_links.id),created_at,first_visited_at`,
		input.Code, strings.TrimSpace(input.Name), input.Enabled, platformChannel(input.AdPlatform), input.MetaConnectionID,
		input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold))
	if err != nil {
		return Link{}, err
	}
	if err = audit(ctx, tx, actor, "cover_link.create", item); err != nil {
		return Link{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) Update(ctx context.Context, id int64, input LinkInput, actor string) (Link, error) {
	normalizeInput(&input)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return Link{}, err
	}
	defer tx.Rollback(ctx)
	var currentPlatform string
	var currentMetaConnectionID, currentMetaPixelID, currentTikTokPixelID *int64
	var firstVisitedAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id,first_visited_at
		FROM short_links WHERE id=$1 AND product_type='cover' AND archived_at IS NULL FOR UPDATE`, id).
		Scan(&currentPlatform, &currentMetaConnectionID, &currentMetaPixelID, &currentTikTokPixelID, &firstVisitedAt); err != nil {
		return Link{}, err
	}
	locked := firstVisitedAt != nil
	if locked && (currentPlatform != input.AdPlatform || !sameID(currentMetaConnectionID, input.MetaConnectionID) ||
		!sameID(currentMetaPixelID, input.MetaPixelID) || !sameID(currentTikTokPixelID, input.TikTokPixelID)) {
		return Link{}, ErrBindingLocked
	}
	if err = validatePixel(ctx, tx, input, locked); err != nil {
		return Link{}, err
	}
	item, err := scanLink(tx.QueryRow(ctx, `UPDATE short_links SET name=$2,enabled=$3,channel=$4,
		meta_connection_id=$5,meta_pixel_id=$6,tiktok_pixel_id=$7,ad_platform=$8,attribution_mode='dynamic',
		time_spent_threshold=$9,startup_theme='cover_wall',startup_tail_seconds=5
		WHERE id=$1 AND product_type='cover' AND archived_at IS NULL
		RETURNING id,code,name,enabled,product_type,meta_connection_id,meta_pixel_id,tiktok_pixel_id,
		COALESCE((SELECT pixel_code FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),
		COALESCE((SELECT name FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),ad_platform,attribution_mode,time_spent_threshold,
		(SELECT count(*) FROM click_events WHERE link_id=short_links.id),created_at,first_visited_at`,
		id, strings.TrimSpace(input.Name), input.Enabled, platformChannel(input.AdPlatform), input.MetaConnectionID,
		input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold))
	if err != nil {
		return Link{}, err
	}
	if err = audit(ctx, tx, actor, "cover_link.update", item); err != nil {
		return Link{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) Delete(ctx context.Context, id int64, actor string) error {
	_, err := r.archive(ctx, []int64{id}, actor, "cover_link.delete")
	if errors.Is(err, ErrLinksNotFound) {
		return pgx.ErrNoRows
	}
	return err
}

// DeleteBatch archives the complete selection atomically while retaining every historical report row.
func (r Repository) DeleteBatch(ctx context.Context, ids []int64, actor string) (int64, error) {
	return r.archive(ctx, ids, actor, "cover_link.delete_batch")
}

func (r Repository) archive(ctx context.Context, ids []int64, actor, action string) (int64, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	orderedIDs := append([]int64(nil), ids...)
	slices.Sort(orderedIDs)
	rows, err := tx.Query(ctx, `SELECT id,code,name FROM short_links
		WHERE id=ANY($1::bigint[]) AND product_type='cover' AND archived_at IS NULL ORDER BY id FOR UPDATE`, orderedIDs)
	if err != nil {
		return 0, err
	}
	type archivedLink struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}
	items := make([]archivedLink, 0, len(orderedIDs))
	for rows.Next() {
		var item archivedLink
		if err = rows.Scan(&item.ID, &item.Code, &item.Name); err != nil {
			rows.Close()
			return 0, err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return 0, err
	}
	if len(items) != len(orderedIDs) {
		return 0, ErrLinksNotFound
	}
	// Disabling and archiving makes public access fail immediately without deleting analytics or delivery history.
	result, err := tx.Exec(ctx, `UPDATE short_links SET enabled=false,archived_at=now()
		WHERE id=ANY($1::bigint[]) AND product_type='cover' AND archived_at IS NULL`, orderedIDs)
	if err != nil {
		return 0, err
	}
	if result.RowsAffected() != int64(len(orderedIDs)) {
		return 0, ErrLinksNotFound
	}
	detail, err := json.Marshal(map[string]any{"count": len(items), "links": items})
	if err != nil {
		return 0, err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return int64(len(items)), nil
}

func validatePixel(ctx context.Context, tx pgx.Tx, input LinkInput, allowDisabled bool) error {
	if allowDisabled {
		return nil
	}
	var valid bool
	if input.AdPlatform == "tiktok" {
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM tiktok_pixels p JOIN tiktok_connections c ON c.id=p.connection_id
			WHERE p.id=$1 AND p.enabled AND c.enabled)`, input.TikTokPixelID).Scan(&valid)
		if err != nil {
			return err
		}
	} else {
		err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM meta_pixels p
			WHERE p.id=$1 AND p.connection_id=$2 AND p.enabled)`, input.MetaPixelID, input.MetaConnectionID).Scan(&valid)
		if err != nil {
			return err
		}
	}
	if !valid {
		return ErrPixelInvalid
	}
	return nil
}

func sameID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func platformChannel(platform string) string {
	if platform == "tiktok" {
		return "tiktok"
	}
	return "facebook"
}

func audit(ctx context.Context, tx pgx.Tx, actor, action string, item Link) error {
	detail, _ := json.Marshal(map[string]any{"id": item.ID, "code": item.Code})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
