package app

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestAudioNovelCampaignKeepsLegacyMP3WithoutParsedDurationPlayable(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "Legacy Duration Audio", "legacy-duration-audio", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)

	// NOT VALID preserves pre-migration MP3 rows whose duration metadata could
	// not be parsed. Dropping the check here recreates that legal legacy state.
	if _, err := a.DB.Exec(context.Background(), "ALTER TABLE audio_novels DROP CONSTRAINT audio_novels_audio_fields_check"); err != nil {
		t.Fatal(err)
	}
	if _, err := a.DB.Exec(context.Background(), "UPDATE audio_novels SET audio_duration='unknown',audio_duration_seconds=0,audio_size_bytes=0 WHERE id=$1", audioNovelID); err != nil {
		t.Fatal(err)
	}
	create := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"Legacy duration buyer","code":"legacy-duration-buyer","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID), admin)
	if create.Code != 200 {
		t.Fatalf("create legacy-duration link: %d %s", create.Code, create.Body.String())
	}

	page := call(a, "GET", "/audio-novel/legacy-duration-buyer?fbclid=legacy-click&ad_id=legacy-ad", "", nil)
	if page.Code != 200 {
		t.Fatalf("legacy MP3 campaign entry: %d %s", page.Code, page.Body.String())
	}
	bootstrap := decodeAudioBootstrap(t, page)
	if bootstrap.PlaybackTicket == "" || bootstrap.EntryAudioSlug != "legacy-duration-audio" {
		t.Fatalf("legacy MP3 bootstrap incomplete: %+v", bootstrap)
	}

	start := call(a, "POST", "/audio-novel/legacy-duration-buyer/start-listening", fmt.Sprintf(`{"ticket":%q}`, bootstrap.PlaybackTicket), nil)
	if start.Code != 200 || !strings.Contains(start.Body.String(), `"started":true`) {
		t.Fatalf("legacy MP3 did not start: %d %s", start.Code, start.Body.String())
	}
	// Unknown total duration disables only completion; listening and all other
	// internal counters remain available for the historical MP3.
	complete := call(a, "POST", "/audio-novel/legacy-duration-buyer/complete", fmt.Sprintf(`{"ticket":%q,"ended":true}`, bootstrap.PlaybackTicket), nil)
	if complete.Code != 400 {
		t.Fatalf("legacy MP3 with unknown duration completed: %d %s", complete.Code, complete.Body.String())
	}
}
