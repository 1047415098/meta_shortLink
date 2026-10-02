package landing

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/tiktok"
)

// ErrTimeSpentTooEarly prevents a browser from manufacturing the event before
// the server-observed visit has existed for the frozen threshold.
var ErrTimeSpentTooEarly = errors.New("time spent threshold has not been reached")

type Repository struct {
	DB     *pgxpool.Pool
	Meta   *meta.Service
	TikTok *tiktok.Service
}

// queueTikTok keeps event delivery best-effort. A savepoint isolates a queue
// failure so the confirmed WhatsApp action can still be committed and opened.
func (r Repository) queueTikTok(ctx context.Context, tx pgx.Tx, visitID, eventName, eventID string, input tiktok.ShortLinkEventInput) tiktok.BrowserEvent {
	if r.TikTok == nil {
		return tiktok.BrowserEvent{}
	}
	if _, err := tx.Exec(ctx, "SAVEPOINT short_link_tiktok"); err != nil {
		slog.Error("TIKTOK_SHORT_LINK_SAVEPOINT_FAILED", "visit_id", visitID, "error", err)
		return tiktok.BrowserEvent{}
	}
	event, _, err := r.TikTok.EnqueueShortLinkEventTx(ctx, tx, visitID, eventName, eventID, input)
	if err != nil {
		_, _ = tx.Exec(ctx, "ROLLBACK TO SAVEPOINT short_link_tiktok")
		slog.Error("TIKTOK_SHORT_LINK_QUEUE_FAILED", "visit_id", visitID, "event_name", eventName, "error", err)
	}
	_, _ = tx.Exec(ctx, "RELEASE SAVEPOINT short_link_tiktok")
	if err != nil {
		return tiktok.BrowserEvent{}
	}
	return event
}

// MarkDirect records only the automatic WhatsApp handoff. Direct mode never
// renders the landing application, so it must not manufacture a PageView.
func (r Repository) MarkDirect(ctx context.Context, eventID string, linkID int64, input meta.ContactContext) error {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var automatic *time.Time
	var platform string
	e = tx.QueryRow(ctx, `SELECT auto_redirected_at,ad_platform FROM click_events WHERE id=$1 AND link_id=$2 AND event_type='redirect' AND method='GET' FOR UPDATE`, eventID, linkID).Scan(&automatic, &platform)
	if e != nil {
		return e
	}
	if automatic == nil {
		// The timestamp and `_auto` event ID make retries idempotent without
		// recording a page view that the visitor never saw.
		if _, e = tx.Exec(ctx, `UPDATE click_events SET auto_redirected_at=COALESCE(auto_redirected_at,now()) WHERE id=$1`, eventID); e != nil {
			return e
		}
		if platform == "meta" && r.Meta != nil {
			if e = r.Meta.EnqueueAutoRedirect(ctx, tx, eventID, input); e != nil {
				return e
			}
		}
		if platform == "tiktok" {
			r.queueTikTok(ctx, tx, eventID, "Contact", "short_"+eventID+"_auto", tiktok.ShortLinkEventInput{LinkID: linkID, EventAt: time.Now(), Trigger: "auto"})
		}
	}
	return tx.Commit(ctx)
}

// MarkContact is idempotent per trigger and requires a recent GET landing event.
func (r Repository) MarkContact(ctx context.Context, eventID string, linkID int64, surface string, automatic bool, input meta.ContactContext) (string, tiktok.BrowserEvent, error) {
	column := "whatsapp_clicked_at"
	if automatic {
		column = "auto_redirected_at"
	}
	var target string
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return target, tiktok.BrowserEvent{}, e
	}
	defer tx.Rollback(ctx)
	var previous *time.Time
	var platform string
	// Only a landing visit can be promoted into a consultation action; direct
	// links keep their visit statistics without manufacturing action events.
	e = tx.QueryRow(ctx, `SELECT target_url,`+column+`,ad_platform FROM click_events WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE`, eventID, linkID, surface).Scan(&target, &previous, &platform)
	if e != nil {
		return target, tiktok.BrowserEvent{}, e
	}
	eventAt := time.Now()
	if previous == nil {
		if _, e = tx.Exec(ctx, `UPDATE click_events SET `+column+`=$2 WHERE id=$1`, eventID, eventAt); e != nil {
			return target, tiktok.BrowserEvent{}, e
		}
		if platform == "meta" && r.Meta != nil {
			// Preserve the browser trigger when creating the Meta event so manual
			// consultations and timer redirects have independent event IDs.
			if automatic {
				e = r.Meta.EnqueueAutoRedirect(ctx, tx, eventID, input)
			} else {
				e = r.Meta.EnqueueContact(ctx, tx, eventID, input)
			}
			if e != nil {
				return target, tiktok.BrowserEvent{}, e
			}
		}
	} else {
		eventAt = *previous
	}
	var event tiktok.BrowserEvent
	if platform == "tiktok" {
		trigger, suffix := "manual", "manual"
		if automatic {
			trigger, suffix = "auto", "auto"
		}
		event = r.queueTikTok(ctx, tx, eventID, "Contact", "short_"+eventID+"_"+suffix, tiktok.ShortLinkEventInput{LinkID: linkID, EventAt: eventAt, Trigger: trigger})
	}
	return target, event, tx.Commit(ctx)
}

