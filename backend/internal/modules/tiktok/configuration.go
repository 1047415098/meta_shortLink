package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

var pixelCodePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{5,64}$`)

type ConfigInUseError struct{ Message string }

func (e *ConfigInUseError) Error() string { return e.Message }

type Connection struct {
	ID               int64      `json:"id"`
	Name             string     `json:"name"`
	Enabled          bool       `json:"enabled"`
	HasAccessToken   bool       `json:"has_access_token"`
	CredentialStatus string     `json:"credential_status"`
	ValidatedAt      *time.Time `json:"validated_at,omitempty"`
	LastError        string     `json:"last_error"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	cipher           string
}

type ConnectionInput struct {
	Name        string `json:"name"`
	Enabled     bool   `json:"enabled"`
	AccessToken string `json:"access_token"`
}

type Pixel struct {
	ID            int64      `json:"id"`
	ConnectionID  int64      `json:"connection_id"`
	Name          string     `json:"name"`
	PixelCode     string     `json:"pixel_code"`
	TestEventCode string     `json:"test_event_code"`
	Enabled       bool       `json:"enabled"`
	ValidatedAt   *time.Time `json:"validated_at,omitempty"`
	LastError     string     `json:"last_error"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type PixelInput struct {
	ConnectionID  int64  `json:"connection_id"`
	Name          string `json:"name"`
	PixelCode     string `json:"pixel_code"`
	TestEventCode string `json:"test_event_code"`
	Enabled       bool   `json:"enabled"`
}

const connectionColumns = "id,name,enabled,access_token_cipher,credential_status,validated_at,last_error,created_at,updated_at"
const pixelColumns = "id,connection_id,name,pixel_code,test_event_code,enabled,validated_at,last_error,created_at,updated_at"

func scanConnection(row pgx.Row) (Connection, error) {
	var item Connection
	err := row.Scan(&item.ID, &item.Name, &item.Enabled, &item.cipher, &item.CredentialStatus, &item.ValidatedAt, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	item.HasAccessToken = item.cipher != ""
	return item, err
}

func scanPixel(row pgx.Row) (Pixel, error) {
	var item Pixel
	err := row.Scan(&item.ID, &item.ConnectionID, &item.Name, &item.PixelCode, &item.TestEventCode, &item.Enabled, &item.ValidatedAt, &item.LastError, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}

func validateConnectionInput(input ConnectionInput, creating bool) error {
	name := strings.TrimSpace(input.Name)
	if name == "" || len([]rune(name)) > 120 {
		return errors.New("请填写 TikTok 凭证名称（最多 120 个字符）")
	}
	token := input.AccessToken
	if creating && token == "" {
		return errors.New("请填写 TikTok Access Token")
	}
	if len(token) > 8192 || strings.ContainsAny(token, " \r\n\t") {
		return errors.New("TikTok Access Token 格式无效")
	}
	return nil
}

func validatePixelInput(input PixelInput) error {
	if input.ConnectionID < 1 || strings.TrimSpace(input.Name) == "" || len([]rune(strings.TrimSpace(input.Name))) > 120 {
		return errors.New("请选择凭证并填写 Pixel 名称")
	}
	if !pixelCodePattern.MatchString(strings.TrimSpace(input.PixelCode)) {
		return errors.New("TikTok Pixel Code 格式无效")
	}
	if len(input.TestEventCode) > 120 || strings.ContainsAny(input.TestEventCode, "\r\n\t") {
		return errors.New("TikTok Test Event Code 格式无效")
	}
	return nil
}

func (s *Service) Connection(ctx context.Context, id int64) (Connection, error) {
	return scanConnection(s.Core.DB.QueryRow(ctx, "SELECT "+connectionColumns+" FROM tiktok_connections WHERE id=$1", id))
}

func (s *Service) Connections(ctx context.Context) ([]Connection, error) {
	rows, err := s.Core.DB.Query(ctx, "SELECT "+connectionColumns+" FROM tiktok_connections ORDER BY id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Connection{}
	for rows.Next() {
		item, scanErr := scanConnection(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SaveConnection(ctx context.Context, input ConnectionInput, id int64) (Connection, error) {
	input.Name = strings.TrimSpace(input.Name)
	if err := validateConnectionInput(input, id == 0); err != nil {
		return Connection{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return Connection{}, err
	}
	defer tx.Rollback(ctx)

	var item Connection
	if id == 0 {
		if err = tx.QueryRow(ctx, `INSERT INTO tiktok_connections(name,access_token_cipher,enabled) VALUES($1,'',$2) RETURNING id`, input.Name, input.Enabled).Scan(&id); err != nil {
			return item, err
		}
		ciphertext, sealErr := s.sealToken(input.AccessToken, id)
		if sealErr != nil {
			return item, sealErr
		}
		item, err = scanConnection(tx.QueryRow(ctx, `UPDATE tiktok_connections SET access_token_cipher=$2 WHERE id=$1 RETURNING `+connectionColumns, id, ciphertext))
	} else {
		current, loadErr := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM tiktok_connections WHERE id=$1 FOR UPDATE", id))
		if loadErr != nil {
			return item, loadErr
		}
		ciphertext := current.cipher
		status, validatedAt, lastError := current.CredentialStatus, current.ValidatedAt, current.LastError
		if input.AccessToken != "" {
			ciphertext, err = s.sealToken(input.AccessToken, id)
			if err != nil {
				return item, err
			}
			status, validatedAt, lastError = "unverified", nil, ""
		}
		item, err = scanConnection(tx.QueryRow(ctx, `UPDATE tiktok_connections SET name=$2,enabled=$3,access_token_cipher=$4,credential_status=$5,validated_at=$6,last_error=$7,updated_at=now() WHERE id=$1 RETURNING `+connectionColumns, id, input.Name, input.Enabled, ciphertext, status, validatedAt, lastError))
	}
	if err != nil {
		return item, err
	}
	detail, _ := json.Marshal(map[string]any{"connection_id": item.ID, "name": item.Name, "enabled": item.Enabled, "credential_changed": input.AccessToken != ""})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'tiktok.connection.save',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (s *Service) DeleteConnection(ctx context.Context, id int64) error {
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM tiktok_connections WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	var pixels, events int64
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM tiktok_pixels WHERE connection_id=$1),(SELECT count(*) FROM tiktok_events WHERE connection_id=$1)`, id).Scan(&pixels, &events); err != nil {
		return err
	}
	if pixels > 0 {
		return &ConfigInUseError{Message: "该 TikTok 凭证仍有关联 Pixel，请先处理这些 Pixel"}
	}
	if events > 0 {
		return &ConfigInUseError{Message: "该 TikTok 凭证已有事件记录，为保留历史数据不能删除；可以将其停用"}
	}
	if tag, deleteErr := tx.Exec(ctx, "DELETE FROM tiktok_connections WHERE id=$1", id); deleteErr != nil {
		return deleteErr
	} else if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	detail, _ := json.Marshal(map[string]any{"connection_id": item.ID, "name": item.Name})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'tiktok.connection.delete',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) Pixel(ctx context.Context, id int64) (Pixel, error) {
	return scanPixel(s.Core.DB.QueryRow(ctx, "SELECT "+pixelColumns+" FROM tiktok_pixels WHERE id=$1", id))
}

