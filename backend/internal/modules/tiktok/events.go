package tiktok

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type BrowserEvent struct {
	Name    string `json:"name"`
	EventID string `json:"event_id"`
}

type frozenVisit struct {
	Platform     string
	PixelID      int64
	PixelCode    string
	ConnectionID int64
	NovelID      int64
	TTCLID       string
	TTP          string
	Context      VisitContext
}

type EventRecord struct {
	ID            string    `json:"id"`
	VisitID       string    `json:"visit_id"`
	LinkID        int64     `json:"link_id"`
	NovelID       *int64    `json:"novel_id,omitempty"`
	AudioNovelID  *int64    `json:"audio_novel_id,omitempty"`
	PixelRecordID int64     `json:"pixel_record_id"`
	PixelName     string    `json:"pixel_name"`
	PixelCode     string    `json:"pixel_code"`
	EventName     string    `json:"event_name"`
	EventTime     time.Time `json:"event_time"`
	Status        string    `json:"status"`
	Attempts      int       `json:"attempts"`
	HTTPStatus    int       `json:"http_status"`
	BusinessCode  int64     `json:"business_code"`
	RequestID     string    `json:"request_id"`
	LastError     string    `json:"last_error"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EventFilters struct {
	Page          int
	Status        string
	EventName     string
	PixelRecordID int64
	LinkID        int64
}

func VisitEventID(visitID, eventName string) string {
	suffix := "qualified"
	if eventName == "StartReading" {
		suffix = "start"
	}
	return "novel_" + visitID + "_" + suffix
}

type AudioEventInput struct {
	LinkID  int64
	EventAt time.Time
}

func recordAudioDiagnosticFailure(ctx context.Context, tx pgx.Tx, visit frozenVisit, visitID, eventName, eventID string, input AudioEventInput, audioNovelID int64, reason string) (bool, error) {
	// A delivery blocker must remain visible to operators without returning a
	// browser event or storing the private attribution snapshot in diagnostics.
	tag, err := tx.Exec(ctx, `INSERT INTO tiktok_events(
		id,visit_id,link_id,audio_novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,
		payload_cipher,status,attempts,last_error)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'','failed',1,$11)
		ON CONFLICT(pixel_code,event_name,event_id) DO NOTHING`, eventID, visitID, input.LinkID, audioNovelID,
		visit.ConnectionID, visit.PixelID, visit.PixelCode, eventName, eventID, input.EventAt, reason)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// EnqueueAudioEvent persists the selected TikTok target with the same event ID
// used by the browser Pixel. It returns a browser event only while the current
// server-delivery configuration can accept the matching event as well.
func (s *Service) EnqueueAudioEvent(ctx context.Context, tx pgx.Tx, visitID, eventName, eventID string, input AudioEventInput) (BrowserEvent, bool, error) {
	if eventName != "StartListening" && eventName != "ViewContent" {
		return BrowserEvent{}, false, errors.New("不支持的 TikTok 语音事件")
	}
	if input.EventAt.IsZero() {
		return BrowserEvent{}, false, errors.New("TikTok 语音事件时间缺失")
	}
	var visit frozenVisit
	var audioNovelID int64
	var title, contextCipher, classification, method, surface string
	var pixelEnabled, connectionEnabled, hasCredential bool
	var credentialStatus string
	err := tx.QueryRow(ctx, `SELECT e.ad_platform,COALESCE(e.tiktok_pixel_id,0),e.tiktok_pixel_code,
		COALESCE(p.connection_id,0),COALESCE(e.audio_novel_id,0),e.audio_novel_title,
		e.tiktok_ttclid,e.tiktok_ttp,e.tiktok_context_cipher,e.classification,e.method,e.surface,
		COALESCE(p.enabled,false),COALESCE(c.enabled,false),COALESCE(c.access_token_cipher,'')<>'',
		COALESCE(c.credential_status,'')
		FROM click_events e
		LEFT JOIN tiktok_pixels p ON p.id=e.tiktok_pixel_id
		LEFT JOIN tiktok_connections c ON c.id=p.connection_id
		WHERE e.id=$1 AND e.link_id=$2`, visitID, input.LinkID).
		Scan(&visit.Platform, &visit.PixelID, &visit.PixelCode, &visit.ConnectionID, &audioNovelID, &title,
			&visit.TTCLID, &visit.TTP, &contextCipher, &classification, &method, &surface,
			&pixelEnabled, &connectionEnabled, &hasCredential, &credentialStatus)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	if visit.Platform != "tiktok" {
		return BrowserEvent{}, false, nil
	}
	if classification != "normal" || method != "GET" || surface != "audio_novel" {
		return BrowserEvent{}, false, nil
	}
	if visit.PixelID < 1 || visit.PixelCode == "" || visit.ConnectionID < 1 || audioNovelID < 1 {
		// Historical or damaged bindings must never make the player unusable.
		return BrowserEvent{}, false, nil
	}
	// The page may stay open while an operator disables TikTok delivery. Recheck
	// the live target before authorizing the matching browser ttq event.
	blockedReason := ""
	switch {
	case !s.Core.Config.TikTokEnabled:
		blockedReason = "TikTok 服务器回传总开关未启用"
	case !pixelEnabled:
		blockedReason = "TikTok Pixel 已停用"
	case !connectionEnabled:
		blockedReason = "TikTok 凭证已停用"
	case !hasCredential:
		blockedReason = "TikTok Access Token 未配置"
	case credentialStatus == "invalid":
		blockedReason = "TikTok Access Token 无效"
	case credentialStatus == "error":
		blockedReason = "TikTok Access Token 状态异常"
	}
	if blockedReason != "" {
		inserted, recordErr := recordAudioDiagnosticFailure(ctx, tx, visit, visitID, eventName, eventID, input, audioNovelID, blockedReason)
		return BrowserEvent{}, inserted, recordErr
	}
	if contextCipher == "" {
		inserted, err := recordAudioDiagnosticFailure(ctx, tx, visit, visitID, eventName, eventID, input, audioNovelID, "TikTok 访问归因快照不可用")
		return BrowserEvent{}, inserted, err
	}
	plain, err := s.open(contextCipher, "tiktok:visit:"+visitID)
	if err != nil {
		inserted, recordErr := recordAudioDiagnosticFailure(ctx, tx, visit, visitID, eventName, eventID, input, audioNovelID, "TikTok 访问归因快照无法解密")
		return BrowserEvent{}, inserted, recordErr
	}
	if err = json.Unmarshal([]byte(plain), &visit.Context); err != nil {
		inserted, recordErr := recordAudioDiagnosticFailure(ctx, tx, visit, visitID, eventName, eventID, input, audioNovelID, "TikTok 访问归因快照无法读取")
		return BrowserEvent{}, inserted, recordErr
	}
	payload := EventRequest{
		EventSource: "web", EventSourceID: visit.PixelCode,
		Data: []EventData{{
			Event: eventName, EventTime: input.EventAt.Unix(), EventID: eventID,
			User: UserContext{TTCLID: visit.TTCLID, TTP: visit.TTP, IP: visit.Context.IP, UserAgent: visit.Context.UserAgent},
			Page: PageContext{URL: visit.Context.PageURL, Referrer: visit.Context.Referrer},
			Properties: map[string]any{
				"content_type": "audio_novel",
				"content_name": title,
				"contents":     []Content{{ContentID: fmt.Sprintf("audio_novel:%d", audioNovelID), Quantity: 1}},
			},
		}},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	ciphertext, err := s.sealEvent(string(raw), eventID)
	if err != nil {
		inserted, recordErr := recordAudioDiagnosticFailure(ctx, tx, visit, visitID, eventName, eventID, input, audioNovelID, "TikTok 事件载荷无法加密")
		return BrowserEvent{}, inserted, recordErr
	}
	tag, err := tx.Exec(ctx, `INSERT INTO tiktok_events(
		id,visit_id,link_id,audio_novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,payload_cipher)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(pixel_code,event_name,event_id) DO NOTHING`, eventID, visitID, input.LinkID, audioNovelID,
		visit.ConnectionID, visit.PixelID, visit.PixelCode, eventName, eventID, input.EventAt, ciphertext)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	return BrowserEvent{Name: eventName, EventID: eventID}, tag.RowsAffected() == 1, nil
}

func buildVisitEventRequest(visit frozenVisit, eventName, eventID string, eventAt time.Time) EventRequest {
	return EventRequest{
		EventSource:   "web",
		EventSourceID: visit.PixelCode,
		Data: []EventData{{
			Event:     eventName,
			EventTime: eventAt.Unix(),
			EventID:   eventID,
			User: UserContext{
				TTCLID: visit.TTCLID, TTP: visit.TTP, IP: visit.Context.IP, UserAgent: visit.Context.UserAgent,
			},
			Page: PageContext{URL: visit.Context.PageURL, Referrer: visit.Context.Referrer},
			Properties: map[string]any{
				"content_type": "product",
				"contents":     []Content{{ContentID: fmt.Sprintf("novel:%d", visit.NovelID), Quantity: 1}},
			},
		}},
	}
}

// QueueVisitEventTx creates the durable server event in the same transaction
// that records the corresponding reading action.
func (s *Service) QueueVisitEventTx(ctx context.Context, tx pgx.Tx, visitID string, linkID int64, eventName string, eventAt time.Time) (BrowserEvent, bool, error) {
	if eventName != "StartReading" && eventName != "ViewContent" {
		return BrowserEvent{}, false, errors.New("不支持的 TikTok 小说事件")
	}
	if eventAt.IsZero() {
		return BrowserEvent{}, false, errors.New("TikTok 事件时间缺失")
	}
	var visit frozenVisit
	var contextCipher, classification, method, surface string
	err := tx.QueryRow(ctx, `SELECT e.ad_platform,COALESCE(e.tiktok_pixel_id,0),e.tiktok_pixel_code,
		COALESCE(p.connection_id,0),COALESCE(e.novel_id,0),e.tiktok_ttclid,e.tiktok_ttp,e.tiktok_context_cipher,
		e.classification,e.method,e.surface
		FROM click_events e LEFT JOIN tiktok_pixels p ON p.id=e.tiktok_pixel_id
		WHERE e.id=$1 AND e.link_id=$2`, visitID, linkID).Scan(&visit.Platform, &visit.PixelID, &visit.PixelCode,
		&visit.ConnectionID, &visit.NovelID, &visit.TTCLID, &visit.TTP, &contextCipher, &classification, &method, &surface)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	if visit.Platform != "tiktok" {
		return BrowserEvent{}, false, nil
	}
	if classification != "normal" || method != "GET" || surface != "novel" {
		return BrowserEvent{}, false, nil
	}
	if visit.PixelID < 1 || visit.PixelCode == "" || visit.ConnectionID < 1 || visit.NovelID < 1 || contextCipher == "" {
		return BrowserEvent{}, false, errors.New("TikTok 访问归因快照不完整")
	}
	plain, err := s.open(contextCipher, "tiktok:visit:"+visitID)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	if err = json.Unmarshal([]byte(plain), &visit.Context); err != nil {
		return BrowserEvent{}, false, errors.New("TikTok 访问归因快照无法读取")
	}
	eventID := VisitEventID(visitID, eventName)
	payload, err := json.Marshal(buildVisitEventRequest(visit, eventName, eventID, eventAt))
	if err != nil {
		return BrowserEvent{}, false, err
	}
	ciphertext, err := s.sealEvent(string(payload), eventID)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	tag, err := tx.Exec(ctx, `INSERT INTO tiktok_events
		(id,visit_id,link_id,novel_id,connection_id,pixel_record_id,pixel_code,event_name,event_id,event_time,payload_cipher)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		ON CONFLICT(pixel_code,event_name,event_id) DO NOTHING`, eventID, visitID, linkID, visit.NovelID,
		visit.ConnectionID, visit.PixelID, visit.PixelCode, eventName, eventID, eventAt, ciphertext)
	if err != nil {
		return BrowserEvent{}, false, err
	}
	return BrowserEvent{Name: eventName, EventID: eventID}, tag.RowsAffected() == 1, nil
}

func (s *Service) RetryEvent(ctx context.Context, id string) error {
	tag, err := s.Core.DB.Exec(ctx, `UPDATE tiktok_events SET status='pending',attempts=0,next_attempt_at=now(),
		locked_until=NULL,lock_token='',http_status=0,business_code=0,request_id='',last_error='',updated_at=now()
		WHERE id=$1 AND status IN ('failed','retry') AND event_time>now()-interval '24 hours' AND payload_cipher<>''`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return errors.New("此 TikTok 事件不可重试：可能已接收、正在发送或超过 24 小时安全期限")
	}
	return nil
}

func (s *Service) ListEvents(ctx context.Context, filters EventFilters) ([]EventRecord, int, error) {
	where := ` WHERE ($1::text='' OR e.status=$1) AND ($2::text='' OR e.event_name=$2)
		AND ($3::bigint=0 OR e.pixel_record_id=$3) AND ($4::bigint=0 OR e.link_id=$4)`
	args := []any{filters.Status, filters.EventName, filters.PixelRecordID, filters.LinkID}
	var total int
	if err := s.Core.DB.QueryRow(ctx, "SELECT count(*) FROM tiktok_events e"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.Core.DB.Query(ctx, `SELECT e.id,e.visit_id,e.link_id,e.novel_id,e.audio_novel_id,e.pixel_record_id,p.name,e.pixel_code,
		e.event_name,e.event_time,e.status,e.attempts,e.http_status,e.business_code,e.request_id,e.last_error,e.created_at,e.updated_at
		FROM tiktok_events e JOIN tiktok_pixels p ON p.id=e.pixel_record_id`+where+
		" ORDER BY e.created_at DESC,e.id DESC LIMIT 50 OFFSET $5", append(args, (filters.Page-1)*50)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []EventRecord{}
	for rows.Next() {
		var item EventRecord
		if err = rows.Scan(&item.ID, &item.VisitID, &item.LinkID, &item.NovelID, &item.AudioNovelID, &item.PixelRecordID, &item.PixelName, &item.PixelCode,
			&item.EventName, &item.EventTime, &item.Status, &item.Attempts, &item.HTTPStatus, &item.BusinessCode, &item.RequestID,
			&item.LastError, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}
