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
	// Ignore legacy or forged client versions; all accounts use the system
	// version verified by the current CAPI implementation.
	c.APIVersion = GraphAPIVersion

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

func (s *Service) DeleteConnection(ctx context.Context, id int64) error {
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Accounts are grouping records for Pixels and all historical Meta data.
	// Only a completely unused account can be removed without changing reports.
	connection, err := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM meta_connections WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	var pixels, links, visits, events, jobs, entities, insights, coverage int64
	err = tx.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM meta_pixels WHERE connection_id=$1),
		(SELECT count(*) FROM short_links WHERE meta_connection_id=$1),
		(SELECT count(*) FROM click_events WHERE meta_connection_id=$1),
		(SELECT count(*) FROM meta_events WHERE connection_id=$1),
		(SELECT count(*) FROM meta_sync_jobs WHERE connection_id=$1),
		(SELECT count(*) FROM meta_ad_entities WHERE connection_id=$1),
		(SELECT count(*) FROM meta_insights_daily WHERE connection_id=$1),
		(SELECT count(*) FROM meta_sync_coverage WHERE connection_id=$1)`, id).Scan(&pixels, &links, &visits, &events, &jobs, &entities, &insights, &coverage)
	if err != nil {
		return err
	}
	if pixels > 0 {
		return &ConfigInUseError{Message: "该 Meta 帐号下仍有 Pixel，请先处理这些 Pixel"}
	}
	if links > 0 {
		return &ConfigInUseError{Message: "该 Meta 帐号仍被短链接使用，请先解除绑定"}
	}
	if visits+events+jobs+entities+insights+coverage > 0 {
		return &ConfigInUseError{Message: "该 Meta 帐号已产生历史访问、回传或同步记录，为保留历史数据不能删除"}
	}
	if tag, deleteErr := tx.Exec(ctx, "DELETE FROM meta_connections WHERE id=$1", id); deleteErr != nil {
		return deleteErr
	} else if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	detail, _ := json.Marshal(map[string]any{"connection_id": connection.ID, "account_id": connection.AccountID, "name": connection.Name})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.connection.delete',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