func (s *Service) Pixels(ctx context.Context, connectionID int64) ([]Pixel, error) {
	rows, err := s.Core.DB.Query(ctx, "SELECT "+pixelColumns+" FROM tiktok_pixels WHERE ($1::bigint=0 OR connection_id=$1) ORDER BY connection_id,id DESC", connectionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []Pixel{}
	for rows.Next() {
		item, scanErr := scanPixel(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Service) SavePixel(ctx context.Context, input PixelInput, id int64) (Pixel, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.PixelCode = strings.TrimSpace(input.PixelCode)
	input.TestEventCode = strings.TrimSpace(input.TestEventCode)
	if err := validatePixelInput(input); err != nil {
		return Pixel{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return Pixel{}, err
	}
	defer tx.Rollback(ctx)
	if _, err = scanConnection(tx.QueryRow(ctx, "SELECT "+connectionColumns+" FROM tiktok_connections WHERE id=$1 FOR UPDATE", input.ConnectionID)); err != nil {
		return Pixel{}, err
	}
	if id > 0 {
		old, loadErr := scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM tiktok_pixels WHERE id=$1 FOR UPDATE", id))
		if loadErr != nil {
			return Pixel{}, loadErr
		}
		if old.ConnectionID != input.ConnectionID || old.PixelCode != input.PixelCode {
			return Pixel{}, errors.New("Pixel Code 和所属凭证创建后不可修改，请新建 Pixel")
		}
	}
	query := `INSERT INTO tiktok_pixels(connection_id,name,pixel_code,test_event_code,enabled) VALUES($1,$2,$3,$4,$5) RETURNING ` + pixelColumns
	args := []any{input.ConnectionID, input.Name, input.PixelCode, input.TestEventCode, input.Enabled}
	if id > 0 {
		query = `UPDATE tiktok_pixels SET name=$2,test_event_code=$4,enabled=$5,updated_at=now() WHERE id=$6 AND connection_id=$1 AND pixel_code=$3 RETURNING ` + pixelColumns
		args = append(args, id)
	}
	item, err := scanPixel(tx.QueryRow(ctx, query, args...))
	if err != nil {
		return item, err
	}
	detail, _ := json.Marshal(map[string]any{"pixel_record_id": item.ID, "connection_id": item.ConnectionID, "pixel_code": item.PixelCode, "enabled": item.Enabled})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'tiktok.pixel.save',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return item, err
	}
	return item, tx.Commit(ctx)
}

func (s *Service) DeletePixel(ctx context.Context, id int64) error {
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	item, err := scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM tiktok_pixels WHERE id=$1 FOR UPDATE", id))
	if err != nil {
		return err
	}
	var links, visits, events int64
	if err = tx.QueryRow(ctx, `SELECT (SELECT count(*) FROM short_links WHERE tiktok_pixel_id=$1),(SELECT count(*) FROM click_events WHERE tiktok_pixel_id=$1),(SELECT count(*) FROM tiktok_events WHERE pixel_record_id=$1)`, id).Scan(&links, &visits, &events); err != nil {
		return err
	}
	if links > 0 {
		return &ConfigInUseError{Message: "该 TikTok Pixel 仍被小说投放链接使用，请先停用相关链接"}
	}
	if visits+events > 0 {
		return &ConfigInUseError{Message: "该 TikTok Pixel 已产生访问或事件记录，为保留历史数据不能删除；可以将其停用"}
	}
	if tag, deleteErr := tx.Exec(ctx, "DELETE FROM tiktok_pixels WHERE id=$1", id); deleteErr != nil {
		return deleteErr
	} else if tag.RowsAffected() != 1 {
		return pgx.ErrNoRows
	}
	detail, _ := json.Marshal(map[string]any{"pixel_record_id": item.ID, "connection_id": item.ConnectionID, "pixel_code": item.PixelCode, "name": item.Name})
	if _, err = tx.Exec(ctx, "INSERT INTO audit_logs(actor,action,detail) VALUES($1,'tiktok.pixel.delete',$2)", s.Core.Config.AdminUser, detail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func buildPixelTestRequest(pixel Pixel, eventID string, at time.Time, publicURL string) EventRequest {
	return EventRequest{
		EventSource:   "web",
		EventSourceID: pixel.PixelCode,
		TestEventCode: pixel.TestEventCode,
		Data: []EventData{{
			Event:     "PageView",
			EventTime: at.Unix(),
			EventID:   eventID,
			Page:      PageContext{URL: strings.TrimRight(publicURL, "/") + "/admin/tiktok/pixels"},
		}},
	}
}

func (s *Service) TestPixel(ctx context.Context, id int64) (DeliveryResponse, error) {
	pixel, err := s.Pixel(ctx, id)
	if err != nil {
		return DeliveryResponse{}, err
	}
	if pixel.TestEventCode == "" {
		return DeliveryResponse{}, errors.New("请先填写 TikTok Test Event Code")
	}
	connection, err := s.Connection(ctx, pixel.ConnectionID)
	if err != nil {
		return DeliveryResponse{}, err
	}
	token, err := s.open(connection.cipher, fmt.Sprintf("tiktok:token:%d", connection.ID))
	if err != nil {
		return DeliveryResponse{}, err
	}
	eventID := "tiktok_test_" + runtime.Token()
	response, sendErr := s.Client.Post(ctx, token, buildPixelTestRequest(pixel, eventID, time.Now(), s.Core.Config.PublicURL))
	status, message := "valid", ""
	var validatedAt any = time.Now()
	if sendErr != nil {
		status, message, validatedAt = "error", cleanMessage(sendErr.Error(), token, eventID), connection.ValidatedAt
		var apiError *APIError
		if errors.As(sendErr, &apiError) && (apiError.HTTPStatus == httpStatusUnauthorized || apiError.HTTPStatus == httpStatusForbidden) {
			status = "invalid"
		}
	}
	// Persist only the redacted diagnostic and validation timestamp.
	_, _ = s.Core.DB.Exec(ctx, `UPDATE tiktok_pixels SET validated_at=$2,last_error=$3,updated_at=now() WHERE id=$1`, pixel.ID, validatedAt, message)
	_, _ = s.Core.DB.Exec(ctx, `UPDATE tiktok_connections SET credential_status=$2,validated_at=$3,last_error=$4,updated_at=now() WHERE id=$1`, connection.ID, status, validatedAt, message)
	return response, sendErr
}

const (
	httpStatusUnauthorized = 401
	httpStatusForbidden    = 403
)

func IsNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
