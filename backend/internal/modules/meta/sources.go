package meta

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"strings"
	"whatsapp-analytics/internal/platform/runtime"
)

func (h *Handler) inspectSource(c *gin.Context) {
	var in struct {
		URL string `json:"url"`
	}
	if c.ShouldBindJSON(&in) != nil || len(in.URL) > 12000 {
		runtime.Bad(c, "请输入完整网址或网址参数（最多 12000 字符）")
		return
	}
	out := InspectSource(in.URL)
	c.JSON(200, out)
}

type SourceInspection struct {
	Valid      bool              `json:"valid"`
	Parameters map[string]string `json:"parameters"`
	Issues     []string          `json:"issues"`
	Source     string            `json:"source"`
	CampaignID string            `json:"campaign_id"`
	AdsetID    string            `json:"adset_id"`
	AdID       string            `json:"ad_id"`
}

func InspectSource(raw string) SourceInspection {
	out := SourceInspection{Parameters: map[string]string{}, Issues: []string{}}
	query := strings.TrimPrefix(strings.TrimSpace(raw), "?")
	if strings.Contains(query, "://") {
		u, e := url.Parse(query)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			out.Issues = append(out.Issues, "网址格式无效")
			return out
		}
		query = u.RawQuery
		if u.User != nil {
			out.Issues = append(out.Issues, "网址不得包含登录凭证")
		}
	}
	params, e := url.ParseQuery(query)
	if e != nil {
		out.Issues = append(out.Issues, "网址参数编码无效")
		return out
	}
	allowed := map[string]bool{"campaign_id": true, "adset_id": true, "ad_id": true, "campaign_name": true, "adset_name": true, "ad_name": true, "utm_source": true, "utm_medium": true, "utm_campaign": true, "utm_content": true, "utm_term": true, "site_source_name": true, "placement": true, "fbclid": true}
	for k, values := range params {
		lower := strings.ToLower(k)
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "authorization") || strings.Contains(lower, "api_key") {
			out.Issues = append(out.Issues, "检测到凭证参数，广告网址中必须移除，值不会保存或返回")
			continue
		}
		if !allowed[k] {
			continue
		}
		if len(values) != 1 {
			out.Issues = append(out.Issues, k+" 重复出现，请只保留一个值")
		}
		v := values[0]
		if len(v) > 2048 {
			out.Issues = append(out.Issues, k+" 超过长度限制")
			continue
		}
		out.Parameters[k] = v
	}
	for _, k := range []string{"campaign_id", "adset_id", "ad_id"} {
		v := out.Parameters[k]
		if v == "" {
			out.Issues = append(out.Issues, "缺少 "+k)
		} else if !idPattern.MatchString(v) {
			if strings.Contains(v, "{{") {
				out.Issues = append(out.Issues, k+" 是待 Meta 替换的动态参数，实际点击后应为数字")
			} else {
				out.Issues = append(out.Issues, k+" 必须是数字 ID")
			}
		}
	}
	out.CampaignID = out.Parameters["campaign_id"]
	out.AdsetID = out.Parameters["adset_id"]
	out.AdID = out.Parameters["ad_id"]
	for _, key := range []string{"site_source_name", "utm_source"} {
		v := strings.TrimSpace(out.Parameters[key])
		if strings.Contains(v, "{{") || strings.Contains(v, "}}") {
			out.Issues = append(out.Issues, key+" 是待 Meta 替换的动态参数，请用实际点击后的值核对来源")
			continue
		}
		if v != "" {
			out.Source = strings.ToLower(v)
			break
		}
	}
	switch out.Source {
	case "fb":
		out.Source = "facebook"
	case "ig":
		out.Source = "instagram"
	case "msg":
		out.Source = "messenger"
	case "an":
		out.Source = "audience_network"
	}
	out.Valid = len(out.Issues) == 0
	return out
}
