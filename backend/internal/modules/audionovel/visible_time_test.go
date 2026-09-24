package audionovel

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestVisibleTimeIsMonotonicClippedAndDoesNotQueueAdEvents(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()
	code, ticket := f.createVisit(t, 31, "meta", 10, 20)

	// Client-reported time may run ahead, but the server accepts at most observed time plus five seconds.
	clipped, err := f.service.UpdateVisibleTime(ctx, code, VisibleTimeUpdate{Ticket: ticket, VisibleSeconds: 1_000})
	if err != nil || clipped.VisibleSeconds != 25 {
		t.Fatalf("clipped visible time=%+v err=%v", clipped, err)
	}
	f.now = f.now.Add(10 * time.Second)
	backward, err := f.service.UpdateVisibleTime(ctx, code, VisibleTimeUpdate{Ticket: ticket, VisibleSeconds: 7})
	if err != nil || backward.VisibleSeconds != 25 {
		t.Fatalf("backward visible time=%+v err=%v", backward, err)
	}
	advanced, err := f.service.UpdateVisibleTime(ctx, code, VisibleTimeUpdate{Ticket: ticket, VisibleSeconds: 33})
	if err != nil || advanced.VisibleSeconds != 33 {
		t.Fatalf("advanced visible time=%+v err=%v", advanced, err)
	}

	visitID := fmt.Sprintf("%048x", 31)
	var visible, playback, metaEvents int
	var updatedAt *time.Time
	if err = f.db.QueryRow(ctx, `SELECT visible_seconds,visible_updated_at,playback_seconds,
		(SELECT count(*) FROM meta_events WHERE visit_id=e.id) FROM click_events e WHERE id=$1`, visitID).
		Scan(&visible, &updatedAt, &playback, &metaEvents); err != nil {
		t.Fatal(err)
	}
	if visible != 33 || updatedAt == nil || playback != 0 || metaEvents != 0 {
		t.Fatalf("stored visible=%d updated=%v playback=%d events=%d", visible, updatedAt, playback, metaEvents)
	}
}

func TestVisibleTimeRequiresAValidNormalAudioCampaignVisit(t *testing.T) {
	f := newPlaybackFixture(t)
	ctx := context.Background()

	code, ticket := f.createVisit(t, 32, "tiktok", 10, 7_300)
	capped, err := f.service.UpdateVisibleTime(ctx, code, VisibleTimeUpdate{Ticket: ticket, VisibleSeconds: 9_999})
	if err != nil || capped.VisibleSeconds != 7_200 {
		t.Fatalf("two-hour cap result=%+v err=%v", capped, err)
	}

	invalidCode, invalidTicket := f.createVisit(t, 33, "meta", 10, 20)
	if _, err = f.db.Exec(ctx, "UPDATE click_events SET classification='bot' WHERE id=$1", fmt.Sprintf("%048x", 33)); err != nil {
		t.Fatal(err)
	}
	if _, err = f.service.UpdateVisibleTime(ctx, invalidCode, VisibleTimeUpdate{Ticket: invalidTicket, VisibleSeconds: 10}); !errors.Is(err, ErrPlaybackTicket) {
		t.Fatalf("non-normal visit error=%v", err)
	}

	expiredCode, _ := f.createVisit(t, 34, "meta", 10, 20)
	expiredTicket := issuePlaybackTicket(f.core, fmt.Sprintf("%048x", 34), 1, f.audioNovelID, f.now.Add(-playbackTicketLifetime-time.Second))
	if _, err = f.service.UpdateVisibleTime(ctx, expiredCode, VisibleTimeUpdate{Ticket: expiredTicket, VisibleSeconds: 10}); !errors.Is(err, ErrPlaybackTicket) {
		t.Fatalf("expired ticket error=%v", err)
	}
	if _, err = f.service.UpdateVisibleTime(ctx, code, VisibleTimeUpdate{Ticket: ticket, VisibleSeconds: 0}); !errors.Is(err, ErrVisibleTimeInput) {
		t.Fatalf("zero visible time error=%v", err)
	}
}
