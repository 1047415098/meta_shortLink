package auth

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ DB *pgxpool.Pool }

func (r Repository) Create(ctx context.Context, hash string) error {
	_, e := r.DB.Exec(ctx, "INSERT INTO admin_sessions(token_hash,expires_at) VALUES($1,now()+interval '12 hours')", hash)
	return e
}
func (r Repository) Valid(ctx context.Context, hash string) (bool, error) {
	var ok bool
	e := r.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM admin_sessions WHERE token_hash=$1 AND expires_at>now())", hash).Scan(&ok)
	return ok, e
}
func (r Repository) Delete(ctx context.Context, hash string) error {
	_, e := r.DB.Exec(ctx, "DELETE FROM admin_sessions WHERE token_hash=$1", hash)
	return e
}
