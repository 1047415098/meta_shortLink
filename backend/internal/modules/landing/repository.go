package landing

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"whatsapp-analytics/internal/modules/meta"
)

// ErrTimeSpentTooEarly prevents a browser from manufacturing the event before
// the server-observed visit has existed for the frozen threshold.
var ErrTimeSpentTooEarly = errors.New("time spent threshold has not been reached")

type Repository struct {
	DB   *pgxpool.Pool
	Meta *meta.Service
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
	e = tx.QueryRow(ctx, `SELECT auto_redirected_at FROM click_events WHERE id=$1 AND link_id=$2 AND event_type='redirect' AND method='GET' FOR UPDATE`, eventID, linkID).Scan(&automatic)
	if e != nil {
		return e
	}
	if automatic == nil {
		// The timestamp and `_auto` event ID make retries idempotent without
		// recording a page view that the visitor never saw.
		if _, e = tx.Exec(ctx, `UPDATE click_events SET auto_redirected_at=COALESCE(auto_redirected_at,now()) WHERE id=$1`, eventID); e != nil {
			return e
		}
		if r.Meta != nil {
			if e = r.Meta.EnqueueAutoRedirect(ctx, tx, eventID, input); e != nil {
				return e
			}
		}
	}
	return tx.Commit(ctx)
}

// MarkContact is idempotent per trigger and requires a recent GET landing event.
func (r Repository) MarkContact(ctx context.Context, eventID string, linkID int64, surface string, automatic bool, input meta.ContactContext) (string, error) {
	column := "whatsapp_clicked_at"
	if automatic {
		column = "auto_redirected_at"
	}
	var target string
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return target, e
	}
	defer tx.Rollback(ctx)
	var previous *time.Time
	// Only a landing visit can be promoted into a consultation action; direct
	// links keep their visit statistics without manufacturing action events.
	e = tx.QueryRow(ctx, `SELECT target_url,`+column+` FROM click_events WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE`, eventID, linkID, surface).Scan(&target, &previous)
	if e != nil {
		return target, e
	}
	if previous == nil {
		if _, e = tx.Exec(ctx, `UPDATE click_events SET `+column+`=now() WHERE id=$1`, eventID); e != nil {
			return target, e
		}
		if r.Meta != nil {
			// Preserve the browser trigger when creating the Meta event so manual
			// consultations and timer redirects have independent event IDs.
			if automatic {
				e = r.Meta.EnqueueAutoRedirect(ctx, tx, eventID, input)
			} else {
				e = r.Meta.EnqueueContact(ctx, tx, eventID, input)
			}
			if e != nil {
				return target, e
			}
		}
	}
	return target, tx.Commit(ctx)
}

func (r Repository) MarkView(ctx context.Context, eventID string, linkID int64, surface string, input meta.ContactContext) error {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var previous *time.Time
	e = tx.QueryRow(ctx, "SELECT pageview_reported_at FROM click_events WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE", eventID, linkID, surface).Scan(&previous)
	if e != nil {
		return e
	}
	if previous == nil {
		if _, e = tx.Exec(ctx, "UPDATE click_events SET pageview_reported_at=now() WHERE id=$1", eventID); e != nil {
			return e
		}
		if r.Meta != nil {
			if e = r.Meta.EnqueuePageView(ctx, tx, eventID, input); e != nil {
				return e
			}
		}
	}
	return tx.Commit(ctx)
}

// MarkTimeSpent records the custom event once for one signed page visit.
func (r Repository) MarkTimeSpent(ctx context.Context, eventID string, linkID int64, surface string, input meta.ContactContext) error {
	tx, err := r.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var threshold int
	var occurred time.Time
	var reported *time.Time
	err = tx.QueryRow(ctx, `SELECT time_spent_threshold,occurred_at,time_spent_reported_at
		FROM click_events
		WHERE id=$1 AND link_id=$2 AND surface=$3 AND event_type='landing' AND method='GET'
		AND occurred_at>now()-interval '2 hours' FOR UPDATE`, eventID, linkID, surface).Scan(&threshold, &occurred, &reported)
	if err != nil {
		return err
	}
	if threshold == 0 || time.Now().Before(occurred.Add(time.Duration(threshold)*time.Second)) {
		return ErrTimeSpentTooEarly
	}
	if reported == nil {
		if _, err = tx.Exec(ctx, "UPDATE click_events SET time_spent_reported_at=now() WHERE id=$1", eventID); err != nil {
			return err
		}
		if r.Meta != nil {
			if err = r.Meta.EnqueueTimeSpent(ctx, tx, eventID, input); err != nil {
				return err
			}
		}
	}
	return tx.Commit(ctx)
}
