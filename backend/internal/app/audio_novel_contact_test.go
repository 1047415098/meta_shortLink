package app

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestAudioNovelCampaignWithoutTargetRejectsContactWithoutMutation(t *testing.T) {
	a := setup(t)
	admin := login(t, a)
	audioNovelID := createAudioDistributionNovel(t, a, admin, "No Contact Audio", "no-contact-audio", true)
	metaConnectionID, metaPixelID := testLinkMetaBinding(t, a)
	created := call(a, "POST", "/api/v1/audio-novel-links", fmt.Sprintf(`{"name":"No Contact Buyer","code":"no-contact-audio-link","audio_novel_id":%d,"enabled":true,"ad_platform":"meta","meta_connection_id":%d,"meta_pixel_id":%d,"time_spent_threshold":10}`, audioNovelID, metaConnectionID, metaPixelID), admin)
	if created.Code != 200 {
		t.Fatalf("create audio campaign: %d %s", created.Code, created.Body.String())
	}

	page := call(a, "GET", "/audio-novel/no-contact-audio-link?fbclid=click-1&ad_id=ad-1", "", nil)
	if page.Code != 200 {
		t.Fatalf("open audio campaign: %d %s", page.Code, page.Body.String())
	}
	bootstrap := decodeAudioBootstrap(t, page)
	if bootstrap.Link == nil || bootstrap.Link.TargetURL != "" || bootstrap.Ticket == "" {
		t.Fatalf("campaign bootstrap must expose an empty target and keep its view ticket: %+v", bootstrap)
	}
	visitID := strings.Split(bootstrap.Ticket, ".")[0]
	contact := postNovelForm(a, "/audio-novel/no-contact-audio-link/contact", url.Values{"ticket": {bootstrap.Ticket}, "trigger": {"manual"}})
	if contact.Code < 400 {
		t.Fatalf("empty contact target returned success: %d %s", contact.Code, contact.Body.String())
	}

	var contactWrites, contactEvents int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM click_events WHERE id=$1 AND whatsapp_clicked_at IS NOT NULL", visitID).Scan(&contactWrites); err != nil {
		t.Fatal(err)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM meta_events WHERE visit_id=$1 AND event_name IN ('WhatsAppConsultClick','Contact','AddToCart')", visitID).Scan(&contactEvents); err != nil {
		t.Fatal(err)
	}
	if contactWrites != 0 || contactEvents != 0 {
		t.Fatalf("empty target polluted contact analytics: writes=%d events=%d", contactWrites, contactEvents)
	}
}

func TestLegacyAudioNovelTargetStillAllowsContact(t *testing.T) {
	a := setup(t)
	page := call(a, "GET", "/audio-novel/hello", "", nil)
	if page.Code != 200 {
		t.Fatalf("open legacy audio link: %d %s", page.Code, page.Body.String())
	}
	bootstrap := decodeAudioBootstrap(t, page)
	if bootstrap.Link == nil || bootstrap.Link.TargetURL == "" || bootstrap.Ticket == "" {
		t.Fatalf("legacy bootstrap lost its contact target: %+v", bootstrap)
	}

	contact := postNovelForm(a, "/audio-novel/hello/contact", url.Values{"ticket": {bootstrap.Ticket}, "trigger": {"manual"}})
	if contact.Code != 200 || !strings.Contains(contact.Body.String(), bootstrap.Link.TargetURL) {
		t.Fatalf("legacy contact changed: %d %s", contact.Code, contact.Body.String())
	}
}
