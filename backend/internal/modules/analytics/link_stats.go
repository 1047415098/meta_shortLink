package analytics

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

// Link statistics use one strict real-ad-click cohort; both consultation types
// belong to the qualifying entry date and count at most once each for that visit.
type LinkStatsMetrics struct {
	Visits              int64 `json:"visits"`
	UniqueVisitors      int64 `json:"unique_visitors"`
	ManualConsultations int64 `json:"manual_consultations"`
	AutoRedirects       int64 `json:"auto_redirects"`
	NoCookie            int64 `json:"no_cookie"`
}

type LinkStatsRow struct {
	ConnectionID int64  `json:"connection_id"`
	AccountID    string `json:"account_id"`
	SourceKind   string `json:"source_kind"`
	SourceValue  string `json:"source_value"`
	AdName       string `json:"ad_name"`
	NameSource   string `json:"name_source"`
	LinkStatsMetrics
}

type LinkStatsResponse struct {
	Link struct {
		ID              int64  `json:"id"`
		Name            string `json:"name"`
		Code            string `json:"code"`
		Mode            string `json:"mode"`
		AttributionMode string `json:"attribution_mode"`
		AdID            string `json:"ad_id"`
	} `json:"link"`
	Items    []LinkStatsRow   `json:"items"`
	Summary  LinkStatsMetrics `json:"summary"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Timezone string           `json:"timezone"`
}

// Resolve one advertising ID for reports: the dedicated ad_id wins, while
// utm_content supports Meta links that intentionally place {{ad.id}} there.
const linkStatsResolvedAdID = `(CASE
 WHEN btrim(COALESCE(e.ad_id,''))<>''
  AND e.ad_id NOT LIKE '%{{%'
  AND e.ad_id NOT LIKE '%}}%' THEN btrim(e.ad_id)
 WHEN jsonb_typeof(e.parameters->'utm_content')='string'
  AND btrim(e.parameters->>'utm_content')<>''
  AND e.parameters->>'utm_content' NOT LIKE '%{{%'
  AND e.parameters->>'utm_content' NOT LIKE '%}}%' THEN btrim(e.parameters->>'utm_content')
 ELSE '' END)`

// A real ad click must be a normal landing-page visit with Meta's click
// identifier and an advertising ID resolved from ad_id or utm_content.
const linkStatsRealAdClickPredicate = `(jsonb_typeof(e.parameters->'fbclid')='string'
 AND btrim(e.parameters->>'fbclid')<>''
 AND e.parameters->>'fbclid' NOT LIKE '%{{%'
 AND e.parameters->>'fbclid' NOT LIKE '%}}%'
 AND ` + linkStatsResolvedAdID + `<>'')`

// Historical values are trimmed in the query so Meta URL fields that previously
// contained spaces still join to the same ad without a destructive data migration.
const linkStatsSource = `WITH filtered AS (
 SELECT e.*,COALESCE(e.meta_connection_id,0) AS connection_id,
 -- The public row remains an ad_id row because source_value is the resolved advertising ID.
 'ad_id'::text AS source_kind,` + linkStatsResolvedAdID + ` AS source_value
 FROM click_events e WHERE e.link_id=$3 AND e.occurred_at >= $1 AND e.occurred_at < $2
 AND e.classification='normal' AND e.event_type='landing'
 AND ` + linkStatsRealAdClickPredicate + `
 AND ($4::text='' OR ` + linkStatsResolvedAdID + `=btrim($4::text))
) `

const linkStatsMetricsSQL = `count(*) AS visits,count(DISTINCT NULLIF(visitor_id,'')) AS unique_visitors,
 count(*) FILTER(WHERE whatsapp_clicked_at IS NOT NULL) AS manual_consultations,
 count(*) FILTER(WHERE auto_redirected_at IS NOT NULL) AS auto_redirects,
 count(*) FILTER(WHERE NULLIF(visitor_id,'') IS NULL) AS no_cookie`

func linkStatsTargets(m *LinkStatsMetrics) []any {
	return []any{&m.Visits, &m.UniqueVisitors, &m.ManualConsultations, &m.AutoRedirects, &m.NoCookie}
}

// Optional date fields retain report defaults; explicit invalid values are still
// validated. Link scope always comes from the URL path, never from JSON or query parameters.
type linkStatsRequest struct {
	Start *string `json:"start"`
	End   *string `json:"end"`
	TZ    *string `json:"tz"`
	AdID  string  `json:"ad_id"`
	Page  int     `json:"page"`
	Sort  string  `json:"sort"`
	Order string  `json:"order"`
}

// LinkStats does not depend on Meta credentials or account timezone validation;
// it reads first-party events and uses previously synced names only when available.
func (a *Handler) LinkStats(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		runtime.Bad(c, "链接编号无效")
		return
	}
	// Read JSON exclusively so stale URL filters cannot override the POST body.
	in := &linkStatsRequest{Page: 1, Sort: "visits", Order: "desc"}
	// Decode before validating fields: null and multiple JSON values are not a filter object.
	decoder := json.NewDecoder(c.Request.Body)
	if c.ContentType() != "application/json" || decoder.Decode(&in) != nil || in == nil {
		runtime.Bad(c, "请使用 JSON 请求体提交统计条件")
		return
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		runtime.Bad(c, "请使用 JSON 请求体提交统计条件")
		return
	}
	values := url.Values{"ad_id": {in.AdID}}
	for key, value := range map[string]*string{"start": in.Start, "end": in.End, "tz": in.TZ} {
		if value != nil {
			values.Set(key, *value)
		}
	}
	f, err := parseFilterValues(values, a.Config.Timezone)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	f.LinkID = id
	if in.Page < 1 || in.Page > 100000 {
		runtime.Bad(c, "页码无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	out, err := (Repository{DB: a.DB}).LinkStats(ctx, f, in.Page, in.Sort, in.Order)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(404, gin.H{"error": "短链接不存在"})
		return
	}
	if errors.Is(err, errLinkStatsSort) {
		runtime.Bad(c, err.Error())
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, out)
}

var errLinkStatsSort = errors.New("排序方式无效")

func (r Repository) LinkStats(ctx context.Context, f Filter, page int, sort, order string) (LinkStatsResponse, error) {
	out := LinkStatsResponse{Items: []LinkStatsRow{}, Page: page, PageSize: 50, Timezone: f.TZ}
	// Only allow fixed column names and directions in ORDER BY; filter values use bindings.
	allowed := map[string]bool{"visits": true, "unique_visitors": true, "manual_consultations": true, "auto_redirects": true, "source_value": true}
	if !allowed[sort] || (order != "asc" && order != "desc") {
		return out, errLinkStatsSort
	}
	tx, err := r.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, err
	}
	defer tx.Rollback(ctx)
	err = tx.QueryRow(ctx, `SELECT id,name,code,mode,attribution_mode,ad_id FROM short_links WHERE id=$1`, f.LinkID).Scan(&out.Link.ID, &out.Link.Name, &out.Link.Code, &out.Link.Mode, &out.Link.AttributionMode, &out.Link.AdID)
	if err != nil {
		return out, err
	}
	args := f.args()
	// Compute the overall UV from raw visits, never by summing each ad's distinct visitors.
	if err = tx.QueryRow(ctx, linkStatsSource+`SELECT `+linkStatsMetricsSQL+` FROM filtered`, args...).Scan(linkStatsTargets(&out.Summary)...); err != nil {
		return out, err
	}
	if err = tx.QueryRow(ctx, linkStatsSource+`SELECT count(*) FROM (SELECT 1 FROM filtered GROUP BY connection_id,meta_account_id,source_kind,source_value) groups`, args...).Scan(&out.Total); err != nil {
		return out, err
	}
	query := linkStatsSource + `, groups AS (
 SELECT connection_id,meta_account_id,source_kind,source_value,
 -- Every row is already grouped by its resolved ID, so its captured ad name is safe as a fallback.
 min(NULLIF(btrim(parameters->>'ad_name'),'')) AS captured_name,
 ` + linkStatsMetricsSQL + ` FROM filtered GROUP BY connection_id,meta_account_id,source_kind,source_value
 ) SELECT g.connection_id,g.meta_account_id,g.source_kind,g.source_value,
 CASE WHEN g.source_kind='ad_id' THEN COALESCE(NULLIF(e.ad_name,''),g.captured_name,'') ELSE '' END,
 CASE WHEN g.source_kind<>'ad_id' THEN '' WHEN NULLIF(e.ad_name,'') IS NOT NULL THEN 'meta' WHEN g.captured_name IS NOT NULL THEN 'parameter' ELSE '' END,
 -- Keep real-click row metrics in the same order as linkStatsTargets.
 g.visits,g.unique_visitors,g.manual_consultations,g.auto_redirects,g.no_cookie
 FROM groups g LEFT JOIN meta_connections c ON c.id=g.connection_id AND c.account_id=g.meta_account_id
 LEFT JOIN meta_ad_entities e ON e.connection_id=c.id AND e.ad_id=g.source_value AND g.source_kind='ad_id'
 ORDER BY g.` + sort + ` ` + order + `,g.connection_id,g.meta_account_id,g.source_kind,g.source_value LIMIT 50 OFFSET $5`
	rows, err := tx.Query(ctx, query, append(args, (page-1)*50)...)
	if err != nil {
		return out, err
	}
	for rows.Next() {
		var row LinkStatsRow
		targets := []any{&row.ConnectionID, &row.AccountID, &row.SourceKind, &row.SourceValue, &row.AdName, &row.NameSource}
		if err = rows.Scan(append(targets, linkStatsTargets(&row.LinkStatsMetrics)...)...); err != nil {
			rows.Close()
			return out, err
		}
		out.Items = append(out.Items, row)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}
