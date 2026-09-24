package meta

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

type ContactContext struct{ IP, UserAgent, FBC, FBP string }
type ServerEvent struct {
	Name       string            `json:"event_name"`
	Time       int64             `json:"event_time"`
	ID         string            `json:"event_id"`
	Source     string            `json:"action_source"`
	URL        string            `json:"event_source_url"`
	UserData   map[string]string `json:"user_data"`
	CustomData map[string]any    `json:"custom_data,omitempty"`
}
type eventPayload struct {
	Data     []ServerEvent `json:"data"`
	TestCode string        `json:"test_event_code,omitempty"`
}

var clickIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var fbcPattern = regexp.MustCompile(`^fb\.[0-9]{1,2}\.[0-9]{13}\.[a-zA-Z0-9._-]+$`)
var fbpPattern = regexp.MustCompile(`^fb\.[0-9]{1,2}\.[0-9]{13}\.[0-9]{1,40}$`)

func matchingData(input ContactContext, fbclid string, at time.Time) map[string]string {
	data := map[string]string{}
	if ip, e := netip.ParseAddr(input.IP); e == nil {
		data["client_ip_address"] = ip.String()
	}
	if input.UserAgent != "" {
		data["client_user_agent"] = runtime.Bounded(strings.Map(func(r rune) rune {
			if r < 32 {
				return -1
			}
			return r
		}, input.UserAgent), 1024)
	}
	if len(fbclid) <= 2048 && clickIDPattern.MatchString(fbclid) {
		data["fbc"] = fmt.Sprintf("fb.1.%d.%s", at.UnixMilli(), fbclid)
	} else if len(input.FBC) <= 2100 && fbcPattern.MatchString(input.FBC) {
		data["fbc"] = input.FBC
	}
	if fbpPattern.MatchString(input.FBP) {
		data["fbp"] = input.FBP
	}
	return data
}

// Called within the transaction marking the business action. Routing/rules are frozen at visit time.
func (s *Service) EnqueueContact(ctx context.Context, tx pgx.Tx, visit string, input ContactContext) error {
	return s.enqueueVisit(ctx, tx, visit, input, "manual")
}

// EnqueueAutoRedirect records a timer redirect independently from a manual consultation.
func (s *Service) EnqueueAutoRedirect(ctx context.Context, tx pgx.Tx, visit string, input ContactContext) error {
	return s.enqueueVisit(ctx, tx, visit, input, "auto")
}
func (s *Service) EnqueuePageView(ctx context.Context, tx pgx.Tx, visit string, input ContactContext) error {
	return s.enqueueVisit(ctx, tx, visit, input, "view")
}

// EnqueueTimeSpent shares one visit-scoped ID with the browser custom event.
func (s *Service) EnqueueTimeSpent(ctx context.Context, tx pgx.Tx, visit string, input ContactContext) error {
	return s.enqueueVisit(ctx, tx, visit, input, "time_spent")
}

type AudioEventInput struct {
	LinkID  int64
	EventAt time.Time
	Context ContactContext
}

