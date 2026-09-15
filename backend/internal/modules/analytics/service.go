package analytics

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func ParseFilter(c *gin.Context, timezone string) (Filter, error) {
	return parseFilterValues(c.Request.URL.Query(), timezone)
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
	f := Filter{TZ: defaultQuery("tz", timezone), AdID: query.Get("ad_id")}
	loc, e := time.LoadLocation(f.TZ)
	if e != nil {
		return f, errors.New("时区无效")
	}
	now := time.Now().In(loc)
	f.Start, e = time.ParseInLocation("2006-01-02", defaultQuery("start", now.AddDate(0, 0, -6).Format("2006-01-02")), loc)
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
	if len(f.AdID) > 120 {
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
