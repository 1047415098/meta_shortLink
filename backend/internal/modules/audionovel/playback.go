package audionovel

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/runtime"
)

const maxPlaybackRate = 4.0

var (
	ErrPlaybackTicket   = errors.New("播放票据无效或已过期")
	ErrPlaybackInput    = errors.New("播放进度无效")
	ErrPlaybackComplete = errors.New("播放尚未满足完成条件")
	ErrVisibleTimeInput = errors.New("可见时长无效")
)

type PlaybackUpdate struct {
	Ticket               string  `json:"ticket"`
	PlaybackSeconds      int     `json:"playback_seconds"`
	MediaConsumedSeconds float64 `json:"media_consumed_seconds"`
}

type PlaybackComplete struct {
	PlaybackUpdate
	Ended bool `json:"ended"`
}

type ConfirmedEvent struct {
	Name    string `json:"name"`
	EventID string `json:"event_id"`
}

type PlaybackResult struct {
	PlaybackSeconds      int              `json:"playback_seconds"`
	MediaConsumedSeconds float64          `json:"media_consumed_seconds"`
	Started              bool             `json:"started"`
	Qualified            bool             `json:"qualified"`
	Completed            bool             `json:"completed"`
	ConfirmedEvents      []ConfirmedEvent `json:"confirmed_events"`
}

type VisibleTimeUpdate struct {
	Ticket         string `json:"ticket"`
	VisibleSeconds int    `json:"visible_seconds"`
}

type VisibleTimeResult struct {
	VisibleSeconds int `json:"visible_seconds"`
}

type PlaybackRequestContext struct {
	Meta      meta.ContactContext
	TikTokTTP string
}

type PlaybackService struct {
	Core   *runtime.Core
	Meta   *meta.Service
	TikTok *tiktok.Service
	Now    func() time.Time
}

type playbackVisit struct {
	OccurredAt           time.Time
	StartedAt            *time.Time
	QualifiedAt          *time.Time
	CompletedAt          *time.Time
	PageViewAt           *time.Time
	LinkID               int64
	AudioNovelID         int64
	AudioDurationSeconds int
	Threshold            int
	PlaybackSeconds      int
	MediaConsumedSeconds float64
	VisibleSeconds       int
	Platform             string
	MetaPageViewEnabled  bool
}

func (s *PlaybackService) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s *PlaybackService) claims(ticket string) (playbackTicketClaims, error) {
	claims, err := parsePlaybackTicket(s.Core, ticket, s.now())
	if err != nil {
		return playbackTicketClaims{}, fmt.Errorf("%w: %v", ErrPlaybackTicket, err)
	}
	return claims, nil
}

func (s *PlaybackService) lockVisit(ctx context.Context, tx pgx.Tx, code string, claims playbackTicketClaims) (playbackVisit, error) {
	var visit playbackVisit
	err := tx.QueryRow(ctx, `SELECT e.occurred_at,e.audio_started_at,e.audio_qualified_at,e.audio_completed_at,e.pageview_reported_at,
		e.link_id,e.audio_novel_id,COALESCE(n.audio_duration_seconds,0),e.time_spent_threshold,
		e.playback_seconds,e.media_consumed_seconds::float8,e.visible_seconds,e.ad_platform,e.meta_pageview_enabled
		FROM click_events e
		JOIN short_links l ON l.id=e.link_id
		JOIN audio_novels n ON n.id=e.audio_novel_id
		WHERE e.id=$1 AND e.link_id=$2 AND e.audio_novel_id=$3 AND l.code=$4 AND l.product_type='audio_novel'
		AND l.enabled AND n.enabled AND n.deleted_at IS NULL AND n.audio_path<>''
		AND e.surface='audio_novel' AND e.event_type='landing' AND e.method='GET' AND e.classification='normal'
		FOR UPDATE OF e`, claims.VisitID, claims.LinkID, claims.AudioNovelID, code).
		Scan(&visit.OccurredAt, &visit.StartedAt, &visit.QualifiedAt, &visit.CompletedAt, &visit.PageViewAt,
			&visit.LinkID, &visit.AudioNovelID, &visit.AudioDurationSeconds, &visit.Threshold,
			&visit.PlaybackSeconds, &visit.MediaConsumedSeconds, &visit.VisibleSeconds, &visit.Platform, &visit.MetaPageViewEnabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return playbackVisit{}, ErrPlaybackTicket
	}
	return visit, err
}

