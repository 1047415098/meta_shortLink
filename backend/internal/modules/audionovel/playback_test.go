package audionovel

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"whatsapp-analytics/internal/config"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/database"
	"whatsapp-analytics/internal/platform/runtime"
)

type playbackFixture struct {
	db                 *pgxpool.Pool
	core               *runtime.Core
	service            *PlaybackService
	tikTok             *tiktok.Service
	now                time.Time
	audioNovelID       int64
	metaConnectionID   int64
	metaPixelID        int64
	tikTokConnectionID int64
	tikTokPixelID      int64
}

func newPlaybackFixture(t *testing.T) *playbackFixture {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is required for playback integration tests")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	var databaseName string
	if err = db.QueryRow(ctx, "SELECT current_database()").Scan(&databaseName); err != nil || !strings.Contains(databaseName, "_test") {
		t.Fatalf("refusing to reset database %q: %v", databaseName, err)
	}
	if _, err = db.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err = database.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	core := runtime.New(config.Config{PublicURL: "https://audio.example", Secret: strings.Repeat("p", 32), TikTokEnabled: true}, db)
	metaService := meta.New(core)
	tikTokService := tiktok.New(core)
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	fixture := &playbackFixture{db: db, core: core, tikTok: tikTokService, now: now}
	fixture.service = &PlaybackService{Core: core, Meta: metaService, TikTok: tikTokService, Now: func() time.Time { return fixture.now }}
	if err = db.QueryRow(ctx, `INSERT INTO meta_connections(name,account_id) VALUES('Playback Meta','123456') RETURNING id`).Scan(&fixture.metaConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO meta_pixels(connection_id,name,pixel_id,enabled,pageview_enabled,capi_token_cipher)
		VALUES($1,'Playback Pixel','987654',true,true,'test-cipher') RETURNING id`, fixture.metaConnectionID).Scan(&fixture.metaPixelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_connections(name,access_token_cipher,enabled)
		VALUES('Playback TikTok','test-cipher',true) RETURNING id`).Scan(&fixture.tikTokConnectionID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO tiktok_pixels(connection_id,name,pixel_code,enabled)
		VALUES($1,'Playback TikTok Pixel','PX_PLAYBACK',true) RETURNING id`, fixture.tikTokConnectionID).Scan(&fixture.tikTokPixelID); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow(ctx, `INSERT INTO audio_novels(
		title,slug,category,excerpt,body_markdown,audio_path,audio_duration,audio_duration_seconds,audio_size_bytes,published_at,enabled)
		VALUES('Frozen Audio Title','frozen-audio','Drama','Excerpt','# Body','/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3','00:30',30,1024,'2026-09-24',true)
		RETURNING id`).Scan(&fixture.audioNovelID); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *playbackFixture) createVisit(t *testing.T, sequence int, platform string, threshold, ageSeconds int) (string, string) {
	t.Helper()
	ctx := context.Background()
	code := fmt.Sprintf("playback-%s-%d", platform, sequence)
	var linkID int64
	if platform == "tiktok" {
		if err := f.db.QueryRow(ctx, `INSERT INTO short_links(
			code,name,target_url,product_type,audio_novel_id,enabled,ad_platform,tiktok_pixel_id,attribution_mode,time_spent_threshold,first_visited_at)
			VALUES($1,$2,'','audio_novel',$3,true,'tiktok',$4,'dynamic',$5,$6) RETURNING id`, code, code, f.audioNovelID, f.tikTokPixelID, threshold, f.now.Add(-time.Duration(ageSeconds)*time.Second)).Scan(&linkID); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := f.db.QueryRow(ctx, `INSERT INTO short_links(
			code,name,target_url,product_type,audio_novel_id,enabled,ad_platform,meta_connection_id,meta_pixel_id,attribution_mode,time_spent_threshold,first_visited_at)
			VALUES($1,$2,'','audio_novel',$3,true,'meta',$4,$5,'dynamic',$6,$7) RETURNING id`, code, code, f.audioNovelID, f.metaConnectionID, f.metaPixelID, threshold, f.now.Add(-time.Duration(ageSeconds)*time.Second)).Scan(&linkID); err != nil {
			t.Fatal(err)
		}
	}
	visitID := fmt.Sprintf("%048x", sequence)
	contextCipher := ""
	if platform == "tiktok" {
		var err error
		contextCipher, err = f.tikTok.SealVisitContext(tiktok.VisitContext{IP: "192.0.2.9", UserAgent: "Test Browser", PageURL: "https://audio.example/audio-novel/" + code}, visitID)
		if err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.db.Exec(ctx, `INSERT INTO click_events(
		id,occurred_at,link_id,visitor_id,cookie_status,method,target_url,status,device,os,browser,
		country,region,city,source,campaign_id,adset_id,ad_id,referrer,parameters,classification,reason,event_type,surface,
		time_spent_threshold,audio_novel_id,audio_novel_title,ad_platform,
		meta_connection_id,meta_account_id,meta_measurement,meta_pixel_id,meta_pageview_enabled,
		tiktok_pixel_id,tiktok_pixel_code,tiktok_ttclid,tiktok_context_cipher)
		VALUES($1,$2,$3,$4,'issued','GET','',200,'mobile','iOS','Safari',
		'US','','','facebook','campaign','group','ad-1','', '{"fbclid":"click-1"}', 'normal','','landing','audio_novel',
		$5,$6,'Frozen Audio Title',$7,$8,'123456',true,$9,true,$10,$11,'ttclid-1',$12)`,
		visitID, f.now.Add(-time.Duration(ageSeconds)*time.Second), linkID, "visitor-1", threshold, f.audioNovelID, platform,
		nullableID(platform == "meta", f.metaConnectionID), nullableID(platform == "meta", f.metaPixelID),
		nullableID(platform == "tiktok", f.tikTokPixelID), map[bool]string{true: "PX_PLAYBACK"}[platform == "tiktok"], contextCipher)
	if err != nil {
		t.Fatal(err)
	}
	ticket := issuePlaybackTicket(f.core, visitID, linkID, f.audioNovelID, f.now.Add(-time.Duration(ageSeconds)*time.Second))
	return code, ticket
}

func nullableID(include bool, value int64) any {
	if include {
		return value
	}
	return nil
}

func playbackContext() PlaybackRequestContext {
	return PlaybackRequestContext{Meta: meta.ContactContext{IP: "192.0.2.9", UserAgent: "Test Browser", FBP: "fb.1.1720000000000.123"}, TikTokTTP: "ttp-test"}
}

func TestPlaybackServiceValidatesTicketsAndAdvancesMonotonically(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	code, ticket := f.createVisit(t, 1, "meta", 10, 40)
	if _, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: "bad.ticket"}, playbackContext()); err == nil {
		t.Fatal("tampered ticket was accepted")
	}
	otherCode, _ := f.createVisit(t, 2, "meta", 10, 40)
	if _, err := f.service.StartListening(ctx, otherCode, PlaybackUpdate{Ticket: ticket}, playbackContext()); err == nil {
		t.Fatal("cross-code ticket was accepted")
	}
	expired := issuePlaybackTicket(f.core, strings.Repeat("e", 48), 1, f.audioNovelID, f.now.Add(-playbackTicketLifetime-time.Second))
	if _, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: expired}, playbackContext()); err == nil {
		t.Fatal("expired ticket was accepted")
	}

	started, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext())
	if err != nil || !started.Started || len(started.ConfirmedEvents) != 1 || started.ConfirmedEvents[0].Name != "StartListening" || started.ConfirmedEvents[0].EventID != "audio_"+strings.Repeat("0", 47)+"1_start" {
		t.Fatalf("start result=%+v err=%v", started, err)
	}
	repeated, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext())
	if err != nil || len(repeated.ConfirmedEvents) != 1 || repeated.ConfirmedEvents[0] != started.ConfirmedEvents[0] {
		t.Fatalf("repeated start result=%+v err=%v", repeated, err)
	}

	five, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 5, MediaConsumedSeconds: 5}, playbackContext())
	if err != nil || five.PlaybackSeconds != 5 || five.Qualified {
		t.Fatalf("five-second result=%+v err=%v", five, err)
	}
	twelve, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 12, MediaConsumedSeconds: 12}, playbackContext())
	if err != nil || twelve.PlaybackSeconds != 12 || !twelve.Qualified || len(twelve.ConfirmedEvents) != 1 || twelve.ConfirmedEvents[0].EventID != "audio_"+strings.Repeat("0", 47)+"1_qualified" {
		t.Fatalf("qualified result=%+v err=%v", twelve, err)
	}
	backward, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 8, MediaConsumedSeconds: 8}, playbackContext())
	if err != nil || backward.PlaybackSeconds != 12 || backward.MediaConsumedSeconds != 12 || !backward.Qualified {
		t.Fatalf("backward report changed state: result=%+v err=%v", backward, err)
	}
	var startEvents, qualifiedEvents, wrongAudioBindings int
	if err = f.db.QueryRow(ctx, `SELECT count(*) FILTER(WHERE event_name='StartListening'),count(*) FILTER(WHERE event_name='ViewContent'),
		count(*) FILTER(WHERE audio_novel_id IS DISTINCT FROM $2)
		FROM meta_events WHERE visit_id=$1`, strings.Repeat("0", 47)+"1", f.audioNovelID).Scan(&startEvents, &qualifiedEvents, &wrongAudioBindings); err != nil || startEvents != 1 || qualifiedEvents != 1 || wrongAudioBindings != 0 {
		t.Fatalf("Meta event idempotency start=%d qualified=%d wrong_audio_binding=%d err=%v", startEvents, qualifiedEvents, wrongAudioBindings, err)
	}

	forgedCode, forgedTicket := f.createVisit(t, 3, "meta", 10, 3)
	forged, err := f.service.UpdatePlayback(ctx, forgedCode, PlaybackUpdate{Ticket: forgedTicket, PlaybackSeconds: 1000, MediaConsumedSeconds: 1000}, playbackContext())
	if err != nil || forged.PlaybackSeconds > 3 || forged.MediaConsumedSeconds > float64(forged.PlaybackSeconds)*maxPlaybackRate {
		t.Fatalf("forged progress was not clipped: result=%+v err=%v", forged, err)
	}

	disabledCode, disabledTicket := f.createVisit(t, 4, "meta", 0, 40)
	disabled, err := f.service.UpdatePlayback(ctx, disabledCode, PlaybackUpdate{Ticket: disabledTicket, PlaybackSeconds: 20, MediaConsumedSeconds: 20}, playbackContext())
	if err != nil || disabled.Qualified || len(disabled.ConfirmedEvents) != 0 {
		t.Fatalf("zero threshold qualified: result=%+v err=%v", disabled, err)
	}
}

func TestPlaybackServiceIsConcurrentAndCompletionIsInternalOnly(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	code, ticket := f.createVisit(t, 10, "meta", 10, 60)
	var wait sync.WaitGroup
	errorsSeen := make(chan error, 8)
	for range 8 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			result, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext())
			if err == nil && (len(result.ConfirmedEvents) != 1 || result.ConfirmedEvents[0].EventID != "audio_"+fmt.Sprintf("%048x", 10)+"_start") {
				err = fmt.Errorf("unexpected concurrent result: %+v", result)
			}
			errorsSeen <- err
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatal(err)
		}
	}
	var eventCount int
	if err := f.db.QueryRow(ctx, "SELECT count(*) FROM meta_events WHERE visit_id=$1 AND event_name='StartListening'", fmt.Sprintf("%048x", 10)).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("concurrent start events=%d err=%v", eventCount, err)
	}
	if _, err := f.service.Complete(ctx, code, PlaybackComplete{PlaybackUpdate: PlaybackUpdate{Ticket: ticket}, Ended: true}, playbackContext()); err == nil {
		t.Fatal("completion below 90 percent was accepted")
	}
	if _, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 20, MediaConsumedSeconds: 27}, playbackContext()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Complete(ctx, code, PlaybackComplete{PlaybackUpdate: PlaybackUpdate{Ticket: ticket}, Ended: false}, playbackContext()); err == nil {
		t.Fatal("completion without ended proof was accepted")
	}
	completed, err := f.service.Complete(ctx, code, PlaybackComplete{PlaybackUpdate: PlaybackUpdate{Ticket: ticket}, Ended: true}, playbackContext())
	if err != nil || !completed.Completed || len(completed.ConfirmedEvents) != 0 {
		t.Fatalf("completion result=%+v err=%v", completed, err)
	}
	repeated, err := f.service.Complete(ctx, code, PlaybackComplete{PlaybackUpdate: PlaybackUpdate{Ticket: ticket}, Ended: true}, playbackContext())
	if err != nil || !repeated.Completed {
		t.Fatalf("repeated completion result=%+v err=%v", repeated, err)
	}
	if err = f.db.QueryRow(ctx, "SELECT count(*) FROM meta_events WHERE visit_id=$1", fmt.Sprintf("%048x", 10)).Scan(&eventCount); err != nil || eventCount != 2 {
		t.Fatalf("completion created an advertising event: count=%d err=%v", eventCount, err)
	}
}

func TestPlaybackServiceQueuesTikTokAndRollsBackWhenOutboxFails(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	tikTokCode, tikTokTicket := f.createVisit(t, 20, "tiktok", 10, 40)
	if _, err := f.service.StartListening(ctx, tikTokCode, PlaybackUpdate{Ticket: tikTokTicket}, playbackContext()); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.UpdatePlayback(ctx, tikTokCode, PlaybackUpdate{Ticket: tikTokTicket, PlaybackSeconds: 12, MediaConsumedSeconds: 12}, playbackContext()); err != nil {
		t.Fatal(err)
	}
	var tikTokEvents, wrongBinding int
	if err := f.db.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE novel_id IS NOT NULL OR audio_novel_id<>$2)
		FROM tiktok_events WHERE visit_id=$1`, fmt.Sprintf("%048x", 20), f.audioNovelID).Scan(&tikTokEvents, &wrongBinding); err != nil || tikTokEvents != 2 || wrongBinding != 0 {
		t.Fatalf("TikTok audio events=%d wrong_binding=%d err=%v", tikTokEvents, wrongBinding, err)
	}

	rollbackCode, rollbackTicket := f.createVisit(t, 21, "meta", 10, 40)
	if _, err := f.db.Exec(ctx, `CREATE FUNCTION reject_audio_start() RETURNS trigger LANGUAGE plpgsql AS $$
		BEGIN IF NEW.event_name='StartListening' THEN RAISE EXCEPTION 'test outbox failure'; END IF; RETURN NEW; END $$;
		CREATE TRIGGER reject_audio_start BEFORE INSERT ON meta_events FOR EACH ROW EXECUTE FUNCTION reject_audio_start()`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.StartListening(ctx, rollbackCode, PlaybackUpdate{Ticket: rollbackTicket}, playbackContext()); err == nil {
		t.Fatal("outbox failure did not fail the state transition")
	}
	var startedAt *time.Time
	if err := f.db.QueryRow(ctx, "SELECT audio_started_at FROM click_events WHERE id=$1", fmt.Sprintf("%048x", 21)).Scan(&startedAt); err != nil || startedAt != nil {
		t.Fatalf("failed outbox left started state: value=%v err=%v", startedAt, err)
	}
	if _, err := f.db.Exec(ctx, "DROP TRIGGER reject_audio_start ON meta_events; DROP FUNCTION reject_audio_start()"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.StartListening(ctx, rollbackCode, PlaybackUpdate{Ticket: rollbackTicket}, playbackContext()); err != nil {
		t.Fatalf("retry after outbox recovery: %v", err)
	}

	if _, err := f.db.Exec(ctx, "UPDATE meta_pixels SET enabled=false WHERE id=$1", f.metaPixelID); err != nil {
		t.Fatal(err)
	}
	disabledCode, disabledTicket := f.createVisit(t, 22, "meta", 10, 40)
	result, err := f.service.StartListening(ctx, disabledCode, PlaybackUpdate{Ticket: disabledTicket}, playbackContext())
	if err != nil || !result.Started || len(result.ConfirmedEvents) != 0 {
		t.Fatalf("disabled Pixel blocked internal state: result=%+v err=%v", result, err)
	}
	var pending int
	if err = f.db.QueryRow(ctx, "SELECT count(*) FROM meta_events WHERE visit_id=$1 AND status='pending'", fmt.Sprintf("%048x", 22)).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("disabled Pixel event was not retained: pending=%d err=%v", pending, err)
	}

	organicCode, organicTicket := f.createVisit(t, 23, "meta", 10, 40)
	if _, err = f.db.Exec(ctx, "UPDATE click_events SET parameters='{}',ad_id='' WHERE id=$1", fmt.Sprintf("%048x", 23)); err != nil {
		t.Fatal(err)
	}
	organic, err := f.service.StartListening(ctx, organicCode, PlaybackUpdate{Ticket: organicTicket}, playbackContext())
	// Internal listening still advances, but a visit that failed the frozen
	// dynamic-attribution gate must not authorize the browser Pixel event.
	if err != nil || !organic.Started || len(organic.ConfirmedEvents) != 0 {
		t.Fatalf("organic Meta visit confirmed a browser event: result=%+v err=%v", organic, err)
	}
	var skipped int
	if err = f.db.QueryRow(ctx, "SELECT count(*) FROM meta_events WHERE visit_id=$1 AND status='skipped'", fmt.Sprintf("%048x", 23)).Scan(&skipped); err != nil || skipped != 1 {
		t.Fatalf("organic Meta diagnostic event missing: skipped=%d err=%v", skipped, err)
	}
}

func TestPlaybackServiceKeepsInternalStateWhenTikTokSnapshotIsUnavailable(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	code, ticket := f.createVisit(t, 24, "tiktok", 10, 40)
	visitID := fmt.Sprintf("%048x", 24)
	// Entry tracking intentionally keeps the reader available when encrypting
	// the private TikTok request snapshot fails, so playback must do the same.
	if _, err := f.db.Exec(ctx, "UPDATE click_events SET tiktok_context_cipher='' WHERE id=$1", visitID); err != nil {
		t.Fatal(err)
	}
	started, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext())
	if err != nil || !started.Started || len(started.ConfirmedEvents) != 0 {
		t.Fatalf("missing TikTok snapshot blocked start: result=%+v err=%v", started, err)
	}
	qualified, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 12, MediaConsumedSeconds: 12}, playbackContext())
	if err != nil || !qualified.Qualified || len(qualified.ConfirmedEvents) != 0 {
		t.Fatalf("missing TikTok snapshot blocked qualification: result=%+v err=%v", qualified, err)
	}
	var failed int
	if err = f.db.QueryRow(ctx, `SELECT count(*) FROM tiktok_events
		WHERE visit_id=$1 AND status='failed' AND payload_cipher=''`, visitID).Scan(&failed); err != nil || failed != 2 {
		t.Fatalf("TikTok snapshot diagnostics=%d err=%v", failed, err)
	}
}