func (r Repository) MarkView(ctx context.Context, eventID string, linkID int64, surface string, input meta.ContactContext) (tiktok.BrowserEvent, error) {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return tiktok.BrowserEvent{}, e
	}
	defer tx.Rollback(ctx)
	var previous *time.Time
	var platform string
	e = tx.QueryRow(ctx, "SELECT pageview_reported_at,ad_platform FROM click_events WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE", eventID, linkID, surface).Scan(&previous, &platform)
	if e != nil {
		return tiktok.BrowserEvent{}, e
	}
	eventAt := time.Now()
	if previous == nil {
		if _, e = tx.Exec(ctx, "UPDATE click_events SET pageview_reported_at=$2 WHERE id=$1", eventID, eventAt); e != nil {
			return tiktok.BrowserEvent{}, e
		}
		if platform == "meta" && r.Meta != nil {
			if e = r.Meta.EnqueuePageView(ctx, tx, eventID, input); e != nil {
				return tiktok.BrowserEvent{}, e
			}
		}
	} else {
		eventAt = *previous
	}
	var event tiktok.BrowserEvent
	if platform == "tiktok" {
		event = r.queueTikTok(ctx, tx, eventID, "PageView", "short_"+eventID+"_view", tiktok.ShortLinkEventInput{LinkID: linkID, EventAt: eventAt})
	}
	return event, tx.Commit(ctx)
}

// MarkTimeSpent records the custom event once for one signed page visit.
func (r Repository) MarkTimeSpent(ctx context.Context, eventID string, linkID int64, surface string, input meta.ContactContext) (tiktok.BrowserEvent, error) {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return tiktok.BrowserEvent{}, err
	}
	defer tx.Rollback(ctx)
	var threshold int
	var occurred time.Time
	var reported *time.Time
	var platform string
	err = tx.QueryRow(ctx, `SELECT time_spent_threshold,occurred_at,time_spent_reported_at
		,ad_platform
		FROM click_events
		WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET'
		AND occurred_at>now()-interval '2 hours' FOR UPDATE`, eventID, linkID, surface).Scan(&threshold, &occurred, &reported, &platform)
	if err != nil {
		return tiktok.BrowserEvent{}, err
	}
	if threshold == 0 || time.Now().Before(occurred.Add(time.Duration(threshold)*time.Second)) {
		return tiktok.BrowserEvent{}, ErrTimeSpentTooEarly
	}
	eventAt := time.Now()
	if reported == nil {
		if _, err = tx.Exec(ctx, "UPDATE click_events SET time_spent_reported_at=$2 WHERE id=$1", eventID, eventAt); err != nil {
			return tiktok.BrowserEvent{}, err
		}
		if platform == "meta" && r.Meta != nil {
			if err = r.Meta.EnqueueTimeSpent(ctx, tx, eventID, input); err != nil {
				return tiktok.BrowserEvent{}, err
			}
		}
	} else {
		eventAt = *reported
	}
	var event tiktok.BrowserEvent
	if platform == "tiktok" {
		event = r.queueTikTok(ctx, tx, eventID, "ViewContent", "short_"+eventID+"_qualified", tiktok.ShortLinkEventInput{LinkID: linkID, EventAt: eventAt})
	}
	return event, tx.Commit(ctx)
}
