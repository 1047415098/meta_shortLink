package meta

import (
	"context"
	"errors"
	"fmt"
)

// A bounded transaction rewraps only encrypted values, never credentials supplied by a browser.
func (s *Service) Rewrap(ctx context.Context) (int, error) {
	if s.encryptionKeyID() == "legacy" {
		return 0, errors.New("请先在服务端配置 META_ENCRYPTION_KEYS 和 META_ENCRYPTION_KEY_ID，重启后再轮换")
	}
	if _, e := s.aeadFor(s.encryptionKeyID()); e != nil {
		return 0, e
	}
	tx, e := s.Core.DB.Begin(ctx)
	if e != nil {
		return 0, e
	}
	defer tx.Rollback(ctx)
	n := 0
	prefix := "v2:" + s.encryptionKeyID() + ":"
	// Rewrap only credentials and payloads still used by CAPI delivery.
	for _, v := range []struct{ table, column, context string }{{"meta_pixels", "capi_token_cipher", "'capi:'||(SELECT account_id FROM meta_connections c WHERE c.id=meta_pixels.connection_id)"}, {"meta_events", "payload_cipher", "'event:'||id"}} {
		rows, e := tx.Query(ctx, fmt.Sprintf("SELECT id::text,%s,%s FROM %s WHERE %s<>'' AND NOT starts_with(%s,$1) ORDER BY id LIMIT 500 FOR UPDATE", v.column, v.context, v.table, v.column, v.column), prefix)
		if e != nil {
			return 0, e
		}
		type item struct{ id, cipher, purpose string }
		items := []item{}
		for rows.Next() {
			var i item
			if e = rows.Scan(&i.id, &i.cipher, &i.purpose); e != nil {
				rows.Close()
				return 0, e
			}
			items = append(items, i)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return 0, e
		}
		for _, i := range items {
			plain, e := s.open(i.cipher, i.purpose)
			if e != nil {
				return 0, e
			}
			cipher, e := s.seal(plain, i.purpose)
			if e != nil {
				return 0, e
			}
			if _, e = tx.Exec(ctx, fmt.Sprintf("UPDATE %s SET %s=$2 WHERE id::text=$1", v.table, v.column), i.id, cipher); e != nil {
				return 0, e
			}
			n++
		}
	}
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.credentials.rewrap',jsonb_build_object('updated',$2::int,'key_id',$3::text))", s.Core.Config.AdminUser, n, s.encryptionKeyID()); e != nil {
		return 0, e
	}
	return n, tx.Commit(ctx)
}
