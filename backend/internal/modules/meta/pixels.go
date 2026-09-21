package meta

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type Pixel struct {
	ID               int64      `json:"id"`
	ConnectionID     int64      `json:"connection_id"`
	Name             string     `json:"name"`
	PixelID          string     `json:"pixel_id"`
	Enabled          bool       `json:"enabled"`
	PageviewEnabled  bool       `json:"pageview_enabled"`
	ManualEnabled    bool       `json:"manual_enabled"`
	AutoEnabled      bool       `json:"auto_enabled"`
	ManualEventName  string     `json:"manual_event_name"`
	HasCapiToken     bool       `json:"has_capi_token"`
	CredentialStatus string     `json:"credential_status"`
	ValidatedAt      *time.Time `json:"validated_at"`
	LastError        string     `json:"last_error"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Cipher           string     `json:"-"`
}
type PixelInput struct {
	Pixel
	CapiToken string `json:"capi_token"`
}

const pixelColumns = "id,connection_id,name,pixel_id,enabled,pageview_enabled,manual_enabled,auto_enabled,manual_event_name,capi_token_cipher,credential_status,validated_at,last_error,updated_at"

func scanPixel(row pgx.Row) (p Pixel, e error) {
	e = row.Scan(&p.ID, &p.ConnectionID, &p.Name, &p.PixelID, &p.Enabled, &p.PageviewEnabled, &p.ManualEnabled, &p.AutoEnabled, &p.ManualEventName, &p.Cipher, &p.CredentialStatus, &p.ValidatedAt, &p.LastError, &p.UpdatedAt)
	p.HasCapiToken = p.Cipher != ""
	p.CredentialStatus = credentialStatus(p.HasCapiToken, p.CredentialStatus)
	return
}
func credentialStatus(configured bool, status string) string {
	if !configured {
		return "missing"
	}
	// Legacy expiry markers no longer block a non-expiring CAPI credential.
	if status == "" || status == "missing" || status == "expired" {
		return "unverified"
	}
	return status
}
func (s *Service) Pixel(ctx context.Context, id int64) (Pixel, error) {
	return scanPixel(s.Core.DB.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1", id))
}
func (s *Service) PixelCredential(ctx context.Context, id int64) (string, error) {
	var cipher, accountID string
	// Resolve the account at read time because it is part of the authenticated
	// encryption context and Pixel ownership cannot be changed after creation.
	e := s.Core.DB.QueryRow(ctx, `SELECT p.capi_token_cipher,c.account_id
		FROM meta_pixels p JOIN meta_connections c ON c.id=p.connection_id
		WHERE p.id=$1`, id).Scan(&cipher, &accountID)
	if e != nil {
		return "", e
	}
	return s.open(cipher, "capi:"+accountID)
}
func (s *Service) Pixels(ctx context.Context, id int64) ([]Pixel, error) {
	rows, e := s.Core.DB.Query(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE ($1::bigint=0 OR connection_id=$1) ORDER BY connection_id,id", id)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Pixel{}
	for rows.Next() {
		p, e := scanPixel(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
func (s *Service) SavePixel(ctx context.Context, in PixelInput, id int64) (Pixel, error) {
	p := in.Pixel
	p.ID = id
	p.Name = strings.TrimSpace(p.Name)
	// Manual consultations always use the one standard conversion selected for
	// the WhatsApp funnel; client-supplied legacy names are ignored.
	p.ManualEventName = EventName
	tx, e := s.Core.DB.Begin(ctx)
	if e != nil {
		return p, e
	}
	defer tx.Rollback(ctx)
	c, e := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM meta_connections WHERE id=$1 FOR UPDATE", p.ConnectionID))
	if e != nil {
		return p, e
	}
	var old Pixel
	if id > 0 {
		old, e = scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1 FOR UPDATE", id))
		if e != nil {
			return p, e
		}
		if old.ConnectionID != p.ConnectionID || old.PixelID != p.PixelID {
			return p, errors.New("Pixel 编号及所属账户不可更改")
		}
		p.Cipher = old.Cipher
		p.ValidatedAt = old.ValidatedAt
		p.CredentialStatus = old.CredentialStatus
		p.LastError = old.LastError
	} else {
		p.Cipher = ""
		p.ValidatedAt = nil
		p.CredentialStatus = "unverified"
		p.LastError = ""
	}
	if p.Name == "" || len(p.Name) > 120 || !idPattern.MatchString(p.PixelID) {
		return p, errors.New("请填写名称和有效 Pixel 数字编号")
	}
	if len(in.CapiToken) > 8192 || strings.ContainsAny(in.CapiToken, " \r\n\t") {
		return p, errors.New("回传凭证格式无效")
	}
	if in.CapiToken != "" {
		p.Cipher, e = s.seal(in.CapiToken, "capi:"+c.AccountID)
		if e != nil {
			return p, e
		}
		p.ValidatedAt = nil
		p.CredentialStatus = "unverified"
		p.LastError = ""
	}
	// Every Pixel keeps one delivery credential even while paused. The API only
	// supports preserving or replacing it, so a Pixel can never be saved empty.
	if p.Cipher == "" {
		return p, errors.New("请填写当前 Pixel 的 CAPI Token")
	}
	args := []any{p.ConnectionID, p.Name, p.PixelID, p.Enabled, p.PageviewEnabled, p.ManualEnabled, p.AutoEnabled, p.ManualEventName, p.Cipher, p.CredentialStatus, p.ValidatedAt, p.LastError}
	q := "INSERT INTO meta_pixels(connection_id,name,pixel_id,enabled,pageview_enabled,manual_enabled,auto_enabled,manual_event_name,capi_token_cipher,credential_status,validated_at,last_error) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING " + pixelColumns
	if id > 0 {
		args = append(args, id)
		q = "UPDATE meta_pixels SET connection_id=$1,name=$2,pixel_id=$3,enabled=$4,pageview_enabled=$5,manual_enabled=$6,auto_enabled=$7,manual_event_name=$8,capi_token_cipher=$9,credential_status=$10,validated_at=$11,last_error=$12,updated_at=now() WHERE id=$13 RETURNING " + pixelColumns
	}
	p, e = scanPixel(tx.QueryRow(ctx, q, args...))
	if e != nil {
		return p, e
	}
	// Pixel credentials stay on this Pixel row; account rows only group Pixels.
	detail, _ := json.Marshal(map[string]any{"pixel_record_id": p.ID, "pixel_id": p.PixelID, "connection_id": p.ConnectionID, "enabled": p.Enabled, "credential_changed": in.CapiToken != ""})
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.pixel.save',$2)", s.Core.Config.AdminUser, detail); e != nil {
		return p, e
	}
	return p, tx.Commit(ctx)
}

func (s *Service) DeletePixel(ctx context.Context, id int64) error {
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Lock the secret-bearing row while checking every business reference so a
	// delete can never silently rewrite link attribution or historical reports.
	pixel, err := scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	var links, visits, events int64
	err = tx.QueryRow(ctx, `SELECT
		(SELECT count(*) FROM short_links WHERE meta_pixel_id=$1),
		(SELECT count(*) FROM click_events WHERE meta_pixel_id=$1),
		(SELECT count(*) FROM meta_events WHERE pixel_record_id=$1)`, id).Scan(&links, &visits, &events)
	if err != nil {
		return err
	}
	if links > 0 {
		return &ConfigInUseError{Message: "该 Pixel 仍被短链接使用，请先将相关短链接绑定到其他 Pixel"}
	}
	if visits+events > 0 {
		return &ConfigInUseError{Message: "该 Pixel 已产生历史访问或回传记录，为保留统计记录不能删除；可以将其停用"}
	}
	if tag, deleteErr := tx.Exec(ctx, "DELETE FROM meta_pixels WHERE id=$1", id); deleteErr != nil {
		return deleteErr
	} else if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	// Audit identifiers only; the deleted plaintext/encrypted CAPI token is
	// deliberately absent from the permanent audit trail.
	detail, _ := json.Marshal(map[string]any{"pixel_record_id": pixel.ID, "pixel_id": pixel.PixelID, "connection_id": pixel.ConnectionID, "name": pixel.Name})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.pixel.delete',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
