package adspend

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) Import(ctx context.Context, records [][]string, actor string) error {
	tx, e := r.DB.Begin(ctx)
	if e != nil {
		return e
	}
	defer tx.Rollback(ctx)
	for _, r := range records {
		_, e = tx.Exec(ctx, `INSERT INTO ad_spend_daily(date,ad_id,amount,currency,time_zone) VALUES($1,$2,$3,$4,$5) ON CONFLICT(date,ad_id,currency,time_zone) DO UPDATE SET amount=EXCLUDED.amount`, r[0], r[1], r[2], r[3], r[4])
		if e != nil {
			return e
		}
	}
	b, _ := json.Marshal(map[string]any{"rows": len(records)})
	_, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'spend.import',$2)", actor, b)
	if e == nil {
		e = tx.Commit(ctx)
	}
	return e
}
