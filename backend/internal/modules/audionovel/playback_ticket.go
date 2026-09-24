package audionovel

import (
	"crypto/hmac"
	"errors"
	"strconv"
	"strings"
	"time"

	"whatsapp-analytics/internal/platform/runtime"
)

// Tickets cover the longest accepted 24-hour recording plus a small delivery
// grace period, while remaining bounded bearer credentials.
const playbackTicketLifetime = 24*time.Hour + 5*time.Minute

type playbackTicketClaims struct {
	VisitID      string
	LinkID       int64
	AudioNovelID int64
	IssuedAt     time.Time
	ExpiresAt    time.Time
}

func issuePlaybackTicket(core *runtime.Core, visitID string, linkID, audioNovelID int64, now time.Time) string {
	issuedAt := now.UTC().Truncate(time.Second)
	expiresAt := issuedAt.Add(playbackTicketLifetime)
	payload := strings.Join([]string{
		visitID,
		strconv.FormatInt(linkID, 10),
		strconv.FormatInt(audioNovelID, 10),
		strconv.FormatInt(issuedAt.Unix(), 10),
		strconv.FormatInt(expiresAt.Unix(), 10),
	}, ".")
	return payload + "." + core.Sign("audio-playback:"+payload)
}

func parsePlaybackTicket(core *runtime.Core, ticket string, now time.Time) (playbackTicketClaims, error) {
	parts := strings.Split(ticket, ".")
	if len(parts) != 6 || !runtime.VisitorPattern.MatchString(parts[0]) {
		return playbackTicketClaims{}, errors.New("播放票据格式无效")
	}
	payload := strings.Join(parts[:5], ".")
	if !hmac.Equal([]byte(parts[5]), []byte(core.Sign("audio-playback:"+payload))) {
		return playbackTicketClaims{}, errors.New("播放票据签名无效")
	}
	linkID, linkErr := strconv.ParseInt(parts[1], 10, 64)
	audioNovelID, audioErr := strconv.ParseInt(parts[2], 10, 64)
	issuedUnix, issuedErr := strconv.ParseInt(parts[3], 10, 64)
	expiresUnix, expiresErr := strconv.ParseInt(parts[4], 10, 64)
	if linkErr != nil || audioErr != nil || issuedErr != nil || expiresErr != nil || linkID < 1 || audioNovelID < 1 || expiresUnix <= issuedUnix {
		return playbackTicketClaims{}, errors.New("播放票据内容无效")
	}
	issuedAt, expiresAt := time.Unix(issuedUnix, 0).UTC(), time.Unix(expiresUnix, 0).UTC()
	// A bounded lifetime prevents a re-signed or stale browser payload from
	// becoming a long-lived bearer credential.
	if expiresAt.Sub(issuedAt) > playbackTicketLifetime || now.UTC().Before(issuedAt.Add(-time.Minute)) || !now.UTC().Before(expiresAt) {
		return playbackTicketClaims{}, errors.New("播放票据已过期")
	}
	return playbackTicketClaims{VisitID: parts[0], LinkID: linkID, AudioNovelID: audioNovelID, IssuedAt: issuedAt, ExpiresAt: expiresAt}, nil
}
