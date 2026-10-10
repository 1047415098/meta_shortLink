package analytics

import (
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"whatsapp-analytics/internal/config"
)

// FilterInput carries operator report filters in a JSON body so they never
// appear in browser history, proxy logs, or API query strings.
type FilterInput struct {
	Start        string `json:"start"`
	End          string `json:"end"`
	TZ           string `json:"tz"`
	LinkID       string `json:"link_id"`
	AdID         string `json:"ad_id"`
	Surface      string `json:"surface"`
	CampaignID   string `json:"campaign_id"`
	AdgroupID    string `json:"adgroup_id"`
	CreativeID   string `json:"creative_id"`
	AdIDV2       string `json:"ad_id_v2"`
	EventStatus  string `json:"event_status"`
	TrafficScope string `json:"traffic_scope"`
	Page         int    `json:"page"`
}

func ParseFilter(c *gin.Context, timezone string) (Filter, error) {
	return parseFilterValues(c.Request.URL.Query(), timezone)
}

// ParseFilterJSON accepts the same validated report shape as GET queries but
// keeps all operator-entered values in the POST body.
func ParseFilterJSON(c *gin.Context, timezone string) (Filter, FilterInput, error) {
	var input FilterInput
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return Filter{}, input, errors.New("统计条件格式无效")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return Filter{}, input, errors.New("统计条件格式无效")
	}
	return ParseFilterInput(input, timezone)
}

// ParseFilterInput is shared by JSON report handlers that add their own fields.
func ParseFilterInput(input FilterInput, timezone string) (Filter, FilterInput, error) {
	values := url.Values{}
	for key, value := range map[string]string{
		"start": input.Start, "end": input.End, "tz": input.TZ,
		"link_id": input.LinkID, "ad_id": input.AdID, "surface": input.Surface,
		"campaign_id": input.CampaignID, "adgroup_id": input.AdgroupID,
		"creative_id": input.CreativeID, "ad_id_v2": input.AdIDV2,
		"event_status": input.EventStatus, "traffic_scope": input.TrafficScope,
	} {
		if strings.TrimSpace(value) != "" {
			values.Set(key, value)
		}
	}
	filter, err := parseFilterValues(values, timezone)
	return filter, input, err
}

// Share date and timezone validation between existing GET reports and JSON POST
// statistics without rewriting the incoming request URL or merging input sources.
func parseFilterValues(query url.Values, timezone string) (Filter, error) {
	defaultQuery := func(key, fallback string) string {
		if values, ok := query[key]; ok && len(values) > 0 {
			return values[0]
		}
		return fallback
	}
	f := Filter{
		TZ: defaultQuery("tz", timezone), AdID: query.Get("ad_id"), Surface: query.Get("surface"),
		CampaignID: query.Get("campaign_id"), AdgroupID: query.Get("adgroup_id"),
		CreativeID: query.Get("creative_id"), AdIDV2: query.Get("ad_id_v2"),
		EventStatus: query.Get("event_status"), TrafficScope: defaultQuery("traffic_scope", "all"),
	}
	// Every standalone frontend owns one visit surface; cover records must be queryable by the shared admin list too.
	if f.Surface != "" && f.Surface != "short_link" && f.Surface != "audio_novel" && f.Surface != "novel" && f.Surface != "cover" {
		return f, errors.New("入口类型无效")
	}
	if f.TrafficScope != "all" && f.TrafficScope != "valid" && f.TrafficScope != "abnormal" {
		return f, errors.New("流量范围无效")
	}
	validStatuses := map[string]bool{"": true, "pending": true, "processing": true, "sending": true, "succeeded": true, "accepted": true, "retry": true, "failed": true, "expired": true, "skipped": true}
	if !validStatuses[f.EventStatus] {
		return f, errors.New("事件状态无效")
	}
	// Keep direct API calls aligned with the two fixed-offset choices in the admin UI.
	if !config.IsReportTimezone(f.TZ) {
		return f, errors.New("时区无效")
	}
	loc, e := time.LoadLocation(f.TZ)
	if e != nil {
		return f, errors.New("时区无效")
	}
	now := time.Now().In(loc)
	// Keep direct API queries aligned with the UI: no dates means the current report day.
	f.Start, e = time.ParseInLocation("2006-01-02", defaultQuery("start", now.Format("2006-01-02")), loc)
	if e != nil {
		return f, errors.New("开始日期无效")
	}
	f.End, e = time.ParseInLocation("2006-01-02", defaultQuery("end", now.Format("2006-01-02")), loc)
	if e != nil {
		return f, errors.New("结束日期无效")
	}
	f.End = f.End.AddDate(0, 0, 1)
	if !f.End.After(f.Start) || f.End.Sub(f.Start) > 366*24*time.Hour {
		return f, errors.New("请选择不超过 365 天的有效日期范围")
	}
	if s := query.Get("link_id"); s != "" {
		f.LinkID, e = strconv.ParseInt(s, 10, 64)
		if e != nil || f.LinkID < 1 {
			return f, errors.New("链接编号无效")
		}
	}
	if len(f.AdID) > 120 || len(f.CampaignID) > 120 || len(f.AdgroupID) > 120 || len(f.CreativeID) > 120 || len(f.AdIDV2) > 120 {
		return f, errors.New("广告编号过长")
	}
	return f, nil
}

func CSVSafe(s string) string {
	if strings.HasPrefix(s, "=") || strings.HasPrefix(s, "+") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "@") || strings.HasPrefix(s, "\t") || strings.HasPrefix(s, "\r") {
		return "'" + s
	}
	return s
}
