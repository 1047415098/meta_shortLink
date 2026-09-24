package novel

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
}

type distributionStatsFilter struct {
	Start, End                                      time.Time
	Timezone                                        string
	AdID, CampaignID, AdgroupID, CreativeID, AdIDV2 string
	EventStatus                                     string
	Page                                            int
}

type DistributionStatsSummary struct {
	Visits                int64   `json:"visits"`
	UniqueVisitors        int64   `json:"unique_visitors"`
	CollectedVisits       int64   `json:"collected_visits"`
	AverageVisibleSeconds float64 `json:"average_visible_seconds"`
	TotalVisibleSeconds   int64   `json:"total_visible_seconds"`
	StartReadingCount     int64   `json:"start_reading_count"`
	StartReadingVisitors  int64   `json:"start_reading_visitors"`
	QualifiedCount        int64   `json:"qualified_count"`
	QualifiedVisitors     int64   `json:"qualified_visitors"`
	QualifiedRate         float64 `json:"qualified_rate"`
	TikTokPendingEvents   int64   `json:"tiktok_pending_events"`
	TikTokAcceptedEvents  int64   `json:"tiktok_accepted_events"`
	TikTokFailedEvents    int64   `json:"tiktok_failed_events"`
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
	QualifiedEventStatus string    `json:"qualified_event_status"`
}

type DistributionStatsResponse struct {
	Link     DistributionLink         `json:"link"`
	Summary  DistributionStatsSummary `json:"summary"`
	Items    []DistributionVisit      `json:"items"`
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Timezone string                   `json:"timezone"`
}

func parseDistributionStatsFilter(input distributionStatsRequest, fallbackTimezone string) (distributionStatsFilter, error) {
	timezone := fallbackTimezone
	if input.TZ != nil {
		timezone = *input.TZ
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return distributionStatsFilter{}, errors.New("时区无效")
	}
	now := time.Now().In(location)
	startText, endText := now.AddDate(0, 0, -6).Format("2006-01-02"), now.Format("2006-01-02")
	if input.Start != nil {
		startText = *input.Start
	}
	if input.End != nil {
		endText = *input.End
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
	identifiers := []string{input.AdID, input.CampaignID, input.AdgroupID, input.CreativeID, input.AdIDV2}
	for _, value := range identifiers {
		if len(value) > 120 {
			return distributionStatsFilter{}, errors.New("广告参数过长")
		}
	}
	validStatuses := map[string]bool{"": true, "pending": true, "sending": true, "accepted": true, "retry": true, "failed": true}
	if !validStatuses[input.EventStatus] || input.Page < 1 || input.Page > 100000 {
		return distributionStatsFilter{}, errors.New("广告编号或页码无效")
	}
	return distributionStatsFilter{
		Start: start, End: end, Timezone: timezone, AdID: strings.TrimSpace(input.AdID),
		CampaignID: strings.TrimSpace(input.CampaignID), AdgroupID: strings.TrimSpace(input.AdgroupID),
		CreativeID: strings.TrimSpace(input.CreativeID), AdIDV2: strings.TrimSpace(input.AdIDV2),
		EventStatus: input.EventStatus, Page: input.Page,
	}, nil
}

func maskTikTokID(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "…" + value[len(value)-4:]
}

func (h *Handler) DistributionStats(c *gin.Context) {
	id, ok := positiveID(c, "id", "投放链接编号无效")
	if !ok {
		return
	}
	input := distributionStatsRequest{Page: 1}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "请使用 JSON 提交统计条件")
		return
	}
	filter, err := parseDistributionStatsFilter(input, h.Config.Timezone)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	out, err := (Repository{DB: h.DB}).DistributionStats(ctx, id, filter)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		h.setDistributionPublicURL(&out.Link)
		c.JSON(200, out)
	}
}

