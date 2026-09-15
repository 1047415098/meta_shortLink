package meta

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand/v2"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

type deliveryResponse struct {
	Received int      `json:"events_received"`
	Messages []string `json:"messages"`
	Trace    string   `json:"fbtrace_id"`
}

func eventBackoff(attempt int, err error) time.Duration {
	if attempt > 8 {
		attempt = 8
	}
	d := time.Duration(1<<uint(attempt))*15*time.Second + time.Duration(rand.IntN(15))*time.Second
	var ge *GraphError
	if errors.As(err, &ge) && ge.RetryAfter > d {
		d = ge.RetryAfter
	}
	if d > 6*time.Hour {
		d = 6 * time.Hour
	}
	return d
}

// One durable claim. A fencing token prevents an expired worker overwriting its replacement.
func (s *Service) ProcessEvent(ctx context.Context) (bool, error) {
	_, e := s.Core.DB.Exec(ctx, `UPDATE meta_events SET status='expired',payload_cipher='',locked_until=NULL,lock_token='',updated_at=now(),last_error='事件超过本站 6 天发送期限，匹配信息已清除' WHERE event_time<=now()-interval '6 days' AND status NOT IN ('succeeded','expired','skipped')`)
	if e != nil {
		return false, e
	}
	lock := runtime.Token()
	var id, cipher, pixel string
	var connectionID int64
	var pixelRecordID *int64
	var count int
	var test bool
	e = s.Core.DB.QueryRow(ctx, `WITH candidate AS (
 SELECT e.id FROM meta_events e JOIN meta_connections c ON c.id=e.connection_id
 WHERE ((e.status IN ('pending','retry') AND e.next_attempt_at<=now()) OR (e.status='processing' AND e.locked_until<now()))
 -- Pixel-level state is authoritative for CAPI. The connection remains only to
 -- provide API version and grouping metadata for the selected Pixel.
 AND ((EXISTS(SELECT 1 FROM meta_pixels p WHERE p.id=e.pixel_record_id AND p.enabled)) OR e.is_test) AND e.event_time>now()-interval '6 days'
 ORDER BY e.next_attempt_at FOR UPDATE OF e SKIP LOCKED LIMIT 1)
 UPDATE meta_events e SET status='processing',attempts=e.attempts+1,retry_count=e.retry_count+1,lock_token=$1,locked_until=now()+interval '2 minutes',updated_at=now()
 FROM candidate WHERE e.id=candidate.id RETURNING e.id,e.connection_id,e.payload_cipher,e.pixel_id,e.retry_count,e.is_test,e.pixel_record_id`, lock).Scan(&id, &connectionID, &cipher, &pixel, &count, &test, &pixelRecordID)
	if errors.Is(e, pgx.ErrNoRows) {
		return false, nil
	}
	if e != nil {
		return false, e
	}
	c, e := s.Connection(ctx, connectionID)
	var response deliveryResponse
	var target Pixel
	if e == nil && pixelRecordID != nil {
		target, e = s.Pixel(ctx, *pixelRecordID)
	}
	if e == nil && pixelRecordID == nil {
		e = errors.New("回传目标缺失")
	}
	if e == nil {
		// Live delivery follows the frozen Pixel target and no longer depends on
		// the legacy account-level CAPI switch.
		if !target.Enabled && !test {
			_, e = s.Core.DB.Exec(ctx, "UPDATE meta_events SET status='pending',locked_until=NULL,lock_token='' WHERE id=$1 AND lock_token=$2", id, lock)
			return true, e
		}
		var token, plain string
		token, e = s.open(target.Cipher, "capi:"+c.AccountID)
		if e == nil {
			plain, e = s.open(cipher, "event:"+id)
		}
		if e == nil {
			var payload eventPayload
			if plain == "" || json.Unmarshal([]byte(plain), &payload) != nil {
				e = errors.New("事件数据已清除或无法读取")
			} else {
				e = s.Client.Post(ctx, c.APIVersion, token, pixel+"/events", payload, &response)
				for i, v := range response.Messages {
					response.Messages[i] = cleanMessage(v, token)
				}
				if e == nil && response.Received != 1 {
					e = errors.New("Meta 未确认接收该事件，请检查事件管理工具中的诊断信息")
				}
			}
		}
	}
	status, message := "succeeded", ""
	next := time.Now()
	if e != nil {
		status, message = "failed", cleanMessage(e.Error())
		if retryable(e) && count < 8 {
			status = "retry"
			next = next.Add(eventBackoff(count, e))
		}
	}
	if pixelRecordID != nil && target.Cipher != "" {
		state := "valid"
		var verified any = time.Now()
		last := ""
		if e != nil {
			state = "error"
			verified = target.ValidatedAt
			last = message
			var ge *GraphError
			if errors.As(e, &ge) && ge.Code == 190 {
				state = "invalid"
			}
		}
		_, _ = s.Core.DB.Exec(ctx, "UPDATE meta_pixels SET credential_status=$3,validated_at=$4,last_error=$5 WHERE id=$1 AND capi_token_cipher=$2", *pixelRecordID, target.Cipher, state, verified, last)
	}
	messages, _ := json.Marshal(response.Messages)
	if string(messages) == "null" {
		messages = []byte("[]")
	}
	// A short cancellation-independent completion window avoids losing acknowledgements during shutdown.
	finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_, saveErr := s.Core.DB.Exec(finish, `UPDATE meta_events SET status=$3,last_error=$4,next_attempt_at=$5,events_received=$6,fbtrace_id=$7,response_messages=$8,locked_until=NULL,lock_token='',payload_cipher=CASE WHEN $3='succeeded' THEN '' ELSE payload_cipher END,updated_at=now() WHERE id=$1 AND lock_token=$2 AND status='processing'`, id, lock, status, message, next, response.Received, runtime.Bounded(response.Trace, 255), messages)
	return true, saveErr
}
