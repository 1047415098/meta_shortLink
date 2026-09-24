package audionovel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

const defaultAudioStatsPageSize = 50

type distributionStatsRequest struct {
	Start       *string `json:"start"`
	End         *string `json:"end"`
	TZ          *string `json:"tz"`
	AdID        string  `json:"ad_id"`
	CampaignID  string  `json:"campaign_id"`
	AdgroupID   string  `json:"adgroup_id"`
	CreativeID  string  `json:"creative_id"`
	AdIDV2      string  `json:"ad_id_v2"`
	EventStatus string  `json:"event_status"`
	Page        int     `json:"page"`
	PageSize    int     `json:"page_size"`
}

type distributionStatsFilter struct {
	Start, End                                      time.Time
	Timezone                                        string
	AdID, CampaignID, AdgroupID, CreativeID, AdIDV2 string
	EventStatus                                     string
	Page, PageSize                                  int
}

type DistributionStatsSummary struct {
	Visits                 int64    `json:"visits"`
	UniqueVisitors         int64    `json:"unique_visitors"`
	CollectedVisits        int64    `json:"collected_visits"`
	AverageVisibleSeconds  *float64 `json:"average_visible_seconds"`
	TotalVisibleSeconds    *int64   `json:"total_visible_seconds"`
	StartedCount           int64    `json:"started_count"`
	StartedVisitors        int64    `json:"started_visitors"`
	QualifiedCount         int64    `json:"qualified_count"`
	QualifiedVisitors      int64    `json:"qualified_visitors"`
	CompletedCount         int64    `json:"completed_count"`
	CompletedVisitors      int64    `json:"completed_visitors"`
	AveragePlaybackSeconds float64  `json:"average_playback_seconds"`
	TotalPlaybackSeconds   int64    `json:"total_playback_seconds"`
	StartRate              float64  `json:"start_rate"`
	QualifiedRate          float64  `json:"qualified_rate"`
	CompletionRate         float64  `json:"completion_rate"`
	SavedEvents            int64    `json:"saved_events"`
	PendingEvents          int64    `json:"pending_events"`
	AcceptedEvents         int64    `json:"accepted_events"`
	FailedEvents           int64    `json:"failed_events"`
	SkippedEvents          int64    `json:"skipped_events"`
}

type DistributionVisit struct {
	ID                   string    `json:"id"`
	OccurredAt           time.Time `json:"occurred_at"`
	VisitorID            string    `json:"visitor_id"`
	Country              string    `json:"country"`
	Region               string    `json:"region"`
	City                 string    `json:"city"`
	Device               string    `json:"device"`
	Browser              string    `json:"browser"`
	Source               string    `json:"source"`
	AdID                 string    `json:"ad_id"`
	VisibleSeconds       *int      `json:"visible_seconds"`
	PlaybackSeconds      *int      `json:"playback_seconds"`
	MediaConsumedSeconds *float64  `json:"media_consumed_seconds"`
	AdPlatform           string    `json:"ad_platform"`
	PixelName            string    `json:"pixel_name"`
	PixelCode            string    `json:"pixel_code"`
	CampaignID           string    `json:"campaign_id"`
	AdgroupID            string    `json:"adgroup_id"`
	CreativeID           string    `json:"creative_id"`
	AdIDV2               string    `json:"ad_id_v2"`
	Placement            string    `json:"placement"`
	TTCLID               string    `json:"ttclid"`
	Started              bool      `json:"started"`
	Qualified            bool      `json:"qualified"`
	Completed            bool      `json:"completed"`
	PageViewEventStatus  string    `json:"pageview_event_status"`
	StartEventStatus     string    `json:"start_event_status"`
	QualifiedEventStatus string    `json:"qualified_event_status"`
}

type DistributionStatsResponse struct {
	Link                  DistributionLink         `json:"link"`
	DeliveryConfigStatus  string                   `json:"delivery_config_status"`
	DeliveryBlockedReason string                   `json:"delivery_blocked_reason,omitempty"`
	Summary               DistributionStatsSummary `json:"summary"`
	Items                 []DistributionVisit      `json:"items"`
	Total                 int64                    `json:"total"`
	Page                  int                      `json:"page"`
	PageSize              int                      `json:"page_size"`
	Timezone              string                   `json:"timezone"`
}

