package meta

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
)

// SaveConnection persists account grouping only. Pixel credentials and event
// rules belong to meta_pixels, so deprecated account secrets are ignored.
func (s *Service) SaveConnection(ctx context.Context, in ConnectionInput, id int64) (Connection, error) {
	c := in.Connection
	c.ID = id
	c.Name = strings.TrimSpace(c.Name)
	c.AccountID = strings.TrimSpace(c.AccountID)
	c.APIVersion = strings.TrimSpace(c.APIVersion)
	if c.APIVersion == "" {
		c.APIVersion = "v26.0"
	}

	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return c, err
	}
	defer tx.Rollback(ctx)

	if id > 0 {
		// Lock the account while enforcing its immutable Meta account ID.
		old, loadErr := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM meta_connections WHERE id=$1 FOR UPDATE", id))
		if loadErr != nil {
			return c, loadErr
		}
		if c.AccountID != old.AccountID {
			return c, errors.New("广告账户 ID 创建后不可更改，请新建账户")
		}
	}
	if err = validateConnection(c); err != nil {
		return c, err
	}

	query := `INSERT INTO meta_connections(name,account_id,api_version) VALUES($1,$2,$3) RETURNING ` + connectionColumns
	args := []any{c.Name, c.AccountID, c.APIVersion}
	if id > 0 {
		query = `UPDATE meta_connections SET name=$1,account_id=$2,api_version=$3,updated_at=now() WHERE id=$4 RETURNING ` + connectionColumns
		args = append(args, id)
	}
	c, err = scanConnection(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return c, err
	}
	detail, _ := json.Marshal(map[string]any{"connection_id": c.ID, "account_id": c.AccountID})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.connection.save',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return c, err
	}
	return c, tx.Commit(ctx)
}

func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
