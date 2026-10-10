package cover

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/platform/runtime"
)

type statsRequest struct {
	Start      *string `json:"start"`
	End        *string `json:"end"`
	TZ         *string `json:"tz"`
	AdID       string  `json:"ad_id"`
	CampaignID string  `json:"campaign_id"`
	AdgroupID  string  `json:"adgroup_id"`
	CreativeID string  `json:"creative_id"`
	AdIDV2     string  `json:"ad_id_v2"`
	Page       int     `json:"page"`
}

type statsFilter struct {
	Start, End                              time.Time
	Timezone                                string
	AdID, CampaignID, AdgroupID, CreativeID string
	AdIDV2                                  string
	Page                                    int
}

type StatsSummary struct {
	Visits                  int64   `json:"visits"`
	UniqueVisitors          int64   `json:"unique_visitors"`
	TenSecondUniqueVisitors int64   `json:"ten_second_unique_visitors"`
	CollectedVisits         int64   `json:"collected_visits"`
	AverageVisibleSeconds   float64 `json:"average_visible_seconds"`
	TotalVisibleSeconds     int64   `json:"total_visible_seconds"`
	PendingEvents           int64   `json:"pending_events"`
	AcceptedEvents          int64   `json:"accepted_events"`
	FailedEvents            int64   `json:"failed_events"`
}

type StatsVisit struct {
	ID             string    `json:"id"`
	OccurredAt     time.Time `json:"occurred_at"`
	VisitorID      string    `json:"visitor_id"`
	Country        string    `json:"country"`
	Region         string    `json:"region"`
	City           string    `json:"city"`
	Device         string    `json:"device"`
	Browser        string    `json:"browser"`
	Source         string    `json:"source"`
	AdID           string    `json:"ad_id"`
	VisibleSeconds *int      `json:"visible_seconds"`
	CampaignID     string    `json:"campaign_id"`
	AdgroupID      string    `json:"adgroup_id"`
	CreativeID     string    `json:"creative_id"`
	AdIDV2         string    `json:"ad_id_v2"`
	Placement      string    `json:"placement"`
	TTCLID         string    `json:"ttclid"`
}

