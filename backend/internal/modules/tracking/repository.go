package tracking

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }
type Event struct {
	MetaPixelID                                                                                                              *int64
	MetaConnectionID                                                                                                         *int64
	ID                                                                                                                       string
	LinkID                                                                                                                   int64
	VisitorID                                                                                                                any
	CookieStatus, Method, TargetURL, Device, OS, Browser, Country, Region, City, Source, CampaignID, AdsetID, AdID, Referrer string
	Parameters                                                                                                               []byte
	AttributionConflict                                                                                                      bool
	Classification, Reason, Type                                                                                             string
	Surface                                                                                                                  string
	Status                                                                                                                   int
	TimeSpentThreshold                                                                                                       int
}

func (r Repository) Record(ctx context.Context, v Event) error {
	_, e := r.DB.Exec(ctx, `WITH target AS (
 -- The account supplies grouping and API metadata; the selected Pixel owns the
 -- credential and all event switches used to freeze this visit's routing.
	-- Manual and automatic consultation switches are frozen independently so later
	-- Pixel edits cannot rewrite how an existing visit is delivered.
 SELECT c.account_id,p.id AS pixel_record_id,p.enabled,p.manual_enabled,p.auto_enabled,p.pageview_enabled,p.manual_event_name FROM meta_connections c
 LEFT JOIN LATERAL (SELECT * FROM meta_pixels p WHERE p.connection_id=c.id AND ($25::bigint IS NULL OR p.id=$25) ORDER BY (p.pixel_id=c.pixel_id) DESC,p.id LIMIT 1) p ON true WHERE c.id=$24
 ) INSERT INTO click_events(id,link_id,visitor_id,cookie_status,method,target_url,device,os,browser,country,region,city,source,campaign_id,adset_id,ad_id,referrer,parameters,attribution_conflict,classification,reason,event_type,status,meta_connection_id,meta_account_id,meta_measurement,meta_pixel_id,meta_pageview_enabled,meta_manual_enabled,meta_auto_enabled,meta_manual_event_name,surface,time_spent_threshold)
 SELECT $1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,COALESCE(t.account_id,''),COALESCE(t.enabled,false),t.pixel_record_id,COALESCE(t.pageview_enabled,false),COALESCE(t.manual_enabled,false),COALESCE(t.auto_enabled,false),COALESCE(t.manual_event_name,'Contact'),$26,$27 FROM (SELECT 1) anchor LEFT JOIN target t ON true`, v.ID, v.LinkID, v.VisitorID, v.CookieStatus, v.Method, v.TargetURL, v.Device, v.OS, v.Browser, v.Country, v.Region, v.City, v.Source, v.CampaignID, v.AdsetID, v.AdID, v.Referrer, v.Parameters, v.AttributionConflict, v.Classification, v.Reason, v.Type, v.Status, v.MetaConnectionID, v.MetaPixelID, v.Surface, v.TimeSpentThreshold)
	return e
}
