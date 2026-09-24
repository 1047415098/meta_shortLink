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

	"whatsapp-analytics/internal/modules/audionovel"
)

func createAudioDistributionNovel(t *testing.T, a *App, admin *http.Cookie, title, slug string, withAudio bool) int64 {
	t.Helper()
	audioFields := ""
	if withAudio {
		audioFields = `,"audio_path":"/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3","audio_duration":"00:30","audio_duration_seconds":30,"audio_size_bytes":1024`
	}
	body := fmt.Sprintf(`{"title":%q,"slug":%q,"category":"Drama","excerpt":"Audio campaign story.","body_markdown":"# Story","cover_path":"","published_at":"2026-09-24","enabled":true,"featured":false%s}`, title, slug, audioFields)
	response := call(a, "POST", "/api/v1/audio-novels", body, admin)
	if response.Code != 200 {
		t.Fatalf("create audio novel: %d %s", response.Code, response.Body.String())
	}
	var item struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	return item.ID
}

func decodeAudioBootstrap(t *testing.T, response *httptest.ResponseRecorder) audionovel.Bootstrap {
	t.Helper()
	match := regexp.MustCompile(`<script id="audio-novel-data"[^>]*>(.*?)</script>`).FindStringSubmatch(response.Body.String())
	if len(match) != 2 {
		t.Fatalf("audio bootstrap missing: %s", response.Body.String())
	}
	var data audionovel.Bootstrap
	if err := json.Unmarshal([]byte(match[1]), &data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAudioNovelDistributionLinkLifecycle(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	firstAudioID := createAudioDistributionNovel(t, a, admin, "Audio Campaign One", "audio-campaign-one", true)
	secondAudioID := createAudioDistributionNovel(t, a, admin, "Audio Campaign Two", "audio-campaign-two", true)
	missingMP3ID := createAudioDistributionNovel(t, a, admin, "Audio Missing MP3", "audio-missing-mp3", false)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	tiktokPixelID := testTikTokBinding(t, a, "AUDIO01")
	secondTikTokPixelID := testTikTokBinding(t, a, "AUDIO02")

	metaBody := fmt.Sprintf(`{"name":"Buyer Meta","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d}`, firstAudioID, metaConnectionID, metaPixelID)
	createdMeta := call(a, "POST", "/api/v1/audio-novel-links", metaBody, admin)
	if createdMeta.Code != 200 {
		t.Fatalf("create Meta audio link: %d %s", createdMeta.Code, createdMeta.Body.String())
	}
	var metaLink struct {
		ID                 int64  `json:"id"`
		Code               string `json:"code"`
		ProductType        string `json:"product_type"`
		PublicURL          string `json:"public_url"`
		TimeSpentThreshold int    `json:"time_spent_threshold"`
	}
	if err := json.Unmarshal(createdMeta.Body.Bytes(), &metaLink); err != nil {
		t.Fatal(err)
	}
	if metaLink.Code == "" || metaLink.ProductType != "audio_novel" || metaLink.TimeSpentThreshold != 10 ||
		metaLink.PublicURL != "http://localhost:8080/audio-novel/"+metaLink.Code {
		t.Fatalf("unexpected Meta audio link: %s", createdMeta.Body.String())
	}

	tiktokBody := fmt.Sprintf(`{"name":"Buyer TikTok","code":"audio-tik-a","audio_novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":1}`, firstAudioID, tiktokPixelID)
	createdTikTok := call(a, "POST", "/api/v1/audio-novel-links", tiktokBody, admin)
	if createdTikTok.Code != 200 || !strings.Contains(createdTikTok.Body.String(), `"tiktok_template_url":"http://localhost:8080/audio-novel/audio-tik-a`) {
		t.Fatalf("create TikTok audio link: %d %s", createdTikTok.Code, createdTikTok.Body.String())
	}
	var tiktokLink struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(createdTikTok.Body.Bytes(), &tiktokLink); err != nil {
		t.Fatal(err)
	}
	if duplicate := call(a, "POST", "/api/v1/audio-novel-links", strings.Replace(tiktokBody, "Buyer TikTok", "Duplicate", 1), admin); duplicate.Code != 409 {
		t.Fatalf("duplicate code status=%d body=%s", duplicate.Code, duplicate.Body.String())
	}

	invalidBodies := []string{
		fmt.Sprintf(`{"name":"Both Pixels","code":"audio-both","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"tiktok_pixel_id":%d,"time_spent_threshold":10}`, firstAudioID, metaConnectionID, metaPixelID, tiktokPixelID),
		fmt.Sprintf(`{"name":"Missing Pixel","code":"audio-no-pixel","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"time_spent_threshold":10}`, firstAudioID, metaConnectionID),
		fmt.Sprintf(`{"name":"Missing MP3","code":"audio-no-mp3","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, missingMP3ID, metaConnectionID, metaPixelID),
		fmt.Sprintf(`{"name":"Low Threshold","code":"audio-low","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":-1}`, firstAudioID, metaConnectionID, metaPixelID),
		fmt.Sprintf(`{"name":"High Threshold","code":"audio-high","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":3601}`, firstAudioID, metaConnectionID, metaPixelID),
	}
	for index, body := range invalidBodies {
		response := call(a, "POST", "/api/v1/audio-novel-links", body, admin)
		if response.Code != 400 {
			t.Fatalf("invalid create %d status=%d body=%s", index, response.Code, response.Body.String())
		}
	}
	unknown := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"Unknown","code":"audio-unknown","audio_novel_id":999999,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, metaConnectionID, metaPixelID), admin)
	if unknown.Code != 404 {
		t.Fatalf("unknown content status=%d body=%s", unknown.Code, unknown.Body.String())
	}

	var disabledTikTokPixelID int64
	if err := a.DB.QueryRow(context.Background(), `INSERT INTO tiktok_pixels(connection_id,name,pixel_code,enabled)
		SELECT connection_id,'Disabled Audio Pixel','PX_AUDIO_DISABLED',false FROM tiktok_pixels WHERE id=$1 RETURNING id`, tiktokPixelID).Scan(&disabledTikTokPixelID); err != nil {
		t.Fatal(err)
	}
	disabledPixelBody := fmt.Sprintf(`{"name":"Disabled Pixel","code":"audio-disabled-pixel","audio_novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, firstAudioID, disabledTikTokPixelID)
	if response := call(a, "POST", "/api/v1/audio-novel-links", disabledPixelBody, admin); response.Code != 400 {
		t.Fatalf("disabled Pixel status=%d body=%s", response.Code, response.Body.String())
	}

	list := call(a, "GET", "/api/v1/audio-novel-links?audio_novel_id="+itoa(firstAudioID), "", admin)
	if list.Code != 200 {
		t.Fatalf("list audio links: %d %s", list.Code, list.Body.String())
	}
	var listed struct {
		Items []struct {
			ID          int64  `json:"id"`
			ProductType string `json:"product_type"`
		} `json:"items"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 {
		t.Fatalf("audio list length=%d body=%s", len(listed.Items), list.Body.String())
	}
	for _, item := range listed.Items {
		if item.ProductType != "audio_novel" {
			t.Fatalf("non-audio link leaked into list: %+v", item)
		}
	}
	// The original short-link screen must never mix campaign links from the
	// audio product into its edit and deletion workflow.
	ordinaryList := call(a, "GET", "/api/v1/links", "", admin)
	if ordinaryList.Code != 200 {
		t.Fatalf("list ordinary links: %d %s", ordinaryList.Code, ordinaryList.Body.String())
	}
	var ordinaryItems []map[string]any
	if err := json.Unmarshal(ordinaryList.Body.Bytes(), &ordinaryItems); err != nil {
		t.Fatal(err)
	}
	for _, item := range ordinaryItems {
		if item["product_type"] == "audio_novel" || item["code"] == metaLink.Code || item["code"] == "audio-tik-a" {
			t.Fatalf("audio campaign leaked into ordinary links: %+v", item)
		}
	}
	// Generic short-link mutation and deletion must not convert or remove an
	// audio campaign link, even before its first normal reader visit.
	ordinaryPatch := `{"code":"ordinary-hijack","name":"Hijacked","target_url":"https://wa.me/13365661092","product_type":"short_link","audio_novel_id":null,"mode":"redirect","attribution_mode":"dynamic"}`
	if response := call(a, "PATCH", "/api/v1/links/"+itoa(metaLink.ID), ordinaryPatch, admin); response.Code != 404 {
		t.Fatalf("generic update accepted audio link: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "POST", "/api/v1/links/batch-delete", fmt.Sprintf(`{"ids":[%d]}`, metaLink.ID), admin); response.Code != 404 {
		t.Fatalf("generic delete accepted audio link: %d %s", response.Code, response.Body.String())
	}
	var preservedProduct, preservedCode string
	if err := a.DB.QueryRow(context.Background(), "SELECT product_type,code FROM short_links WHERE id=$1", metaLink.ID).Scan(&preservedProduct, &preservedCode); err != nil || preservedProduct != "audio_novel" || preservedCode != metaLink.Code {
		t.Fatalf("generic API changed audio link: product=%q code=%q err=%v", preservedProduct, preservedCode, err)
	}

	// Before the first visit, operators may fix both content and platform binding.
	updateBeforeVisit := fmt.Sprintf(`{"name":"Buyer Meta Updated","audio_novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":20}`, secondAudioID, secondTikTokPixelID)
	if response := call(a, "PATCH", "/api/v1/audio-novel-links/"+itoa(metaLink.ID), updateBeforeVisit, admin); response.Code != 200 {
		t.Fatalf("update unused audio link: %d %s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE short_links SET first_visited_at=now() WHERE id=$1", metaLink.ID); err != nil {
		t.Fatal(err)
	}
	lockedRebind := fmt.Sprintf(`{"name":"Buyer Meta Updated","audio_novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":20}`, firstAudioID, secondTikTokPixelID)
	if response := call(a, "PATCH", "/api/v1/audio-novel-links/"+itoa(metaLink.ID), lockedRebind, admin); response.Code != 409 {
		t.Fatalf("visited audio rebind status=%d body=%s", response.Code, response.Body.String())
	}
	mutableUpdate := fmt.Sprintf(`{"name":"Buyer Renamed","audio_novel_id":%d,"enabled":false,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":30}`, secondAudioID, secondTikTokPixelID)
	if response := call(a, "PATCH", "/api/v1/audio-novel-links/"+itoa(metaLink.ID), mutableUpdate, admin); response.Code != 200 {
		t.Fatalf("visited mutable fields status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call(a, "DELETE", "/api/v1/audio-novel-links/"+itoa(metaLink.ID), "", admin); response.Code != 409 {
		t.Fatalf("visited delete status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call(a, "DELETE", "/api/v1/audio-novel-links/"+itoa(tiktokLink.ID), "", admin); response.Code != 204 {
		t.Fatalf("unused delete status=%d body=%s", response.Code, response.Body.String())
	}

	if response := call(a, "DELETE", "/api/v1/meta/pixels/"+itoa(metaPixelID), "", admin); response.Code != 200 {
		// Meta is no longer referenced after the pre-visit platform switch.
		t.Fatalf("unreferenced Meta Pixel delete status=%d body=%s", response.Code, response.Body.String())
	}
	if response := call(a, "DELETE", "/api/v1/tiktok-pixels/"+itoa(secondTikTokPixelID), "", admin); response.Code != 409 {
		t.Fatalf("referenced TikTok Pixel delete status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAudioNovelDistributionEntryFreezesAttributionAndDirectOpensPlayer(t *testing.T) {
	a := setup(t)
	a.Config.TikTokEnabled = true
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Audio Entry Story", "audio-entry-story", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	tiktokPixelID := testTikTokBinding(t, a, "AUDIOENTRY")

	create := func(name, code, platform string, tiktokID *int64) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"audio_novel_id":%d,"enabled":true,"ad_platform":%q,"time_spent_threshold":10`, name, code, audioNovelID, platform)
		if platform == "tiktok" {
			body += fmt.Sprintf(`,"tiktok_pixel_id":%d}`, *tiktokID)
		} else {
			body += fmt.Sprintf(`,"meta_connection_id":%d,"meta_pixel_id":%d}`, metaConnectionID, metaPixelID)
		}
		response := call(a, "POST", "/api/v1/audio-novel-links", body, admin)
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

	firstID := create("Buyer A", "audio-entry-a", "meta", nil)
	secondID := create("Buyer B", "audio-entry-b", "meta", nil)
	tiktokID := create("Buyer TikTok", "audio-entry-tiktok", "tiktok", &tiktokPixelID)
	for _, wrongSurface := range []string{"/audio-entry-a", "/novel/audio-entry-a"} {
		if response := call(a, "GET", wrongSurface, "", nil); response.Code != 404 {
			t.Fatalf("audio campaign entered wrong surface %s: %d %s", wrongSurface, response.Code, response.Body.String())
		}
	}

	firstPage := call(a, "GET", "/audio-novel/audio-entry-a?utm_source=facebook&campaign_id=campaign-a&adset_id=group-a&ad_id=ad-a", "", nil)
	if firstPage.Code != 200 || !strings.Contains(firstPage.Body.String(), `"entry_audio_slug":"audio-entry-story"`) ||
		!strings.Contains(firstPage.Body.String(), `"playback_threshold_seconds":10`) ||
		!strings.Contains(firstPage.Body.String(), `"playback_ticket":"`) ||
		!strings.Contains(firstPage.Body.String(), `"ad_platform":"meta"`) {
		t.Fatalf("Meta entry bootstrap: %d %s", firstPage.Code, firstPage.Body.String())
	}
	if strings.Contains(firstPage.Body.String(), `"tiktok_pixel_code"`) {
		t.Fatalf("Meta entry exposed TikTok configuration: %s", firstPage.Body.String())
	}
	visitorCookie := firstPage.Result().Cookies()[0]
	secondPage := call(a, "GET", "/audio-novel/audio-entry-b?campaign_id=campaign-b&ad_id=ad-b", "", visitorCookie)
	if secondPage.Code != 200 {
		t.Fatalf("second buyer entry: %d %s", secondPage.Code, secondPage.Body.String())
	}
	tiktokPage := call(a, "GET", "/audio-novel/audio-entry-tiktok?ttclid=click-tiktok&campaign_id=campaign-tiktok&adgroup_id=group-tiktok&creative_id=creative-tiktok&ad_id_v2=ad-tiktok", "", visitorCookie)
	if tiktokPage.Code != 200 || !strings.Contains(tiktokPage.Body.String(), `"ad_platform":"tiktok"`) ||
		!strings.Contains(tiktokPage.Body.String(), `"tiktok_enabled":true`) ||
		!strings.Contains(tiktokPage.Body.String(), `"tiktok_pixel_code":"PX_AUDIOENTRY"`) ||
		strings.Contains(tiktokPage.Body.String(), `"meta_browser_pixel_id"`) {
		t.Fatalf("TikTok entry bootstrap: %d %s", tiktokPage.Code, tiktokPage.Body.String())
	}

	type snapshot struct {
		LinkID, AudioNovelID, PixelID, Threshold int64
		Platform, CampaignID, AdsetID, AdID      string
	}
	readSnapshot := func(linkID int64) snapshot {
		t.Helper()
		var item snapshot
		if err := a.DB.QueryRow(context.Background(), `SELECT link_id,audio_novel_id,COALESCE(meta_pixel_id,tiktok_pixel_id),time_spent_threshold,
			ad_platform,campaign_id,adset_id,ad_id FROM click_events
			WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at DESC LIMIT 1`, linkID).
			Scan(&item.LinkID, &item.AudioNovelID, &item.PixelID, &item.Threshold, &item.Platform, &item.CampaignID, &item.AdsetID, &item.AdID); err != nil {
			t.Fatal(err)
		}
		return item
	}
	first, second, tikTok := readSnapshot(firstID), readSnapshot(secondID), readSnapshot(tiktokID)
	if first.LinkID == second.LinkID || first.AudioNovelID != audioNovelID || second.AudioNovelID != audioNovelID ||
		first.PixelID != metaPixelID || first.Threshold != 10 || first.Platform != "meta" ||
		first.CampaignID != "campaign-a" || first.AdsetID != "group-a" || first.AdID != "ad-a" ||
		tikTok.AudioNovelID != audioNovelID || tikTok.PixelID != tiktokPixelID || tikTok.Platform != "tiktok" || tikTok.CampaignID != "campaign-tiktok" {
		t.Fatalf("audio attribution snapshots are not isolated: first=%+v second=%+v TikTok=%+v", first, second, tikTok)
	}

	// Content API and in-document route changes reuse the original campaign visit.
	if response := call(a, "GET", "/audio-novel-api/audio-entry-a/audio/audio-entry-story", "", visitorCookie); response.Code != 200 {
		t.Fatalf("bound audio content: %d %s", response.Code, response.Body.String())
	}
	var firstVisits int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal'", firstID).Scan(&firstVisits); err != nil || firstVisits != 1 {
		t.Fatalf("SPA content navigation created another entry: visits=%d err=%v", firstVisits, err)
	}
}

func TestAudioNovelDistributionEntryKeepsProbesEditableAndUnavailableContentUncounted(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Audio Availability", "audio-availability", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	createBody := fmt.Sprintf(`{"name":"Availability Buyer","code":"audio-availability-link","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID)
	created := call(a, "POST", "/api/v1/audio-novel-links", createBody, admin)
	if created.Code != 200 {
		t.Fatalf("create availability link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}

	if response := call(a, "HEAD", "/audio-novel/audio-availability-link", "", nil); response.Code != 200 {
		t.Fatalf("HEAD probe: %d", response.Code)
	}
	botRequest := httptest.NewRequest("GET", "/audio-novel/audio-availability-link", nil)
	botRequest.RemoteAddr = "192.0.2.40:43123"
	botRequest.Header.Set("User-Agent", "facebookexternalhit/1.1")
	botResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(botResponse, botRequest)
	if botResponse.Code != 200 || len(botResponse.Result().Cookies()) != 0 || strings.Contains(botResponse.Body.String(), `"playback_ticket"`) {
		t.Fatalf("bot entry status=%d cookies=%d", botResponse.Code, len(botResponse.Result().Cookies()))
	}
	suspiciousIP := "198.51.100.20"
	for range 120 {
		a.Exceed("click:"+a.Sign(suspiciousIP), 120, time.Minute)
	}
	suspiciousRequest := httptest.NewRequest("GET", "/audio-novel/audio-availability-link", nil)
	suspiciousRequest.RemoteAddr = suspiciousIP + ":43123"
	suspiciousRequest.Header.Set("User-Agent", "Mozilla/5.0 Safari/604.1")
	suspiciousResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(suspiciousResponse, suspiciousRequest)
	if suspiciousResponse.Code != 200 || strings.Contains(suspiciousResponse.Body.String(), `"playback_ticket"`) {
		t.Fatalf("suspicious entry: %d %s", suspiciousResponse.Code, suspiciousResponse.Body.String())
	}
	var firstVisitedAt *time.Time
	if err := a.DB.QueryRow(context.Background(), "SELECT first_visited_at FROM short_links WHERE id=$1", link.ID).Scan(&firstVisitedAt); err != nil || firstVisitedAt != nil {
		t.Fatalf("probe froze audio link: first_visited_at=%v err=%v", firstVisitedAt, err)
	}
	var probes int
	if err := a.DB.QueryRow(context.Background(), `SELECT count(*) FROM click_events WHERE link_id=$1 AND classification IN ('head','bot','suspicious')`, link.ID).Scan(&probes); err != nil || probes != 3 {
		t.Fatalf("probe visits not observable: count=%d err=%v", probes, err)
	}
	// Probe rows still own foreign-key protected history, so deletion must be a
	// clear business conflict rather than leaking a database error as HTTP 500.
	if response := call(a, "DELETE", "/api/v1/audio-novel-links/"+itoa(link.ID), "", admin); response.Code != 409 {
		t.Fatalf("probe-only audio link delete status=%d body=%s", response.Code, response.Body.String())
	}

	// Each unavailable state must fail before a normal campaign visit is created.
	if _, err := a.DB.Exec(context.Background(), "UPDATE audio_novels SET enabled=false WHERE id=$1", audioNovelID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, "GET", "/audio-novel/audio-availability-link", "", nil); response.Code != 410 {
		t.Fatalf("disabled audio status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE audio_novels SET enabled=true,audio_path='',audio_duration='',audio_duration_seconds=0,audio_size_bytes=0 WHERE id=$1", audioNovelID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, "GET", "/audio-novel/audio-availability-link", "", nil); response.Code != 410 {
		t.Fatalf("MP3-less audio status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), `UPDATE audio_novels SET audio_path='/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3',audio_duration='00:30',audio_duration_seconds=30,audio_size_bytes=1024,deleted_at=now() WHERE id=$1`, audioNovelID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, "GET", "/audio-novel/audio-availability-link", "", nil); response.Code != 410 {
		t.Fatalf("deleted audio status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE audio_novels SET deleted_at=NULL WHERE id=$1", audioNovelID); err != nil {
		t.Fatal(err)
	}
	if response := call(a, "GET", "/audio-novel/audio-availability-link", "", nil); response.Code != 200 {
		t.Fatalf("restored audio status=%d body=%s", response.Code, response.Body.String())
	}
	var normalVisits int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal'", link.ID).Scan(&normalVisits); err != nil || normalVisits != 1 {
		t.Fatalf("unavailable entries counted as normal visits: count=%d err=%v", normalVisits, err)
	}

	textNovelID := createDistributionNovel(t, a, admin, "Wrong Product", "wrong-product")
	textBody := fmt.Sprintf(`{"name":"Text Buyer","code":"text-on-audio","novel_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"ad_platform":"meta","time_spent_threshold":10}`, textNovelID, metaConnectionID, metaPixelID)
	if response := call(a, "POST", "/api/v1/novel-links", textBody, admin); response.Code != 200 {
		t.Fatalf("create text link: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "GET", "/audio-novel/text-on-audio", "", nil); response.Code != 404 {
		t.Fatalf("text link entered audio surface: %d %s", response.Code, response.Body.String())
	}
	legacy := call(a, "GET", "/audio-novel/hello", "", nil)
	if legacy.Code != 200 || strings.Contains(legacy.Body.String(), `"entry_audio_slug"`) {
		t.Fatalf("legacy audio home changed: %d %s", legacy.Code, legacy.Body.String())
	}
}

func TestAudioNovelPlaybackRoutesConfirmMetaAndTikTokEvents(t *testing.T) {
	a := setup(t)
	a.Config.TikTokEnabled = true
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Playback Route Story", "playback-route-story", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	metaCreate := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"Meta Playback","code":"meta-playback","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID), admin)
	if metaCreate.Code != 200 {
		t.Fatalf("create Meta playback link: %d %s", metaCreate.Code, metaCreate.Body.String())
	}
	var metaLink audionovel.DistributionLink
	if err := json.Unmarshal(metaCreate.Body.Bytes(), &metaLink); err != nil {
		t.Fatal(err)
	}
	metaPage := call(a, "GET", "/audio-novel/meta-playback?fbclid=valid-click&ad_id=ad-1&campaign_id=campaign-1", "", nil)
	if metaPage.Code != 200 {
		t.Fatalf("Meta playback entry: %d %s", metaPage.Code, metaPage.Body.String())
	}
	metaBootstrap := decodeAudioBootstrap(t, metaPage)
	if metaBootstrap.PlaybackTicket == "" || metaBootstrap.Ticket == "" || metaBootstrap.MetaPageViewEventID == "" {
		t.Fatalf("Meta playback bootstrap incomplete: %+v", metaBootstrap)
	}
	metaVisitID := strings.Split(metaBootstrap.PlaybackTicket, ".")[0]
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '40 seconds' WHERE id=$1", metaVisitID); err != nil {
		t.Fatal(err)
	}
	var metaVisitLinkID int64
	var metaVisitorID string
	if err := a.DB.QueryRow(context.Background(), "SELECT link_id,COALESCE(visitor_id,'') FROM click_events WHERE id=$1", metaVisitID).Scan(&metaVisitLinkID, &metaVisitorID); err != nil || metaVisitLinkID != metaLink.ID || metaVisitorID == "" {
		t.Fatalf("Meta visit ownership link=%d visitor=%q err=%v", metaVisitLinkID, metaVisitorID, err)
	}
	view := postNovelForm(a, "/audio-novel/meta-playback/view", url.Values{"ticket": {metaBootstrap.Ticket}})
	if view.Code != 200 || !strings.Contains(view.Body.String(), `"event_id":"audio_`+metaVisitID+`_view"`) {
		t.Fatalf("Meta PageView confirmation: %d %s", view.Code, view.Body.String())
	}
	start := call(a, "POST", "/audio-novel/meta-playback/start-listening", fmt.Sprintf(`{"ticket":%q}`, metaBootstrap.PlaybackTicket), nil)
	if start.Code != 200 || !strings.Contains(start.Body.String(), `"event_id":"audio_`+metaVisitID+`_start"`) {
		t.Fatalf("Meta start confirmation: %d %s", start.Code, start.Body.String())
	}
	belowThreshold := call(a, "POST", "/audio-novel/meta-playback/playback-time", fmt.Sprintf("{\"ticket\":%q,\"playback_seconds\":5,\"media_consumed_seconds\":5}", metaBootstrap.PlaybackTicket), nil)
	if belowThreshold.Code != 200 || !strings.Contains(belowThreshold.Body.String(), "\"qualified\":false") || !strings.Contains(belowThreshold.Body.String(), "\"confirmed_events\":[]") {
		t.Fatalf("Meta sub-threshold playback: %d %s", belowThreshold.Code, belowThreshold.Body.String())
	}
	qualified := call(a, "POST", "/audio-novel/meta-playback/playback-time", fmt.Sprintf(`{"ticket":%q,"playback_seconds":12,"media_consumed_seconds":12}`, metaBootstrap.PlaybackTicket), nil)
	if qualified.Code != 200 || !strings.Contains(qualified.Body.String(), `"event_id":"audio_`+metaVisitID+`_qualified"`) {
		t.Fatalf("Meta qualification: %d %s", qualified.Code, qualified.Body.String())
	}
	// Repeating the same state must keep the deterministic browser/server ID
	// without creating another outbox row or increasing playback counters.
	duplicateQualified := call(a, "POST", "/audio-novel/meta-playback/playback-time", fmt.Sprintf("{\"ticket\":%q,\"playback_seconds\":12,\"media_consumed_seconds\":12}", metaBootstrap.PlaybackTicket), nil)
	if duplicateQualified.Code != 200 || !strings.Contains(duplicateQualified.Body.String(), "\"event_id\":\"audio_"+metaVisitID+"_qualified\"") {
		t.Fatalf("Meta duplicate qualification: %d %s", duplicateQualified.Code, duplicateQualified.Body.String())
	}
	beaconRequest := httptest.NewRequest("POST", "/audio-novel/meta-playback/playback-time", strings.NewReader(fmt.Sprintf(`{"ticket":%q,"playback_seconds":20,"media_consumed_seconds":27}`, metaBootstrap.PlaybackTicket)))
	beaconRequest.RemoteAddr = "192.0.2.3:43123"
	beaconRequest.Header.Set("User-Agent", "Mozilla/5.0 Safari/604.1")
	beaconRequest.Header.Set("Content-Type", "text/plain;charset=UTF-8")
	beaconResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(beaconResponse, beaconRequest)
	if beaconResponse.Code != 200 {
		t.Fatalf("Beacon progress: %d %s", beaconResponse.Code, beaconResponse.Body.String())
	}
	completed := call(a, "POST", "/audio-novel/meta-playback/complete", fmt.Sprintf(`{"ticket":%q,"ended":true}`, metaBootstrap.PlaybackTicket), nil)
	if completed.Code != 200 || !strings.Contains(completed.Body.String(), `"completed":true`) || strings.Contains(completed.Body.String(), `"event_id"`) {
		t.Fatalf("internal completion: %d %s", completed.Code, completed.Body.String())
	}
	var metaEvents, expectedMetaIDs int
	metaEventSQL := "SELECT count(*),count(*) FILTER(WHERE id IN ($2,$3,$4)) FROM meta_events WHERE visit_id=$1"
	if err := a.DB.QueryRow(context.Background(), metaEventSQL, metaVisitID, "audio_"+metaVisitID+"_view", "audio_"+metaVisitID+"_start", "audio_"+metaVisitID+"_qualified").Scan(&metaEvents, &expectedMetaIDs); err != nil || metaEvents != 3 || expectedMetaIDs != 3 {
		t.Fatalf("Meta deterministic audio events count=%d expected_ids=%d err=%v", metaEvents, expectedMetaIDs, err)
	}
	var loggedTicket string
	if err := a.DB.QueryRow(context.Background(), `SELECT request_body->>'ticket' FROM request_logs
		WHERE path='/audio-novel/meta-playback/start-listening' ORDER BY occurred_at DESC LIMIT 1`).Scan(&loggedTicket); err != nil || loggedTicket != "[REDACTED]" {
		t.Fatalf("playback ticket leaked to request logs: value=%q err=%v", loggedTicket, err)
	}
	crossOrigin := httptest.NewRequest("POST", "/audio-novel/meta-playback/start-listening", strings.NewReader(fmt.Sprintf(`{"ticket":%q}`, metaBootstrap.PlaybackTicket)))
	crossOrigin.Header.Set("Content-Type", "application/json")
	crossOrigin.Header.Set("Origin", "https://evil.example")
	crossOriginResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(crossOriginResponse, crossOrigin)
	if crossOriginResponse.Code != 403 {
		t.Fatalf("cross-origin playback status=%d", crossOriginResponse.Code)
	}

	tikTokPixelID := testTikTokBinding(t, a, "AUDIOPLAYBACK")
	tikTokCreate := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"TikTok Playback","code":"tiktok-playback","audio_novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, tikTokPixelID), admin)
	if tikTokCreate.Code != 200 {
		t.Fatalf("create TikTok playback link: %d %s", tikTokCreate.Code, tikTokCreate.Body.String())
	}
	var tikTokLink audionovel.DistributionLink
	if err := json.Unmarshal(tikTokCreate.Body.Bytes(), &tikTokLink); err != nil {
		t.Fatal(err)
	}
	tikTokPage := call(a, "GET", "/audio-novel/tiktok-playback?ttclid=tiktok-click&campaign_id=tiktok-campaign&adgroup_id=tiktok-group&ad_id_v2=tiktok-ad", "", nil)
	if tikTokPage.Code != 200 {
		t.Fatalf("TikTok playback entry: %d %s", tikTokPage.Code, tikTokPage.Body.String())
	}
	tikTokBootstrap := decodeAudioBootstrap(t, tikTokPage)
	tikTokVisitID := strings.Split(tikTokBootstrap.PlaybackTicket, ".")[0]
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '20 seconds' WHERE id=$1", tikTokVisitID); err != nil {
		t.Fatal(err)
	}
	var tikTokVisitLinkID int64
	var tikTokVisitorID string
	if err := a.DB.QueryRow(context.Background(), "SELECT link_id,COALESCE(visitor_id,'') FROM click_events WHERE id=$1", tikTokVisitID).Scan(&tikTokVisitLinkID, &tikTokVisitorID); err != nil || tikTokVisitLinkID != tikTokLink.ID || tikTokVisitorID == "" || tikTokVisitorID == metaVisitorID {
		t.Fatalf("TikTok visit ownership link=%d visitor=%q meta_visitor=%q err=%v", tikTokVisitLinkID, tikTokVisitorID, metaVisitorID, err)
	}
	tikTokView := postNovelForm(a, "/audio-novel/tiktok-playback/view", url.Values{"ticket": {tikTokBootstrap.Ticket}})
	if tikTokView.Code != 200 || !strings.Contains(tikTokView.Body.String(), "\"event_id\":\"audio_"+tikTokVisitID+"_view\"") {
		t.Fatalf("TikTok PageView confirmation: %d %s", tikTokView.Code, tikTokView.Body.String())
	}
	if response := call(a, "POST", "/audio-novel/tiktok-playback/start-listening", fmt.Sprintf(`{"ticket":%q}`, tikTokBootstrap.PlaybackTicket), nil); response.Code != 200 || !strings.Contains(response.Body.String(), "\"event_id\":\"audio_"+tikTokVisitID+"_start\"") {
		t.Fatalf("TikTok start: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "POST", "/audio-novel/tiktok-playback/playback-time", fmt.Sprintf(`{"ticket":%q,"playback_seconds":5,"media_consumed_seconds":5}`, tikTokBootstrap.PlaybackTicket), nil); response.Code != 200 || !strings.Contains(response.Body.String(), "\"qualified\":false") {
		t.Fatalf("TikTok sub-threshold playback: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "POST", "/audio-novel/tiktok-playback/playback-time", fmt.Sprintf(`{"ticket":%q,"playback_seconds":12,"media_consumed_seconds":12}`, tikTokBootstrap.PlaybackTicket), nil); response.Code != 200 || !strings.Contains(response.Body.String(), "\"event_id\":\"audio_"+tikTokVisitID+"_qualified\"") {
		t.Fatalf("TikTok qualified: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "POST", "/audio-novel/tiktok-playback/playback-time", fmt.Sprintf(`{"ticket":%q,"playback_seconds":12,"media_consumed_seconds":12}`, tikTokBootstrap.PlaybackTicket), nil); response.Code != 200 || !strings.Contains(response.Body.String(), "\"event_id\":\"audio_"+tikTokVisitID+"_qualified\"") {
		t.Fatalf("TikTok duplicate qualification: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "POST", "/audio-novel/tiktok-playback/playback-time", fmt.Sprintf(`{"ticket":%q,"playback_seconds":20,"media_consumed_seconds":27}`, tikTokBootstrap.PlaybackTicket), nil); response.Code != 200 {
		t.Fatalf("TikTok completion progress: %d %s", response.Code, response.Body.String())
	}
	tikTokComplete := call(a, "POST", "/audio-novel/tiktok-playback/complete", fmt.Sprintf(`{"ticket":%q,"ended":true}`, tikTokBootstrap.PlaybackTicket), nil)
	if tikTokComplete.Code != 200 || !strings.Contains(tikTokComplete.Body.String(), "\"completed\":true") || strings.Contains(tikTokComplete.Body.String(), "\"event_id\"") {
		t.Fatalf("TikTok internal completion: %d %s", tikTokComplete.Code, tikTokComplete.Body.String())
	}
	var tikTokEvents, tikTokPageViews, expectedTikTokIDs, wrongMetaEvents int
	tikTokEventSQL := "SELECT count(*),count(*) FILTER(WHERE event_name='PageView'),count(*) FILTER(WHERE id IN ($2,$3)) FROM tiktok_events WHERE visit_id=$1"
	if err := a.DB.QueryRow(context.Background(), tikTokEventSQL, tikTokVisitID, "audio_"+tikTokVisitID+"_start", "audio_"+tikTokVisitID+"_qualified").Scan(&tikTokEvents, &tikTokPageViews, &expectedTikTokIDs); err != nil || tikTokEvents != 2 || tikTokPageViews != 0 || expectedTikTokIDs != 2 {
		t.Fatalf("TikTok server events=%d pageviews=%d expected_ids=%d err=%v", tikTokEvents, tikTokPageViews, expectedTikTokIDs, err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_events WHERE visit_id=$1", tikTokVisitID).Scan(&wrongMetaEvents); err != nil || wrongMetaEvents != 0 {
		t.Fatalf("TikTok visit created Meta events=%d err=%v", wrongMetaEvents, err)
	}

	// Delivery failure is an outbox concern only. It must not erase the local
	// playback funnel or merge the two buyers' independently scoped visits.
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_events SET status='failed' WHERE visit_id=$1", metaVisitID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_events SET status='failed' WHERE visit_id=$1", tikTokVisitID); err != nil {
		t.Fatal(err)
	}
	assertFailedDeliveryKeepsStats := func(linkID int64) {
		t.Helper()
		response := call(a, "POST", "/api/v1/audio-novel-links/"+itoa(linkID)+"/stats", `{"page":1}`, admin)
		var stats audionovel.DistributionStatsResponse
		if response.Code != 200 {
			t.Fatalf("campaign stats: %d %s", response.Code, response.Body.String())
		}
		if err := json.Unmarshal(response.Body.Bytes(), &stats); err != nil {
			t.Fatal(err)
		}
		if stats.Summary.Visits != 1 || stats.Summary.StartedCount != 1 || stats.Summary.QualifiedCount != 1 || stats.Summary.CompletedCount != 1 || stats.Summary.SavedEvents < 2 || stats.Summary.SavedEvents != stats.Summary.FailedEvents || stats.Summary.TotalPlaybackSeconds != 20 {
			t.Fatalf("delivery failure changed local funnel: %s", response.Body.String())
		}
	}
	assertFailedDeliveryKeepsStats(metaLink.ID)
	assertFailedDeliveryKeepsStats(tikTokLink.ID)
}

func TestAudioNovelDistributionStatsIsolatePlaybackFunnelsAndDelivery(t *testing.T) {
	a := setup(t)
	a.Config.TikTokEnabled = true
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Audio Stats Story", "audio-stats-story", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	tikTokPixelID := testTikTokBinding(t, a, "AUDIOSTATS")

	createLink := func(name, code, platform string) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"audio_novel_id":%d,"enabled":true,"ad_platform":%q,"time_spent_threshold":10`, name, code, audioNovelID, platform)
		if platform == "tiktok" {
			body += fmt.Sprintf(`,"tiktok_pixel_id":%d}`, tikTokPixelID)
		} else {
			body += fmt.Sprintf(`,"meta_connection_id":%d,"meta_pixel_id":%d}`, metaConnectionID, metaPixelID)
		}
		response := call(a, "POST", "/api/v1/audio-novel-links", body, admin)
		if response.Code != 200 {
			t.Fatalf("create %s stats link: %d %s", platform, response.Code, response.Body.String())
		}
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		return item.ID
	}
	firstID := createLink("TikTok Buyer A", "audio-stats-a", "tiktok")
	secondID := createLink("TikTok Buyer B", "audio-stats-b", "tiktok")
	metaID := createLink("Meta Buyer", "audio-stats-meta", "meta")

	firstPage := call(a, "GET", "/audio-novel/audio-stats-a?ttclid=1234567890abcdef&campaign_id=campaign-a&adgroup_id=group-a&creative_id=creative-a&ad_id_v2=ad-a&placement=TikTok", "", nil)
	if firstPage.Code != 200 || len(firstPage.Result().Cookies()) == 0 {
		t.Fatalf("first audio stats visit: %d %s", firstPage.Code, firstPage.Body.String())
	}
	firstBootstrap := decodeAudioBootstrap(t, firstPage)
	visitor := firstPage.Result().Cookies()[0]
	if response := call(a, "GET", "/audio-novel/audio-stats-a?ttclid=abcdefghij123456&campaign_id=campaign-b&adgroup_id=group-b&creative_id=creative-b&ad_id_v2=ad-b&placement=Pangle", "", visitor); response.Code != 200 {
		t.Fatalf("second audio stats visit: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "GET", "/audio-novel/audio-stats-a?ttclid=third-click&campaign_id=campaign-c", "", nil); response.Code != 200 {
		t.Fatalf("third audio stats visit: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "GET", "/audio-novel/audio-stats-b?ttclid=other-link", "", visitor); response.Code != 200 {
		t.Fatalf("other buyer visit: %d %s", response.Code, response.Body.String())
	}
	if response := call(a, "HEAD", "/audio-novel/audio-stats-a", "", visitor); response.Code != 200 {
		t.Fatalf("stats HEAD probe: %d", response.Code)
	}
	botRequest := httptest.NewRequest("GET", "/audio-novel/audio-stats-a", nil)
	botRequest.RemoteAddr = "192.0.2.90:43123"
	botRequest.Header.Set("User-Agent", "facebookexternalhit/1.1")
	botResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(botResponse, botRequest)
	suspiciousIP := "198.51.100.90"
	for range 120 {
		a.Exceed("click:"+a.Sign(suspiciousIP), 120, time.Minute)
	}
	suspiciousRequest := httptest.NewRequest("GET", "/audio-novel/audio-stats-a", nil)
	suspiciousRequest.RemoteAddr = suspiciousIP + ":43123"
	suspiciousRequest.Header.Set("User-Agent", "Mozilla/5.0 Safari/604.1")
	suspiciousResponse := httptest.NewRecorder()
	a.Router.ServeHTTP(suspiciousResponse, suspiciousRequest)

	rows, err := a.DB.Query(context.Background(), `SELECT id FROM click_events WHERE link_id=$1 AND method='GET' AND classification='normal' ORDER BY occurred_at,id`, firstID)
	if err != nil {
		t.Fatal(err)
	}
	var visits []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		visits = append(visits, id)
	}
	rows.Close()
	if len(visits) != 3 {
		t.Fatalf("normal visits=%d, want 3", len(visits))
	}
	updates := []struct {
		id, at                        string
		visible, playback             int
		started, qualified, completed bool
	}{
		{visits[0], "2026-09-01 15:30:00+00", 0, 30, true, true, true},
		{visits[1], "2026-09-01 16:30:00+00", 10, 5, true, false, false},
		{visits[2], "2026-09-02 01:00:00+00", 0, 0, false, false, false},
	}
	for _, update := range updates {
		_, err = a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at=$2::timestamptz,
			visible_seconds=$3,visible_updated_at=CASE WHEN $3>0 THEN $2::timestamptz ELSE NULL END,
			playback_seconds=$4::integer,media_consumed_seconds=$4::numeric,playback_updated_at=CASE WHEN $4::integer>0 THEN $2::timestamptz ELSE NULL END,
			audio_started_at=CASE WHEN $5 THEN $2::timestamptz ELSE NULL END,
			audio_qualified_at=CASE WHEN $6 THEN $2::timestamptz ELSE NULL END,
			audio_completed_at=CASE WHEN $7 THEN $2::timestamptz ELSE NULL END WHERE id=$1`,
			update.id, update.at, update.visible, update.playback, update.started, update.qualified, update.completed)
		if err != nil {
			t.Fatal(err)
		}
	}
	// Exercise the production collection route for one visit instead of making
	// every visible-duration assertion depend on direct fixture updates.
	visible := call(a, "POST", "/audio-novel/audio-stats-a/visible-time", fmt.Sprintf(`{"ticket":%q,"visible_seconds":20}`, firstBootstrap.PlaybackTicket), nil)
	if visible.Code != 200 || !strings.Contains(visible.Body.String(), `"visible_seconds":20`) {
		t.Fatalf("visible-time collection: %d %s", visible.Code, visible.Body.String())
	}
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at='2026-09-02 02:00:00+00'::timestamptz
		WHERE link_id=$1 AND classification<>'normal'`, firstID); err != nil {
		t.Fatal(err)
	}
	// Keep the HEAD probe inside the selected period so the method predicate is
	// proven independently from the date filter.
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at='2026-09-02 02:15:00+00'::timestamptz
		WHERE link_id=$1 AND method='HEAD'`, firstID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at='2026-09-02 03:00:00+00'::timestamptz WHERE link_id=$1`, secondID); err != nil {
		t.Fatal(err)
	}

	var tikTokConnectionID int64
	if err = a.DB.QueryRow(context.Background(), "SELECT connection_id FROM tiktok_pixels WHERE id=$1", tikTokPixelID).Scan(&tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	insertTikTokEvent := func(id, visitID, name, status string) {
		t.Helper()
		_, insertErr := a.DB.Exec(context.Background(), `INSERT INTO tiktok_events
			(id,visit_id,link_id,audio_novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,payload_cipher,status)
			VALUES($1,$2,$3,$4,$5,$6,'PX_AUDIOSTATS',$7,$1,now(),'encrypted',$8)`, id, visitID, firstID, audioNovelID, tikTokConnectionID, tikTokPixelID, name, status)
		if insertErr != nil {
			t.Fatal(insertErr)
		}
	}
	insertTikTokEvent("audio-stats-start-pending", visits[0], "StartListening", "pending")
	insertTikTokEvent("audio-stats-qualified-accepted", visits[0], "ViewContent", "accepted")
	insertTikTokEvent("audio-stats-start-failed", visits[1], "StartListening", "failed")
	if _, err = a.DB.Exec(context.Background(), `INSERT INTO tiktok_events
		(id,visit_id,link_id,audio_novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,payload_cipher,status,is_test)
		VALUES('audio-stats-test-event',$1,$2,$3,$4,$5,'PX_AUDIOSTATS','ViewContent','audio-stats-test-event',now(),'encrypted','accepted',true)`,
		visits[0], firstID, audioNovelID, tikTokConnectionID, tikTokPixelID); err != nil {
		t.Fatal(err)
	}

	type statsResponse struct {
		Link struct {
			ID         int64  `json:"id"`
			AdPlatform string `json:"ad_platform"`
		} `json:"link"`
		Summary struct {
			Visits                 int64   `json:"visits"`
			UniqueVisitors         int64   `json:"unique_visitors"`
			CollectedVisits        int64   `json:"collected_visits"`
			AverageVisibleSeconds  float64 `json:"average_visible_seconds"`
			TotalVisibleSeconds    int64   `json:"total_visible_seconds"`
			StartedCount           int64   `json:"started_count"`
			StartedVisitors        int64   `json:"started_visitors"`
			QualifiedCount         int64   `json:"qualified_count"`
			QualifiedVisitors      int64   `json:"qualified_visitors"`
			CompletedCount         int64   `json:"completed_count"`
			CompletedVisitors      int64   `json:"completed_visitors"`
			AveragePlaybackSeconds float64 `json:"average_playback_seconds"`
			TotalPlaybackSeconds   int64   `json:"total_playback_seconds"`
			StartRate              float64 `json:"start_rate"`
			QualifiedRate          float64 `json:"qualified_rate"`
			CompletionRate         float64 `json:"completion_rate"`
			SavedEvents            int64   `json:"saved_events"`
			PendingEvents          int64   `json:"pending_events"`
			AcceptedEvents         int64   `json:"accepted_events"`
			FailedEvents           int64   `json:"failed_events"`
		} `json:"summary"`
		Items []struct {
			ID                   string `json:"id"`
			TTCLID               string `json:"ttclid"`
			VisibleSeconds       *int   `json:"visible_seconds"`
			PlaybackSeconds      *int   `json:"playback_seconds"`
			Started              bool   `json:"started"`
			Qualified            bool   `json:"qualified"`
			Completed            bool   `json:"completed"`
			StartEventStatus     string `json:"start_event_status"`
			QualifiedEventStatus string `json:"qualified_event_status"`
		} `json:"items"`
		Total    int64 `json:"total"`
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
	}
	stats := func(id int64, body string) (statsResponse, *httptest.ResponseRecorder) {
		t.Helper()
		response := call(a, "POST", "/api/v1/audio-novel-links/"+itoa(id)+"/stats", body, admin)
		var out statsResponse
		if response.Code == 200 {
			if decodeErr := json.Unmarshal(response.Body.Bytes(), &out); decodeErr != nil {
				t.Fatal(decodeErr)
			}
		}
		return out, response
	}
	full, response := stats(firstID, `{"start":"2026-09-01","end":"2026-09-02","tz":"Asia/Shanghai","page":1,"page_size":2}`)
	if response.Code != 200 {
		t.Fatalf("audio stats: %d %s", response.Code, response.Body.String())
	}
	if full.Link.ID != firstID || full.Link.AdPlatform != "tiktok" || full.Summary.Visits != 3 || full.Summary.UniqueVisitors != 2 ||
		full.Summary.CollectedVisits != 2 || full.Summary.AverageVisibleSeconds != 15 || full.Summary.TotalVisibleSeconds != 30 ||
		full.Summary.StartedCount != 2 || full.Summary.StartedVisitors != 1 || full.Summary.QualifiedCount != 1 || full.Summary.QualifiedVisitors != 1 ||
		full.Summary.CompletedCount != 1 || full.Summary.CompletedVisitors != 1 || full.Summary.AveragePlaybackSeconds != 17.5 || full.Summary.TotalPlaybackSeconds != 35 ||
		full.Summary.StartRate < 66.6 || full.Summary.StartRate > 66.7 || full.Summary.QualifiedRate != 50 || full.Summary.CompletionRate != 50 ||
		full.Summary.SavedEvents != 3 || full.Summary.PendingEvents != 1 || full.Summary.AcceptedEvents != 1 || full.Summary.FailedEvents != 1 ||
		full.Total != 3 || full.Page != 1 || full.PageSize != 2 || len(full.Items) != 2 || full.Items[0].ID != visits[2] ||
		full.Items[0].VisibleSeconds != nil || full.Items[0].PlaybackSeconds != nil || full.Items[1].ID != visits[1] {
		t.Fatalf("unexpected audio stats: %s", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "1234567890abcdef") || strings.Contains(response.Body.String(), "tiktok_context_cipher") || strings.Contains(response.Body.String(), "tiktok_ttp") {
		t.Fatalf("audio stats exposed private TikTok data: %s", response.Body.String())
	}
	secondPage, response := stats(firstID, `{"start":"2026-09-01","end":"2026-09-02","tz":"Asia/Shanghai","page":2,"page_size":2}`)
	if response.Code != 200 || len(secondPage.Items) != 1 || secondPage.Items[0].ID != visits[0] || secondPage.Items[0].TTCLID != "1234…cdef" ||
		secondPage.Items[0].PlaybackSeconds == nil || *secondPage.Items[0].PlaybackSeconds != 30 || secondPage.Items[0].StartEventStatus != "pending" || secondPage.Items[0].QualifiedEventStatus != "accepted" {
		t.Fatalf("audio stats second page: %d %s", response.Code, response.Body.String())
	}

	shanghaiDay, response := stats(firstID, `{"start":"2026-09-02","end":"2026-09-02","tz":"Asia/Shanghai","page":1}`)
	if response.Code != 200 || shanghaiDay.Summary.Visits != 2 {
		t.Fatalf("Shanghai date boundary: %d %s", response.Code, response.Body.String())
	}
	utcDay, response := stats(firstID, `{"start":"2026-09-01","end":"2026-09-01","tz":"UTC","page":1}`)
	if response.Code != 200 || utcDay.Summary.Visits != 2 {
		t.Fatalf("UTC date boundary: %d %s", response.Code, response.Body.String())
	}
	filtered, response := stats(firstID, `{"start":"2026-09-01","end":"2026-09-02","tz":"UTC","campaign_id":"campaign-a","adgroup_id":"group-a","creative_id":"creative-a","ad_id_v2":"ad-a","event_status":"accepted","page":1}`)
	if response.Code != 200 || filtered.Summary.Visits != 1 {
		t.Fatalf("TikTok audio filters: %d %s", response.Code, response.Body.String())
	}
	other, response := stats(secondID, `{"start":"2026-09-01","end":"2026-09-02","tz":"UTC","page":1}`)
	if response.Code != 200 || other.Summary.Visits != 1 || other.Summary.StartRate != 0 || other.Summary.QualifiedRate != 0 || other.Summary.CompletionRate != 0 {
		t.Fatalf("audio link isolation: %d %s", response.Code, response.Body.String())
	}
	_, response = stats(firstID, `{"start":"2026-09-01","end":"2026-09-02","tz":"Europe/London","page":1}`)
	if response.Code != 400 {
		t.Fatalf("unapproved timezone status=%d body=%s", response.Code, response.Body.String())
	}
	_, response = stats(firstID, `{"start":"2026-09-01","end":"2026-09-02","tz":"UTC","page":1,"page_size":1000}`)
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"page_size":100`) {
		t.Fatalf("page-size cap: %d %s", response.Code, response.Body.String())
	}

	metaPage := call(a, "GET", "/audio-novel/audio-stats-meta?fbclid=meta-click&campaign_id=meta-campaign&ad_id=meta-ad", "", nil)
	metaBootstrap := decodeAudioBootstrap(t, metaPage)
	metaVisitID := strings.Split(metaBootstrap.PlaybackTicket, ".")[0]
	if _, err = a.DB.Exec(context.Background(), `UPDATE click_events SET occurred_at='2026-09-02 02:30:00+00'::timestamptz WHERE id=$1`, metaVisitID); err != nil {
		t.Fatal(err)
	}
	var metaPixelCode string
	if err = a.DB.QueryRow(context.Background(), "SELECT pixel_id FROM meta_pixels WHERE id=$1", metaPixelID).Scan(&metaPixelCode); err != nil {
		t.Fatal(err)
	}
	for index, event := range []struct{ name, status string }{{"PageView", "succeeded"}, {"StartListening", "pending"}, {"ViewContent", "failed"}, {"TimeSpent", "skipped"}} {
		_, err = a.DB.Exec(context.Background(), `INSERT INTO meta_events(id,connection_id,pixel_record_id,visit_id,event_name,event_time,pixel_id,status)
			VALUES($1,$2,$3,$4,$5,now(),$6,$7)`, fmt.Sprintf("audio-meta-stats-%d", index), metaConnectionID, metaPixelID, metaVisitID, event.name, metaPixelCode, event.status)
		if err != nil {
			t.Fatal(err)
		}
	}
	metaStats, response := stats(metaID, `{"start":"2026-09-02","end":"2026-09-02","tz":"UTC","ad_id":"meta-ad","event_status":"succeeded","page":1}`)
	// Legacy TimeSpent is not an audio playback event and must not inflate the
	// new PageView/StartListening/ViewContent delivery totals.
	if response.Code != 200 || metaStats.Summary.Visits != 1 || metaStats.Summary.SavedEvents != 3 || metaStats.Summary.PendingEvents != 1 || metaStats.Summary.AcceptedEvents != 1 || metaStats.Summary.FailedEvents != 1 {
		t.Fatalf("Meta audio delivery stats: %d %s", response.Code, response.Body.String())
	}
	wrongMeta, response := stats(metaID, `{"start":"2026-09-02","end":"2026-09-02","tz":"UTC","ad_id":"wrong","page":1}`)
	if response.Code != 200 || wrongMeta.Summary.Visits != 0 {
		t.Fatalf("Meta ad filter: %d %s", response.Code, response.Body.String())
	}
	_, response = stats(999999, `{"start":"2026-09-02","end":"2026-09-02","tz":"UTC","page":1}`)
	if response.Code != 404 {
		t.Fatalf("unknown audio link status=%d body=%s", response.Code, response.Body.String())
	}
	var ordinaryLinkID int64
	if err = a.DB.QueryRow(context.Background(), "SELECT id FROM short_links WHERE product_type<>'audio_novel' ORDER BY id LIMIT 1").Scan(&ordinaryLinkID); err != nil {
		t.Fatal(err)
	}
	_, response = stats(ordinaryLinkID, `{"start":"2026-09-02","end":"2026-09-02","tz":"UTC","page":1}`)
	if response.Code != 404 {
		t.Fatalf("non-audio link status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestAudioNovelDistributionStatsReportsDeliveryConfigurationBlockers(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Audio Delivery Config", "audio-delivery-config", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	tikTokPixelID := testTikTokBinding(t, a, "AUDIOCONFIG")

	createLink := func(name, code, platform string) int64 {
		t.Helper()
		body := fmt.Sprintf(`{"name":%q,"code":%q,"audio_novel_id":%d,"enabled":true,"ad_platform":%q,"time_spent_threshold":10`, name, code, audioNovelID, platform)
		if platform == "tiktok" {
			body += fmt.Sprintf(`,"tiktok_pixel_id":%d}`, tikTokPixelID)
		} else {
			body += fmt.Sprintf(`,"meta_connection_id":%d,"meta_pixel_id":%d}`, metaConnectionID, metaPixelID)
		}
		response := call(a, "POST", "/api/v1/audio-novel-links", body, admin)
		if response.Code != 200 {
			t.Fatalf("create %s delivery-config link: %d %s", platform, response.Code, response.Body.String())
		}
		var item struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &item); err != nil {
			t.Fatal(err)
		}
		return item.ID
	}
	metaLinkID := createLink("Meta Config", "audio-config-meta", "meta")
	tikTokLinkID := createLink("TikTok Config", "audio-config-tiktok", "tiktok")

	type deliveryStatus struct {
		Status string `json:"delivery_config_status"`
		Reason string `json:"delivery_blocked_reason"`
	}
	stats := func(linkID int64) (deliveryStatus, string) {
		t.Helper()
		response := call(a, "POST", "/api/v1/audio-novel-links/"+itoa(linkID)+"/stats", `{"page":1}`, admin)
		if response.Code != 200 {
			t.Fatalf("delivery-config stats: %d %s", response.Code, response.Body.String())
		}
		var result deliveryStatus
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result, response.Body.String()
	}
	assertStatus := func(linkID int64, wantStatus, reasonPart string) {
		t.Helper()
		result, body := stats(linkID)
		if result.Status != wantStatus || !strings.Contains(result.Reason, reasonPart) {
			t.Fatalf("delivery config status=%q reason=%q, want %q containing %q; body=%s", result.Status, result.Reason, wantStatus, reasonPart, body)
		}
		// Configuration diagnostics must never return encrypted credentials.
		if strings.Contains(body, "fake-capi-token") || strings.Contains(body, "test-cipher") {
			t.Fatalf("delivery config exposed a credential: %s", body)
		}
	}

	assertStatus(metaLinkID, "ready", "")
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_pixels SET enabled=false WHERE id=$1", metaPixelID); err != nil {
		t.Fatal(err)
	}
	assertStatus(metaLinkID, "blocked", "Meta Pixel 已停用")
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_pixels SET enabled=true,capi_token_cipher='',credential_status='unverified' WHERE id=$1", metaPixelID); err != nil {
		t.Fatal(err)
	}
	assertStatus(metaLinkID, "blocked", "CAPI 凭证未配置")
	if _, err := a.DB.Exec(context.Background(), "UPDATE meta_pixels SET capi_token_cipher='encrypted',credential_status='invalid' WHERE id=$1", metaPixelID); err != nil {
		t.Fatal(err)
	}
	assertStatus(metaLinkID, "blocked", "CAPI 凭证无效")

	// The process-wide TikTok switch is checked before database credentials,
	// because no server worker runs while the switch is off.
	assertStatus(tikTokLinkID, "blocked", "TikTok 服务器回传总开关未启用")
	a.Config.TikTokEnabled = true
	assertStatus(tikTokLinkID, "ready", "")
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_pixels SET enabled=false WHERE id=$1", tikTokPixelID); err != nil {
		t.Fatal(err)
	}
	assertStatus(tikTokLinkID, "blocked", "TikTok Pixel 已停用")
	var tikTokConnectionID int64
	if err := a.DB.QueryRow(context.Background(), "SELECT connection_id FROM tiktok_pixels WHERE id=$1", tikTokPixelID).Scan(&tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_pixels SET enabled=true WHERE id=$1", tikTokPixelID); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_connections SET enabled=false WHERE id=$1", tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	assertStatus(tikTokLinkID, "blocked", "TikTok 凭证已停用")
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_connections SET enabled=true,access_token_cipher='',credential_status='unverified' WHERE id=$1", tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	assertStatus(tikTokLinkID, "blocked", "Access Token 未配置")
	if _, err := a.DB.Exec(context.Background(), "UPDATE tiktok_connections SET access_token_cipher='encrypted',credential_status='error' WHERE id=$1", tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	assertStatus(tikTokLinkID, "blocked", "Access Token 状态异常")
}

func TestAudioNovelDistributionStatsDistinguishesUncollectedAndCollectedZeroVisibleTime(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Visible Summary", "visible-summary", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	created := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"Visible Summary","code":"visible-summary-link","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID), admin)
	if created.Code != 200 {
		t.Fatalf("create visible-summary link: %d %s", created.Code, created.Body.String())
	}
	var link struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &link); err != nil {
		t.Fatal(err)
	}
	if response := call(a, "GET", "/audio-novel/visible-summary-link", "", nil); response.Code != 200 {
		t.Fatalf("create uncollected visit: %d %s", response.Code, response.Body.String())
	}

	type visibleSummary struct {
		Summary struct {
			CollectedVisits       int64    `json:"collected_visits"`
			AverageVisibleSeconds *float64 `json:"average_visible_seconds"`
			TotalVisibleSeconds   *int64   `json:"total_visible_seconds"`
		} `json:"summary"`
	}
	stats := func() visibleSummary {
		t.Helper()
		response := call(a, "POST", "/api/v1/audio-novel-links/"+itoa(link.ID)+"/stats", `{"page":1}`, admin)
		if response.Code != 200 {
			t.Fatalf("visible-summary stats: %d %s", response.Code, response.Body.String())
		}
		var result visibleSummary
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}

	uncollected := stats()
	if uncollected.Summary.CollectedVisits != 0 || uncollected.Summary.AverageVisibleSeconds != nil || uncollected.Summary.TotalVisibleSeconds != nil {
		t.Fatalf("uncollected duration must stay null: %+v", uncollected.Summary)
	}

	// The collection marker, not the numeric value, distinguishes a genuine
	// zero sample from a historical visit that never reported visible time.
	if _, err := a.DB.Exec(context.Background(), `UPDATE click_events SET visible_seconds=0,visible_updated_at=now()
		WHERE link_id=$1 AND surface='audio_novel' AND method='GET' AND classification='normal'`, link.ID); err != nil {
		t.Fatal(err)
	}
	collectedZero := stats()
	if collectedZero.Summary.CollectedVisits != 1 || collectedZero.Summary.AverageVisibleSeconds == nil || *collectedZero.Summary.AverageVisibleSeconds != 0 ||
		collectedZero.Summary.TotalVisibleSeconds == nil || *collectedZero.Summary.TotalVisibleSeconds != 0 {
		t.Fatalf("collected zero duration lost its sample marker: %+v", collectedZero.Summary)
	}
}