func parseDistributionStatsFilter(input distributionStatsRequest, fallbackTimezone string) (distributionStatsFilter, error) {
	// The allowlist matches the administration UI; the configured timezone is
	// also accepted so a deployment can retain its established reporting day.
	allowedTimezones := map[string]bool{
		"UTC": true, "Asia/Shanghai": true,
		"America/New_York": true, "America/Los_Angeles": true,
	}
	if fallbackTimezone == "" {
		fallbackTimezone = "UTC"
	}
	allowedTimezones[fallbackTimezone] = true
	timezone := fallbackTimezone
	if input.TZ != nil {
		timezone = strings.TrimSpace(*input.TZ)
	}
	if !allowedTimezones[timezone] {
		return distributionStatsFilter{}, errors.New("时区无效")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return distributionStatsFilter{}, errors.New("时区无效")
	}
	now := time.Now().In(location)
	startText := now.AddDate(0, 0, -6).Format("2006-01-02")
	endText := now.Format("2006-01-02")
	if input.Start != nil {
		startText = strings.TrimSpace(*input.Start)
	}
	if input.End != nil {
		endText = strings.TrimSpace(*input.End)
	}
	start, err := time.ParseInLocation("2006-01-02", startText, location)
	if err != nil {
		return distributionStatsFilter{}, errors.New("开始日期无效")
	}
	end, err := time.ParseInLocation("2006-01-02", endText, location)
	if err != nil {
		return distributionStatsFilter{}, errors.New("结束日期无效")
	}
	end = end.AddDate(0, 0, 1)
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour {
		return distributionStatsFilter{}, errors.New("请选择不超过 365 天的有效日期范围")
	}
	for _, value := range []string{input.AdID, input.CampaignID, input.AdgroupID, input.CreativeID, input.AdIDV2} {
		if len(value) > 120 {
			return distributionStatsFilter{}, errors.New("广告参数过长")
		}
	}
	validStatuses := map[string]bool{
		"": true, "pending": true, "processing": true, "sending": true,
		"succeeded": true, "accepted": true, "retry": true,
		"failed": true, "expired": true, "skipped": true,
	}
	if !validStatuses[input.EventStatus] || input.Page < 1 || input.Page > 100000 || input.PageSize < 0 {
		return distributionStatsFilter{}, errors.New("事件状态或页码无效")
	}
	pageSize := input.PageSize
	if pageSize == 0 {
		pageSize = defaultAudioStatsPageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return distributionStatsFilter{
		Start: start, End: end, Timezone: timezone,
		AdID: strings.TrimSpace(input.AdID), CampaignID: strings.TrimSpace(input.CampaignID),
		AdgroupID: strings.TrimSpace(input.AdgroupID), CreativeID: strings.TrimSpace(input.CreativeID),
		AdIDV2: strings.TrimSpace(input.AdIDV2), EventStatus: input.EventStatus,
		Page: input.Page, PageSize: pageSize,
	}, nil
}

func maskAudioTikTokID(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "…" + value[len(value)-4:]
}

// DistributionStats returns one immutable campaign link's traffic, playback
// funnel and selected advertising platform delivery state.
func (a *Handler) DistributionStats(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	input := distributionStatsRequest{Page: 1}
	if decodeAudioNovelJSON(c, &input) != nil {
		runtime.Bad(c, "请使用 JSON 提交统计条件")
		return
	}
	filter, err := parseDistributionStatsFilter(input, a.Config.Timezone)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	out, err := (Repository{DB: a.DB}).DistributionStats(ctx, id, filter, a.Config.TikTokEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	a.setDistributionPublicURL(&out.Link)
	c.JSON(200, out)
}

func (r Repository) DistributionStats(ctx context.Context, linkID int64, filter distributionStatsFilter, tikTokEnabled bool) (DistributionStatsResponse, error) {
	out := DistributionStatsResponse{
		Items: []DistributionVisit{}, Page: filter.Page, PageSize: filter.PageSize, Timezone: filter.Timezone,
	}
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out.Link, err = scanAudioDistribution(tx.QueryRow(ctx, "SELECT "+audioDistributionColumns+`
		FROM short_links l
		JOIN audio_novels n ON n.id=l.audio_novel_id
		LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id
		WHERE l.id=$1 AND l.product_type='audio_novel'`, linkID))
	if err != nil {
		return out, err
	}
	out.DeliveryConfigStatus, out.DeliveryBlockedReason, err = audioDeliveryConfigStatus(ctx, tx, out.Link, tikTokEnabled)
	if err != nil {
		return out, err
	}

	where := `e.link_id=$1 AND e.audio_novel_id=$2 AND e.surface='audio_novel'
		AND e.method='GET' AND e.event_type='landing' AND e.classification='normal'
		AND e.occurred_at>=$3 AND e.occurred_at<$4`
	args := []any{linkID, out.Link.AudioNovelID, filter.Start, filter.End}
	for _, item := range []struct {
		value, column string
	}{
		{filter.AdID, "ad_id"},
		{filter.CampaignID, "campaign_id"},
		{filter.AdgroupID, "tiktok_adgroup_id"},
		{filter.CreativeID, "tiktok_creative_id"},
		{filter.AdIDV2, "tiktok_ad_id_v2"},
	} {
		if item.value == "" {
			continue
		}
		args = append(args, item.value)
		where += fmt.Sprintf(" AND e.%s=$%d", item.column, len(args))
	}
	if filter.EventStatus != "" {
		args = append(args, filter.EventStatus)
		if out.Link.AdPlatform == "meta" {
			where += fmt.Sprintf(` AND EXISTS(SELECT 1 FROM meta_events me_filter
				WHERE me_filter.visit_id=e.id AND NOT me_filter.is_test
				AND me_filter.event_name IN ('PageView','StartListening','ViewContent')
				AND me_filter.status=$%d)`, len(args))
		} else {
			where += fmt.Sprintf(` AND EXISTS(SELECT 1 FROM tiktok_events te_filter
				WHERE te_filter.visit_id=e.id AND te_filter.link_id=e.link_id AND NOT te_filter.is_test
				AND te_filter.event_name IN ('StartListening','ViewContent')
				AND te_filter.status=$%d)`, len(args))
		}
	}
	filteredCTE := `WITH filtered AS (SELECT e.* FROM click_events e WHERE ` + where + `)`
	eventSummarySQL := audioEventSummarySQL(out.Link.AdPlatform)
	// Keep empty aggregates as NULL: the API must distinguish no visible-time
	// sample from a genuine collected value of zero.
	err = tx.QueryRow(ctx, filteredCTE+`, visit_summary AS (SELECT
		count(*),count(DISTINCT NULLIF(e.visitor_id,'')),
		count(*) FILTER(WHERE e.visible_updated_at IS NOT NULL),
		(avg(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL))::float8,
		sum(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),
		count(*) FILTER(WHERE e.audio_started_at IS NOT NULL),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.audio_started_at IS NOT NULL),
		count(*) FILTER(WHERE e.audio_qualified_at IS NOT NULL),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.audio_qualified_at IS NOT NULL),
		count(*) FILTER(WHERE e.audio_completed_at IS NOT NULL),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.audio_completed_at IS NOT NULL),
		COALESCE(avg(e.playback_seconds) FILTER(WHERE e.audio_started_at IS NOT NULL),0)::float8,
		COALESCE(sum(e.playback_seconds),0),
		CASE WHEN count(*)=0 THEN 0 ELSE count(*) FILTER(WHERE e.audio_started_at IS NOT NULL)*100.0/count(*) END::float8,
		CASE WHEN count(*) FILTER(WHERE e.audio_started_at IS NOT NULL)=0 THEN 0
			ELSE count(*) FILTER(WHERE e.audio_qualified_at IS NOT NULL)*100.0/count(*) FILTER(WHERE e.audio_started_at IS NOT NULL) END::float8,
		CASE WHEN count(*) FILTER(WHERE e.audio_started_at IS NOT NULL)=0 THEN 0
			ELSE count(*) FILTER(WHERE e.audio_completed_at IS NOT NULL)*100.0/count(*) FILTER(WHERE e.audio_started_at IS NOT NULL) END::float8
		FROM filtered e), event_summary AS (`+eventSummarySQL+`)
		SELECT visit_summary.*,event_summary.* FROM visit_summary CROSS JOIN event_summary`, args...).Scan(
		&out.Summary.Visits, &out.Summary.UniqueVisitors, &out.Summary.CollectedVisits,
		&out.Summary.AverageVisibleSeconds, &out.Summary.TotalVisibleSeconds,
		&out.Summary.StartedCount, &out.Summary.StartedVisitors,
		&out.Summary.QualifiedCount, &out.Summary.QualifiedVisitors,
		&out.Summary.CompletedCount, &out.Summary.CompletedVisitors,
		&out.Summary.AveragePlaybackSeconds, &out.Summary.TotalPlaybackSeconds,
		&out.Summary.StartRate, &out.Summary.QualifiedRate, &out.Summary.CompletionRate,
		&out.Summary.SavedEvents, &out.Summary.PendingEvents, &out.Summary.AcceptedEvents,
		&out.Summary.FailedEvents, &out.Summary.SkippedEvents,
	)
	if err != nil {
		return out, err
	}
	out.Total = out.Summary.Visits

	queryArgs := append([]any{}, args...)
	queryArgs = append(queryArgs, filter.PageSize, (filter.Page-1)*filter.PageSize)
	limitPlaceholder, offsetPlaceholder := len(queryArgs)-1, len(queryArgs)
	detailStatusSQL := audioDetailStatusSQL(out.Link.AdPlatform)
	rows, err := tx.Query(ctx, filteredCTE+` SELECT
		e.id,e.occurred_at,COALESCE(e.visitor_id,''),e.country,e.region,e.city,e.device,e.browser,e.source,e.ad_id,
		CASE WHEN e.visible_updated_at IS NULL THEN NULL ELSE e.visible_seconds END,
		CASE WHEN e.playback_updated_at IS NULL THEN NULL ELSE e.playback_seconds END,
		CASE WHEN e.playback_updated_at IS NULL THEN NULL ELSE e.media_consumed_seconds::float8 END,
		e.ad_platform,
		CASE WHEN e.ad_platform='meta' THEN COALESCE(mp.name,'') ELSE COALESCE(tp.name,'') END,
		CASE WHEN e.ad_platform='meta' THEN COALESCE(mp.pixel_id,'') ELSE COALESCE(NULLIF(e.tiktok_pixel_code,''),tp.pixel_code,'') END,
		e.campaign_id,e.tiktok_adgroup_id,e.tiktok_creative_id,e.tiktok_ad_id_v2,e.tiktok_placement,e.tiktok_ttclid,
		e.audio_started_at IS NOT NULL,e.audio_qualified_at IS NOT NULL,e.audio_completed_at IS NOT NULL,
		statuses.pageview_status,statuses.start_status,statuses.qualified_status
		FROM filtered e
		LEFT JOIN meta_pixels mp ON mp.id=e.meta_pixel_id
		LEFT JOIN tiktok_pixels tp ON tp.id=e.tiktok_pixel_id
		CROSS JOIN LATERAL (`+detailStatusSQL+`) statuses
		ORDER BY e.occurred_at DESC,e.id DESC`+
		fmt.Sprintf(" LIMIT $%d OFFSET $%d", limitPlaceholder, offsetPlaceholder), queryArgs...)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item DistributionVisit
		if err = rows.Scan(
			&item.ID, &item.OccurredAt, &item.VisitorID, &item.Country, &item.Region, &item.City,
			&item.Device, &item.Browser, &item.Source, &item.AdID, &item.VisibleSeconds,
			&item.PlaybackSeconds, &item.MediaConsumedSeconds, &item.AdPlatform, &item.PixelName,
			&item.PixelCode, &item.CampaignID, &item.AdgroupID, &item.CreativeID, &item.AdIDV2,
			&item.Placement, &item.TTCLID, &item.Started, &item.Qualified, &item.Completed,
			&item.PageViewEventStatus, &item.StartEventStatus, &item.QualifiedEventStatus,
		); err != nil {
			return out, err
		}
		// The full click identifier is sensitive and is never returned to admins.
		item.TTCLID = maskAudioTikTokID(item.TTCLID)
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if err = tx.Commit(ctx); err != nil {
		return out, err
	}
	return out, nil
}

func audioDeliveryConfigStatus(ctx context.Context, tx pgx.Tx, link DistributionLink, tikTokEnabled bool) (string, string, error) {
	// Only fixed, non-secret diagnostics leave this function. Ciphertext, tokens
	// and provider error payloads remain confined to their configuration tables.
	switch link.AdPlatform {
	case "meta":
		if link.MetaConnectionID == nil || link.MetaPixelID == nil {
			return "blocked", "Meta 回传目标未绑定，请重新创建投放链接", nil
		}
		var enabled, hasCredential bool
		var credentialStatus string
		err := tx.QueryRow(ctx, `SELECT p.enabled,p.capi_token_cipher<>'',p.credential_status
			FROM meta_pixels p JOIN meta_connections c ON c.id=p.connection_id
			WHERE p.id=$1 AND p.connection_id=$2`, *link.MetaPixelID, *link.MetaConnectionID).
			Scan(&enabled, &hasCredential, &credentialStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return "blocked", "Meta 回传目标已不存在，请重新创建投放链接", nil
		}
		if err != nil {
			return "", "", err
		}
		if !enabled {
			return "blocked", "Meta Pixel 已停用，服务器事件将暂停回传", nil
		}
		if !hasCredential {
			return "blocked", "Meta CAPI 凭证未配置，服务器事件无法回传", nil
		}
		switch strings.ToLower(credentialStatus) {
		case "invalid":
			return "blocked", "Meta CAPI 凭证无效，请重新保存并验证", nil
		case "error":
			return "blocked", "Meta CAPI 凭证状态异常，请检查 Meta 事件记录", nil
		}
		return "ready", "", nil
	case "tiktok":
		if !tikTokEnabled {
			return "blocked", "TikTok 服务器回传总开关未启用", nil
		}
		if link.TikTokPixelID == nil {
			return "blocked", "TikTok 回传目标未绑定，请重新创建投放链接", nil
		}
		var pixelEnabled, connectionEnabled, hasCredential bool
		var credentialStatus string
		err := tx.QueryRow(ctx, `SELECT p.enabled,c.enabled,c.access_token_cipher<>'',c.credential_status
			FROM tiktok_pixels p JOIN tiktok_connections c ON c.id=p.connection_id
			WHERE p.id=$1`, *link.TikTokPixelID).
			Scan(&pixelEnabled, &connectionEnabled, &hasCredential, &credentialStatus)
		if errors.Is(err, pgx.ErrNoRows) {
			return "blocked", "TikTok 回传目标已不存在，请重新创建投放链接", nil
		}
		if err != nil {
			return "", "", err
		}
		if !pixelEnabled {
			return "blocked", "TikTok Pixel 已停用，服务器事件将暂停回传", nil
		}
		if !connectionEnabled {
			return "blocked", "TikTok 凭证已停用，服务器事件将暂停回传", nil
		}
		if !hasCredential {
			return "blocked", "TikTok Access Token 未配置，服务器事件无法回传", nil
		}
		switch strings.ToLower(credentialStatus) {
		case "invalid":
			return "blocked", "TikTok Access Token 无效，请重新保存并验证", nil
		case "error":
			return "blocked", "TikTok Access Token 状态异常，请检查 TikTok 事件记录", nil
		}
		return "ready", "", nil
	default:
		return "blocked", "广告平台配置无效，请重新创建投放链接", nil
	}
}

func audioEventSummarySQL(platform string) string {
	if platform == "meta" {
		return `SELECT count(*) AS saved,
			count(*) FILTER(WHERE me.status IN ('pending','processing','retry')) AS pending,
			count(*) FILTER(WHERE me.status='succeeded') AS accepted,
			count(*) FILTER(WHERE me.status IN ('failed','expired')) AS failed,
			count(*) FILTER(WHERE me.status='skipped') AS skipped
			FROM meta_events me JOIN filtered fv ON fv.id=me.visit_id
			WHERE NOT me.is_test AND me.event_name IN ('PageView','StartListening','ViewContent')`
	}
	return `SELECT count(*) AS saved,
		count(*) FILTER(WHERE te.status IN ('pending','sending','retry')) AS pending,
		count(*) FILTER(WHERE te.status='accepted') AS accepted,
		count(*) FILTER(WHERE te.status='failed') AS failed,0::bigint AS skipped
		FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id AND fv.link_id=te.link_id
		WHERE NOT te.is_test AND te.event_name IN ('StartListening','ViewContent')`
}

func audioDetailStatusSQL(platform string) string {
	if platform == "meta" {
		return `SELECT
			COALESCE((SELECT me.status FROM meta_events me WHERE me.visit_id=e.id AND NOT me.is_test AND me.event_name='PageView' ORDER BY me.updated_at DESC LIMIT 1),'') AS pageview_status,
			COALESCE((SELECT me.status FROM meta_events me WHERE me.visit_id=e.id AND NOT me.is_test AND me.event_name='StartListening' ORDER BY me.updated_at DESC LIMIT 1),'') AS start_status,
			COALESCE((SELECT me.status FROM meta_events me WHERE me.visit_id=e.id AND NOT me.is_test AND me.event_name='ViewContent' ORDER BY me.updated_at DESC LIMIT 1),'') AS qualified_status`
	}
	return `SELECT
		CASE WHEN e.pageview_reported_at IS NULL THEN '' ELSE 'browser_only' END AS pageview_status,
		COALESCE((SELECT te.status FROM tiktok_events te WHERE te.visit_id=e.id AND te.link_id=e.link_id AND NOT te.is_test AND te.event_name='StartListening' ORDER BY te.updated_at DESC LIMIT 1),'') AS start_status,
		COALESCE((SELECT te.status FROM tiktok_events te WHERE te.visit_id=e.id AND te.link_id=e.link_id AND NOT te.is_test AND te.event_name='ViewContent' ORDER BY te.updated_at DESC LIMIT 1),'') AS qualified_status`
}
