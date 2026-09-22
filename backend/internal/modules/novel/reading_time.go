package novel

import (
	"context"
	"errors"
	"strconv"
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if err = (Repository{DB: h.DB}).UpdateVisibleSeconds(ctx, eventID, link.ID, seconds); errors.Is(err, pgx.ErrNoRows) {
		runtime.Bad(c, "访问票据无效或已过期")
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		c.Status(204)
	}
}

func (r Repository) UpdateVisibleSeconds(ctx context.Context, eventID string, linkID int64, reported int) error {
	// The five-second allowance covers network and timer scheduling delay without trusting client time.
	command, err := r.DB.Exec(ctx, `WITH target AS (
		SELECT id,LEAST($3,7200,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (now()-occurred_at)))::int+5)) AS accepted
		FROM click_events WHERE id=$1 AND link_id=$2 AND surface='novel' AND method='GET'
		AND event_type='landing' AND classification='normal' AND occurred_at>=now()-interval '2 hours 5 minutes' FOR UPDATE
	) UPDATE click_events e SET
		visible_updated_at=CASE WHEN target.accepted>e.visible_seconds THEN now() ELSE e.visible_updated_at END,
		visible_seconds=GREATEST(e.visible_seconds,target.accepted)
		FROM target WHERE e.id=target.id`, eventID, linkID, reported)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
