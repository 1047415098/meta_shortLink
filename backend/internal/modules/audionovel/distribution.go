package audionovel

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
	ErrDistributionBindingLocked  = errors.New("audio novel binding is locked after the first visit")
	ErrDistributionHasVisits      = errors.New("visited audio distribution links cannot be deleted")
	ErrDistributionPixelInvalid   = errors.New("advertising pixel is invalid or disabled")
	ErrDistributionContentInvalid = errors.New("audio novel is disabled, deleted, or missing its MP3")
)

// DistributionInput accepts legacy hidden attribution fields only so the
// server can erase them instead of trusting a stale or crafted admin client.
type DistributionInput struct {
	Name               string `json:"name"`
	Code               string `json:"code,omitempty"`
	AudioNovelID       int64  `json:"audio_novel_id"`
	Enabled            bool   `json:"enabled"`
	AdPlatform         string `json:"ad_platform"`
	MetaConnectionID   *int64 `json:"meta_connection_id"`
	MetaPixelID        *int64 `json:"meta_pixel_id"`
	TikTokPixelID      *int64 `json:"tiktok_pixel_id"`
	TimeSpentThreshold int    `json:"time_spent_threshold"`
	AttributionMode    string `json:"attribution_mode,omitempty"`
	Channel            string `json:"channel,omitempty"`
	CampaignID         string `json:"campaign_id,omitempty"`
	AdsetID            string `json:"adset_id,omitempty"`
	AdID               string `json:"ad_id,omitempty"`
}

