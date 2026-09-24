package novel

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/modules/links"
)

var (
	ErrDistributionBindingLocked = errors.New("novel binding is locked after the first visit")
	ErrDistributionHasVisits     = errors.New("visited distribution links cannot be deleted")
	ErrDistributionPixelInvalid  = errors.New("advertising pixel is invalid or disabled")
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
	TikTokPixelID      *int64 `json:"tiktok_pixel_id"`
	AdPlatform         string `json:"ad_platform"`
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
	TikTokPixelID      *int64     `json:"tiktok_pixel_id,omitempty"`
	TikTokPixelCode    string     `json:"tiktok_pixel_code,omitempty"`
	TikTokPixelName    string     `json:"tiktok_pixel_name,omitempty"`
	AdPlatform         string     `json:"ad_platform"`
	AttributionMode    string     `json:"attribution_mode"`
	TimeSpentThreshold int        `json:"time_spent_threshold"`
	VisitCount         int64      `json:"visit_count"`
	CreatedAt          time.Time  `json:"created_at"`
	FirstVisitedAt     *time.Time `json:"first_visited_at,omitempty"`
	PublicURL          string     `json:"public_url,omitempty"`
	TikTokTemplateURL  string     `json:"tiktok_template_url,omitempty"`
}

// NormalizeDistributionInput keeps campaign attribution dynamic and prevents
// hidden client fields from silently changing the selected advertising platform.
func NormalizeDistributionInput(input *DistributionInput) {
	if input.AdPlatform == "" {
		input.AdPlatform = "meta"
	}
	input.AttributionMode = "dynamic"
	input.CampaignID, input.AdsetID, input.AdID = "", "", ""
	if input.AdPlatform == "tiktok" {
		input.Channel = "tiktok"
	} else {
		input.Channel = "facebook"
	}
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
		return errors.New("停留事件阈值应为 5–3600 秒，或设为 0")
	}
	switch input.AdPlatform {
	case "meta":
		if input.TikTokPixelID != nil {
			return errors.New("一条链接只能绑定一个广告平台")
		}
		binding := links.Link{MetaConnectionID: input.MetaConnectionID, MetaPixelID: input.MetaPixelID}
		if !links.HasMetaBinding(binding) {
			return errors.New("请选择 Meta Pixel")
		}
	case "tiktok":
		if input.MetaConnectionID != nil || input.MetaPixelID != nil {
			return errors.New("一条链接只能绑定一个广告平台")
		}
		if input.TikTokPixelID == nil || *input.TikTokPixelID < 1 {
			return errors.New("请选择 TikTok Pixel")
		}
	default:
		return errors.New("广告平台无效")
	}
	return nil
}

const distributionColumns = `l.id,l.code,l.name,l.enabled,l.product_type,l.novel_id,n.title,n.slug,
	l.channel,l.campaign_id,l.adset_id,l.ad_id,l.meta_connection_id,l.meta_pixel_id,l.tiktok_pixel_id,
	COALESCE(tp.pixel_code,''),COALESCE(tp.name,''),l.ad_platform,l.attribution_mode,
	l.time_spent_threshold,(SELECT count(*) FROM click_events e WHERE e.link_id=l.id),l.created_at,l.first_visited_at`

func scanDistribution(row pgx.Row) (DistributionLink, error) {
	var item DistributionLink
	err := row.Scan(&item.ID, &item.Code, &item.Name, &item.Enabled, &item.ProductType, &item.NovelID, &item.NovelTitle, &item.NovelSlug,
		&item.Channel, &item.CampaignID, &item.AdsetID, &item.AdID, &item.MetaConnectionID, &item.MetaPixelID, &item.TikTokPixelID,
		&item.TikTokPixelCode, &item.TikTokPixelName, &item.AdPlatform, &item.AttributionMode,
		&item.TimeSpentThreshold, &item.VisitCount, &item.CreatedAt, &item.FirstVisitedAt)
	return item, err
}