// EnqueueAudioEvent uses the immutable visit snapshot and a deterministic ID;
// disabled Pixels remain pending for diagnostics instead of blocking playback.
func (s *Service) EnqueueAudioEvent(ctx context.Context, tx pgx.Tx, visitID, eventName, eventID string, input AudioEventInput) (bool, error) {
	if eventName != "PageView" && eventName != "StartListening" && eventName != "ViewContent" {
		return false, errors.New("不支持的 Meta 语音事件")
	}
	if input.EventAt.IsZero() {
		return false, errors.New("Meta 语音事件时间缺失")
	}
	var connectionID, pixelID *int64
	var occurredAt time.Time
	var code, classification, platform, adID, title, surface, method string
	var audioNovelID *int64
	var conflict bool
	var params map[string]string
	err := tx.QueryRow(ctx, `SELECT e.meta_connection_id,e.meta_pixel_id,e.occurred_at,l.code,e.classification,
		e.attribution_conflict,e.parameters,e.ad_id,e.ad_platform,e.audio_novel_id,e.audio_novel_title,e.surface,e.method
		FROM click_events e JOIN short_links l ON l.id=e.link_id
		WHERE e.id=$1 AND e.link_id=$2`, visitID, input.LinkID).
		Scan(&connectionID, &pixelID, &occurredAt, &code, &classification, &conflict, &params, &adID, &platform, &audioNovelID, &title, &surface, &method)
	if err != nil {
		return false, err
	}
	if platform != "meta" {
		return false, nil
	}
	if connectionID == nil || pixelID == nil || audioNovelID == nil || surface != "audio_novel" || method != "GET" {
		return false, errors.New("Meta 语音访问归因快照不完整")
	}
	pixel, err := scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1 AND connection_id=$2", *pixelID, *connectionID))
	if err != nil {
		return false, err
	}
	status, reason := "pending", ""
	if classification != "normal" || conflict {
		status, reason = "skipped", "访问未通过统计过滤或广告来源冲突"
	} else if !isResolvedAdAttribution(params["fbclid"], adID) {
		status, reason = "skipped", "缺少有效 fbclid 或广告 ID，不属于真实广告点击"
	}
	payload := eventPayload{Data: []ServerEvent{{
		Name: eventName, Time: input.EventAt.Unix(), ID: eventID, Source: "website",
		URL:      strings.TrimRight(s.Core.Config.PublicURL, "/") + "/audio-novel/" + code,
		UserData: matchingData(input.Context, params["fbclid"], occurredAt),
		CustomData: map[string]any{
			"content_ids":  []string{fmt.Sprintf("audio_novel:%d", *audioNovelID)},
			"content_name": title,
			"content_type": "audio_novel",
		},
	}}}
	raw, err := json.Marshal(payload)
	if err != nil {
		return false, err
	}
	ciphertext := ""
	if status == "pending" {
		ciphertext, err = s.seal(string(raw), "event:"+eventID)
		if err != nil {
			return false, err
		}
	}
	// Persist the frozen content owner on the outbox row itself. Visit details
	// can expire before delivery diagnostics, so later views must not depend on
	// joining back to click_events.
	_, err = tx.Exec(ctx, `INSERT INTO meta_events(
		id,connection_id,pixel_record_id,visit_id,event_name,event_time,pixel_id,payload_cipher,status,last_error,audio_novel_id)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) ON CONFLICT(id) DO NOTHING`,
		eventID, *connectionID, pixel.ID, visitID, eventName, input.EventAt, pixel.PixelID, ciphertext, status, reason, *audioNovelID)
	// Only a fully attributed visit on an enabled Pixel may authorize the
	// matching browser event. Skipped rows remain available for diagnostics.
	return err == nil && status == "pending" && pixel.Enabled, err
}

// isResolvedAdAttribution mirrors the strict advertising report contract: both
// Meta's click identifier and a resolved ad ID must be present.
func isResolvedAdAttribution(fbclid, adID string) bool {
	for _, value := range []string{fbclid, adID} {
		value = strings.TrimSpace(value)
		if value == "" || strings.Contains(value, "{{") || strings.Contains(value, "}}") {
			return false
		}
	}
	return true
}

func (s *Service) enqueueVisit(ctx context.Context, tx pgx.Tx, visit string, input ContactContext, trigger string) error {
	var connectionID, pixelID *int64
	var at time.Time
	var clicked, automatic, viewed, timeSpent *time.Time
	var code, class, eventName, adID, surface string
	var conflict, measurement, manualEnabled, autoEnabled, pageEnabled bool
	var params map[string]string
	// Read the frozen visit configuration and both consultation timestamps in the
	// same transaction that marked the browser action.
	e := tx.QueryRow(ctx, `SELECT e.meta_connection_id,e.meta_pixel_id,e.occurred_at,e.whatsapp_clicked_at,e.auto_redirected_at,e.pageview_reported_at,e.time_spent_reported_at,l.code,e.classification,e.attribution_conflict,e.parameters,e.ad_id,e.meta_measurement,e.meta_manual_enabled,e.meta_auto_enabled,e.meta_pageview_enabled,e.meta_manual_event_name,e.surface FROM click_events e JOIN short_links l ON l.id=e.link_id WHERE e.id=$1`, visit).Scan(&connectionID, &pixelID, &at, &clicked, &automatic, &viewed, &timeSpent, &code, &class, &conflict, &params, &adID, &measurement, &manualEnabled, &autoEnabled, &pageEnabled, &eventName, &surface)
	if e != nil {
		return e
	}
	if connectionID == nil || pixelID == nil || !measurement {
		return nil
	}
	eventAt := clicked
	suffix := "manual"
	switch trigger {
	case "view":
		if !pageEnabled {
			return nil
		}
		eventName = "PageView"
		eventAt = viewed
		suffix = "view"
	case "auto":
		if !autoEnabled {
			return nil
		}
		eventName = AutoRedirectEventName
		eventAt = automatic
		suffix = "auto"
	case "time_spent":
		eventName = TimeSpentEventName
		eventAt = timeSpent
		suffix = "time_spent"
	default:
		if !manualEnabled {
			return nil
		}
		// Existing visit rows may contain a frozen legacy event name. New manual
		// actions always follow the current AddToCart contract after deployment.
		eventName = EventName
	}
	if eventAt == nil {
		return errors.New("事件时间缺失")
	}
	p, e := scanPixel(tx.QueryRow(ctx, "SELECT "+pixelColumns+" FROM meta_pixels WHERE id=$1 AND connection_id=$2", *pixelID, *connectionID))
	if e != nil {
		return e
	}
	id := "wa_" + visit + "_" + suffix
	status, reason := "pending", ""
	if class != "normal" || conflict {
		status, reason = "skipped", "访问未通过统计过滤或广告来源冲突"
	} else if !isResolvedAdAttribution(params["fbclid"], adID) {
		// Keep a skipped audit record, but never expose incomplete or organic
		// attribution to the worker's delivery queue.
		status, reason = "skipped", "缺少有效 fbclid 或广告 ID，不属于真实广告点击"
	}
	path := "/" + code
	if surface == "audio_novel" {
		path = "/audio-novel/" + code
	}
	payload := eventPayload{Data: []ServerEvent{{Name: eventName, Time: eventAt.Unix(), ID: id, Source: "website", URL: strings.TrimRight(s.Core.Config.PublicURL, "/") + path, UserData: matchingData(input, params["fbclid"], at)}}}
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	cipher := ""
	if status == "pending" {
		cipher, e = s.seal(string(b), "event:"+id)
		if e != nil {
			return e
		}
	}
	_, e = tx.Exec(ctx, `INSERT INTO meta_events(id,connection_id,pixel_record_id,visit_id,event_name,event_time,pixel_id,payload_cipher,status,last_error)VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)ON CONFLICT(id)DO NOTHING`, id, *connectionID, p.ID, visit, eventName, *eventAt, p.PixelID, cipher, status, reason)
	return e
}