type DistributionLink struct {
	ID                 int64      `json:"id"`
	Code               string     `json:"code"`
	Name               string     `json:"name"`
	Enabled            bool       `json:"enabled"`
	ProductType        string     `json:"product_type"`
	AudioNovelID       int64      `json:"audio_novel_id"`
	AudioNovelTitle    string     `json:"audio_novel_title"`
	AudioNovelSlug     string     `json:"audio_novel_slug"`
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

// NormalizeDistributionInput makes dynamic attribution a server-owned rule.
func NormalizeDistributionInput(input *DistributionInput) {
	if input.AdPlatform == "" {
		input.AdPlatform = "meta"
	}
	input.AttributionMode = "dynamic"
	input.Channel = ""
	input.CampaignID = ""
	input.AdsetID = ""
	input.AdID = ""
}

func ValidateDistributionInput(input DistributionInput, requireCode bool) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 {
		return errors.New("链接名称不能为空且不能超过 120 个字符")
	}
	if requireCode && input.Code != "" && !links.ValidCode(input.Code) {
		return errors.New("短码只能包含 3–40 位字母、数字、下划线或横线")
	}
	if input.AudioNovelID < 1 {
		return errors.New("请选择语音小说")
	}
	if input.AttributionMode != "dynamic" {
		return errors.New("语音投放链接只支持动态归因")
	}
	if input.TimeSpentThreshold != 0 && (input.TimeSpentThreshold < 1 || input.TimeSpentThreshold > 3600) {
		return errors.New("播放达标阈值应为 1–3600 秒，或设为 0")
	}
	switch input.AdPlatform {
	case "meta":
		if input.TikTokPixelID != nil {
			return errors.New("一条链接只能绑定一个广告平台")
		}
		if input.MetaConnectionID == nil || *input.MetaConnectionID < 1 || input.MetaPixelID == nil || *input.MetaPixelID < 1 {
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

const audioDistributionColumns = `l.id,l.code,l.name,l.enabled,l.product_type,l.audio_novel_id,n.title,n.slug,
	l.meta_connection_id,l.meta_pixel_id,l.tiktok_pixel_id,COALESCE(tp.pixel_code,''),COALESCE(tp.name,''),
	l.ad_platform,l.attribution_mode,l.time_spent_threshold,
	(SELECT count(*) FROM click_events e WHERE e.link_id=l.id),l.created_at,l.first_visited_at`

func scanAudioDistribution(row pgx.Row) (DistributionLink, error) {
	var item DistributionLink
	err := row.Scan(
		&item.ID, &item.Code, &item.Name, &item.Enabled, &item.ProductType,
		&item.AudioNovelID, &item.AudioNovelTitle, &item.AudioNovelSlug,
		&item.MetaConnectionID, &item.MetaPixelID, &item.TikTokPixelID,
		&item.TikTokPixelCode, &item.TikTokPixelName, &item.AdPlatform,
		&item.AttributionMode, &item.TimeSpentThreshold, &item.VisitCount,
		&item.CreatedAt, &item.FirstVisitedAt,
	)
	return item, err
}

func (r Repository) ListDistributionLinks(ctx context.Context, audioNovelID int64) ([]DistributionLink, error) {
	rows, err := r.DB.Query(ctx, "SELECT "+audioDistributionColumns+`
		FROM short_links l
		JOIN audio_novels n ON n.id=l.audio_novel_id
		LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id
		WHERE l.product_type='audio_novel' AND l.audio_novel_id=$1
		ORDER BY l.id DESC`, audioNovelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []DistributionLink{}
	for rows.Next() {
		item, scanErr := scanAudioDistribution(rows)
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
	if err = validateAudioDistributionContent(ctx, tx, input.AudioNovelID); err != nil {
		return DistributionLink{}, err
	}
	if err = validateAudioDistributionPixel(ctx, tx, input, false); err != nil {
		return DistributionLink{}, err
	}
	item, err := scanAudioDistribution(tx.QueryRow(ctx, `INSERT INTO short_links
		(code,name,target_url,enabled,campaign_id,adset_id,ad_id,channel,mode,meta_connection_id,
		 attribution_mode,meta_pixel_id,tiktok_pixel_id,ad_platform,time_spent_threshold,product_type,audio_novel_id)
		VALUES($1,$2,'',$3,'','','','','',$4,'dynamic',$5,$6,$7,$8,'audio_novel',$9)
		RETURNING `+audioDistributionColumnsForReturn(),
		input.Code, strings.TrimSpace(input.Name), input.Enabled, input.MetaConnectionID,
		input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold, input.AudioNovelID))
	if err != nil {
		return DistributionLink{}, err
	}
	if err = auditAudioDistribution(ctx, tx, actor, "audio_novel_link.create", item); err != nil {
		return DistributionLink{}, err
	}
	return item, tx.Commit(ctx)
}

func audioDistributionColumnsForReturn() string {
	return `id,code,name,enabled,product_type,audio_novel_id,
		(SELECT title FROM audio_novels WHERE id=audio_novel_id),
		(SELECT slug FROM audio_novels WHERE id=audio_novel_id),
		meta_connection_id,meta_pixel_id,tiktok_pixel_id,
		COALESCE((SELECT pixel_code FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),
		COALESCE((SELECT name FROM tiktok_pixels WHERE id=tiktok_pixel_id),''),
		ad_platform,attribution_mode,time_spent_threshold,
		(SELECT count(*) FROM click_events WHERE link_id=short_links.id),created_at,first_visited_at`
}

func (r Repository) UpdateDistributionLink(ctx context.Context, id int64, input DistributionInput, actor string) (DistributionLink, error) {
	NormalizeDistributionInput(&input)
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return DistributionLink{}, err
	}
	defer tx.Rollback(ctx)

	var currentAudioNovelID int64
	var currentPlatform string
	var currentMetaConnectionID, currentMetaPixelID, currentTikTokPixelID *int64
	var firstVisitedAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT audio_novel_id,ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id,first_visited_at
		FROM short_links WHERE id=$1 AND product_type='audio_novel' FOR UPDATE`, id).
		Scan(&currentAudioNovelID, &currentPlatform, &currentMetaConnectionID, &currentMetaPixelID, &currentTikTokPixelID, &firstVisitedAt); err != nil {
		return DistributionLink{}, err
	}
	bindingChanged := currentAudioNovelID != input.AudioNovelID || currentPlatform != input.AdPlatform ||
		!sameDistributionID(currentMetaConnectionID, input.MetaConnectionID) ||
		!sameDistributionID(currentMetaPixelID, input.MetaPixelID) ||
		!sameDistributionID(currentTikTokPixelID, input.TikTokPixelID)
	if firstVisitedAt != nil && bindingChanged {
		return DistributionLink{}, ErrDistributionBindingLocked
	}
	if currentAudioNovelID != input.AudioNovelID {
		if err = validateAudioDistributionContent(ctx, tx, input.AudioNovelID); err != nil {
			return DistributionLink{}, err
		}
	}
	unchangedPixel := currentPlatform == input.AdPlatform &&
		sameDistributionID(currentMetaConnectionID, input.MetaConnectionID) &&
		sameDistributionID(currentMetaPixelID, input.MetaPixelID) &&
		sameDistributionID(currentTikTokPixelID, input.TikTokPixelID)
	if err = validateAudioDistributionPixel(ctx, tx, input, unchangedPixel); err != nil {
		return DistributionLink{}, err
	}
	item, err := scanAudioDistribution(tx.QueryRow(ctx, `UPDATE short_links SET
		name=$2,enabled=$3,campaign_id='',adset_id='',ad_id='',channel='',mode='',
		meta_connection_id=$4,attribution_mode='dynamic',meta_pixel_id=$5,tiktok_pixel_id=$6,
		ad_platform=$7,time_spent_threshold=$8,audio_novel_id=$9
		WHERE id=$1 AND product_type='audio_novel'
		RETURNING `+audioDistributionColumnsForReturn(),
		id, strings.TrimSpace(input.Name), input.Enabled, input.MetaConnectionID,
		input.MetaPixelID, input.TikTokPixelID, input.AdPlatform, input.TimeSpentThreshold, input.AudioNovelID))
	if err != nil {
		return DistributionLink{}, err
	}
	if err = auditAudioDistribution(ctx, tx, actor, "audio_novel_link.update", item); err != nil {
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
	item, err := scanAudioDistribution(tx.QueryRow(ctx, "SELECT "+audioDistributionColumns+`
		FROM short_links l
		JOIN audio_novels n ON n.id=l.audio_novel_id
		LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id
		WHERE l.id=$1 AND l.product_type='audio_novel' FOR UPDATE OF l`, id))
	if err != nil {
		return err
	}
	// Probe and bot rows do not freeze the campaign binding, but they are still
	// retained visit history and hold a foreign key to this link.
	if item.FirstVisitedAt != nil || item.VisitCount > 0 {
		return ErrDistributionHasVisits
	}
	if _, err = tx.Exec(ctx, "DELETE FROM short_links WHERE id=$1 AND product_type='audio_novel'", id); err != nil {
		return err
	}
	if err = auditAudioDistribution(ctx, tx, actor, "audio_novel_link.delete", item); err != nil {
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

func validateAudioDistributionContent(ctx context.Context, tx pgx.Tx, audioNovelID int64) error {
	var enabled bool
	var deletedAt *time.Time
	var audioPath string
	if err := tx.QueryRow(ctx, "SELECT enabled,deleted_at,audio_path FROM audio_novels WHERE id=$1 FOR SHARE", audioNovelID).
		Scan(&enabled, &deletedAt, &audioPath); err != nil {
		return err
	}
	if !enabled || deletedAt != nil || audioPath == "" {
		return ErrDistributionContentInvalid
	}
	return nil
}

func validateAudioDistributionPixel(ctx context.Context, tx pgx.Tx, input DistributionInput, allowDisabled bool) error {
	if allowDisabled {
		// A frozen or unchanged Pixel remains editable even after an operator pauses it.
		return nil
	}
	var valid bool
	if input.AdPlatform == "tiktok" {
		err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM tiktok_pixels p
			JOIN tiktok_connections c ON c.id=p.connection_id
			WHERE p.id=$1 AND p.enabled AND c.enabled
		)`, input.TikTokPixelID).Scan(&valid)
		if err != nil {
			return err
		}
	} else {
		err := tx.QueryRow(ctx, `SELECT EXISTS(
			SELECT 1 FROM meta_pixels p
			WHERE p.id=$1 AND p.connection_id=$2 AND p.enabled
		)`, input.MetaPixelID, input.MetaConnectionID).Scan(&valid)
		if err != nil {
			return err
		}
	}
	if !valid {
		return ErrDistributionPixelInvalid
	}
	return nil
}

// AudioTikTokTemplate returns the copy-ready dynamic URL for TikTok Ads.
func AudioTikTokTemplate(publicURL, code string) string {
	return strings.TrimRight(publicURL, "/") + "/audio-novel/" + url.PathEscape(code) +
		"?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__" +
		"&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__"
}

func auditAudioDistribution(ctx context.Context, tx pgx.Tx, actor, action string, item DistributionLink) error {
	detail, _ := json.Marshal(map[string]any{
		"id": item.ID, "code": item.Code, "audio_novel_id": item.AudioNovelID, "ad_platform": item.AdPlatform,
	})
	_, err := tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,$2,$3)", actor, action, detail)
	return err
}
