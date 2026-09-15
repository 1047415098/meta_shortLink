package tracking

import (
	"crypto/hmac"
	"strings"

	"github.com/gin-gonic/gin"

	"whatsapp-analytics/internal/platform/runtime"
)

func Classify(method, ua, purpose string, high bool) (string, string) {
	if method == "HEAD" {
		return "head", "HEAD 请求"
	}
	if strings.Contains(strings.ToLower(purpose), "prefetch") || strings.Contains(strings.ToLower(purpose), "preview") {
		return "prefetch", "预取或预览请求"
	}
	s := strings.ToLower(ua)
	for _, v := range []string{"facebookexternalhit", "facebot", "meta-externalagent", "meta-externalfetcher", "googlebot", "bingbot", "twitterbot", "linkedinbot", "slackbot", "discordbot", "telegrambot", "whatsapp", "crawler", "spider", "headlesschrome", "curl/", "python-requests"} {
		if strings.Contains(s, v) {
			return "bot", "请求头匹配机器人规则"
		}
	}
	if high {
		return "suspicious", "短时间内高频访问"
	}
	if ua == "" {
		return "unclassified", "缺少设备信息"
	}
	return "normal", "未命中异常规则"
}

func (a *Handler) visitor(c *gin.Context, allowed bool) (string, string) {
	if a.Config.CookieMode != "all" || !allowed {
		return "", "disabled"
	}
	name := "wa_sid_dev"
	if a.Config.SecureCookies {
		name = "__Host-sid"
	}
	v, _ := c.Cookie(name)
	parts := strings.Split(v, ".")
	if len(parts) == 2 && runtime.VisitorPattern.MatchString(parts[0]) && hmac.Equal([]byte(a.Sign(parts[0])), []byte(parts[1])) {
		return parts[0], "recognized"
	}
	id := runtime.Token()
	a.SetCookie(c, name, id+"."+a.Sign(id), 30*86400)
	return id, "issued"
}
