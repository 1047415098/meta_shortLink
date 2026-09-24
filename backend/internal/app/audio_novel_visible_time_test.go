package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestAudioNovelVisibleTimeRouteStoresCampaignDurationAndRedactsTicket(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Visible Route Story", "visible-route-story", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	created := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"Visible Route","code":"visible-route","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID), admin)
	if created.Code != 200 {
		t.Fatalf("create link: %d %s", created.Code, created.Body.String())
	}
	page := call(a, "GET", "/audio-novel/visible-route", "", nil)
	bootstrap := decodeAudioBootstrap(t, page)
	visitID := strings.Split(bootstrap.PlaybackTicket, ".")[0]
	if _, err := a.DB.Exec(context.Background(), "UPDATE click_events SET occurred_at=now()-interval '20 seconds' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}

	reported := call(a, "POST", "/audio-novel/visible-route/visible-time", fmt.Sprintf(`{"ticket":%q,"visible_seconds":1000}`, bootstrap.PlaybackTicket), nil)
	var result struct {
		VisibleSeconds int `json:"visible_seconds"`
	}
	if err := json.Unmarshal(reported.Body.Bytes(), &result); reported.Code != 200 || err != nil || result.VisibleSeconds < 25 || result.VisibleSeconds > 27 {
		t.Fatalf("visible response: status=%d body=%s err=%v", reported.Code, reported.Body.String(), err)
	}
	var stored, adEvents int
	if err := a.DB.QueryRow(context.Background(), `SELECT visible_seconds,
		(SELECT count(*) FROM meta_events WHERE visit_id=e.id) FROM click_events e WHERE id=$1`, visitID).Scan(&stored, &adEvents); err != nil || stored != result.VisibleSeconds || adEvents != 0 {
		t.Fatalf("stored=%d events=%d err=%v", stored, adEvents, err)
	}
	var loggedTicket string
	if err := a.DB.QueryRow(context.Background(), `SELECT request_body->>'ticket' FROM request_logs
		WHERE path='/audio-novel/visible-route/visible-time' ORDER BY occurred_at DESC LIMIT 1`).Scan(&loggedTicket); err != nil || loggedTicket != "[REDACTED]" {
		t.Fatalf("visible ticket leaked to request logs: value=%q err=%v", loggedTicket, err)
	}
}
