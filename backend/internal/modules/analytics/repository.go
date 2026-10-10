package analytics

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

// eventListQuery adds project-only filters to the shared visit list while the
// dashboard keeps its existing broad aggregation scope and positional SQL.
func eventListQuery(f Filter) (string, []any) {
	where, args := eventWhere, f.args()
	for _, item := range []struct {
		value, column string
	}{
		{f.CampaignID, "campaign_id"},
		{f.AdgroupID, "tiktok_adgroup_id"},
		{f.CreativeID, "tiktok_creative_id"},
		{f.AdIDV2, "tiktok_ad_id_v2"},
	} {
		if item.value == "" {
			continue
		}
		args = append(args, item.value)
		where += fmt.Sprintf(" AND e.%s=$%d", item.column, len(args))
	}
	validVisit := `e.classification='normal' AND e.method='GET' AND
		((e.surface='short_link' AND e.event_type IN ('landing','redirect')) OR
		(e.surface IN ('audio_novel','novel','cover') AND e.event_type='landing'))`
	if f.TrafficScope == "valid" {
		where += " AND (" + validVisit + ")"
	} else if f.TrafficScope == "abnormal" {
		where += " AND NOT (" + validVisit + ")"
	}
	if f.EventStatus != "" {
		args = append(args, f.EventStatus)
		where += fmt.Sprintf(` AND (EXISTS(SELECT 1 FROM meta_events me_filter
			WHERE me_filter.visit_id=e.id AND NOT me_filter.is_test AND me_filter.status=$%d)
			OR EXISTS(SELECT 1 FROM tiktok_events te_filter
			WHERE te_filter.visit_id=e.id AND NOT te_filter.is_test AND te_filter.status=$%d))`, len(args), len(args))
	}
	return where, args
}

