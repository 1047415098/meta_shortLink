package landing

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"whatsapp-analytics/internal/modules/meta"
)

type Repository struct {
	DB   *pgxpool.Pool
	Meta *meta.Service
}

// MarkContact is idempotent per trigger and requires a recent GET landing event.
func (r Repository) MarkContact(ctx context.Context, eventID string, linkID int64, automatic bool, input meta.ContactContext) (string, error) {
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
	e = tx.QueryRow(ctx, `SELECT target_url,`+column+` FROM click_events WHERE id=$1 AND link_id=$2 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE`, eventID, linkID).Scan(&target, &previous)
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

func (r Repository) MarkView(ctx context.Context, eventID string, linkID int64, input meta.ContactContext) error {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	var previous *time.Time
	e = tx.QueryRow(ctx, "SELECT pageview_reported_at FROM click_events WHERE id=$1 AND link_id=$2 AND event_type='landing' AND method='GET' AND occurred_at>now()-interval '1 hour' FOR UPDATE", eventID, linkID).Scan(&previous)
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
