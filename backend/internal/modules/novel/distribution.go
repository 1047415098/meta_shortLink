package novel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/modules/links"
)

var (
	ErrDistributionBindingLocked = errors.New("novel binding is locked after the first visit")
	ErrDistributionHasVisits     = errors.New("visited distribution links cannot be deleted")
)

// DistributionInput is the editable configuration of one independently attributed novel link.
type DistributionInput struct {
	Name               string `json:"name"`
	Code               string `json:"code,omitempty"`
	NovelID            int64  `json:"novel_id"`
	Enabled            bool   `json:"enabled"`
	Channel            string `json:"channel"`
	CampaignID         string `json:"campaign_id"`
	AdsetID            string `json:"adset_id"`
	AdID               string `json:"ad_id"`
	MetaConnectionID   *int64 `json:"meta_connection_id"`
	MetaPixelID        *int64 `json:"meta_pixel_id"`
	AttributionMode    string `json:"attribution_mode"`
	TimeSpentThreshold int    `json:"time_spent_threshold"`
}

type DistributionLink struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Enabled            bool       `json:"enabled"`
	ProductType        string     `json:"product_type"`
	NovelID            int64      `json:"novel_id"`
	NovelTitle         string     `json:"novel_title"`
	NovelSlug          string     `json:"novel_slug"`
	Channel            string     `json:"channel"`
	CampaignID         string     `json:"campaign_id"`
	AdsetID            string     `json:"adset_id"`
	AdID               string     `json:"ad_id"`
	MetaConnectionID   *int64     `json:"meta_connection_id"`
	MetaPixelID        *int64     `json:"meta_pixel_id"`
	AttributionMode    string     `json:"attribution_mode"`
	TimeSpentThreshold int        `json:"time_spent_threshold"`
	VisitCount         int64      `json:"visit_count"`
	CreatedAt          time.Time  `json:"created_at"`
	FirstVisitedAt     *time.Time `json:"first_visited_at,omitempty"`
	PublicURL          string     `json:"public_url,omitempty"`
}

func ValidateDistributionInput(input DistributionInput, requireCode bool) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 {
		return errors.New("链接名称不能为空且不能超过 120 个字符")
	}
	if requireCode && input.Code != "" && !links.ValidCode(input.Code) {
		return errors.New("短码只能包含 3–40 位字母、数字、下划线或横线")
	}
	if input.NovelID < 1 || len(input.Channel) > 60 || len(input.CampaignID) > 120 || len(input.AdsetID) > 120 || len(input.AdID) > 120 {
		return errors.New("小说或渠道参数无效")
	}
	if input.AttributionMode != "bound" && input.AttributionMode != "dynamic" {
		return errors.New("归因方式无效")
	}
	if input.TimeSpentThreshold != 0 && (input.TimeSpentThreshold < 5 || input.TimeSpentThreshold > 3600) {
		return errors.New("Meta 停留事件阈值应为 5–3600 秒，或设为 0")
	}
	binding := links.Link{MetaConnectionID: input.MetaConnectionID, MetaPixelID: input.MetaPixelID}
	if !links.HasMetaBinding(binding) {
		return errors.New("请选择 Meta Pixel")
	}
	return nil
}

const distributionColumns = `l.id,l.code,l.name,l.enabled,l.product_type,l.novel_id,n.title,n.slug,
	l.channel,l.campaign_id,l.adset_id,l.ad_id,l.meta_connection_id,l.meta_pixel_id,l.attribution_mode,
	l.time_spent_threshold,(SELECT count(*) FROM click_events e WHERE e.link_id=l.id),l.created_at,l.first_visited_at`

func scanDistribution(row pgx.Row) (DistributionLink, error) {
	var item DistributionLink
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Enabled, &item.ProductType, &item.NovelID, &item.NovelTitle, &item.NovelSlug,
		&item.Channel, &item.CampaignID, &item.AdsetID, &item.AdID, &item.MetaConnectionID, &item.MetaPixelID, &item.AttributionMode,
		&item.TimeSpentThreshold, &item.VisitCount, &item.CreatedAt, &item.FirstVisitedAt)
	return item, err
}

