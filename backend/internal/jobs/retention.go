package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Retention(ctx context.Context, db *pgxpool.Pool, days int) {
	run := func() {
		q, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		// The rollup uses UTC dates and is for long-term additive counts only, never UV.
		tx, e := db.Begin(q)
		if e != nil {
			slog.Error("retention begin", "error", e)
			return
		}
		defer tx.Rollback(q)
		_, e = tx.Exec(q, `INSERT INTO daily_totals(date,total,filtered,bot,suspicious) SELECT (occurred_at AT TIME ZONE 'UTC')::date,count(*),count(*) FILTER(WHERE classification='normal'),count(*) FILTER(WHERE classification='bot'),count(*) FILTER(WHERE classification='suspicious') FROM click_events GROUP BY 1 ON CONFLICT(date) DO UPDATE SET total=EXCLUDED.total,filtered=EXCLUDED.filtered,bot=EXCLUDED.bot,suspicious=EXCLUDED.suspicious,updated_at=now()`)
		if e == nil {
			_, e = tx.Exec(q, "DELETE FROM click_events WHERE occurred_at<(date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC')-make_interval(days=>$1)", days)
		}
		if e == nil {
			_, e = tx.Exec(q, "DELETE FROM request_logs WHERE occurred_at<now()-interval '7 days'; DELETE FROM admin_sessions WHERE expires_at<now(); DELETE FROM daily_totals WHERE date<current_date-365; DELETE FROM audit_logs WHERE occurred_at<now()-interval '365 days'")
		}
		if e == nil {
			_, e = tx.Exec(q, "DELETE FROM meta_events WHERE created_at<now()-make_interval(days=>$1) AND status IN ('succeeded','expired','skipped','failed')", days)
		}
		if e == nil {
			e = tx.Commit(q)
		}
		if e != nil {
			slog.Error("retention failure", "error", e)
		}
	}
	run()
	tick := time.NewTicker(6 * time.Hour)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			run()
		}
	}
}
