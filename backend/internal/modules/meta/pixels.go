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
	ManualEventName  string     `json:"manual_event_name"`
	HasCapiToken     bool       `json:"has_capi_token"`
	TokenExpiresAt   *time.Time `json:"token_expires_at"`
	CredentialStatus string     `json:"credential_status"`
	ValidatedAt      *time.Time `json:"validated_at"`
	LastError        string     `json:"last_error"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Cipher           string     `json:"-"`
}
type PixelInput struct {
	Pixel
	CapiToken      string `json:"capi_token"`
	ClearCapiToken bool   `json:"clear_capi_token"`
}

const pixelColumns = "id,connection_id,name,pixel_id,enabled,pageview_enabled,manual_enabled,manual_event_name,capi_token_cipher,token_expires_at,credential_status,validated_at,last_error,updated_at"

func scanPixel(row pgx.Row) (p Pixel, e error) {
	e = row.Scan(&p.ID, &p.ConnectionID, &p.Name, &p.PixelID, &p.Enabled, &p.PageviewEnabled, &p.ManualEnabled, &p.ManualEventName, &p.Cipher, &p.TokenExpiresAt, &p.CredentialStatus, &p.ValidatedAt, &p.LastError, &p.UpdatedAt)
	p.HasCapiToken = p.Cipher != ""
	p.CredentialStatus = credentialStatus(p.HasCapiToken, p.CredentialStatus, p.TokenExpiresAt)
	return
}
func credentialStatus(configured bool, status string, expires *time.Time) string {
	if !configured {
		return "missing"
	}
	if expires != nil && !expires.After(time.Now()) {
		return "expired"
	}
	if status == "" || status == "missing" || status == "expired" {
		return "unverified"
	}
	return status
}
func (s *Service) Pixel(ctx context.Context, id int64) (Pixel, error) {
	return scanPixel(s.Core.DB.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1", id))
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
	if p.ManualEventName != "WhatsAppConsultClick" && p.ManualEventName != "Contact" {
		return p, errors.New("手动咨询事件只能是 WhatsAppConsultClick 或 Contact")
	}
	if len(in.CapiToken) > 8192 || strings.ContainsAny(in.CapiToken, " \r\n\t") {
		return p, errors.New("回传凭证格式无效")
	}
	if in.ClearCapiToken {
		p.Cipher = ""
		p.ValidatedAt = nil
		p.CredentialStatus = "missing"
		p.LastError = ""
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
	if p.Enabled && (p.Cipher == "" || credentialStatus(true, p.CredentialStatus, p.TokenExpiresAt) == "expired") {
		return p, errors.New("启用 Pixel 前需要未过期的回传凭证")
	}
	args := []any{p.ConnectionID, p.Name, p.PixelID, p.Enabled, p.PageviewEnabled, p.ManualEnabled, p.ManualEventName, p.Cipher, p.TokenExpiresAt, p.CredentialStatus, p.ValidatedAt, p.LastError}
	q := "INSERT INTO meta_pixels(connection_id,name,pixel_id,enabled,pageview_enabled,manual_enabled,manual_event_name,capi_token_cipher,token_expires_at,credential_status,validated_at,last_error) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING " + pixelColumns
	if id > 0 {
		args = append(args, id)
		q = "UPDATE meta_pixels SET connection_id=$1,name=$2,pixel_id=$3,enabled=$4,pageview_enabled=$5,manual_enabled=$6,manual_event_name=$7,capi_token_cipher=$8,token_expires_at=$9,credential_status=$10,validated_at=$11,last_error=$12,updated_at=now() WHERE id=$13 RETURNING " + pixelColumns
	}
	p, e = scanPixel(tx.QueryRow(ctx, q, args...))
	if e != nil {
		return p, e
	}
	// Pixel credentials stay on this Pixel row; account rows only group Pixels.
	detail, _ := json.Marshal(map[string]any{"pixel_record_id": p.ID, "pixel_id": p.PixelID, "connection_id": p.ConnectionID, "enabled": p.Enabled, "credential_changed": in.CapiToken != "" || in.ClearCapiToken})
	if _, e = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail)VALUES($1,'meta.pixel.save',$2)", s.Core.Config.AdminUser, detail); e != nil {
		return p, e
	}
	return p, tx.Commit(ctx)
}