func (r Repository) ListDistributionLinks(ctx context.Context, novelID int64) ([]DistributionLink, error) {
	query := "SELECT " + distributionColumns + " FROM short_links l JOIN novels n ON n.id=l.novel_id LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.product_type='novel'"
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
	NormalizeDistributionInput(&input)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return DistributionLink{}, err
	}
	defer tx.Rollback(ctx)
	if err = validateDistributionPixel(ctx, tx, input, false); err != nil {
		return DistributionLink{}, err
	}
	item, err := scanDistribution(tx.QueryRow(ctx, `INSERT INTO short_links
		(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,mode,meta_connection_id,attribution_mode,meta_pixel_id,tiktok_pixel_id,ad_platform,time_spent_threshold,product_type,novel_id)
		SELECT $1,$2,'',$3,$4,$5,$6,$7,'',$8,$9,$10,$11,$12,$13,'novel',n.id FROM novels n WHERE n.id=$14 AND n.deleted_at IS NULL
		RETURNING `+distributionColumnsForReturn(), input.Code, strings.TrimSpace(input.Name), input.Enabled, input.CampaignID, input.AdsetID, input.AdID, input.Channel, input.MetaConnectionID, input.AttributionMode, input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold, input.NovelID))
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
		channel,campaign_id,adset_id,ad_id,meta_connection_id,meta_pixel_id,tiktok_pixel_id,
		COALESCE((SELECT pixel_code FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),
		COALESCE((SELECT name FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),ad_platform,attribution_mode,time_spent_threshold,
		(SELECT count(*) FROM click_events WHERE link_id=short_links.id),created_at,first_visited_at`
}

func (r Repository) UpdateDistributionLink(ctx context.Context, id int64, input DistributionInput, actor string) (DistributionLink, error) {
	NormalizeDistributionInput(&input)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return DistributionLink{}, err
	}
	defer tx.Rollback(ctx)
	var currentNovelID int64
	var currentPlatform string
	var currentMetaConnectionID, currentMetaPixelID, currentTikTokPixelID *int64
	var firstVisitedAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT novel_id,ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id,first_visited_at
		FROM short_links WHERE id=$1 AND product_type='novel' FOR UPDATE`, id).Scan(&currentNovelID, &currentPlatform, &currentMetaConnectionID, &currentMetaPixelID, &currentTikTokPixelID, &firstVisitedAt); err != nil {
		return DistributionLink{}, err
	}
	if firstVisitedAt != nil && (currentNovelID != input.NovelID || currentPlatform != input.AdPlatform ||
		!sameDistributionID(currentMetaConnectionID, input.MetaConnectionID) || !sameDistributionID(currentMetaPixelID, input.MetaPixelID) ||
		!sameDistributionID(currentTikTokPixelID, input.TikTokPixelID)) {
		return DistributionLink{}, ErrDistributionBindingLocked
	}
	unchangedPixel := currentPlatform == input.AdPlatform && sameDistributionID(currentMetaConnectionID, input.MetaConnectionID) &&
		sameDistributionID(currentMetaPixelID, input.MetaPixelID) && sameDistributionID(currentTikTokPixelID, input.TikTokPixelID)
	if err = validateDistributionPixel(ctx, tx, input, unchangedPixel); err != nil {
		return DistributionLink{}, err
	}
	item, err := scanDistribution(tx.QueryRow(ctx, `UPDATE short_links SET name=$2,enabled=$3,campaign_id=$4,adset_id=$5,ad_id=$6,
		channel=$7,meta_connection_id=$8,attribution_mode=$9,meta_pixel_id=$10,tiktok_pixel_id=$11,ad_platform=$12,time_spent_threshold=$13,novel_id=$14
		WHERE id=$1 AND product_type='novel' AND EXISTS(SELECT 1 FROM novels WHERE id=$14 AND deleted_at IS NULL)
		RETURNING `+distributionColumnsForReturn(), id, strings.TrimSpace(input.Name), input.Enabled, input.CampaignID, input.AdsetID, input.AdID, input.Channel, input.MetaConnectionID, input.AttributionMode, input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold, input.NovelID))
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
	item, err := scanDistribution(tx.QueryRow(ctx, "SELECT "+distributionColumns+" FROM short_links l JOIN novels n ON n.id=l.novel_id LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.id=$1 AND l.product_type='novel' FOR UPDATE OF l", id))
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

func sameDistributionID(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func validateDistributionPixel(ctx context.Context, tx pgx.Tx, input DistributionInput, allowDisabled bool) error {
	if allowDisabled {
		// Existing disabled configurations stay editable without forcing operators
		// to replace a previously valid frozen binding.
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
		return ErrDistributionPixelInvalid
	}
	return nil
}

// TikTokTemplate returns the copy-ready URL used in TikTok Ads dynamic macros.
func TikTokTemplate(publicURL, code string) string {
	return strings.TrimRight(publicURL, "/") + "/novel/" + url.PathEscape(code) +
		"?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__" +
		"&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__"
}

func auditDistribution(ctx context.Context, tx pgx.Tx, actor, action string, item DistributionLink) error {
	detail, _ := json.Marshal(map[string]any{"id": item.ID, "code": item.Code, "novel_id": item.NovelID})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
