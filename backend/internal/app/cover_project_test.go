package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

var coverTicketPattern = regexp.MustCompile(`"ticket":"([^"]+)"`)

func coverTicket(t *testing.T, body string) string {
	t.Helper()
	match := coverTicketPattern.FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("cover bootstrap ticket missing: %s", body)
	}
	return match[1]
}

func TestCoverProjectLinkAttributionAndStatisticsAreIsolated(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	connectionID, pixelID := testLinkMetaBinding(t, a)
	tiktokPixelID := testTikTokBinding(t, a, "COVER")

	// 封面项目只创建独立投放链接，不接收 novel_id、章节或推荐设置。
	created := call(a, http.MethodPost, "/api/v1/cover-links", fmt.Sprintf(`{"name":"Cover buyer A","code":"cover-a","enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, connectionID, pixelID), admin)
	if created.Code != http.StatusOK {
		t.Fatalf("create cover link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID          int64  `json:"id"`
		Code        string `json:"code"`
		ProductType string `json:"product_type"`
		PublicURL   string `json:"public_url"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if link.ProductType != "cover" || link.Code != "cover-a" || link.PublicURL != "http://localhost:8080/cover/cover-a" {
		t.Fatalf("unexpected cover link: %+v", link)
	}

	// 封面链接不能进入免费小说、普通短链或其他产品的页面和管理列表。
	for _, path := range []string{"/cover-a", "/novel/cover-a", "/audio-novel/cover-a"} {
		if response := call(a, http.MethodGet, path, "", nil); response.Code != http.StatusNotFound {
			t.Fatalf("cover link entered wrong surface %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	ordinary := call(a, http.MethodGet, "/api/v1/links", "", admin)
	if ordinary.Code != http.StatusOK || strings.Contains(ordinary.Body.String(), `"code":"cover-a"`) {
		t.Fatalf("cover link leaked into ordinary list: %d %s", ordinary.Code, ordinary.Body.String())
	}
	novelLinks := call(a, http.MethodPost, "/api/v1/novel-links/query", `{}`, admin)
	if novelLinks.Code != http.StatusOK || strings.Contains(novelLinks.Body.String(), `"code":"cover-a"`) {
		t.Fatalf("cover link leaked into novel list: %d %s", novelLinks.Code, novelLinks.Body.String())
	}

	page := call(a, http.MethodGet, "/cover/cover-a?utm_source=facebook&campaign_id=campaign-a&adset_id=group-a&ad_id=ad-a", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"surface":"cover"`) || !strings.Contains(page.Body.String(), `"ad_platform":"meta"`) {
		t.Fatalf("cover bootstrap: %d %s", page.Code, page.Body.String())
	}
	ticket := coverTicket(t, page.Body.String())

	view := postNovelForm(a, "/cover/cover-a/view", url.Values{"ticket": {ticket}})
	if view.Code != http.StatusOK {
		t.Fatalf("confirm cover PageView: %d %s", view.Code, view.Body.String())
	}
	// Move only this fixture's server clock backwards so the hard 10-second gate can be tested without sleeping.
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '20 seconds' WHERE link_id=$1 AND surface='cover'", link.ID); err != nil {
		t.Fatal(err)
	}
	visible := postNovelForm(a, "/cover/cover-a/visible-time", url.Values{"ticket": {ticket}, "seconds": {"12"}})
	if visible.Code != http.StatusOK || !strings.Contains(visible.Body.String(), `"visible_seconds":12`) {
		t.Fatalf("cover visible time: %d %s", visible.Code, visible.Body.String())
	}
	qualified := postNovelForm(a, "/cover/cover-a/time-spent", url.Values{"ticket": {ticket}})
	if qualified.Code != http.StatusNoContent {
		t.Fatalf("cover qualified event: %d %s", qualified.Code, qualified.Body.String())
	}

	var surface, campaignID, adID string
	var visibleSeconds int
	if err := a.DB.QueryRow(context.Background(), `SELECT surface,campaign_id,ad_id,visible_seconds FROM click_events
		WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at DESC LIMIT 1`, link.ID).
		Scan(&surface, &campaignID, &adID, &visibleSeconds); err != nil {
		t.Fatal(err)
	}
	if surface != "cover" || campaignID != "campaign-a" || adID != "ad-a" || visibleSeconds != 12 {
		t.Fatalf("cover attribution snapshot surface=%q campaign=%q ad=%q seconds=%d", surface, campaignID, adID, visibleSeconds)
	}

	today := time.Now().In(mustLocation("Etc/GMT+8")).Format("2006-01-02")
	stats := call(a, http.MethodPost, "/api/v1/cover-links/"+itoa(link.ID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Etc/GMT+8","page":1}`, today, today), admin)
	if stats.Code != http.StatusOK {
		t.Fatalf("cover stats: %d %s", stats.Code, stats.Body.String())
	}
	var report struct {
		Summary struct {
			Visits                  int `json:"visits"`
			UniqueVisitors          int `json:"unique_visitors"`
			TenSecondUniqueVisitors int `json:"ten_second_unique_visitors"`
			TotalVisibleSeconds     int `json:"total_visible_seconds"`
		} `json:"summary"`
		Items []struct {
			CampaignID     string `json:"campaign_id"`
			VisibleSeconds int    `json:"visible_seconds"`
		} `json:"items"`
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Visits != 1 || report.Summary.UniqueVisitors != 1 || report.Summary.TenSecondUniqueVisitors != 1 || report.Summary.TotalVisibleSeconds != 12 || len(report.Items) != 1 || report.Items[0].CampaignID != "campaign-a" {
		t.Fatalf("unexpected cover report: %s", stats.Body.String())
	}

	// 首次正常访问后仅允许改名称、状态和阈值，平台与 Pixel 永久锁定。
	locked := call(a, http.MethodPatch, "/api/v1/cover-links/"+itoa(link.ID), fmt.Sprintf(`{"name":"Cover buyer A","enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, tiktokPixelID), admin)
	if locked.Code != http.StatusConflict {
		t.Fatalf("visited cover binding update: %d %s", locked.Code, locked.Body.String())
	}
	if response := call(a, http.MethodDelete, "/api/v1/cover-links/"+itoa(link.ID), "", admin); response.Code != http.StatusConflict {
		t.Fatalf("visited cover delete: %d %s", response.Code, response.Body.String())
	}
}