// UpdateVisibleTime records foreground-visible document time independently from audio playback.
// It intentionally queues no advertising event; ad qualification remains based on real playback.
func (s *PlaybackService) UpdateVisibleTime(ctx context.Context, code string, input VisibleTimeUpdate) (VisibleTimeResult, error) {
	if input.VisibleSeconds < 1 {
		return VisibleTimeResult{}, ErrVisibleTimeInput
	}
	claims, err := s.claims(input.Ticket)
	if err != nil {
		return VisibleTimeResult{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return VisibleTimeResult{}, err
	}
	defer tx.Rollback(ctx)
	visit, err := s.lockVisit(ctx, tx, code, claims)
	if err != nil {
		return VisibleTimeResult{}, err
	}

	// Five seconds absorbs browser timer/network delay without trusting arbitrary client totals.
	observed := int(s.now().Sub(visit.OccurredAt).Seconds()) + 5
	if observed < 0 {
		observed = 0
	}
	if observed > 7200 {
		observed = 7200
	}
	accepted := input.VisibleSeconds
	if accepted > observed {
		accepted = observed
	}
	if accepted < visit.VisibleSeconds {
		accepted = visit.VisibleSeconds
	}
	if accepted > visit.VisibleSeconds {
		if _, err = tx.Exec(ctx, `UPDATE click_events SET visible_seconds=$2,visible_updated_at=$3
			WHERE id=$1`, claims.VisitID, accepted, s.now()); err != nil {
			return VisibleTimeResult{}, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return VisibleTimeResult{}, err
	}
	return VisibleTimeResult{VisibleSeconds: accepted}, nil
}

func playbackResult(visit playbackVisit) PlaybackResult {
	return PlaybackResult{
		PlaybackSeconds: visit.PlaybackSeconds, MediaConsumedSeconds: visit.MediaConsumedSeconds,
		Started: visit.StartedAt != nil, Qualified: visit.QualifiedAt != nil, Completed: visit.CompletedAt != nil,
		ConfirmedEvents: []ConfirmedEvent{},
	}
}

func validTikTokTTP(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 512 || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func audioEventID(visitID, name string) string {
	suffix := "qualified"
	if name == "PageView" {
		suffix = "view"
	} else if name == "StartListening" {
		suffix = "start"
	}
	return "audio_" + visitID + "_" + suffix
}

func (s *PlaybackService) enqueueEvent(ctx context.Context, tx pgx.Tx, visit playbackVisit, visitID, name string, eventAt time.Time, request PlaybackRequestContext) (ConfirmedEvent, error) {
	eventID := audioEventID(visitID, name)
	event := ConfirmedEvent{Name: name, EventID: eventID}
	switch visit.Platform {
	case "meta":
		if s.Meta == nil {
			return ConfirmedEvent{}, errors.New("Meta 服务尚未初始化")
		}
		browserAllowed, err := s.Meta.EnqueueAudioEvent(ctx, tx, visitID, name, eventID, meta.AudioEventInput{LinkID: visit.LinkID, EventAt: eventAt, Context: request.Meta})
		if err != nil || !browserAllowed {
			return ConfirmedEvent{}, err
		}
		return event, nil
	case "tiktok":
		if s.TikTok == nil {
			return ConfirmedEvent{}, errors.New("TikTok 服务尚未初始化")
		}
		browserEvent, _, err := s.TikTok.EnqueueAudioEvent(ctx, tx, visitID, name, eventID, tiktok.AudioEventInput{LinkID: visit.LinkID, EventAt: eventAt})
		if err != nil || browserEvent.EventID == "" {
			return ConfirmedEvent{}, err
		}
		return event, nil
	default:
		return ConfirmedEvent{}, errors.New("语音访问广告平台无效")
	}
}

func (s *PlaybackService) StartListening(ctx context.Context, code string, input PlaybackUpdate, request PlaybackRequestContext) (PlaybackResult, error) {
	claims, err := s.claims(input.Ticket)
	if err != nil {
		return PlaybackResult{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return PlaybackResult{}, err
	}
	defer tx.Rollback(ctx)
	visit, err := s.lockVisit(ctx, tx, code, claims)
	if err != nil {
		return PlaybackResult{}, err
	}
	if ttp := validTikTokTTP(request.TikTokTTP); visit.Platform == "tiktok" && ttp != "" {
		if _, err = tx.Exec(ctx, "UPDATE click_events SET tiktok_ttp=CASE WHEN tiktok_ttp='' THEN $2 ELSE tiktok_ttp END WHERE id=$1", claims.VisitID, ttp); err != nil {
			return PlaybackResult{}, err
		}
	}
	if visit.StartedAt == nil {
		startedAt := s.now()
		if _, err = tx.Exec(ctx, "UPDATE click_events SET audio_started_at=$2 WHERE id=$1 AND audio_started_at IS NULL", claims.VisitID, startedAt); err != nil {
			return PlaybackResult{}, err
		}
		visit.StartedAt = &startedAt
	}
	event, err := s.enqueueEvent(ctx, tx, visit, claims.VisitID, "StartListening", *visit.StartedAt, request)
	if err != nil {
		return PlaybackResult{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return PlaybackResult{}, err
	}
	result := playbackResult(visit)
	if event.EventID != "" {
		result.ConfirmedEvents = append(result.ConfirmedEvents, event)
	}
	return result, nil
}

func validatePlaybackUpdate(input PlaybackUpdate) error {
	if input.PlaybackSeconds < 0 || input.MediaConsumedSeconds < 0 || math.IsNaN(input.MediaConsumedSeconds) || math.IsInf(input.MediaConsumedSeconds, 0) {
		return ErrPlaybackInput
	}
	return nil
}

func (s *PlaybackService) advanceProgress(visit *playbackVisit, input PlaybackUpdate, now time.Time) {
	observedSeconds := int(now.Sub(visit.OccurredAt).Seconds())
	if observedSeconds < 0 {
		observedSeconds = 0
	}
	if observedSeconds > 86400 {
		observedSeconds = 86400
	}
	incomingPlayback := input.PlaybackSeconds
	if incomingPlayback > observedSeconds {
		incomingPlayback = observedSeconds
	}
	if incomingPlayback > visit.PlaybackSeconds {
		visit.PlaybackSeconds = incomingPlayback
	}
	incomingMedia := math.Min(input.MediaConsumedSeconds, float64(visit.PlaybackSeconds)*maxPlaybackRate)
	if incomingMedia > visit.MediaConsumedSeconds {
		visit.MediaConsumedSeconds = math.Round(incomingMedia*1000) / 1000
	}
}

func (s *PlaybackService) UpdatePlayback(ctx context.Context, code string, input PlaybackUpdate, request PlaybackRequestContext) (PlaybackResult, error) {
	if err := validatePlaybackUpdate(input); err != nil {
		return PlaybackResult{}, err
	}
	claims, err := s.claims(input.Ticket)
	if err != nil {
		return PlaybackResult{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return PlaybackResult{}, err
	}
	defer tx.Rollback(ctx)
	visit, err := s.lockVisit(ctx, tx, code, claims)
	if err != nil {
		return PlaybackResult{}, err
	}
	now := s.now()
	s.advanceProgress(&visit, input, now)
	if _, err = tx.Exec(ctx, `UPDATE click_events SET playback_seconds=$2,media_consumed_seconds=$3,
		playback_updated_at=CASE WHEN playback_seconds<>$2 OR media_consumed_seconds<>$3 THEN $4 ELSE playback_updated_at END
		WHERE id=$1`, claims.VisitID, visit.PlaybackSeconds, visit.MediaConsumedSeconds, now); err != nil {
		return PlaybackResult{}, err
	}
	if ttp := validTikTokTTP(request.TikTokTTP); visit.Platform == "tiktok" && ttp != "" {
		if _, err = tx.Exec(ctx, "UPDATE click_events SET tiktok_ttp=CASE WHEN tiktok_ttp='' THEN $2 ELSE tiktok_ttp END WHERE id=$1", claims.VisitID, ttp); err != nil {
			return PlaybackResult{}, err
		}
	}
	result := playbackResult(visit)
	if visit.StartedAt != nil && visit.Threshold > 0 && visit.PlaybackSeconds >= visit.Threshold {
		if visit.QualifiedAt == nil {
			qualifiedAt := now
			if _, err = tx.Exec(ctx, "UPDATE click_events SET audio_qualified_at=$2 WHERE id=$1 AND audio_qualified_at IS NULL", claims.VisitID, qualifiedAt); err != nil {
				return PlaybackResult{}, err
			}
			visit.QualifiedAt = &qualifiedAt
		}
		event, queueErr := s.enqueueEvent(ctx, tx, visit, claims.VisitID, "ViewContent", *visit.QualifiedAt, request)
		if queueErr != nil {
			return PlaybackResult{}, queueErr
		}
		result.Qualified = true
		if event.EventID != "" {
			result.ConfirmedEvents = append(result.ConfirmedEvents, event)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return PlaybackResult{}, err
	}
	return result, nil
}

func (s *PlaybackService) Complete(ctx context.Context, code string, input PlaybackComplete, request PlaybackRequestContext) (PlaybackResult, error) {
	if !input.Ended || validatePlaybackUpdate(input.PlaybackUpdate) != nil {
		return PlaybackResult{}, ErrPlaybackComplete
	}
	claims, err := s.claims(input.Ticket)
	if err != nil {
		return PlaybackResult{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return PlaybackResult{}, err
	}
	defer tx.Rollback(ctx)
	visit, err := s.lockVisit(ctx, tx, code, claims)
	if err != nil {
		return PlaybackResult{}, err
	}
	now := s.now()
	s.advanceProgress(&visit, input.PlaybackUpdate, now)
	if _, err = tx.Exec(ctx, `UPDATE click_events SET playback_seconds=$2,media_consumed_seconds=$3,
		playback_updated_at=CASE WHEN playback_seconds<>$2 OR media_consumed_seconds<>$3 THEN $4 ELSE playback_updated_at END
		WHERE id=$1`, claims.VisitID, visit.PlaybackSeconds, visit.MediaConsumedSeconds, now); err != nil {
		return PlaybackResult{}, err
	}
	if visit.StartedAt == nil || visit.AudioDurationSeconds < 1 || visit.MediaConsumedSeconds < float64(visit.AudioDurationSeconds)*0.9 {
		return PlaybackResult{}, ErrPlaybackComplete
	}
	result := playbackResult(visit)
	if visit.Threshold > 0 && visit.PlaybackSeconds >= visit.Threshold && visit.QualifiedAt == nil {
		qualifiedAt := now
		if _, err = tx.Exec(ctx, "UPDATE click_events SET audio_qualified_at=$2 WHERE id=$1 AND audio_qualified_at IS NULL", claims.VisitID, qualifiedAt); err != nil {
			return PlaybackResult{}, err
		}
		visit.QualifiedAt = &qualifiedAt
		event, queueErr := s.enqueueEvent(ctx, tx, visit, claims.VisitID, "ViewContent", qualifiedAt, request)
		if queueErr != nil {
			return PlaybackResult{}, queueErr
		}
		result.Qualified = true
		if event.EventID != "" {
			result.ConfirmedEvents = append(result.ConfirmedEvents, event)
		}
	}
	if visit.CompletedAt == nil {
		completedAt := now
		if _, err = tx.Exec(ctx, "UPDATE click_events SET audio_completed_at=$2 WHERE id=$1 AND audio_completed_at IS NULL", claims.VisitID, completedAt); err != nil {
			return PlaybackResult{}, err
		}
		visit.CompletedAt = &completedAt
	}
	if err = tx.Commit(ctx); err != nil {
		return PlaybackResult{}, err
	}
	result.Completed = true
	return result, nil
}

func (s *PlaybackService) ConfirmPageView(ctx context.Context, visitID string, linkID int64, code string, request PlaybackRequestContext) (PlaybackResult, error) {
	claims := playbackTicketClaims{VisitID: visitID, LinkID: linkID}
	if err := s.Core.DB.QueryRow(ctx, "SELECT audio_novel_id FROM click_events WHERE id=$1 AND link_id=$2", visitID, linkID).Scan(&claims.AudioNovelID); err != nil {
		return PlaybackResult{}, err
	}
	tx, err := s.Core.DB.Begin(ctx)
	if err != nil {
		return PlaybackResult{}, err
	}
	defer tx.Rollback(ctx)
	visit, err := s.lockVisit(ctx, tx, code, claims)
	if err != nil {
		return PlaybackResult{}, err
	}
	if visit.PageViewAt == nil {
		viewedAt := s.now()
		if _, err = tx.Exec(ctx, "UPDATE click_events SET pageview_reported_at=$2 WHERE id=$1 AND pageview_reported_at IS NULL", visitID, viewedAt); err != nil {
			return PlaybackResult{}, err
		}
		visit.PageViewAt = &viewedAt
	}
	result := playbackResult(visit)
	if visit.Platform == "tiktok" || visit.MetaPageViewEnabled {
		event := ConfirmedEvent{Name: "PageView", EventID: audioEventID(visitID, "PageView")}
		if visit.Platform == "meta" {
			if s.Meta == nil {
				return PlaybackResult{}, errors.New("Meta 服务尚未初始化")
			}
			var browserAllowed bool
			if browserAllowed, err = s.Meta.EnqueueAudioEvent(ctx, tx, visitID, "PageView", event.EventID, meta.AudioEventInput{LinkID: visit.LinkID, EventAt: *visit.PageViewAt, Context: request.Meta}); err != nil {
				return PlaybackResult{}, err
			}
			if !browserAllowed {
				event = ConfirmedEvent{}
			}
		}
		if event.EventID != "" {
			result.ConfirmedEvents = append(result.ConfirmedEvents, event)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return PlaybackResult{}, err
	}
	return result, nil
}