func (r Repository) DistributionStats(ctx context.Context, linkID int64, filter distributionStatsFilter) (DistributionStatsResponse, error) {
	out := DistributionStatsResponse{Items: []DistributionVisit{}, Page: filter.Page, PageSize: 50, Timezone: filter.Timezone}
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out.Link, err = scanDistribution(tx.QueryRow(ctx, "SELECT "+distributionColumns+" FROM short_links l JOIN novels n ON n.id=l.novel_id LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.id=$1 AND l.product_type='novel'", linkID))
	if err != nil {
		return out, err
	}
	where := `e.link_id=$1 AND e.surface='novel' AND e.method='GET' AND e.event_type='landing'
		AND e.classification='normal' AND e.occurred_at>=$2 AND e.occurred_at<$3`
	args := []any{linkID, filter.Start, filter.End}
	if filter.AdID != "" {
		args = append(args, filter.AdID)
		where += fmt.Sprintf(" AND e.ad_id=$%d", len(args))
	}
	for _, item := range []struct {
		value, column string
	}{
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
		where += fmt.Sprintf(` AND EXISTS(SELECT 1 FROM tiktok_events te_filter
			WHERE te_filter.visit_id=e.id AND te_filter.link_id=e.link_id AND te_filter.status=$%d)`, len(args))
	}
	filteredCTE := `WITH filtered AS (SELECT e.* FROM click_events e WHERE ` + where + `)`
	// Average time excludes historical visits that predate visible-time collection.
	if err = tx.QueryRow(ctx, filteredCTE+` SELECT count(*),count(DISTINCT NULLIF(e.visitor_id,'')),
		count(*) FILTER(WHERE e.visible_updated_at IS NOT NULL),
		COALESCE(avg(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0)::float8,
		COALESCE(sum(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0),
		count(*) FILTER(WHERE e.tiktok_start_reading_at IS NOT NULL),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.tiktok_start_reading_at IS NOT NULL),
		count(*) FILTER(WHERE e.tiktok_view_content_at IS NOT NULL),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.tiktok_view_content_at IS NOT NULL),
		CASE WHEN count(*)=0 THEN 0 ELSE count(*) FILTER(WHERE e.tiktok_view_content_at IS NOT NULL)*100.0/count(*) END::float8,
		(SELECT count(*) FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id AND fv.link_id=te.link_id WHERE te.status IN ('pending','sending','retry')),
		(SELECT count(*) FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id AND fv.link_id=te.link_id WHERE te.status='accepted'),
		(SELECT count(*) FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id AND fv.link_id=te.link_id WHERE te.status='failed')
		FROM filtered e`, args...).Scan(
		&out.Summary.Visits, &out.Summary.UniqueVisitors, &out.Summary.CollectedVisits,
		&out.Summary.AverageVisibleSeconds, &out.Summary.TotalVisibleSeconds,
		&out.Summary.StartReadingCount, &out.Summary.StartReadingVisitors,
		&out.Summary.QualifiedCount, &out.Summary.QualifiedVisitors, &out.Summary.QualifiedRate,
		&out.Summary.TikTokPendingEvents, &out.Summary.TikTokAcceptedEvents, &out.Summary.TikTokFailedEvents,
	); err != nil {
		return out, err
	}
	out.Total = out.Summary.Visits
	queryArgs := append(append([]any{}, args...), (filter.Page-1)*out.PageSize)
	rows, err := tx.Query(ctx, filteredCTE+` SELECT e.id,e.occurred_at,COALESCE(e.visitor_id,''),e.country,e.region,e.city,
		e.device,e.browser,e.source,e.ad_id,CASE WHEN e.visible_updated_at IS NULL THEN NULL ELSE e.visible_seconds END,
		e.ad_platform,COALESCE(tp.name,''),COALESCE(NULLIF(e.tiktok_pixel_code,''),tp.pixel_code,''),e.campaign_id,
		e.tiktok_adgroup_id,e.tiktok_creative_id,e.tiktok_ad_id_v2,e.tiktok_placement,e.tiktok_ttclid,
		e.tiktok_start_reading_at IS NOT NULL,e.tiktok_view_content_at IS NOT NULL,
		COALESCE((SELECT te.status FROM tiktok_events te WHERE te.visit_id=e.id AND te.link_id=e.link_id
			AND te.event_name='ViewContent' ORDER BY te.updated_at DESC LIMIT 1),'')
		FROM filtered e LEFT JOIN tiktok_pixels tp ON tp.id=e.tiktok_pixel_id`+
		fmt.Sprintf(" ORDER BY e.occurred_at DESC,e.id DESC LIMIT 50 OFFSET $%d", len(queryArgs)), queryArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item DistributionVisit
		if err = rows.Scan(&item.ID, &item.OccurredAt, &item.VisitorID, &item.Country, &item.Region, &item.City,
			&item.Device, &item.Browser, &item.Source, &item.AdID, &item.VisibleSeconds, &item.AdPlatform,
			&item.PixelName, &item.PixelCode, &item.CampaignID, &item.AdgroupID, &item.CreativeID, &item.AdIDV2,
			&item.Placement, &item.TTCLID, &item.Started, &item.Qualified, &item.QualifiedEventStatus); err != nil {
			rows.Close()
			return out, err
		}
		// Management responses expose only an operational hint, never the complete click identifier.
		item.TTCLID = maskTikTokID(item.TTCLID)
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
