package analytics

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

func (a Repository) Events(ctx context.Context, f Filter, limit, offset int) ([]Event, error) {
	rows, e := a.DB.Query(ctx, "SELECT "+eventCols+" FROM click_events e JOIN short_links l ON l.id=e.link_id"+eventWhere+" ORDER BY e.occurred_at DESC,e.id LIMIT $5 OFFSET $6", append(f.args(), limit, offset)...)
	out := []Event{}
	if e != nil {
		return out, e
	}
	defer rows.Close()
	for rows.Next() {
		var v Event
		if e = rows.Scan(&v.ID, &v.OccurredAt, &v.LinkID, &v.Code, &v.VisitorID, &v.CookieStatus, &v.Method, &v.Device, &v.OS, &v.Browser, &v.Country, &v.Region, &v.City, &v.Source, &v.AdID, &v.Classification, &v.Reason, &v.Referrer, &v.AttributionConflict, &v.EventType, &v.WhatsAppClickedAt, &v.AutoRedirectedAt); e != nil {
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
	e := r.DB.QueryRow(ctx, `SELECT (SELECT count(*) FROM short_links WHERE enabled),count(*),count(*) FILTER(WHERE classification='normal'),count(DISTINCT visitor_id) FILTER(WHERE classification='normal'),count(*) FILTER(WHERE classification='bot'),count(*) FILTER(WHERE classification='suspicious'),count(*) FILTER(WHERE classification='unclassified'),count(*) FILTER(WHERE visitor_id IS NULL),count(*) FILTER(WHERE classification='head'),count(*) FILTER(WHERE classification='prefetch'),count(*) FILTER(WHERE event_type='landing' AND classification='normal'),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND whatsapp_clicked_at IS NOT NULL),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND auto_redirected_at IS NOT NULL) FROM click_events e`+eventWhere, args...).Scan(&s.AvailableLinks, &s.Total, &s.Filtered, &s.Unique, &s.Bot, &s.Suspicious, &s.Unclassified, &s.NoCookie, &s.Head, &s.Prefetch, &s.LandingViews, &s.WhatsAppClicks, &s.AutoRedirects)
	if e != nil {
		return OverviewData{}, e
	}
	// Group by local calendar boundaries. Hourly detail is available for one-day selections.
	format := "YYYY-MM-DD"
	if f.End.Sub(f.Start) <= 25*time.Hour {
		format = "YYYY-MM-DD HH24:00"
	}
	rows, e := r.DB.Query(ctx, `SELECT to_char(occurred_at AT TIME ZONE $5,$6),count(*),count(*) FILTER(WHERE classification='normal'),count(DISTINCT visitor_id) FILTER(WHERE classification='normal'),count(*) FILTER(WHERE event_type='landing' AND classification='normal'),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND whatsapp_clicked_at IS NOT NULL),count(*) FILTER(WHERE event_type='landing' AND classification='normal' AND auto_redirected_at IS NOT NULL) FROM click_events e`+eventWhere+` GROUP BY 1 ORDER BY 1`, append(args, f.TZ, format)...)
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
	query := `WITH clicks AS (SELECT ad_id,count(*) AS total,count(*) FILTER(WHERE classification='normal') AS filtered,count(DISTINCT visitor_id) FILTER(WHERE classification='normal') AS uv FROM click_events e` + eventWhere + ` GROUP BY ad_id), spend AS (SELECT ad_id,currency,CASE WHEN count(DISTINCT date)=(($2 AT TIME ZONE $5)::date-($1 AT TIME ZONE $5)::date) THEN sum(amount)::float8 ELSE NULL END cost FROM ad_spend_daily WHERE date>=($1 AT TIME ZONE $5)::date AND date<($2 AT TIME ZONE $5)::date AND time_zone=$5 AND ($4::text='' OR ad_id=$4) GROUP BY ad_id,currency) SELECT coalesce(c.ad_id,s.ad_id),coalesce(c.total,0),coalesce(c.filtered,0),coalesce(c.uv,0),CASE WHEN $3::bigint=0 THEN s.cost ELSE NULL END,coalesce(s.currency,'') FROM clicks c FULL OUTER JOIN spend s ON c.ad_id=s.ad_id WHERE $3::bigint=0 OR c.ad_id IS NOT NULL ORDER BY coalesce(c.total,0) DESC LIMIT 500`
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
	e := r.DB.QueryRow(ctx, "SELECT count(*) FROM click_events e"+eventWhere, f.args()...).Scan(&total)
	return total, e
}