type StatsResponse struct {
	Link     Link         `json:"link"`
	Summary  StatsSummary `json:"summary"`
	Items    []StatsVisit `json:"items"`
	Total    int64        `json:"total"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Timezone string       `json:"timezone"`
}

func parseStatsFilter(input statsRequest, fallback string) (statsFilter, error) {
	timezone := fallback
	if input.TZ != nil {
		timezone = strings.TrimSpace(*input.TZ)
	}
	if !config.IsReportTimezone(timezone) {
		return statsFilter{}, errors.New("时区无效")
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return statsFilter{}, errors.New("时区无效")
	}
	today := time.Now().In(location).Format("2006-01-02")
	startText, endText := today, today
	if input.Start != nil {
		startText = *input.Start
	}
	if input.End != nil {
		endText = *input.End
	}
	start, err := time.ParseInLocation("2006-01-02", startText, location)
	if err != nil {
		return statsFilter{}, errors.New("开始日期无效")
	}
	end, err := time.ParseInLocation("2006-01-02", endText, location)
	if err != nil {
		return statsFilter{}, errors.New("结束日期无效")
	}
	end = end.AddDate(0, 0, 1)
	if !end.After(start) || end.Sub(start) > 366*24*time.Hour || input.Page < 1 || input.Page > 100000 {
		return statsFilter{}, errors.New("请选择不超过 365 天的有效日期范围")
	}
	values := []string{input.AdID, input.CampaignID, input.AdgroupID, input.CreativeID, input.AdIDV2}
	for _, value := range values {
		if len(value) > 120 {
			return statsFilter{}, errors.New("广告参数过长")
		}
	}
	return statsFilter{Start: start, End: end, Timezone: timezone, AdID: strings.TrimSpace(input.AdID),
		CampaignID: strings.TrimSpace(input.CampaignID), AdgroupID: strings.TrimSpace(input.AdgroupID),
		CreativeID: strings.TrimSpace(input.CreativeID), AdIDV2: strings.TrimSpace(input.AdIDV2), Page: input.Page}, nil
}

func (h *Handler) Stats(c *gin.Context) {
	id, ok := positiveID(c)
	if !ok {
		return
	}
	input := statsRequest{Page: 1}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "请使用 JSON 提交统计条件")
		return
	}
	filter, err := parseStatsFilter(input, h.Config.Timezone)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	out, err := (Repository{DB: h.DB}).Stats(ctx, id, filter)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		h.setPublicURL(&out.Link)
		c.JSON(200, out)
	}
}

func (r Repository) Stats(ctx context.Context, linkID int64, filter statsFilter) (StatsResponse, error) {
	out := StatsResponse{Items: []StatsVisit{}, Page: filter.Page, PageSize: 50, Timezone: filter.Timezone}
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	out.Link, err = scanLink(tx.QueryRow(ctx, "SELECT "+linkColumns+" FROM short_links l LEFT JOIN tiktok_pixels tp ON tp.id=l.tiktok_pixel_id WHERE l.id=$1 AND l.product_type='cover'", linkID))
	if err != nil {
		return out, err
	}
	where := `e.link_id=$1 AND e.surface='cover' AND e.method='GET' AND e.event_type='landing'
		AND e.classification='normal' AND e.occurred_at>=$2 AND e.occurred_at<$3`
	args := []any{linkID, filter.Start, filter.End}
	for _, item := range []struct{ value, column string }{
		{filter.AdID, "ad_id"}, {filter.CampaignID, "campaign_id"}, {filter.AdgroupID, "tiktok_adgroup_id"},
		{filter.CreativeID, "tiktok_creative_id"}, {filter.AdIDV2, "tiktok_ad_id_v2"},
	} {
		if item.value == "" {
			continue
		}
		args = append(args, item.value)
		where += fmt.Sprintf(" AND e.%s=$%d", item.column, len(args))
	}
	filtered := `WITH filtered AS (SELECT e.* FROM click_events e WHERE ` + where + `)`
	if err = tx.QueryRow(ctx, filtered+` SELECT count(*),count(DISTINCT NULLIF(e.visitor_id,'')),
		count(DISTINCT NULLIF(e.visitor_id,'')) FILTER(WHERE e.visible_updated_at IS NOT NULL AND e.visible_seconds>=10),
		count(*) FILTER(WHERE e.visible_updated_at IS NOT NULL),
		COALESCE(avg(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0)::float8,
		COALESCE(sum(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0),
		(SELECT count(*) FROM (
			SELECT te.status FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id WHERE te.status IN ('pending','sending','retry')
			UNION ALL SELECT me.status FROM meta_events me JOIN filtered fv ON fv.id=me.visit_id WHERE me.status IN ('pending','processing','retry')) pending),
		(SELECT count(*) FROM (
			SELECT te.status FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id WHERE te.status='accepted'
			UNION ALL SELECT me.status FROM meta_events me JOIN filtered fv ON fv.id=me.visit_id WHERE me.status='succeeded') accepted),
		(SELECT count(*) FROM (
			SELECT te.status FROM tiktok_events te JOIN filtered fv ON fv.id=te.visit_id WHERE te.status='failed'
			UNION ALL SELECT me.status FROM meta_events me JOIN filtered fv ON fv.id=me.visit_id WHERE me.status='failed') failed)
		FROM filtered e`, args...).Scan(&out.Summary.Visits, &out.Summary.UniqueVisitors, &out.Summary.TenSecondUniqueVisitors,
		&out.Summary.CollectedVisits, &out.Summary.AverageVisibleSeconds, &out.Summary.TotalVisibleSeconds,
		&out.Summary.PendingEvents, &out.Summary.AcceptedEvents, &out.Summary.FailedEvents); err != nil {
		return out, err
	}
	out.Total = out.Summary.Visits
	queryArgs := append(append([]any{}, args...), (filter.Page-1)*out.PageSize)
	rows, err := tx.Query(ctx, filtered+` SELECT e.id,e.occurred_at,COALESCE(e.visitor_id,''),e.country,e.region,e.city,
		e.device,e.browser,e.source,e.ad_id,CASE WHEN e.visible_updated_at IS NULL THEN NULL ELSE e.visible_seconds END,
		e.campaign_id,e.tiktok_adgroup_id,e.tiktok_creative_id,e.tiktok_ad_id_v2,e.tiktok_placement,e.tiktok_ttclid
		FROM filtered e`+fmt.Sprintf(" ORDER BY e.occurred_at DESC,e.id DESC LIMIT 50 OFFSET $%d", len(queryArgs)), queryArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item StatsVisit
		if err = rows.Scan(&item.ID, &item.OccurredAt, &item.VisitorID, &item.Country, &item.Region, &item.City,
			&item.Device, &item.Browser, &item.Source, &item.AdID, &item.VisibleSeconds, &item.CampaignID,
			&item.AdgroupID, &item.CreativeID, &item.AdIDV2, &item.Placement, &item.TTCLID); err != nil {
			rows.Close()
			return out, err
		}
		if len(item.TTCLID) > 8 {
			item.TTCLID = item.TTCLID[:4] + "…" + item.TTCLID[len(item.TTCLID)-4:]
		} else if item.TTCLID != "" {
			item.TTCLID = "********"
		}
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
