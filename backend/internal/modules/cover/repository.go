package cover

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrBindingLocked = errors.New("advertising binding is locked after the first visit")
	ErrHasVisits     = errors.New("visited cover links cannot be deleted")
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
	rows, err := r.DB.Query(ctx, "SELECT "+linkColumns+" FROM short_links l LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.product_type='cover' ORDER BY l.id DESC")
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
		FROM short_links WHERE id=$1 AND product_type='cover' FOR UPDATE`, id).
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
		WHERE id=$1 AND product_type='cover'
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
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanLink(tx.QueryRow(ctx, "SELECT "+linkColumns+" FROM short_links l LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.id=$1 AND l.product_type='cover' FOR UPDATE OF l", id))
	if err != nil {
		return err
	}
	if item.FirstVisitedAt != nil {
		return ErrHasVisits
	}
	if _, err = tx.Exec(ctx, "DELETE FROM short_links WHERE id=$1 AND product_type='cover'", id); err != nil {
		return err
	}
	if err = audit(ctx, tx, actor, "cover_link.delete", item); err != nil {
		return err
	}
	return tx.Commit(ctx)
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
