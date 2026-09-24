package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

var errEventLeaseLost = errors.New("TikTok 事件发送租约已失效")

func eventBackoff(attempt int, err error) time.Duration {
	if attempt > 8 {
		attempt = 8
	}
	delay := time.Duration(1<<uint(attempt))*15*time.Second + time.Duration(rand.IntN(15))*time.Second
	var apiError *APIError
	if errors.As(err, &apiError) && apiError.RetryAfter > delay {
		delay = apiError.RetryAfter
	}
	if delay > 6*time.Hour {
		delay = 6 * time.Hour
	}
	return delay
}

func deliveryDecision(attempts int, err error) (string, time.Duration) {
	if err == nil {
		return "accepted", 0
	}
	retryable := false
	var apiError *APIError
	if errors.As(err, &apiError) {
		// TikTok 5xxxx business codes represent service-side processing failures;
		// request/credential 4xxxx failures require operator correction.
		retryable = apiError.Temporary || (apiError.BusinessCode >= 50000 && apiError.BusinessCode < 60000)
	}
	if retryable && attempts < 8 {
		return "retry", eventBackoff(attempts, err)
	}
	return "failed", 0
}

type claimedEvent struct {
	ID, PayloadCipher string
	ConnectionID      int64
	PixelRecordID     int64
	Attempts          int
	LockToken         string
}

func (s *Service) claimEvent(ctx context.Context) (claimedEvent, bool, error) {
	_, err := s.Core.DB.Exec(ctx, `UPDATE tiktok_events SET status='failed',payload_cipher='',locked_until=NULL,lock_token='',
		last_error='事件超过 24 小时安全重试期限，已清除发送数据',updated_at=now()
		WHERE event_time<=now()-interval '24 hours' AND status IN ('pending','retry','sending')`)
	if err != nil {
		return claimedEvent{}, false, err
	}
	item := claimedEvent{LockToken: runtime.Token()}
	err = s.Core.DB.QueryRow(ctx, `WITH candidate AS (
		SELECT e.id FROM tiktok_events e
		JOIN tiktok_pixels p ON p.id=e.pixel_record_id AND p.enabled
		JOIN tiktok_connections c ON c.id=e.connection_id AND c.enabled
		WHERE ((e.status IN ('pending','retry') AND e.next_attempt_at<=now()) OR
			(e.status='sending' AND e.locked_until<now()))
		AND e.event_time>now()-interval '24 hours'
		ORDER BY e.next_attempt_at,e.created_at FOR UPDATE OF e SKIP LOCKED LIMIT 1)
		UPDATE tiktok_events e SET status='sending',attempts=e.attempts+1,lock_token=$1,
			locked_until=now()+interval '2 minutes',updated_at=now()
		FROM candidate WHERE e.id=candidate.id
		RETURNING e.id,e.connection_id,e.pixel_record_id,e.payload_cipher,e.attempts`, item.LockToken).
		Scan(&item.ID, &item.ConnectionID, &item.PixelRecordID, &item.PayloadCipher, &item.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return claimedEvent{}, false, nil
	}
	return item, err == nil, err
}

func (s *Service) pauseClaim(ctx context.Context, item claimedEvent) error {
	// A configuration disabled after claiming must not consume an attempt.
	tag, err := s.Core.DB.Exec(ctx, `UPDATE tiktok_events SET status='pending',attempts=GREATEST(attempts-1,0),
		locked_until=NULL,lock_token='',updated_at=now() WHERE id=$1 AND status='sending' AND lock_token=$2`, item.ID, item.LockToken)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errEventLeaseLost
	}
	return nil
}

// ProcessEvent claims and finishes at most one durable event. The lock token is
// checked on every update so a worker with an expired lease cannot overwrite a replacement.
func (s *Service) ProcessEvent(ctx context.Context) (bool, error) {
	item, found, err := s.claimEvent(ctx)
	if err != nil || !found {
		return false, err
	}
	connection, err := s.Connection(ctx, item.ConnectionID)
	pixel := Pixel{}
	if err == nil {
		pixel, err = s.Pixel(ctx, item.PixelRecordID)
	}
	if err == nil && (!connection.Enabled || !pixel.Enabled) {
		return true, s.pauseClaim(ctx, item)
	}
	var token string
	var payload EventRequest
	if err == nil {
		token, err = s.open(connection.cipher, "tiktok:token:"+formatID(connection.ID))
	}
	if err == nil {
		var plain string
		plain, err = s.open(item.PayloadCipher, "tiktok:event:"+item.ID)
		if err == nil && (plain == "" || json.Unmarshal([]byte(plain), &payload) != nil) {
			err = errors.New("TikTok 事件数据已清除或无法读取")
		}
	}
	response := DeliveryResponse{}
	if err == nil {
		response, err = s.Client.Post(ctx, token, payload)
	}
	status, delay := deliveryDecision(item.Attempts, err)
	nextAttempt := time.Now().Add(delay)
	httpStatus, businessCode, requestID := 0, response.Code, response.RequestID
	var apiError *APIError
	if errors.As(err, &apiError) {
		httpStatus, businessCode, requestID = apiError.HTTPStatus, apiError.BusinessCode, apiError.RequestID
	}
	message := ""
	if err != nil {
		message = cleanMessage(err.Error(), token, item.ID, item.PayloadCipher)
	}
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	tag, saveErr := s.Core.DB.Exec(finish, `UPDATE tiktok_events SET status=$3,next_attempt_at=$4,http_status=$5,
		business_code=$6,request_id=$7,last_error=$8,locked_until=NULL,lock_token='',
		payload_cipher=CASE WHEN $3='accepted' THEN '' ELSE payload_cipher END,updated_at=now()
		WHERE id=$1 AND status='sending' AND lock_token=$2`, item.ID, item.LockToken, status, nextAttempt,
		httpStatus, businessCode, runtime.Bounded(requestID, 255), message)
	if saveErr != nil {
		return true, saveErr
	}
	if tag.RowsAffected() != 1 {
		return true, errEventLeaseLost
	}
	return true, nil
}

func formatID(id int64) string {
	// Keep the numeric connection identifier stable inside the encryption purpose.
	return strconv.FormatInt(id, 10)
}
