package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func createDistributionNovel(t *testing.T, a *App, admin *http.Cookie, title, slug string) int64 {
	t.Helper()
	// Helper only creates the content prerequisite; every test still exercises the real HTTP contracts.
	body := fmt.Sprintf(`{"title":%q,"slug":%q,"author":"Nine","category":"Drama","excerpt":"Distribution story.","cover_path":"","published_at":"2026-09-21","enabled":true,"featured":false,"sort_order":0}`, title, slug)
	response := call(a, "POST", "/api/v1/novels", body, admin)
	if response.Code != 200 {
		t.Fatalf("create novel: %d %s", response.Code, response.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	return item.ID
}

func postNovelForm(a *App, path string, values url.Values) *httptest.ResponseRecorder {
	request := httptest.NewRequest("POST", path, strings.NewReader(values.Encode()))
	request.RemoteAddr = "192.0.2.3:43123"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Safari/604.1")
	response := httptest.NewRecorder()
	a.Router.ServeHTTP(response, request)
	return response
}

func testTikTokBinding(t *testing.T, a *App, suffix string) int64 {
	t.Helper()
	// Link validation needs only an enabled configuration; no plaintext token is
	// used by this attribution test or exposed through the public request.
	var connectionID, pixelID int64
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO tiktok_connections(name,access_token_cipher,enabled)
		VALUES($1,'test-cipher',true) RETURNING id`, "TikTok "+suffix).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO tiktok_pixels(connection_id,name,pixel_code,enabled)
		VALUES($1,$2,$3,true) RETURNING id`, connectionID, "Pixel "+suffix, "PX_"+suffix).Scan(&pixelID); err != nil {
		t.Fatal(err)
	}
	return pixelID
}

func TestTikTokAttributionFreezesOnlyTheFirstNormalReader(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "TikTok Story", "tiktok-story")
	secondNovelID := createDistributionNovel(t, a, admin, "TikTok Story Two", "tiktok-story-two")
	pixelID := testTikTokBinding(t, a, "ATTR01")
	secondPixelID := testTikTokBinding(t, a, "ATTR02")

	create := func(name, code string) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, name, code, novelID, pixelID)
		response := call(a, "POST", "/api/v1/novel-links", body, admin)
		if response.Code != 200 {
			t.Fatalf("create TikTok link %s: %d %s", code, response.Code, response.Body.String())
		}
		if !strings.Contains(response.Body.String(), `"tiktok_template_url":"http://localhost:8080/novel/`+code) {
			t.Fatalf("missing TikTok template: %s", response.Body.String())
		}
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		return item.ID
	}
	firstID, secondID := create("TikTok A", "tik-a"), create("TikTok B", "tik-b")

	// HEAD and crawler probes are tracked but must not lock editable campaign bindings.
	if response := call(a, "HEAD", "/novel/tik-a", "", nil); response.Code != 200 {
		t.Fatalf("HEAD status=%d", response.Code)
	}
	botRequest := httptest.NewRequest("GET", "/novel/tik-a", nil)
	botRequest.RemoteAddr = "192.0.2.4:43123"
	botRequest.Header.Set("User-Agent", "facebookexternalhit/1.1")
	botResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(botResponse, botRequest)
	if botResponse.Code != 200 || len(botResponse.Result().Cookies()) != 0 {
		t.Fatalf("bot response status=%d cookies=%d", botResponse.Code, len(botResponse.Result().Cookies()))
	}
	var firstVisitedAt *time.Time
	if err := a.DB.QueryRow(context.Background(), "SELECT first_visited_at FROM short_links WHERE id=$1", firstID).Scan(&firstVisitedAt); err != nil || firstVisitedAt != nil {
		t.Fatalf("probe locked link: first_visited_at=%v err=%v", firstVisitedAt, err)
	}

	firstPage := call(a, "GET", "/novel/tik-a?ttclid=click-a&campaign_id=campaign-a&adgroup_id=group-a&creative_id=creative-a&ad_id_v2=ad-a&placement=TikTok", "", nil)
	if firstPage.Code != 200 {
		t.Fatalf("first reader: %d %s", firstPage.Code, firstPage.Body.String())
	}
	secondPage := call(a, "GET", "/novel/tik-b?ttclid=click-b&campaign_id=campaign-b&adgroup_id=group-b&creative_id=creative-b&ad_id_v2=ad-b&placement=Pangle", "", nil)
	if secondPage.Code != 200 {
		t.Fatalf("second reader: %d %s", secondPage.Code, secondPage.Body.String())
	}

	for _, body := range []string{
		fmt.Sprintf(`{"name":"TikTok A","novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, secondNovelID, pixelID),
		fmt.Sprintf(`{"name":"TikTok A","novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, novelID, secondPixelID),
	} {
		if response := call(a, "PATCH", "/api/v1/novel-links/"+itoa(firstID), body, admin); response.Code != 409 {
			t.Fatalf("frozen binding update status=%d body=%s", response.Code, response.Body.String())
		}
	}

	type snapshot struct {
		LinkID, PixelID                                                  int64
		Platform, PixelCode, TTCLID, CampaignID, AdgroupID, AdID, Cipher string
	}
	readSnapshot := func(linkID int64) snapshot {
		t.Helper()
		var item snapshot
		err := a.DB.QueryRow(context.Background(), `SELECT link_id,tiktok_pixel_id,ad_platform,tiktok_pixel_code,tiktok_ttclid,
			campaign_id,tiktok_adgroup_id,tiktok_ad_id_v2,tiktok_context_cipher FROM click_events
			WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at DESC LIMIT 1`, linkID).
			Scan(&item.LinkID, &item.PixelID, &item.Platform, &item.PixelCode, &item.TTCLID, &item.CampaignID, &item.AdgroupID, &item.AdID, &item.Cipher)
		if err != nil {
			t.Fatal(err)
		}
		return item
	}
	firstSnapshot, secondSnapshot := readSnapshot(firstID), readSnapshot(secondID)
	if firstSnapshot.LinkID == secondSnapshot.LinkID || firstSnapshot.TTCLID != "click-a" || secondSnapshot.TTCLID != "click-b" ||
		firstSnapshot.Platform != "tiktok" || firstSnapshot.PixelID != pixelID || firstSnapshot.PixelCode != "PX_ATTR01" ||
		firstSnapshot.CampaignID != "campaign-a" || firstSnapshot.AdgroupID != "group-a" || firstSnapshot.AdID != "ad-a" || firstSnapshot.Cipher == "" {
		t.Fatalf("attribution snapshots not isolated: first=%+v second=%+v", firstSnapshot, secondSnapshot)
	}
}

func TestNovelDistributionLinkLifecycleLocksContentAfterFirstVisit(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	first := createDistributionNovel(t, a, admin, "Craving For My Divorced Wife", "craving-for-my-divorced-wife")
	second := createDistributionNovel(t, a, admin, "Second Story", "second-story")
	connectionID, pixelID := testLinkMetaBinding(t, a)

	createBody := fmt.Sprintf(`{"name":"投手 A","code":"wife-a","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, first, connectionID, pixelID)
	created := call(a, "POST", "/api/v1/novel-links", createBody, admin)
	if created.Code != 200 {
		t.Fatalf("create distribution link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID          int64  `json:"id"`
		ProductType string `json:"product_type"`
		PublicURL   string `json:"public_url"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if link.ProductType != "novel" || link.PublicURL != "http://localhost:8080/novel/wife-a" {
		t.Fatalf("unexpected link: %+v", link)
	}
	// Short codes remain globally unique, while an unused auto-generated link can be removed cleanly.
	if response := call(a, "POST", "/api/v1/novel-links", createBody, admin); response.Code != 409 {
		t.Fatalf("duplicate short code status = %d, body=%s", response.Code, response.Body.String())
	}
	autoBody := fmt.Sprintf(`{"name":"临时投放","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, first, connectionID, pixelID)
	autoCreated := call(a, "POST", "/api/v1/novel-links", autoBody, admin)
	if autoCreated.Code != 200 {
		t.Fatalf("create automatic code: %d %s", autoCreated.Code, autoCreated.Body.String())
	}
	var automatic struct {
		ID   int64  `json:"id"`
		Code string `json:"code"`
	}
	if err := json.Unmarshal(autoCreated.Body.Bytes(), &automatic); err != nil || automatic.Code == "" {
		t.Fatalf("automatic code response: %s, err=%v", autoCreated.Body.String(), err)
	}
	if response := call(a, "DELETE", "/api/v1/novel-links/"+itoa(automatic.ID), "", admin); response.Code != 204 {
		t.Fatalf("delete unused automatic link: %d %s", response.Code, response.Body.String())
	}

	// 没有访问时允许修正小说绑定。
	updateBody := fmt.Sprintf(`{"name":"投手 A","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, second, connectionID, pixelID)
	if response := call(a, "PATCH", "/api/v1/novel-links/"+itoa(link.ID), updateBody, admin); response.Code != 200 {
		t.Fatalf("update unused distribution link: %d %s", response.Code, response.Body.String())
	}
	if page := call(a, "GET", "/novel/wife-a", "", nil); page.Code != 200 {
		t.Fatalf("open distribution link: %d %s", page.Code, page.Body.String())
	}
	// Disabling bound content makes the campaign unavailable and re-enabling restores it.
	if response := call(a, "PATCH", "/api/v1/novels/"+itoa(second)+"/status", `{"enabled":false}`, admin); response.Code != 200 {
		t.Fatalf("disable bound novel: %d %s", response.Code, response.Body.String())
	}
	if page := call(a, "GET", "/novel/wife-a", "", nil); page.Code != 410 {
		t.Fatalf("disabled novel entry status = %d, body=%s", page.Code, page.Body.String())
	}
	if response := call(a, "PATCH", "/api/v1/novels/"+itoa(second)+"/status", `{"enabled":true}`, admin); response.Code != 200 {
		t.Fatalf("restore bound novel: %d %s", response.Code, response.Body.String())
	}
	if page := call(a, "GET", "/novel/wife-a", "", nil); page.Code != 200 {
		t.Fatalf("restored novel entry status = %d, body=%s", page.Code, page.Body.String())
	}
	// 即使明细按保留期清理，永久首访标记也必须继续锁定小说绑定和删除。
	if _, err := a.DB.Exec(context.Background(), "DELETE FROM click_events WHERE link_id=$1", link.ID); err != nil {
		t.Fatal(err)
	}
	lockBody := fmt.Sprintf(`{"name":"投手 A","novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, first, connectionID, pixelID)
	if response := call(a, "PATCH", "/api/v1/novel-links/"+itoa(link.ID), lockBody, admin); response.Code != 409 {
		t.Fatalf("visited link rebinding status = %d, body=%s", response.Code, response.Body.String())
	}
	if response := call(a, "DELETE", "/api/v1/novel-links/"+itoa(link.ID), "", admin); response.Code != 409 {
		t.Fatalf("visited link delete status = %d, body=%s", response.Code, response.Body.String())
	}
	// 普通短链接批量删除接口也不能绕过小说链接的历史数据保护。
	if response := call(a, "POST", "/api/v1/links/batch-delete", fmt.Sprintf(`{"ids":[%d]}`, link.ID), admin); response.Code != 404 {
		t.Fatalf("generic delete accepted novel link: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "GET", "/api/v1/novel-links?novel_id="+itoa(second), "", admin); response.Code != 200 {
		t.Fatalf("list distribution links: %d %s", response.Code, response.Body.String())
	}
}

func TestNovelDistributionLinksKeepIndependentVisitorsAndVisibleTime(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "Craving For My Divorced Wife", "craving-for-my-divorced-wife")
	connectionID, pixelID := testLinkMetaBinding(t, a)
	create := func(name, code string) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"novel_id":%d,"enabled":true,"channel":"facebook","meta_connection_id":%d,"meta_pixel_id":%d,"attribution_mode":"dynamic","time_spent_threshold":0}`, name, code, novelID, connectionID, pixelID)
		response := call(a, "POST", "/api/v1/novel-links", body, admin)
		if response.Code != 200 {
			t.Fatalf("create %s: %d %s", code, response.Code, response.Body.String())
		}
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		return item.ID
	}
	firstID, secondID := create("投手 A", "wife-a"), create("投手 B", "wife-b")
	firstPage := call(a, "GET", "/novel/wife-a?ad_id=ad-a", "", nil)
	if firstPage.Code != 200 || !strings.Contains(firstPage.Body.String(), `"entry_story_slug":"craving-for-my-divorced-wife"`) {
		t.Fatalf("bound bootstrap: %d %s", firstPage.Code, firstPage.Body.String())
	}
	visitorCookie := firstPage.Result().Cookies()[0]
	if secondPage := call(a, "GET", "/novel/wife-b?ad_id=ad-b", "", visitorCookie); secondPage.Code != 200 {
		t.Fatalf("second pitcher entry: %d", secondPage.Code)
	}
	var firstEvent string
	if err := a.DB.QueryRow(context.Background(), "SELECT id FROM click_events WHERE link_id=$1", firstID).Scan(&firstEvent); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '40 seconds' WHERE id=$1", firstEvent); err != nil {
		t.Fatal(err)
	}
	ticket := firstEvent + "." + a.Sign("contact:novel:wife-a:"+firstEvent)
	// A forged ticket cannot update another or invented visit.
	if response := postNovelForm(a, "/novel/wife-a/reading-time", url.Values{"ticket": {"bad.ticket"}, "seconds": {"20"}}); response.Code != 400 {
		t.Fatalf("forged reading ticket status = %d, body=%s", response.Code, response.Body.String())
	}
	for _, seconds := range []string{"20", "10", "9000"} {
		response := postNovelForm(a, "/novel/wife-a/reading-time", url.Values{"ticket": {ticket}, "seconds": {seconds}})
		if response.Code != 200 {
			t.Fatalf("reading time %s: %d %s", seconds, response.Code, response.Body.String())
		}
	}
	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	stats := call(a, "POST", "/api/v1/novel-links/"+itoa(firstID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","ad_id":"","page":1}`, today, today), admin)
	if stats.Code != 200 {
		t.Fatalf("first stats: %d %s", stats.Code, stats.Body.String())
	}
	var report struct {
		Summary struct {
			Visits              int64 `json:"visits"`
			UniqueVisitors      int64 `json:"unique_visitors"`
			TotalVisibleSeconds int64 `json:"total_visible_seconds"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Visits != 1 || report.Summary.UniqueVisitors != 1 || report.Summary.TotalVisibleSeconds < 40 || report.Summary.TotalVisibleSeconds > 46 {
		t.Fatalf("unexpected independent stats: %s", stats.Body.String())
	}
	// Ad filtering stays inside this link and cannot pull the second pitcher's visit.
	matching := call(a, "POST", "/api/v1/novel-links/"+itoa(firstID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","ad_id":"ad-a","page":1}`, today, today), admin)
	nonMatching := call(a, "POST", "/api/v1/novel-links/"+itoa(firstID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","ad_id":"ad-b","page":1}`, today, today), admin)
	if !strings.Contains(matching.Body.String(), `"visits":1`) || !strings.Contains(nonMatching.Body.String(), `"visits":0`) {
		t.Fatalf("ad filters mismatch: matching=%s nonmatching=%s", matching.Body.String(), nonMatching.Body.String())
	}
	var firstVisits, secondVisits int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FILTER(WHERE link_id=$1),count(*) FILTER(WHERE link_id=$2) FROM click_events", firstID, secondID).Scan(&firstVisits, &secondVisits); err != nil || firstVisits != 1 || secondVisits != 1 {
		t.Fatalf("isolated visits = %d/%d, err=%v", firstVisits, secondVisits, err)
	}
}

func TestTikTokDistributionStatsKeepLinkFunnelAndDeliveryIsolated(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "TikTok Stats Story", "tiktok-stats-story")
	pixelID := testTikTokBinding(t, a, "STATS01")
	create := func(name, code string) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, name, code, novelID, pixelID)
		response := call(a, "POST", "/api/v1/novel-links", body, admin)
		if response.Code != 200 {
			t.Fatalf("create TikTok stats link: %d %s", response.Code, response.Body.String())
		}
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		return item.ID
	}
	firstID, secondID := create("TikTok Stats A", "stats-a"), create("TikTok Stats B", "stats-b")

	firstPage := call(a, "GET", "/novel/stats-a?ttclid=1234567890abcdef&campaign_id=campaign-a&adgroup_id=group-a&creative_id=creative-a&ad_id_v2=ad-a&placement=TikTok", "", nil)
	if firstPage.Code != 200 || len(firstPage.Result().Cookies()) == 0 {
		t.Fatalf("first stats visit: %d %s", firstPage.Code, firstPage.Body.String())
	}
	visitor := firstPage.Result().Cookies()[0]
	if response := call(a, "GET", "/novel/stats-a?ttclid=second-click&campaign_id=campaign-b&adgroup_id=group-b&creative_id=creative-b&ad_id_v2=ad-b&placement=Pangle", "", visitor); response.Code != 200 {
		t.Fatalf("second stats visit: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "GET", "/novel/stats-b?ttclid=other-link", "", visitor); response.Code != 200 {
		t.Fatalf("other link visit: %d %s", response.Code, response.Body.String())
	}

	rows, err := a.DB.Query(context.Background(), `SELECT id FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at,id`, firstID)
	if err != nil {
		t.Fatal(err)
	}
	var visitIDs []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		visitIDs = append(visitIDs, id)
	}
	rows.Close()
	if len(visitIDs) != 2 {
		t.Fatalf("first link visits = %d, want 2", len(visitIDs))
	}
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET visible_seconds=20,visible_updated_at=now(),tiktok_start_reading_at=now(),tiktok_view_content_at=now() WHERE id=$1`, visitIDs[0]); err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET visible_seconds=10,visible_updated_at=now(),tiktok_start_reading_at=now() WHERE id=$1`, visitIDs[1]); err != nil {
		t.Fatal(err)
	}
	var connectionID int64
	if err = a.DB.QueryRow(context.Background(), "SELECT connection_id FROM tiktok_pixels WHERE id=$1", pixelID).Scan(&connectionID); err != nil {
		t.Fatal(err)
	}
	insertEvent := func(id, visitID, eventName, status string) {
		t.Helper()
		_, insertErr := a.DB.Exec(context.Background(), `INSERT INTO tiktok_events
			(id,visit_id,link_id,novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,payload_cipher,status)
			VALUES($1,$2,$3,$4,$5,$6,'PX_STATS01',$7,$1,now(),'test-payload',$8)`, id, visitID, firstID, novelID, connectionID, pixelID, eventName, status)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insertEvent("stats-start-pending", visitIDs[0], "StartReading", "pending")
	insertEvent("stats-qualified-accepted", visitIDs[0], "ViewContent", "accepted")
	insertEvent("stats-start-failed", visitIDs[1], "StartReading", "failed")

	today := time.Now().In(mustLocation("Asia/Shanghai")).Format("2006-01-02")
	stats := call(a, "POST", "/api/v1/novel-links/"+itoa(firstID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","page":1}`, today, today), admin)
	if stats.Code != 200 {
		t.Fatalf("TikTok stats: %d %s", stats.Code, stats.Body.String())
	}
	var report struct {
		Summary struct {
			Visits               int64   `json:"visits"`
			UniqueVisitors       int64   `json:"unique_visitors"`
			StartReadingCount    int64   `json:"start_reading_count"`
			StartReadingVisitors int64   `json:"start_reading_visitors"`
			QualifiedCount       int64   `json:"qualified_count"`
			QualifiedVisitors    int64   `json:"qualified_visitors"`
			Pending              int64   `json:"tiktok_pending_events"`
			Accepted             int64   `json:"tiktok_accepted_events"`
			Failed               int64   `json:"tiktok_failed_events"`
			QualifiedRate        float64 `json:"qualified_rate"`
		} `json:"summary"`
	}
	if err = json.Unmarshal(stats.Body.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Summary.Visits != 2 || report.Summary.UniqueVisitors != 1 || report.Summary.StartReadingCount != 2 ||
		report.Summary.StartReadingVisitors != 1 || report.Summary.QualifiedCount != 1 || report.Summary.QualifiedVisitors != 1 ||
		report.Summary.QualifiedRate != 50 || report.Summary.Pending != 1 || report.Summary.Accepted != 1 || report.Summary.Failed != 1 {
		t.Fatalf("unexpected TikTok funnel summary: %s", stats.Body.String())
	}
	if !strings.Contains(stats.Body.String(), `"ttclid":"1234…cdef"`) || strings.Contains(stats.Body.String(), "1234567890abcdef") ||
		strings.Contains(stats.Body.String(), "tiktok_ttp") || strings.Contains(stats.Body.String(), "tiktok_context_cipher") {
		t.Fatalf("TikTok detail was not safely masked: %s", stats.Body.String())
	}
	acceptedOnly := call(a, "POST", "/api/v1/novel-links/"+itoa(firstID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","campaign_id":"campaign-a","adgroup_id":"group-a","creative_id":"creative-a","ad_id_v2":"ad-a","event_status":"accepted","page":1}`, today, today), admin)
	if acceptedOnly.Code != 200 || !strings.Contains(acceptedOnly.Body.String(), `"visits":1`) {
		t.Fatalf("TikTok attribution filters: %d %s", acceptedOnly.Code, acceptedOnly.Body.String())
	}
	other := call(a, "POST", "/api/v1/novel-links/"+itoa(secondID)+"/stats", fmt.Sprintf(`{"start":%q,"end":%q,"tz":"Asia/Shanghai","page":1}`, today, today), admin)
	if other.Code != 200 || !strings.Contains(other.Body.String(), `"visits":1`) {
		t.Fatalf("other link stats were mixed: %d %s", other.Code, other.Body.String())
	}
}
