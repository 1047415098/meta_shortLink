package tracking

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }
type Event struct {
	MetaPixelID                                                                                                                *int64
	MetaConnectionID                                                                                                           *int64
	TikTokPixelID                                                                                                              *int64
	NovelID                                                                                                                    *int64
	AudioNovelID                                                                                                               *int64
	ID                                                                                                                         string
	LinkID                                                                                                                     int64
	VisitorID                                                                                                                  any
	CookieStatus, Method, TargetURL, Device, OS, Browser, Country, Region, City, Source, CampaignID, AdsetID, AdID, Referrer   string
	AdPlatform, TikTokTTCLID, TikTokTTP, TikTokAdgroupID, TikTokCreativeID, TikTokAdIDV2, TikTokPlacement, TikTokContextCipher string
	Parameters                                                                                                                 []byte
	AttributionConflict                                                                                                        bool
	Classification, Reason, Type                                                                                               string
	Surface                                                                                                                    string
	Status                                                                                                                     int
	TimeSpentThreshold                                                                                                         int
}

func (r Repository) Record(ctx context.Context, v Event) error {
	_, e := r.DB.Exec(ctx, `WITH target AS (
 -- The account supplies grouping and API metadata; the selected Pixel owns the
 -- credential and all event switches used to freeze this visit's routing.
	-- Manual and automatic consultation switches are frozen independently so later
	-- Pixel edits cannot rewrite how an existing visit is delivered.
 SELECT c.account_id,p.id AS pixel_record_id,p.enabled,p.manual_enabled,p.auto_enabled,p.pageview_enabled,p.manual_event_name FROM meta_connections c
 LEFT JOIN LATERAL (SELECT * FROM meta_pixels p WHERE p.connection_id=c.id AND ($25::bigint IS NULL OR p.id=$25) ORDER BY (p.pixel_id=c.pixel_id) DESC,p.id LIMIT 1) p ON true WHERE c.id=$24
	 ), tiktok_target AS (
	 SELECT p.id,p.pixel_code FROM tiktok_pixels p WHERE p.id=$29
	 ) INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,parameters,attribution_conflict,classification,reason,event_type,status,meta_connection_id,meta_account_id,meta_measurement,meta_pixel_id,meta_pageview_enabled,meta_manual_enabled,meta_auto_enabled,meta_manual_event_name,surface,time_spent_threshold,novel_id,ad_platform,tiktok_pixel_id,tiktok_pixel_code,tiktok_ttclid,tiktok_ttp,tiktok_adgroup_id,tiktok_creative_id,tiktok_ad_id_v2,tiktok_placement,tiktok_context_cipher,audio_novel_id,audio_novel_title)
	 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,COALESCE(t.account_id,''),COALESCE(t.enabled,false),t.pixel_record_id,COALESCE(t.pageview_enabled,false),COALESCE(t.manual_enabled,false),COALESCE(t.auto_enabled,false),COALESCE(t.manual_event_name,'Contact'),$26,$27,$28,$30,tt.id,COALESCE(tt.pixel_code,''),$31,$32,$33,$34,$35,$36,$37,$38,COALESCE((SELECT title FROM audio_novels WHERE id=$38),'') FROM (SELECT 1) anchor LEFT JOIN target t ON true LEFT JOIN tiktok_target tt ON true`, v.ID, v.LinkID, v.VisitorID, v.CookieStatus, v.Method, v.TargetURL, v.Device, v.OS, v.Browser, v.Country, v.Region, v.City, v.Source, v.CampaignID, v.AdsetID, v.AdID, v.Referrer, v.Parameters, v.AttributionConflict, v.Classification, v.Reason, v.Type, v.Status, v.MetaConnectionID, v.MetaPixelID, v.Surface, v.TimeSpentThreshold, v.NovelID, v.TikTokPixelID, v.AdPlatform, v.TikTokTTCLID, v.TikTokTTP, v.TikTokAdgroupID, v.TikTokCreativeID, v.TikTokAdIDV2, v.TikTokPlacement, v.TikTokContextCipher, v.AudioNovelID)
	return e
}

// FreezeAudioNovelLink serializes the first reader against admin edits and
// refuses a binding whose media can no longer be played.
func (r Repository) FreezeAudioNovelLink(ctx context.Context, linkID int64) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var enabled bool
	var audioNovelID, metaConnectionID, metaPixelID, tikTokPixelID *int64
	var platform string
	if err = tx.QueryRow(ctx, `SELECT enabled,audio_novel_id,ad_platform,meta_connection_id,meta_pixel_id,tiktok_pixel_id
		FROM short_links WHERE id=$1 AND product_type='audio_novel' FOR UPDATE`, linkID).
		Scan(&enabled, &audioNovelID, &platform, &metaConnectionID, &metaPixelID, &tikTokPixelID); err != nil {
		return err
	}
	validBinding := enabled && audioNovelID != nil &&
		((platform == "meta" && metaConnectionID != nil && metaPixelID != nil && tikTokPixelID == nil) ||
			(platform == "tiktok" && tikTokPixelID != nil && metaConnectionID == nil && metaPixelID == nil))
	if !validBinding {
		return pgx.ErrNoRows
	}
	var playable bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM audio_novels
		WHERE id=$1 AND enabled AND deleted_at IS NULL AND audio_path<>'')`, audioNovelID).Scan(&playable); err != nil {
		return err
	}
	if !playable {
		return pgx.ErrNoRows
	}
	if _, err = tx.Exec(ctx, "UPDATE short_links SET first_visited_at=COALESCE(first_visited_at,now()) WHERE id=$1", linkID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// FreezeNovelLink permanently locks the current novel binding before its final tracking snapshot is read.
func (r Repository) FreezeNovelLink(ctx context.Context, linkID int64) error {
	result, err := r.DB.Exec(ctx, `UPDATE short_links l
		SET first_visited_at=COALESCE(l.first_visited_at,now())
		WHERE l.id=$1 AND l.product_type='novel' AND l.enabled
		AND EXISTS (
			SELECT 1 FROM novels n
			WHERE n.id=l.novel_id AND n.enabled AND n.deleted_at IS NULL
		)`, linkID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
