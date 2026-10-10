package analytics

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/platform/runtime"
)

// DeliveryEvent exposes delivery diagnostics without returning encrypted request payloads.
type DeliveryEvent struct {
	Platform          string          `json:"platform"`
	ID                string          `json:"id"`
	EventName         string          `json:"event_name"`
	EventTime         time.Time       `json:"event_time"`
	IsTest            bool            `json:"is_test"`
	Status            string          `json:"status"`
	Attempts          int             `json:"attempts"`
	RetryCount        int             `json:"retry_count"`
	AcceptedCount     int             `json:"accepted_count"`
	ExternalRequestID string          `json:"external_request_id"`
	HTTPStatus        int             `json:"http_status"`
	BusinessCode      int64           `json:"business_code"`
	LastError         string          `json:"last_error"`
	ResponseMessages  json.RawMessage `json:"response_messages"`
	CreatedAt         time.Time       `json:"created_at"`
	UpdatedAt         time.Time       `json:"updated_at"`
	ConnectionID      int64           `json:"connection_id"`
	ConnectionName    string          `json:"connection_name"`
	PixelRecordID     int64           `json:"pixel_record_id"`
	PixelName         string          `json:"pixel_name"`
	PixelCode         string          `json:"pixel_code"`
	PayloadAvailable  bool            `json:"payload_available"`
}

// VisitDetail contains the immutable visit snapshot and safe delivery results used for troubleshooting.
type VisitDetail struct {
	Event
	// Structured versions are stored separately so operations can inspect devices without receiving a raw User-Agent.
	DeviceModel            string            `json:"device_model"`
	OSVersion              string            `json:"os_version"`
	BrowserVersion         string            `json:"browser_version"`
	ClientIP               string            `json:"client_ip"`
	UserAgent              string            `json:"user_agent"`
	RequestURL             string            `json:"request_url"`
	ReferrerURL            string            `json:"referrer_url"`
	AcceptLanguage         string            `json:"accept_language"`
	ClientHints            map[string]string `json:"client_hints"`
	LinkName               string            `json:"link_name"`
	TargetURL              string            `json:"target_url"`
	Status                 int               `json:"status"`
	CampaignID             string            `json:"campaign_id"`
	AdsetID                string            `json:"adset_id"`
	Parameters             map[string]string `json:"parameters"`
	RuleVersion            string            `json:"rule_version"`
	MetaConnectionID       *int64            `json:"meta_connection_id"`
	MetaConnectionName     string            `json:"meta_connection_name"`
	MetaAccountID          string            `json:"meta_account_id"`
	MetaMeasurement        bool              `json:"meta_measurement"`
	MetaPixelID            *int64            `json:"meta_pixel_id"`
	MetaPixelName          string            `json:"meta_pixel_name"`
	MetaPageViewEnabled    bool              `json:"meta_pageview_enabled"`
	MetaManualEnabled      bool              `json:"meta_manual_enabled"`
	MetaAutoEnabled        bool              `json:"meta_auto_enabled"`
	MetaManualEventName    string            `json:"meta_manual_event_name"`
	PageViewReportedAt     *time.Time        `json:"pageview_reported_at"`
	TimeSpentThreshold     int               `json:"time_spent_threshold"`
	TimeSpentReportedAt    *time.Time        `json:"time_spent_reported_at"`
	NovelID                *int64            `json:"novel_id"`
	NovelTitle             string            `json:"novel_title"`
	VisibleSeconds         int               `json:"visible_seconds"`
	VisibleUpdatedAt       *time.Time        `json:"visible_updated_at"`
	AdPlatform             string            `json:"ad_platform"`
	TikTokPixelID          *int64            `json:"tiktok_pixel_id"`
	TikTokPixelName        string            `json:"tiktok_pixel_name"`
	TikTokPixelCode        string            `json:"tiktok_pixel_code"`
	TikTokTTCLID           string            `json:"tiktok_ttclid"`
	TikTokTTP              string            `json:"tiktok_ttp"`
	TikTokAdgroupID        string            `json:"tiktok_adgroup_id"`
	TikTokCreativeID       string            `json:"tiktok_creative_id"`
	TikTokAdIDV2           string            `json:"tiktok_ad_id_v2"`
	TikTokPlacement        string            `json:"tiktok_placement"`
	TikTokContextAvailable bool              `json:"tiktok_context_available"`
	TikTokStartReadingAt   *time.Time        `json:"tiktok_start_reading_at"`
	TikTokViewContentAt    *time.Time        `json:"tiktok_view_content_at"`
	AudioNovelID           *int64            `json:"audio_novel_id"`
	PlaybackSeconds        int               `json:"playback_seconds"`
	MediaConsumedSeconds   float64           `json:"media_consumed_seconds"`
	PlaybackUpdatedAt      *time.Time        `json:"playback_updated_at"`
	AudioStartedAt         *time.Time        `json:"audio_started_at"`
	AudioQualifiedAt       *time.Time        `json:"audio_qualified_at"`
	AudioCompletedAt       *time.Time        `json:"audio_completed_at"`
	AudioNovelTitle        string            `json:"audio_novel_title"`
	EntryChapterID         *int64            `json:"entry_chapter_id"`
	EntryChapterNumber     *int              `json:"entry_chapter_number"`
	EntryChapterTitle      string            `json:"entry_chapter_title"`
	StartupTheme           string            `json:"startup_theme"`
	DeliveryEvents         []DeliveryEvent   `json:"delivery_events"`
}

