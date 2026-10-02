package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"
)

func postShortLinkForm(a *App, path string, values url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.RemoteAddr = "192.0.2.9:43123"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Origin", "http://example.com")
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Safari/604.1")
	// TikTok creates _ttp after the entry HTML loads; signed action requests
	// must backfill it into the immutable visit attribution snapshot.
	request.AddCookie(&http.Cookie{Name: "_ttp", Value: "fresh-short-link-ttp"})
	response := httptest.NewRecorder()
	a.Router.ServeHTTP(response, request)
	return response
}

func TestShortLinkAcceptsExactlyOneTikTokPixel(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	pixelID := testTikTokBinding(t, a, "SHORT01")

	body := fmt.Sprintf(`{"name":"TikTok WhatsApp","code":"wa-tik","target_url":"https://wa.me/13365661092","mode":"landing","landing_brand":"Example","landing_title":"Ask us","landing_description":"Details","enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":5}`, pixelID)
	created := call(a, http.MethodPost, "/api/v1/links", body, admin)
	if created.Code != http.StatusOK {
		t.Fatalf("create TikTok short link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID               int64  `json:"id"`
		AdPlatform       string `json:"ad_platform"`
		TikTokPixelID    *int64 `json:"tiktok_pixel_id"`
		MetaConnectionID *int64 `json:"meta_connection_id"`
		MetaPixelID      *int64 `json:"meta_pixel_id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if link.ID < 1 || link.AdPlatform != "tiktok" || link.TikTokPixelID == nil || *link.TikTokPixelID != pixelID || link.MetaConnectionID != nil || link.MetaPixelID != nil {
		t.Fatalf("unexpected binding: %+v", link)
	}

	connectionID, metaPixelID := testLinkMetaBinding(t, a)
	for name, invalidBody := range map[string]string{
		"missing TikTok Pixel": `{"name":"Missing","code":"wa-missing","target_url":"https://wa.me/13365661092","ad_platform":"tiktok","attribution_mode":"dynamic"}`,
		"mixed platforms":      fmt.Sprintf(`{"name":"Mixed","code":"wa-mixed","target_url":"https://wa.me/13365661092","ad_platform":"tiktok","tiktok_pixel_id":%d,"meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic"}`, pixelID, connectionID, metaPixelID),
	} {
		if response := call(a, http.MethodPost, "/api/v1/links", invalidBody, admin); response.Code != http.StatusBadRequest {
			t.Fatalf("%s accepted: %d %s", name, response.Code, response.Body.String())
		}
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_pixels SET enabled=false WHERE id=$1", pixelID); err != nil {
		t.Fatal(err)
	}
	disabledBody := fmt.Sprintf(`{"name":"Disabled","code":"wa-disabled","target_url":"https://wa.me/13365661092","ad_platform":"tiktok","tiktok_pixel_id":%d,"attribution_mode":"dynamic"}`, pixelID)
	if response := call(a, http.MethodPost, "/api/v1/links", disabledBody, admin); response.Code != http.StatusBadRequest {
		t.Fatalf("disabled TikTok Pixel accepted: %d %s", response.Code, response.Body.String())
	}
}

func TestTikTokShortLinkQueuesDeduplicatedFullFunnelAndStats(t *testing.T) {
	a := setup(t)
	a.Config.TikTokEnabled = true
	admin := login(t, a)
	pixelID := testTikTokBinding(t, a, "SHORT02")
	body := fmt.Sprintf(`{"name":"TikTok Funnel","code":"wa-funnel","target_url":"https://wa.me/13365661092","mode":"landing","landing_brand":"Example","landing_title":"Ask us","landing_description":"Details","landing_delay":3,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":5}`, pixelID)
	created := call(a, http.MethodPost, "/api/v1/links", body, admin)
	if created.Code != http.StatusOK {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}

	page := call(a, http.MethodGet, "/wa-funnel?utm_source=tiktok&campaign_id=camp-1&adgroup_id=group-2&creative_id=creative-3&ad_id_v2=ad-4&placement=TikTok&ttclid=click-short", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `"tiktok_browser_pixel_code":"PX_SHORT02"`) {
		t.Fatalf("TikTok bootstrap: %d %s", page.Code, page.Body.String())
	}
	ticketMatch := regexp.MustCompile(`"ticket":"([^"]+)"`).FindStringSubmatch(page.Body.String())
	if len(ticketMatch) != 2 {
		t.Fatalf("ticket missing: %s", page.Body.String())
	}
	ticket := ticketMatch[1]
	visitID := strings.SplitN(ticket, ".", 2)[0]
	var ttclid, ttp, adgroupID, creativeID, adID, platform string
	if err := a.DB.QueryRow(context.Background(), `SELECT tiktok_ttclid,tiktok_adgroup_id,tiktok_creative_id,tiktok_ad_id_v2,ad_platform
		FROM click_events WHERE id=$1`, visitID).Scan(&ttclid, &adgroupID, &creativeID, &adID, &platform); err != nil {
		t.Fatal(err)
	}
	if ttclid != "click-short" || adgroupID != "group-2" || creativeID != "creative-3" || adID != "ad-4" || platform != "tiktok" {
		t.Fatalf("frozen attribution ttclid=%q adgroup=%q creative=%q ad=%q platform=%q", ttclid, adgroupID, creativeID, adID, platform)
	}

	view := postShortLinkForm(a, "/wa-funnel/view", url.Values{"ticket": {ticket}})
	if view.Code != http.StatusOK || !strings.Contains(view.Body.String(), `"name":"PageView"`) || !strings.Contains(view.Body.String(), `"event_id":"short_`+visitID+`_view"`) {
		t.Fatalf("view event: %d %s", view.Code, view.Body.String())
	}
	if repeated := postShortLinkForm(a, "/wa-funnel/view", url.Values{"ticket": {ticket}}); repeated.Code != http.StatusOK || repeated.Body.String() != view.Body.String() {
		t.Fatalf("view retry differs: %d %s", repeated.Code, repeated.Body.String())
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT tiktok_ttp FROM click_events WHERE id=$1", visitID).Scan(&ttp); err != nil || ttp != "fresh-short-link-ttp" {
		t.Fatalf("TikTok _ttp was not refreshed: value=%q err=%v", ttp, err)
	}
	contact := postShortLinkForm(a, "/wa-funnel/contact", url.Values{"ticket": {ticket}, "trigger": {"manual"}})
	if contact.Code != http.StatusOK || !strings.Contains(contact.Body.String(), `"target_url":"https://wa.me/13365661092"`) || !strings.Contains(contact.Body.String(), `"name":"Contact"`) || !strings.Contains(contact.Body.String(), `"event_id":"short_`+visitID+`_manual"`) {
		t.Fatalf("contact event: %d %s", contact.Code, contact.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '6 seconds' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	qualified := postShortLinkForm(a, "/wa-funnel/time-spent", url.Values{"ticket": {ticket}})
	if qualified.Code != http.StatusOK || !strings.Contains(qualified.Body.String(), `"name":"ViewContent"`) || !strings.Contains(qualified.Body.String(), `"event_id":"short_`+visitID+`_qualified"`) {
		t.Fatalf("qualified event: %d %s", qualified.Code, qualified.Body.String())
	}

	var eventNames, eventIDs string
	var metaEvents int
	if err := a.DB.QueryRow(context.Background(), `SELECT string_agg(event_name,',' ORDER BY event_name),string_agg(event_id,',' ORDER BY event_id)
		FROM tiktok_events WHERE visit_id=$1`, visitID).Scan(&eventNames, &eventIDs); err != nil {
		t.Fatal(err)
	}
	if eventNames != "Contact,PageView,ViewContent" || !strings.Contains(eventIDs, "short_"+visitID+"_manual") {
		t.Fatalf("events names=%q ids=%q", eventNames, eventIDs)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_events WHERE visit_id=$1", visitID).Scan(&metaEvents); err != nil || metaEvents != 0 {
		t.Fatalf("TikTok visit created Meta events=%d err=%v", metaEvents, err)
	}

	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	stats := call(a, http.MethodPost, fmt.Sprintf("/api/v1/links/%d/stats", link.ID), fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai"}`, today, today), admin)
	if stats.Code != http.StatusOK || !strings.Contains(stats.Body.String(), `"ad_platform":"tiktok"`) || !strings.Contains(stats.Body.String(), `"source_value":"ad-4"`) || !strings.Contains(stats.Body.String(), `"visits":1`) {
		t.Fatalf("TikTok stats: %d %s", stats.Code, stats.Body.String())
	}
	secondPixelID := testTikTokBinding(t, a, "SHORT04")
	if changed := call(a, http.MethodPatch, fmt.Sprintf("/api/v1/links/%d", link.ID), fmt.Sprintf(`{"tiktok_pixel_id":%d}`, secondPixelID), admin); changed.Code != http.StatusOK {
		t.Fatalf("visited link could not change TikTok Pixel: %d %s", changed.Code, changed.Body.String())
	}
	metaConnectionID := metaConnection(t, a, "34567", "76543", true)
	var metaPixelID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM meta_pixels WHERE connection_id=$1", metaConnectionID).Scan(&metaPixelID); err != nil {
		t.Fatal(err)
	}
	platformChange := fmt.Sprintf(`{"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"tiktok_pixel_id":null}`, metaConnectionID, metaPixelID)
	if changed := call(a, http.MethodPatch, fmt.Sprintf("/api/v1/links/%d", link.ID), platformChange, admin); changed.Code != http.StatusConflict {
		t.Fatalf("visited link changed platform: %d %s", changed.Code, changed.Body.String())
	}
}

func TestTikTokDirectShortLinkQueuesAutomaticContact(t *testing.T) {
	a := setup(t)
	a.Config.TikTokEnabled = true
	admin := login(t, a)
	pixelID := testTikTokBinding(t, a, "SHORT03")
	body := fmt.Sprintf(`{"name":"TikTok Direct","code":"wa-direct","target_url":"https://wa.me/13365661092","mode":"redirect","enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"attribution_mode":"dynamic"}`, pixelID)
	if response := call(a, http.MethodPost, "/api/v1/links", body, admin); response.Code != http.StatusOK {
		t.Fatalf("create: %d %s", response.Code, response.Body.String())
	}
	page := call(a, http.MethodGet, "/wa-direct?ttclid=direct-click&ad_id_v2=direct-ad", "", nil)
	if page.Code != http.StatusOK || !strings.Contains(page.Body.String(), `top.location = "https://wa.me/13365661092"`) {
		t.Fatalf("direct page: %d %s", page.Code, page.Body.String())
	}
	var visitID, eventName, eventID string
	if err := a.DB.QueryRow(context.Background(), `SELECT e.id,t.event_name,t.event_id FROM click_events e
		JOIN short_links l ON l.id=e.link_id JOIN tiktok_events t ON t.visit_id=e.id
		WHERE l.code='wa-direct'`).Scan(&visitID, &eventName, &eventID); err != nil {
		t.Fatal(err)
	}
	if eventName != "Contact" || eventID != "short_"+visitID+"_auto" {
		t.Fatalf("direct event name=%q id=%q", eventName, eventID)
	}
}
