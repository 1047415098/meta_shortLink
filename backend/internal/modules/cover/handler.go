package cover

import (
	"context"
	"crypto/hmac"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/modules/landing"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/modules/meta"
	"whatsapp-analytics/internal/modules/tiktok"
	"whatsapp-analytics/internal/platform/runtime"
)

type Handler struct {
	*runtime.Core
	Meta   *meta.Service
	TikTok *tiktok.Service
}

func (h *Handler) ListLinks(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := (Repository{DB: h.DB}).List(ctx)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	for index := range items {
		h.setPublicURL(&items[index])
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *Handler) CreateLink(c *gin.Context) {
	input := LinkInput{Enabled: true, AttributionMode: "dynamic", TimeSpentThreshold: 10}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "封面链接数据格式无效")
		return
	}
	if input.Code == "" {
		input.Code = runtime.Token()[:8]
	}
	normalizeInput(&input)
	if err := validateInput(input, true); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: h.DB}).Create(ctx, input, h.Config.AdminUser)
	h.writeLink(c, item, err)
}

func (h *Handler) UpdateLink(c *gin.Context) {
	id, ok := positiveID(c)
	if !ok {
		return
	}
	var input LinkInput
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "封面链接数据格式无效")
		return
	}
	normalizeInput(&input)
	if err := validateInput(input, false); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: h.DB}).Update(ctx, id, input, h.Config.AdminUser)
	h.writeLink(c, item, err)
}

func (h *Handler) DeleteLink(c *gin.Context) {
	id, ok := positiveID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	err := (Repository{DB: h.DB}).Delete(ctx, id, h.Config.AdminUser)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		c.Status(404)
	case err != nil:
		runtime.ServerError(c, err)
	default:
		c.Status(204)
	}
}

type deleteLinksRequest struct {
	IDs []int64 `json:"ids"`
}

// DeleteLinks archives 1–1000 cover links in one transaction and keeps all historical statistics.
func (h *Handler) DeleteLinks(c *gin.Context) {
	var input deleteLinksRequest
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if c.ContentType() != "application/json" || decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		runtime.Bad(c, "请使用 JSON 提交要删除的封面链接")
		return
	}
	if len(input.IDs) == 0 || len(input.IDs) > 1000 {
		runtime.Bad(c, "请选择 1–1000 条封面链接")
		return
	}
	seen := make(map[int64]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if id < 1 {
			runtime.Bad(c, "封面链接编号无效")
			return
		}
		if _, exists := seen[id]; exists {
			runtime.Bad(c, "封面链接编号不能重复")
			return
		}
		seen[id] = struct{}{}
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	deleted, err := (Repository{DB: h.DB}).DeleteBatch(ctx, input.IDs, h.Config.AdminUser)
	if errors.Is(err, ErrLinksNotFound) {
		c.JSON(404, gin.H{"error": "部分封面链接不存在，请刷新后重试"})
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"deleted": deleted})
}

func (h *Handler) writeLink(c *gin.Context, item Link, err error) {
	switch {
	case errors.Is(err, ErrBindingLocked):
		c.JSON(409, gin.H{"error": "该链接已有访问记录，广告平台和 Pixel 已锁定；请新建链接"})
	case errors.Is(err, ErrPixelInvalid):
		runtime.Bad(c, "广告平台或 Pixel 配置无效")
	case errors.Is(err, pgx.ErrNoRows):
		c.Status(404)
	case err != nil && strings.Contains(err.Error(), "23505"):
		c.JSON(409, gin.H{"error": "短码已存在"})
	case err != nil:
		runtime.ServerError(c, err)
	default:
		h.setPublicURL(&item)
		c.JSON(200, item)
	}
}

func (h *Handler) setPublicURL(item *Link) {
	item.PublicURL = strings.TrimRight(h.Config.PublicURL, "/") + "/cover/" + url.PathEscape(item.Code)
	if item.AdPlatform == "tiktok" {
		item.TikTokTemplateURL = item.PublicURL + "?utm_source=tiktok&utm_medium=paid_social&campaign_id=__CAMPAIGN_ID__&adgroup_id=__AID__&creative_id=__CID__&ad_id_v2=__ADID_V2__&placement=__PLACEMENT__"
	}
}

// View confirms one deduplicated PageView after the browser has loaded the standalone app.
func (h *Handler) View(c *gin.Context) {
	eventID, link, ok := h.validTicket(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	fbc, _ := c.Cookie("_fbc")
	fbp, _ := c.Cookie("_fbp")
	event, err := (landing.Repository{DB: h.DB, Meta: h.Meta, TikTok: h.TikTok}).MarkView(ctx, eventID, link.ID, "cover", meta.ContactContext{IP: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"), FBC: fbc, FBP: fbp})
	if err == nil {
		response := gin.H{"ok": true}
		if event.EventID != "" {
			response["tiktok_event"] = event
		}
		c.JSON(200, response)
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(400)
		return
	}
	runtime.ServerError(c, err)
}

// TimeSpent uses the shared server-observed threshold before queuing Meta or TikTok.
func (h *Handler) TimeSpent(c *gin.Context) {
	proxy := landing.Handler{Core: h.Core, Meta: h.Meta, TikTok: h.TikTok}
	proxy.TimeSpentForSurface(c, "cover", "contact:cover:")
}

// VisibleTime stores monotonic foreground time for the cover-link report only.
func (h *Handler) VisibleTime(c *gin.Context) {
	eventID, link, ok := h.validTicket(c)
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
	var accepted int
	err = h.DB.QueryRow(ctx, `WITH target AS (
		SELECT id,LEAST($3,7200,GREATEST(0,FLOOR(EXTRACT(EPOCH FROM (now()-occurred_at)))::int+5)) AS accepted
		FROM click_events WHERE id=$1 AND link_id=$2 AND surface='cover' AND method='GET'
		AND event_type='landing' AND classification='normal' AND occurred_at>=now()-interval '2 hours 5 minutes' FOR UPDATE
	) UPDATE click_events e SET visible_updated_at=CASE WHEN target.accepted>e.visible_seconds THEN now() ELSE e.visible_updated_at END,
	visible_seconds=GREATEST(e.visible_seconds,target.accepted) FROM target WHERE e.id=target.id RETURNING e.visible_seconds`,
		eventID, link.ID, seconds).Scan(&accepted)
	if errors.Is(err, pgx.ErrNoRows) {
		runtime.Bad(c, "访问票据无效或已过期")
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"visible_seconds": accepted})
}

func (h *Handler) validTicket(c *gin.Context) (string, links.Link, bool) {
	if origin := c.GetHeader("Origin"); origin != "" && origin != "null" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != c.Request.Host {
			c.Status(403)
			return "", links.Link{}, false
		}
	}
	parts := strings.Split(c.PostForm("ticket"), ".")
	if len(parts) != 2 || !runtime.VisitorPattern.MatchString(parts[0]) || !hmac.Equal([]byte(parts[1]), []byte(h.Sign("contact:cover:"+c.Param("code")+":"+parts[0]))) {
		c.Status(400)
		return "", links.Link{}, false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: h.DB}).ByCode(ctx, c.Param("code"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return "", links.Link{}, false
	}
	if err != nil {
		runtime.ServerError(c, err)
		return "", links.Link{}, false
	}
	if !link.Enabled || link.ProductType != ProductType {
		c.Status(410)
		return "", links.Link{}, false
	}
	return parts[0], link, true
}

func positiveID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		runtime.Bad(c, "封面链接编号无效")
		return 0, false
	}
	return id, true
}

func decodeJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
