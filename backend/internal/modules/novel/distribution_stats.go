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
	Start *string `json:"start"`
	End   *string `json:"end"`
	TZ    *string `json:"tz"`
	AdID  string  `json:"ad_id"`
	Page  int     `json:"page"`
}

type distributionStatsFilter struct {
	Start, End time.Time
	Timezone   string
	AdID       string
	Page       int
}

type DistributionStatsSummary struct {
	Visits                int64   `json:"visits"`
	UniqueVisitors        int64   `json:"unique_visitors"`
	CollectedVisits       int64   `json:"collected_visits"`
	AverageVisibleSeconds float64 `json:"average_visible_seconds"`
	TotalVisibleSeconds   int64   `json:"total_visible_seconds"`
}

type DistributionVisit struct {
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
	if len(input.AdID) > 120 || input.Page < 1 || input.Page > 100000 {
		return distributionStatsFilter{}, errors.New("广告编号或页码无效")
	}
	return distributionStatsFilter{Start: start, End: end, Timezone: timezone, AdID: input.AdID, Page: input.Page}, nil
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
	out.Link, err = scanDistribution(tx.QueryRow(ctx, "SELECT "+distributionColumns+" FROM short_links l JOIN novels n ON n.id=l.novel_id WHERE l.id=$1 AND l.product_type='novel'", linkID))
	if err != nil {
		return out, err
	}
	where := `e.link_id=$1 AND e.surface='novel' AND e.method='GET' AND e.event_type='landing'
		AND e.classification='normal' AND e.occurred_at>=$2 AND e.occurred_at<$3`
	args := []any{linkID, filter.Start, filter.End}
	if strings.TrimSpace(filter.AdID) != "" {
		args = append(args, strings.TrimSpace(filter.AdID))
		where += fmt.Sprintf(" AND e.ad_id=$%d", len(args))
	}
	// Average time excludes historical visits that predate visible-time collection.
	if err = tx.QueryRow(ctx, `SELECT count(*),count(DISTINCT NULLIF(e.visitor_id,'')),
		count(*) FILTER(WHERE e.visible_updated_at IS NOT NULL),
		COALESCE(avg(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0)::float8,
		COALESCE(sum(e.visible_seconds) FILTER(WHERE e.visible_updated_at IS NOT NULL),0)
		FROM click_events e WHERE `+where, args...).Scan(&out.Summary.Visits, &out.Summary.UniqueVisitors, &out.Summary.CollectedVisits, &out.Summary.AverageVisibleSeconds, &out.Summary.TotalVisibleSeconds); err != nil {
		return out, err
	}
	out.Total = out.Summary.Visits
	queryArgs := append(append([]any{}, args...), (filter.Page-1)*out.PageSize)
	rows, err := tx.Query(ctx, `SELECT e.id,e.occurred_at,COALESCE(e.visitor_id,''),e.country,e.region,e.city,e.device,e.browser,e.source,e.ad_id,
		CASE WHEN e.visible_updated_at IS NULL THEN NULL ELSE e.visible_seconds END
		FROM click_events e WHERE `+where+fmt.Sprintf(" ORDER BY e.occurred_at DESC,e.id DESC LIMIT 50 OFFSET $%d", len(queryArgs)), queryArgs...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var item DistributionVisit
		if err = rows.Scan(&item.ID, &item.OccurredAt, &item.VisitorID, &item.Country, &item.Region, &item.City, &item.Device, &item.Browser, &item.Source, &item.AdID, &item.VisibleSeconds); err != nil {
			rows.Close()
			return out, err
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