func (r Repository) ListDistributionLinks(ctx context.Context, novelID int64) ([]DistributionLink, error) {
	query := "SELECT " + distributionColumns + " FROM short_links l JOIN novels n ON n.id=l.novel_id WHERE l.product_type='novel'"
	args := []any{}
	if novelID > 0 {
		query += " AND l.novel_id=$1"
		args = append(args, novelID)
	}
	query += " ORDER BY l.id DESC"
	rows, err := r.DB.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DistributionLink{}
	for rows.Next() {
		item, scanErr := scanDistribution(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r Repository) CreateDistributionLink(ctx context.Context, input DistributionInput, actor string) (DistributionLink, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return DistributionLink{}, err
	}
	defer tx.Rollback(ctx)
	item, err := scanDistribution(tx.QueryRow(ctx, `INSERT INTO short_links
		(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,mode,meta_connection_id,attribution_mode,meta_pixel_id,time_spent_threshold,product_type,novel_id)
		SELECT $1,$2,'',$3,$4,$5,$6,$7,'',$8,$9,$10,$11,'novel',n.id FROM novels n WHERE n.id=$12 AND n.deleted_at IS NULL
		RETURNING `+distributionColumnsForReturn(), input.Code, strings.TrimSpace(input.Name), input.Enabled, input.CampaignID, input.AdsetID, input.AdID, input.Channel, input.MetaConnectionID, input.AttributionMode, input.MetaPixelID, input.TimeSpentThreshold, input.NovelID))
	if err != nil {
		return DistributionLink{}, err
	}
	if err = auditDistribution(ctx, tx, actor, "novel_link.create", item); err != nil {
		return DistributionLink{}, err
	}
	return item, tx.Commit(ctx)
}

func distributionColumnsForReturn() string {
	// INSERT/UPDATE RETURNING cannot use the source aliases from the list query.
	return `id,code,name,enabled,product_type,novel_id,
		(SELECT title FROM novels WHERE id=novel_id),(SELECT slug FROM novels WHERE id=novel_id),
		channel,campaign_id,adset_id,ad_id,meta_connection_id,meta_pixel_id,attribution_mode,time_spent_threshold,
		(SELECT count(*) FROM click_events WHERE link_id=short_links.id),created_at,first_visited_at`
}

func (r Repository) UpdateDistributionLink(ctx context.Context, id int64, input DistributionInput, actor string) (DistributionLink, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return DistributionLink{}, err
	}
	defer tx.Rollback(ctx)
	var currentNovelID int64
	var firstVisitedAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT novel_id,first_visited_at
		FROM short_links WHERE id=$1 AND product_type='novel' FOR UPDATE`, id).Scan(&currentNovelID, &firstVisitedAt); err != nil {
		return DistributionLink{}, err
	}
	if currentNovelID != input.NovelID && firstVisitedAt != nil {
		return DistributionLink{}, ErrDistributionBindingLocked
	}
	item, err := scanDistribution(tx.QueryRow(ctx, `UPDATE short_links SET name=$2,enabled=$3,campaign_id=$4,adset_id=$5,ad_id=$6,
		channel=$7,meta_connection_id=$8,attribution_mode=$9,meta_pixel_id=$10,time_spent_threshold=$11,novel_id=$12
		WHERE id=$1 AND product_type='novel' AND EXISTS(SELECT 1 FROM novels WHERE id=$12 AND deleted_at IS NULL)
		RETURNING `+distributionColumnsForReturn(), id, strings.TrimSpace(input.Name), input.Enabled, input.CampaignID, input.AdsetID, input.AdID, input.Channel, input.MetaConnectionID, input.AttributionMode, input.MetaPixelID, input.TimeSpentThreshold, input.NovelID))
	if err != nil {
		return DistributionLink{}, err
	}
	if err = auditDistribution(ctx, tx, actor, "novel_link.update", item); err != nil {
		return DistributionLink{}, err
	}
	return item, tx.Commit(ctx)
}

func (r Repository) DeleteDistributionLink(ctx context.Context, id int64, actor string) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanDistribution(tx.QueryRow(ctx, "SELECT "+distributionColumns+" FROM short_links l JOIN novels n ON n.id=l.novel_id WHERE l.id=$1 AND l.product_type='novel' FOR UPDATE OF l", id))
	if err != nil {
		return err
	}
	if item.FirstVisitedAt != nil {
		return ErrDistributionHasVisits
	}
	if _, err = tx.Exec(ctx, "DELETE FROM short_links WHERE id=$1 AND product_type='novel'", id); err != nil {
		return err
	}
	if err = auditDistribution(ctx, tx, actor, "novel_link.delete", item); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func auditDistribution(ctx context.Context, tx pgx.Tx, actor, action string, item DistributionLink) error {
	detail, _ := json.Marshal(map[string]any{"id": item.ID, "code": item.Code, "novel_id": item.NovelID})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