func TestPlaybackServiceSuppressesTikTokBrowserEventsWhenDeliveryBecomesUnavailable(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	tests := []struct {
		name       string
		blockSQL   string
		restoreSQL string
		globalOff  bool
	}{
		{name: "global switch", globalOff: true},
		{name: "disabled Pixel", blockSQL: "UPDATE tiktok_pixels SET enabled=false WHERE id=$1", restoreSQL: "UPDATE tiktok_pixels SET enabled=true WHERE id=$1"},
		{name: "disabled connection", blockSQL: "UPDATE tiktok_connections SET enabled=false WHERE id=$1", restoreSQL: "UPDATE tiktok_connections SET enabled=true WHERE id=$1"},
		{name: "missing credential", blockSQL: "UPDATE tiktok_connections SET access_token_cipher='' WHERE id=$1", restoreSQL: "UPDATE tiktok_connections SET access_token_cipher='test-cipher' WHERE id=$1"},
		{name: "invalid credential", blockSQL: "UPDATE tiktok_connections SET credential_status='invalid' WHERE id=$1", restoreSQL: "UPDATE tiktok_connections SET credential_status='unverified' WHERE id=$1"},
	}

	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, ticket := f.createVisit(t, 30+index, "tiktok", 10, 40)
			visitID := fmt.Sprintf("%048x", 30+index)
			if test.globalOff {
				f.core.Config.TikTokEnabled = false
				t.Cleanup(func() { f.core.Config.TikTokEnabled = true })
			} else {
				id := f.tikTokConnectionID
				if strings.Contains(test.blockSQL, "tiktok_pixels") {
					id = f.tikTokPixelID
				}
				if _, err := f.db.Exec(ctx, test.blockSQL, id); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if _, err := f.db.Exec(ctx, test.restoreSQL, id); err != nil {
						t.Errorf("restore TikTok configuration: %v", err)
					}
				})
			}

			started, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext())
			if err != nil || !started.Started || len(started.ConfirmedEvents) != 0 {
				t.Fatalf("unavailable TikTok delivery confirmed start: result=%+v err=%v", started, err)
			}
			qualified, err := f.service.UpdatePlayback(ctx, code, PlaybackUpdate{Ticket: ticket, PlaybackSeconds: 12, MediaConsumedSeconds: 12}, playbackContext())
			if err != nil || !qualified.Qualified || len(qualified.ConfirmedEvents) != 0 {
				t.Fatalf("unavailable TikTok delivery confirmed qualification: result=%+v err=%v", qualified, err)
			}

			var startedAt, qualifiedAt *time.Time
			var events, failed int
			var diagnostics string
			if err = f.db.QueryRow(ctx, "SELECT audio_started_at,audio_qualified_at FROM click_events WHERE id=$1", visitID).Scan(&startedAt, &qualifiedAt); err != nil || startedAt == nil || qualifiedAt == nil {
				t.Fatalf("internal playback state was lost: started=%v qualified=%v err=%v", startedAt, qualifiedAt, err)
			}
			if err = f.db.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE status='failed' AND payload_cipher='' AND attempts=1),
				COALESCE(string_agg(last_error,' '),'') FROM tiktok_events WHERE visit_id=$1`, visitID).Scan(&events, &failed, &diagnostics); err != nil || events != 2 || failed != 2 {
				t.Fatalf("TikTok blocker diagnostics events=%d failed=%d text=%q err=%v", events, failed, diagnostics, err)
			}
			for _, secret := range []string{"test-cipher", "ttclid-1", "ttp-test", "192.0.2.9"} {
				if strings.Contains(diagnostics, secret) {
					t.Fatalf("TikTok blocker diagnostic exposed %q: %s", secret, diagnostics)
				}
			}
		})
	}
}

func TestPlaybackServiceRejectsDisabledLinkOrContentAfterEntry(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	code, ticket := f.createVisit(t, 25, "meta", 10, 40)
	// An already-open page must not create new advertising events after an
	// operator withdraws the bound audio from publication.
	if _, err := f.db.Exec(ctx, "UPDATE audio_novels SET enabled=false WHERE id=$1", f.audioNovelID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.StartListening(ctx, code, PlaybackUpdate{Ticket: ticket}, playbackContext()); err == nil {
		t.Fatal("disabled audio accepted a new listening event")
	}
	var startedAt *time.Time
	var events int
	visitID := fmt.Sprintf("%048x", 25)
	if err := f.db.QueryRow(ctx, `SELECT audio_started_at,
		(SELECT count(*) FROM meta_events WHERE visit_id=e.id) FROM click_events e WHERE id=$1`, visitID).
		Scan(&startedAt, &events); err != nil || startedAt != nil || events != 0 {
		t.Fatalf("disabled audio changed playback state: started=%v events=%d err=%v", startedAt, events, err)
	}

	if _, err := f.db.Exec(ctx, "UPDATE audio_novels SET enabled=true WHERE id=$1", f.audioNovelID); err != nil {
		t.Fatal(err)
	}
	disabledCode, disabledTicket := f.createVisit(t, 26, "meta", 10, 40)
	if _, err := f.db.Exec(ctx, "UPDATE short_links SET enabled=false WHERE code=$1", disabledCode); err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.StartListening(ctx, disabledCode, PlaybackUpdate{Ticket: disabledTicket}, playbackContext()); err == nil {
		t.Fatal("disabled campaign link accepted a new listening event")
	}
}