func (a Repository) Events(ctx context.Context, f Filter, limit, offset int) ([]Event, error) {
	where, args := eventListQuery(f)
	args = append(args, limit, offset)
	rows, e := a.DB.Query(ctx, "SELECT "+eventCols+" FROM click_events e JOIN short_links l ON l.id=e.link_id"+where+
		fmt.Sprintf(" ORDER BY e.occurred_at DESC,e.id DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	out := []Event{}
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.OccurredAt, &v.LinkID, &v.Code, &v.VisitorID, &v.CookieStatus, &v.Method, &v.Device, &v.OS, &v.Browser, &v.Country, &v.Region, &v.City, &v.Source, &v.AdID, &v.Classification, &v.Reason, &v.Referrer, &v.AttributionConflict, &v.EventType, &v.Surface, &v.WhatsAppClickedAt, &v.AutoRedirectedAt,
			&v.AdPlatform, &v.CampaignID, &v.AdsetID, &v.AdgroupID, &v.CreativeID, &v.AdIDV2, &v.Placement,
			&v.VisibleSeconds, &v.PlaybackSeconds, &v.MediaConsumedSeconds, &v.AudioStarted, &v.AudioQualified, &v.AudioCompleted,
			&v.EntryChapterID, &v.EntryChapterNumber, &v.EntryChapterTitle, &v.ReadingStarted, &v.ReadingQualified); e != nil {
			return out, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

type OverviewData struct {
	Summary                     Summary
	Trends                      []Trend
	Devices, Countries, Sources []Slice
	Ads                         []Ad
}

func (r Repository) Overview(ctx context.Context, f Filter) (OverviewData, error) {
	args := f.args()
	var s Summary
	// Available link totals now follow the same explicit enabled/disabled state
	// shown in link management.
	// User-facing visits include both rendered landing pages and direct handoffs;
	// manual consultations remain landing-only while automatic actions include both modes.
	e := r.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM short_links WHERE enabled),count(*),count(*) FILTER(WHERE classification='normal'),count(DISTINCT visitor_id) FILTER(WHERE classification='normal'),count(*) FILTER(WHERE classification='bot'),count(*) FILTER(WHERE classification='suspicious'),count(*) FILTER(WHERE classification='unclassified'),count(*) FILTER(WHERE visitor_id IS NULL),count(*) FILTER(WHERE classification='head'),count(*) FILTER(WHERE classification='prefetch'),count(*) FILTER(WHERE event_type IN ('landing','redirect') AND classification='normal'),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND whatsapp_clicked_at IS NOT NULL),count(*) FILTER(WHERE event_type IN ('landing','redirect') AND classification='normal' AND auto_redirected_at IS NOT NULL),count(*) FILTER(WHERE classification='normal' AND surface='short_link'),count(*) FILTER(WHERE classification='normal' AND surface='audio_novel'),count(*) FILTER(WHERE classification='normal' AND surface='novel'),count(*) FILTER(WHERE classification='normal' AND surface='short_link' AND whatsapp_clicked_at IS NOT NULL),count(*) FILTER(WHERE classification='normal' AND surface='audio_novel' AND whatsapp_clicked_at IS NOT NULL) FROM click_events e`+eventWhere, args...).Scan(&s.AvailableLinks, &s.Total, &s.Filtered, &s.Unique, &s.Bot, &s.Suspicious, &s.Unclassified, &s.NoCookie, &s.Head, &s.Prefetch, &s.LandingViews, &s.WhatsAppClicks, &s.AutoRedirects, &s.ShortLinkViews, &s.AudioNovelViews, &s.NovelViews, &s.ShortLinkWhatsAppClicks, &s.AudioNovelWhatsAppClicks)
	if e != nil {
		return OverviewData{}, e
	}
	// Group by local calendar boundaries. Hourly detail is available for one-day selections.
	format := "YYYY-MM-DD"
	if f.End.Sub(f.Start) <= 25*time.Hour {
		format = "YYYY-MM-DD HH24:00"
	}
	rows, e := r.DB.Query(ctx, `SELECT to_char(occurred_at AT TIME ZONE $6,$7),count(*),count(*) FILTER(WHERE classification='normal'),count(DISTINCT visitor_id) FILTER(WHERE classification='normal'),count(*) FILTER(WHERE event_type IN ('landing','redirect') AND classification='normal'),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND whatsapp_clicked_at IS NOT NULL),count(*) FILTER(WHERE event_type IN ('landing','redirect') AND classification='normal' AND auto_redirected_at IS NOT NULL) FROM click_events e`+eventWhere+` GROUP BY 1 ORDER BY 1`, append(args, f.TZ, format)...)
	if e != nil {
		return OverviewData{}, e
	}
	trends := []Trend{}
	for rows.Next() {
		var t Trend
		if e = rows.Scan(&t.Date, &t.Total, &t.Filtered, &t.Unique, &t.LandingViews, &t.WhatsAppClicks, &t.AutoRedirects); e != nil {
			break
		}
		trends = append(trends, t)
	}
	re := rows.Err()
	rows.Close()
	if e != nil || re != nil {
		return OverviewData{}, errors.Join(e, re)
	}
	breakdown := func(col string) ([]Slice, error) {
		out := []Slice{}
		rows, e := r.DB.Query(ctx, "SELECT "+col+",count(*) FROM click_events e"+eventWhere+" AND classification='normal' GROUP BY 1 ORDER BY 2 DESC LIMIT 100", args...)
		if e != nil {
			return out, e
		}
		defer rows.Close()
		for rows.Next() {
			var v Slice
			if e = rows.Scan(&v.Name, &v.Count); e != nil {
				return out, e
			}
			out = append(out, v)
		}
		return out, rows.Err()
	}
	devices, e := breakdown("device")
	if e != nil {
		return OverviewData{}, e
	}
	countries, e := breakdown("country")
	if e != nil {
		return OverviewData{}, e
	}
	sources, e := breakdown("source")
	if e != nil {
		return OverviewData{}, e
	}
	// Spend is only comparable for its configured reporting timezone; currencies remain separate.
	query := `WITH clicks AS (SELECT ad_id,count(*) AS total,count(*) FILTER(WHERE classification='normal') AS filtered,count(DISTINCT visitor_id) FILTER(WHERE classification='normal') AS uv FROM click_events e` + eventWhere + ` GROUP BY ad_id), spend AS (SELECT ad_id,currency,CASE WHEN count(DISTINCT date)=(($2 AT TIME ZONE $6)::date-($1 AT TIME ZONE $6)::date) THEN sum(amount)::float8 ELSE NULL END cost FROM ad_spend_daily WHERE date>=($1 AT TIME ZONE $6)::date AND date<($2 AT TIME ZONE $6)::date AND time_zone=$6 AND ($4::text='' OR ad_id=$4) GROUP BY ad_id,currency) SELECT coalesce(c.ad_id,s.ad_id),coalesce(c.total,0),coalesce(c.filtered,0),coalesce(c.uv,0),CASE WHEN $3::bigint=0 THEN s.cost ELSE NULL END,coalesce(s.currency,'') FROM clicks c FULL OUTER JOIN spend s ON c.ad_id=s.ad_id WHERE $3::bigint=0 OR c.ad_id IS NOT NULL ORDER BY coalesce(c.total,0) DESC LIMIT 500`
	rows, e = r.DB.Query(ctx, query, append(args, f.TZ)...)
	if e != nil {
		return OverviewData{}, e
	}
	ads := []Ad{}
	for rows.Next() {
		var v Ad
		if e = rows.Scan(&v.AdID, &v.Total, &v.Filtered, &v.Unique, &v.Cost, &v.Currency); e != nil {
			break
		}
		ads = append(ads, v)
	}
	re = rows.Err()
	rows.Close()
	if e != nil || re != nil {
		return OverviewData{}, errors.Join(e, re)
	}
	return OverviewData{Summary: s, Trends: trends, Devices: devices, Countries: countries, Sources: sources, Ads: ads}, nil
}
func (r Repository) Count(ctx context.Context, f Filter) (int64, error) {
	var total int64
	where, args := eventListQuery(f)
	e := r.DB.QueryRow(ctx, "SELECT count(*) FROM click_events e"+where, args...).Scan(&total)
	return total, e
}
