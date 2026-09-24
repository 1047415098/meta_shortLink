package novel

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

func capVisibleSeconds(reported int, startedAt, observedAt time.Time) int {
	observed := int(observedAt.Sub(startedAt).Seconds()) + 5
	if observed < 0 {
		observed = 0
	}
	if observed > 7200 {
		observed = 7200
	}
	if reported > observed {
		return observed
	}
	return reported
}

func normalizeTikTokTTP(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, "{{") || strings.Contains(value, "}}") ||
		(len(value) > 4 && strings.HasPrefix(value, "__") && strings.HasSuffix(value, "__")) {
		return "", nil
	}
	if len(value) > 512 || strings.ContainsAny(value, "\r\n") {
		return "", errors.New("TikTok _ttp 格式无效")
	}
	return value, nil
}

func actionTikTokTTP(c *gin.Context) (string, error) {
	raw := c.PostForm("_ttp")
	origin := c.GetHeader("Origin")
	if raw == "" || origin == "" || origin == "null" {
		// A Cookie supplied without a browser same-origin signal is not trusted.
		return "", nil
	}
	return normalizeTikTokTTP(raw)
}

// ReadingTime stores one monotonic foreground-visible duration on the original visit.
func (h *Handler) ReadingTime(c *gin.Context) {
	if !h.validActionOrigin(c) {
		return
	}
	eventID, link, ok := h.validActionTicket(c)
	if !ok {
		return
	}
	seconds, err := strconv.Atoi(c.PostForm("seconds"))
	if err != nil || seconds < 1 {
		runtime.Bad(c, "可见时长无效")
		return
	}
	ttp, err := actionTikTokTTP(c)
	if err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	tx, err := h.DB.Begin(ctx)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	defer tx.Rollback(ctx)
	accepted, err := (Repository{DB: h.DB}).UpdateVisibleSecondsTx(ctx, tx, eventID, link.ID, seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		runtime.Bad(c, "访问票据无效或已过期")
	} else if err != nil {
		runtime.ServerError(c, err)
		return
	}
	var platform string
	var threshold int
	var qualifiedAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT ad_platform,time_spent_threshold,tiktok_view_content_at
		FROM click_events WHERE id=$1 AND link_id=$2 FOR UPDATE`, eventID, link.ID).Scan(&platform, &threshold, &qualifiedAt); err != nil {
		runtime.ServerError(c, err)
		return
	}
	if platform == "tiktok" && ttp != "" {
		if _, err = tx.Exec(ctx, `UPDATE click_events SET tiktok_ttp=$3
			WHERE id=$1 AND link_id=$2 AND ad_platform='tiktok' AND tiktok_ttp=''`, eventID, link.ID, ttp); err != nil {
			runtime.ServerError(c, err)
			return
		}
	}
	var browserEvent any
	if platform == "tiktok" && threshold > 0 && accepted >= threshold {
		if qualifiedAt == nil {
			var value time.Time
			if err = tx.QueryRow(ctx, `UPDATE click_events SET tiktok_view_content_at=COALESCE(tiktok_view_content_at,now())
				WHERE id=$1 AND link_id=$2 RETURNING tiktok_view_content_at`, eventID, link.ID).Scan(&value); err != nil {
				runtime.ServerError(c, err)
				return
			}
			qualifiedAt = &value
		}
		if h.TikTok == nil {
			runtime.ServerError(c, errors.New("TikTok 服务尚未初始化"))
			return
		}
		event, _, queueErr := h.TikTok.QueueVisitEventTx(ctx, tx, eventID, link.ID, "ViewContent", *qualifiedAt)
		if queueErr != nil {
			runtime.ServerError(c, queueErr)
			return
		}
		browserEvent = event
	}
	if err = tx.Commit(ctx); err != nil {
		runtime.ServerError(c, err)
		return
	}
	response := gin.H{"visible_seconds": accepted}
	if browserEvent != nil {
		response["tiktok_event"] = browserEvent
	}
	c.JSON(200, response)
}

func (r Repository) UpdateVisibleSecondsTx(ctx context.Context, tx pgx.Tx, eventID string, linkID int64, reported int) (int, error) {
	// The five-second allowance covers network and timer scheduling delay without trusting client time.
	var accepted int
	err := tx.QueryRow(ctx, `WITH target AS (
		SELECT id,LEAST($3,7200,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (now()-occurred_at)))::int+5)) AS accepted
		FROM click_events WHERE id=$1 AND link_id=$2 AND surface='novel' AND method='GET'
		AND event_type='landing' AND classification='normal' AND occurred_at>=now()-interval '2 hours 5 minutes' FOR UPDATE
	) UPDATE click_events e SET
		visible_updated_at=CASE WHEN target.accepted>e.visible_seconds THEN now() ELSE e.visible_updated_at END,
		visible_seconds=GREATEST(e.visible_seconds,target.accepted)
		FROM target WHERE e.id=target.id RETURNING e.visible_seconds`, eventID, linkID, reported).Scan(&accepted)
	return accepted, err
}