type EventRecord struct {
	PixelID        string    `json:"pixel_id"`
	PixelRecordID  *int64    `json:"pixel_record_id"`
	AudioNovelID   *int64    `json:"audio_novel_id"`
	ID             string    `json:"id"`
	ConnectionID   int64     `json:"connection_id"`
	ConnectionName string    `json:"connection_name"`
	VisitID        string    `json:"visit_id"`
	EventName      string    `json:"event_name"`
	EventTime      time.Time `json:"event_time"`
	IsTest         bool      `json:"is_test"`
	Status         string    `json:"status"`
	Attempts       int       `json:"attempts"`
	LastError      string    `json:"last_error"`
	EventsReceived int       `json:"events_received"`
	Trace          string    `json:"fbtrace_id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (s *Service) QueuePixelTest(ctx context.Context, p Pixel, code, name string, input ContactContext) (EventRecord, error) {
	if len(code) < 3 || len(code) > 120 || strings.ContainsAny(code, " \r\n\t") {
		return EventRecord{}, errors.New("请填写事件管理工具中的测试代码")
	}
	// Keep legacy diagnostic names accepted while the administration UI sends
	// only the current PageView and AddToCart contract.
	if name != "PageView" && name != EventName && name != LegacyManualEventName && name != LegacyAutoRedirectEventName {
		return EventRecord{}, errors.New("不支持的测试事件")
	}
	if p.Cipher == "" {
		return EventRecord{}, errors.New("请先保存回传凭证")
	}
	id := "test_" + runtime.Token()
	at := time.Now()
	payload := eventPayload{TestCode: code, Data: []ServerEvent{{Name: name, Time: at.Unix(), ID: id, Source: "website", URL: s.Core.Config.PublicURL + "/admin/meta/connections", UserData: matchingData(input, "", at)}}}
	raw, _ := json.Marshal(payload)
	cipher, e := s.seal(string(raw), "event:"+id)
	if e != nil {
		return EventRecord{}, e
	}
	_, e = s.Core.DB.Exec(ctx, `INSERT INTO meta_events(id,connection_id,pixel_record_id,event_name,event_time,pixel_id,is_test,payload_cipher)VALUES($1,$2,$3,$4,$5,$6,true,$7)`, id, p.ConnectionID, p.ID, name, at, p.PixelID, cipher)
	return EventRecord{ID: id, Status: "pending", IsTest: true, EventTime: at, ConnectionID: p.ConnectionID, PixelID: p.PixelID, PixelRecordID: &p.ID, EventName: name}, e
}
func (s *Service) RetryEvent(ctx context.Context, id string) error {
	tag, e := s.Core.DB.Exec(ctx, `UPDATE meta_events SET status='pending',retry_count=0,next_attempt_at=now(),last_error='',updated_at=now() WHERE id=$1 AND status IN ('failed','retry') AND event_time>now()-interval '6 days' AND payload_cipher<>''`, id)
	if e != nil {
		return e
	}
	if tag.RowsAffected() != 1 {
		return errors.New("此事件不可重试：可能已成功、正在发送或超过保留期限")
	}
	return nil
}
