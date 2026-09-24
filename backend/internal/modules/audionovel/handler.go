package audionovel

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"

	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Meta     *meta.Service
	Playback *PlaybackService
}

func (a *Handler) View(c *gin.Context)    { a.action(c, true) }
func (a *Handler) Contact(c *gin.Context) { a.action(c, false) }

func (a *Handler) StartListening(c *gin.Context) { a.playbackAction(c, "start") }
func (a *Handler) PlaybackTime(c *gin.Context)   { a.playbackAction(c, "progress") }
func (a *Handler) Complete(c *gin.Context)       { a.playbackAction(c, "complete") }

// VisibleTime stores page foreground time without changing playback or ad-conversion state.
func (a *Handler) VisibleTime(c *gin.Context) {
	if !a.validActionOrigin(c) {
		return
	}
	if a.Playback == nil {
		audioNovelError(c, errors.New("playback service is not initialized"))
		return
	}
	if a.Exceed("audio-visible:"+a.Sign(c.ClientIP()), 300, time.Minute) {
		c.Status(429)
		return
	}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if !strings.HasPrefix(contentType, "application/json") && !strings.HasPrefix(contentType, "text/plain") {
		c.String(415, "Visible-time updates require JSON.")
		return
	}
	var input VisibleTimeUpdate
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		runtime.Bad(c, "可见时长上报参数无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	result, err := a.Playback.UpdateVisibleTime(ctx, c.Param("code"), input)
	if err != nil {
		a.writePlaybackError(c, err)
		return
	}
	c.JSON(200, result)
}

// TimeSpent delegates to the shared signed action implementation while retaining the audio novel surface.
func (a *Handler) TimeSpent(c *gin.Context) {
	proxy := landing.Handler{Core: a.Core, Meta: a.Meta}
	proxy.TimeSpentForSurface(c, "audio_novel", "contact:audio_novel:")
}

func (a *Handler) action(c *gin.Context, view bool) {
	if !a.validActionOrigin(c) {
		return
	}
	parts := strings.Split(c.PostForm("ticket"), ".")
	if len(parts) != 2 || !runtime.VisitorPattern.MatchString(parts[0]) || !hmac.Equal([]byte(parts[1]), []byte(a.Sign("contact:audio_novel:"+c.Param("code")+":"+parts[0]))) {
		c.String(400, "This enquiry link is invalid. Refresh the page and try again.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
	if err == pgx.ErrNoRows {
		c.Status(404)
		return
	}
	if err != nil {
		audioNovelError(c, err)
		return
	}
	if !link.Enabled {
		c.String(410, "This enquiry link is disabled. Please reopen the original page.")
		return
	}
	if !view && (link.ProductType == "audio_novel" || strings.TrimSpace(link.TargetURL) == "") {
		// Audio campaign links measure listening only and intentionally have no
		// WhatsApp target; reject crafted contact calls before any analytics write.
		c.String(410, "This audio story does not offer a WhatsApp enquiry.")
		return
	}
	request := a.playbackRequestContext(c)
	if view && link.ProductType == "audio_novel" {
		if a.Playback == nil {
			audioNovelError(c, errors.New("playback service is not initialized"))
			return
		}
		result, pageErr := a.Playback.ConfirmPageView(ctx, parts[0], link.ID, link.Code, request)
		if pageErr != nil {
			a.writePlaybackError(c, pageErr)
			return
		}
		c.JSON(200, result)
		return
	}
	trigger := c.PostForm("trigger")
	if trigger != "" && trigger != "manual" {
		runtime.Bad(c, "The literature site supports manual enquiries only.")
		return
	}
	input := request.Meta
	repository := landing.Repository{DB: a.DB, Meta: a.Meta}
	if view {
		err = repository.MarkView(ctx, parts[0], link.ID, "audio_novel", input)
		if err == nil {
			c.Status(204)
			return
		}
	} else {
		var target string
		target, err = repository.MarkContact(ctx, parts[0], link.ID, "audio_novel", false, input)
		if err == nil {
			c.JSON(200, gin.H{"target_url": target})
			return
		}
	}
	if err == pgx.ErrNoRows {
		c.String(400, "This page has expired. Refresh the page and try again.")
		return
	}
	a.WriteFailures.Add(1)
	audioNovelError(c, err)
}

func (a *Handler) validActionOrigin(c *gin.Context) bool {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		u, err := url.Parse(origin)
		if err != nil || u.Host != c.Request.Host {
			c.String(403, "Please use the player on the original page.")
			return false
		}
	}
	return true
}

func (a *Handler) playbackRequestContext(c *gin.Context) PlaybackRequestContext {
	fbc, _ := c.Cookie("_fbc")
	fbp, _ := c.Cookie("_fbp")
	ttp, _ := c.Cookie("_ttp")
	return PlaybackRequestContext{
		Meta:      meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp},
		TikTokTTP: ttp,
	}
}

func (a *Handler) playbackAction(c *gin.Context, action string) {
	if !a.validActionOrigin(c) {
		return
	}
	if a.Playback == nil {
		audioNovelError(c, errors.New("playback service is not initialized"))
		return
	}
	if a.Exceed("audio-playback:"+a.Sign(c.ClientIP()), 300, time.Minute) {
		c.Status(429)
		return
	}
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	if !strings.HasPrefix(contentType, "application/json") && !strings.HasPrefix(contentType, "text/plain") {
		c.String(415, "Playback updates require JSON.")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	request := a.playbackRequestContext(c)
	var result PlaybackResult
	var err error
	if action == "complete" {
		var input PlaybackComplete
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			runtime.Bad(c, "播放完成参数无效")
			return
		}
		result, err = a.Playback.Complete(ctx, c.Param("code"), input, request)
	} else {
		var input PlaybackUpdate
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		if decoder.Decode(&input) != nil {
			runtime.Bad(c, "播放上报参数无效")
			return
		}
		if action == "start" {
			result, err = a.Playback.StartListening(ctx, c.Param("code"), input, request)
		} else {
			result, err = a.Playback.UpdatePlayback(ctx, c.Param("code"), input, request)
		}
	}
	if err != nil {
		a.writePlaybackError(c, err)
		return
	}
	c.JSON(200, result)
}

func (a *Handler) writePlaybackError(c *gin.Context, err error) {
	if errors.Is(err, ErrPlaybackTicket) || errors.Is(err, ErrPlaybackInput) || errors.Is(err, ErrPlaybackComplete) || errors.Is(err, ErrVisibleTimeInput) || errors.Is(err, pgx.ErrNoRows) {
		runtime.Bad(c, err.Error())
		return
	}
	a.WriteFailures.Add(1)
	audioNovelError(c, err)
}

func audioNovelError(c *gin.Context, err error) {
	slog.Error("audio novel request failed", "error", err)
	c.String(503, "This service is temporarily unavailable. Please try again later.")
}
