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

func postNovelAction(a *App, path string, values url.Values, origin string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(values.Encode()))
	request.RemoteAddr = "192.0.2.8:43123"
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("User-Agent", "Mozilla/5.0 (iPhone) AppleWebKit/605.1.15 Safari/604.1")
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	response := httptest.NewRecorder()
	a.Router.ServeHTTP(response, request)
	return response
}

func TestTikTokStartReadingAndQualifiedViewContentFlow(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	novelID := createDistributionNovel(t, a, admin, "TikTok Qualified Story", "tiktok-qualified-story")
	pixelID := testTikTokBinding(t, a, "FLOW01")
	createBody := fmt.Sprintf(`{"name":"TikTok Flow","code":"tik-flow","novel_id":%d,"enabled":true,"ad_platform":"tiktok","tiktok_pixel_id":%d,"time_spent_threshold":10}`, novelID, pixelID)
	if response := call(a, http.MethodPost, "/api/v1/novel-links", createBody, admin); response.Code != http.StatusOK {
		t.Fatalf("create link: %d %s", response.Code, response.Body.String())
	}
	page := call(a, http.MethodGet, "/novel/tik-flow?ttclid=click-flow", "", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("visit: %d %s", page.Code, page.Body.String())
	}
	var visitID string
	var linkID int64
	if err := a.DB.QueryRow(context.Background(), `SELECT e.id,e.link_id FROM click_events e JOIN short_links l ON l.id=e.link_id
		WHERE l.code='tik-flow' AND e.classification='normal' ORDER BY e.occurred_at DESC LIMIT 1`).Scan(&visitID, &linkID); err != nil {
		t.Fatal(err)
	}
	ticket := visitID + "." + a.Sign("contact:novel:tik-flow:"+visitID)

	if response := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {ticket}, "chapter": {"1"}}, "https://evil.example"); response.Code != http.StatusForbidden {
		t.Fatalf("wrong origin status=%d", response.Code)
	}
	if response := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {"bad.ticket"}, "chapter": {"1"}}, "http://example.com"); response.Code != http.StatusBadRequest {
		t.Fatalf("bad ticket status=%d", response.Code)
	}
	if response := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {ticket}, "chapter": {"2"}}, "http://example.com"); response.Code != http.StatusBadRequest {
		t.Fatalf("wrong chapter status=%d", response.Code)
	}
	if response := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {ticket}, "chapter": {"1"}, "_ttp": {"bad\nvalue"}}, "http://example.com"); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid _ttp status=%d", response.Code)
	}

	start := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {ticket}, "chapter": {"1"}, "_ttp": {"ttp-first"}}, "http://example.com")
	if start.Code != http.StatusOK || !strings.Contains(start.Body.String(), `"name":"StartReading"`) {
		t.Fatalf("start response=%d %s", start.Code, start.Body.String())
	}
	repeated := postNovelAction(a, "/novel/tik-flow/start-reading", url.Values{"ticket": {ticket}, "chapter": {"1"}, "_ttp": {"ttp-second"}}, "http://example.com")
	if repeated.Code != http.StatusOK || repeated.Body.String() != start.Body.String() {
		t.Fatalf("repeated start differs: first=%s second=%s", start.Body.String(), repeated.Body.String())
	}
	var startCount int
	var frozenTTP string
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM tiktok_events WHERE visit_id=$1 AND event_name='StartReading'", visitID).Scan(&startCount); err != nil || startCount != 1 {
		t.Fatalf("start events=%d err=%v", startCount, err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT tiktok_ttp FROM click_events WHERE id=$1", visitID).Scan(&frozenTTP); err != nil || frozenTTP != "ttp-first" {
		t.Fatalf("frozen _ttp=%q err=%v", frozenTTP, err)
	}

	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '30 seconds' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	early := postNovelAction(a, "/novel/tik-flow/reading-time", url.Values{"ticket": {ticket}, "seconds": {"9"}}, "http://example.com")
	if early.Code != http.StatusOK || strings.Contains(early.Body.String(), "ViewContent") {
		t.Fatalf("early qualification=%d %s", early.Code, early.Body.String())
	}
	qualified := postNovelAction(a, "/novel/tik-flow/reading-time", url.Values{"ticket": {ticket}, "seconds": {"10"}}, "http://example.com")
	if qualified.Code != http.StatusOK {
		t.Fatalf("qualified status=%d %s", qualified.Code, qualified.Body.String())
	}
	var result struct {
		VisibleSeconds int `json:"visible_seconds"`
		TikTokEvent    struct {
			Name    string `json:"name"`
			EventID string `json:"event_id"`
		} `json:"tiktok_event"`
	}
	if err := json.Unmarshal(qualified.Body.Bytes(), &result); err != nil || result.VisibleSeconds != 10 || result.TikTokEvent.Name != "ViewContent" || result.TikTokEvent.EventID != "novel_"+visitID+"_qualified" {
		t.Fatalf("qualified response=%s err=%v", qualified.Body.String(), err)
	}
	downgraded := postNovelAction(a, "/novel/tik-flow/reading-time", url.Values{"ticket": {ticket}, "seconds": {"5"}}, "http://example.com")
	if downgraded.Code != http.StatusOK || !strings.Contains(downgraded.Body.String(), `"visible_seconds":10`) || !strings.Contains(downgraded.Body.String(), result.TikTokEvent.EventID) {
		t.Fatalf("downgraded response=%d %s", downgraded.Code, downgraded.Body.String())
	}
	var qualifiedCount int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM tiktok_events WHERE visit_id=$1 AND event_name='ViewContent'", visitID).Scan(&qualifiedCount); err != nil || qualifiedCount != 1 {
		t.Fatalf("qualified events=%d err=%v", qualifiedCount, err)
	}

	// The server-observed duration and hard two-hour cap remain authoritative.
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '2 hours 1 minute' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	capped := postNovelAction(a, "/novel/tik-flow/reading-time", url.Values{"ticket": {ticket}, "seconds": {"9000"}}, "http://example.com")
	if capped.Code != http.StatusOK || !strings.Contains(capped.Body.String(), `"visible_seconds":7200`) {
		t.Fatalf("two-hour cap=%d %s", capped.Code, capped.Body.String())
	}

	// A Meta visit continues collecting duration without creating TikTok events.
	connectionID, metaPixelID := testLinkMetaBinding(t, a)
	metaBody := fmt.Sprintf(`{"name":"Meta Flow","code":"meta-flow","novel_id":%d,"enabled":true,"meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, novelID, connectionID, metaPixelID)
	if response := call(a, http.MethodPost, "/api/v1/novel-links", metaBody, admin); response.Code != http.StatusOK {
		t.Fatalf("create Meta link: %d %s", response.Code, response.Body.String())
	}
	metaPage := call(a, http.MethodGet, "/novel/meta-flow", "", nil)
	var metaVisitID string
	if err := a.DB.QueryRow(context.Background(), `SELECT e.id FROM click_events e JOIN short_links l ON l.id=e.link_id WHERE l.code='meta-flow' ORDER BY e.occurred_at DESC LIMIT 1`).Scan(&metaVisitID); err != nil {
		t.Fatal(err)
	}
	_ = metaPage
	metaTicket := metaVisitID + "." + a.Sign("contact:novel:meta-flow:"+metaVisitID)
	if response := postNovelAction(a, "/novel/meta-flow/start-reading", url.Values{"ticket": {metaTicket}, "chapter": {"1"}}, "http://example.com"); response.Code != http.StatusOK || response.Body.String() != `{"ok":true}` {
		t.Fatalf("Meta start response=%d %s", response.Code, response.Body.String())
	}
	var metaTikTokEvents int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM tiktok_events WHERE visit_id=$1", metaVisitID).Scan(&metaTikTokEvents); err != nil || metaTikTokEvents != 0 {
		t.Fatalf("Meta visit TikTok events=%d err=%v", metaTikTokEvents, err)
	}

	// Keep a real timestamp assertion so retries cannot manufacture a new event time.
	var eventAt time.Time
	if err := a.DB.QueryRow(context.Background(), "SELECT event_time FROM tiktok_events WHERE visit_id=$1 AND event_name='StartReading'", visitID).Scan(&eventAt); err != nil || eventAt.IsZero() {
		t.Fatalf("start event time=%v err=%v", eventAt, err)
	}
	_ = linkID
}