// detailEventCols is intentionally independent from the visit-list projection.
// Detail appends fields in a fixed order, so expanding the list must not silently change its Scan contract.
const detailEventCols = `e.id,e.occurred_at,e.link_id,l.code,coalesce(e.visitor_id,''),e.cookie_status,e.method,
	e.device,e.os,e.browser,e.country,e.region,e.city,e.source,e.ad_id,e.classification,e.reason,e.referrer,
	e.attribution_conflict,e.event_type,e.surface,e.whatsapp_clicked_at,e.auto_redirected_at`

func (r Repository) Detail(ctx context.Context, id string) (VisitDetail, error) {
	var v VisitDetail
	var parameters string
	var clientHints string
	// Current names are joined for readability; every tracking and behavior value comes from the frozen visit row.
	err := r.DB.QueryRow(ctx, `SELECT `+detailEventCols+`,e.device_model,e.os_version,e.browser_version,e.client_ip,e.user_agent,e.request_url,
		e.referrer_url,e.accept_language,e.client_hints::text,l.name,e.target_url,e.status,e.campaign_id,e.adset_id,e.parameters::text,e.rule_version,
		e.meta_connection_id,COALESCE(mc.name,''),e.meta_account_id,e.meta_measurement,e.meta_pixel_id,COALESCE(mp.name,''),
		e.meta_pageview_enabled,e.meta_manual_enabled,e.meta_auto_enabled,e.meta_manual_event_name,e.pageview_reported_at,
		e.time_spent_threshold,e.time_spent_reported_at,e.novel_id,COALESCE(n.title,''),e.visible_seconds,e.visible_updated_at,
		e.ad_platform,e.tiktok_pixel_id,COALESCE(tp.name,''),e.tiktok_pixel_code,e.tiktok_ttclid,e.tiktok_ttp,
		e.tiktok_adgroup_id,e.tiktok_creative_id,e.tiktok_ad_id_v2,e.tiktok_placement,(e.tiktok_context_cipher<>''),
		e.tiktok_start_reading_at,e.tiktok_view_content_at,e.audio_novel_id,e.playback_seconds,e.media_consumed_seconds::float8,
		e.playback_updated_at,e.audio_started_at,e.audio_qualified_at,e.audio_completed_at,e.audio_novel_title,
		e.entry_chapter_id,e.entry_chapter_number,e.entry_chapter_title,e.startup_theme
		FROM click_events e
		JOIN short_links l ON l.id=e.link_id
		LEFT JOIN meta_connections mc ON mc.id=e.meta_connection_id
		LEFT JOIN meta_pixels mp ON mp.id=e.meta_pixel_id
		LEFT JOIN tiktok_pixels tp ON tp.id=e.tiktok_pixel_id
		LEFT JOIN novels n ON n.id=e.novel_id
		WHERE e.id=$1`, id).Scan(
		&v.ID, &v.OccurredAt, &v.LinkID, &v.Code, &v.VisitorID, &v.CookieStatus, &v.Method,
		&v.Device, &v.OS, &v.Browser, &v.Country, &v.Region, &v.City, &v.Source, &v.AdID,
		&v.Classification, &v.Reason, &v.Referrer, &v.AttributionConflict, &v.EventType, &v.Surface,
		&v.WhatsAppClickedAt, &v.AutoRedirectedAt, &v.DeviceModel, &v.OSVersion, &v.BrowserVersion,
		&v.ClientIP, &v.UserAgent, &v.RequestURL, &v.ReferrerURL, &v.AcceptLanguage, &clientHints,
		&v.LinkName, &v.TargetURL, &v.Status, &v.CampaignID,
		&v.AdsetID, &parameters, &v.RuleVersion, &v.MetaConnectionID, &v.MetaConnectionName,
		&v.MetaAccountID, &v.MetaMeasurement, &v.MetaPixelID, &v.MetaPixelName, &v.MetaPageViewEnabled,
		&v.MetaManualEnabled, &v.MetaAutoEnabled, &v.MetaManualEventName, &v.PageViewReportedAt,
		&v.TimeSpentThreshold, &v.TimeSpentReportedAt, &v.NovelID, &v.NovelTitle, &v.VisibleSeconds,
		&v.VisibleUpdatedAt, &v.AdPlatform, &v.TikTokPixelID, &v.TikTokPixelName, &v.TikTokPixelCode,
		&v.TikTokTTCLID, &v.TikTokTTP, &v.TikTokAdgroupID, &v.TikTokCreativeID, &v.TikTokAdIDV2,
		&v.TikTokPlacement, &v.TikTokContextAvailable, &v.TikTokStartReadingAt, &v.TikTokViewContentAt,
		&v.AudioNovelID, &v.PlaybackSeconds, &v.MediaConsumedSeconds, &v.PlaybackUpdatedAt,
		&v.AudioStartedAt, &v.AudioQualifiedAt, &v.AudioCompletedAt, &v.AudioNovelTitle,
		&v.EntryChapterID, &v.EntryChapterNumber, &v.EntryChapterTitle, &v.StartupTheme,
	)
	if err != nil {
		return v, err
	}
	v.Parameters = map[string]string{}
	if err = json.Unmarshal([]byte(parameters), &v.Parameters); err != nil {
		return v, err
	}
	v.ClientHints = map[string]string{}
	// Empty historical snapshots remain valid JSON and display as "未采集" in the admin console.
	if err = json.Unmarshal([]byte(clientHints), &v.ClientHints); err != nil {
		return v, err
	}

	// Both platforms share one diagnostic shape; encrypted payloads stay server-side.
	rows, err := r.DB.Query(ctx, `SELECT platform,id,event_name,event_time,is_test,status,attempts,retry_count,accepted_count,
		external_request_id,http_status,business_code,last_error,response_messages,created_at,updated_at,
		connection_id,connection_name,pixel_record_id,pixel_name,pixel_code,payload_available FROM (
		SELECT 'meta'::text AS platform,me.id,me.event_name,me.event_time,me.is_test,me.status,me.attempts,me.retry_count,
			me.events_received AS accepted_count,me.fbtrace_id AS external_request_id,0 AS http_status,0::bigint AS business_code,
			me.last_error,COALESCE(me.response_messages,'[]'::jsonb)::text AS response_messages,me.created_at,me.updated_at,
			me.connection_id,COALESCE(mc.name,'') AS connection_name,COALESCE(me.pixel_record_id,0) AS pixel_record_id,
			COALESCE(mp.name,'') AS pixel_name,me.pixel_id AS pixel_code,(me.payload_cipher<>'') AS payload_available
		FROM meta_events me
		LEFT JOIN meta_connections mc ON mc.id=me.connection_id
		LEFT JOIN meta_pixels mp ON mp.id=me.pixel_record_id
		WHERE me.visit_id=$1
		UNION ALL
		SELECT 'tiktok'::text,te.id,te.event_name,te.event_time,te.is_test,te.status,te.attempts,0,0,
			te.request_id,te.http_status,te.business_code,te.last_error,'[]'::text,te.created_at,te.updated_at,
			te.connection_id,COALESCE(tc.name,''),te.pixel_record_id,COALESCE(tp.name,''),te.pixel_code,(te.payload_cipher<>'')
		FROM tiktok_events te
		LEFT JOIN tiktok_connections tc ON tc.id=te.connection_id
		LEFT JOIN tiktok_pixels tp ON tp.id=te.pixel_record_id
		WHERE te.visit_id=$1
	) delivery ORDER BY created_at,id`, id)
	if err != nil {
		return v, err
	}
	defer rows.Close()
	v.DeliveryEvents = []DeliveryEvent{}
	for rows.Next() {
		var item DeliveryEvent
		var responseMessages string
		if err = rows.Scan(&item.Platform, &item.ID, &item.EventName, &item.EventTime, &item.IsTest,
			&item.Status, &item.Attempts, &item.RetryCount, &item.AcceptedCount, &item.ExternalRequestID,
			&item.HTTPStatus, &item.BusinessCode, &item.LastError, &responseMessages, &item.CreatedAt,
			&item.UpdatedAt, &item.ConnectionID, &item.ConnectionName, &item.PixelRecordID, &item.PixelName,
			&item.PixelCode, &item.PayloadAvailable); err != nil {
			return v, err
		}
		item.ResponseMessages = json.RawMessage(responseMessages)
		v.DeliveryEvents = append(v.DeliveryEvents, item)
	}
	return v, rows.Err()
}

func (a *Handler) ClickDetail(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).Detail(ctx, c.Param("id"))
	if err == pgx.ErrNoRows {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	// Detail access is audited because it includes anonymous visitor and advertising identifiers.
	if err = a.Audit(ctx, "clicks.detail.view", gin.H{"visit_id": item.ID}); err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}
