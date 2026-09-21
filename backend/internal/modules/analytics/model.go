package analytics

import (
	"time"
)

type Filter struct {
	Start, End time.Time
	TZ         string
	LinkID     int64
	AdID       string
	Surface    string
}

const eventWhere = " WHERE e.occurred_at >= $1 AND e.occurred_at < $2 AND ($3::bigint=0 OR e.link_id=$3) AND ($4::text='' OR e.ad_id=$4) AND ($5::text='' OR e.surface=$5) "

func (f Filter) args() []any { return []any{f.Start, f.End, f.LinkID, f.AdID, f.Surface} }

type Summary struct {
	AvailableLinks           int64 `json:"available_links"`
	AutoRedirects            int64 `json:"auto_redirects"`
	LandingViews             int64 `json:"landing_views"`
	WhatsAppClicks           int64 `json:"whatsapp_clicks"`
	ShortLinkViews           int64 `json:"short_link_views"`
	AudioNovelViews          int64 `json:"audio_novel_views"`
	ShortLinkWhatsAppClicks  int64 `json:"short_link_whatsapp_clicks"`
	AudioNovelWhatsAppClicks int64 `json:"audio_novel_whatsapp_clicks"`
	Total                    int64 `json:"total"`
	Filtered                 int64 `json:"filtered"`
	Unique                   int64 `json:"unique"`
	Bot                      int64 `json:"bot"`
	Suspicious               int64 `json:"suspicious"`
	Unclassified             int64 `json:"unclassified"`
	NoCookie                 int64 `json:"no_cookie"`
	Head                     int64 `json:"head"`
	Prefetch                 int64 `json:"prefetch"`
}

type Trend struct {
	AutoRedirects  int64  `json:"auto_redirects"`
	LandingViews   int64  `json:"landing_views"`
	WhatsAppClicks int64  `json:"whatsapp_clicks"`
	Date           string `json:"date"`
	Total          int64  `json:"total"`
	Filtered       int64  `json:"filtered"`
	Unique         int64  `json:"unique"`
}

type Slice struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

type Ad struct {
	AdID     string   `json:"ad_id"`
	Total    int64    `json:"total"`
	Filtered int64    `json:"filtered"`
	Unique   int64    `json:"unique"`
	Cost     *float64 `json:"cost"`
	Currency string   `json:"currency"`
}

type Event struct {
	AutoRedirectedAt    *time.Time `json:"auto_redirected_at"`
	EventType           string     `json:"event_type"`
	Surface             string     `json:"surface"`
	WhatsAppClickedAt   *time.Time `json:"whatsapp_clicked_at"`
	ID                  string     `json:"id"`
	OccurredAt          time.Time  `json:"occurred_at"`
	LinkID              int64      `json:"link_id"`
	Code                string     `json:"code"`
	VisitorID           string     `json:"visitor_id"`
	CookieStatus        string     `json:"cookie_status"`
	Method              string     `json:"method"`
	Device              string     `json:"device"`
	OS                  string     `json:"os"`
	Browser             string     `json:"browser"`
	Country             string     `json:"country"`
	Region              string     `json:"region"`
	City                string     `json:"city"`
	Source              string     `json:"source"`
	AdID                string     `json:"ad_id"`
	Classification      string     `json:"classification"`
	Reason              string     `json:"reason"`
	Referrer            string     `json:"referrer"`
	AttributionConflict bool       `json:"attribution_conflict"`
}

const eventCols = "e.id,e.occurred_at,e.link_id,l.code,coalesce(e.visitor_id,''),e.cookie_status,e.method,e.device,e.os,e.browser,e.country,e.region,e.city,e.source,e.ad_id,e.classification,e.reason,e.referrer,e.attribution_conflict,e.event_type,e.surface,e.whatsapp_clicked_at,e.auto_redirected_at"
