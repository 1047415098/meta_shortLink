package audionovel

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

func (a *Handler) ListDistributionLinks(c *gin.Context) {
	audioNovelID, err := strconv.ParseInt(c.Query("audio_novel_id"), 10, 64)
	if err != nil || audioNovelID < 1 {
		runtime.Bad(c, "语音小说编号无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := (Repository{DB: a.DB}).ListDistributionLinks(ctx, audioNovelID)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	for index := range items {
		a.setDistributionPublicURL(&items[index])
	}
	c.JSON(200, gin.H{"items": items})
}

func (a *Handler) CreateDistributionLink(c *gin.Context) {
	input := DistributionInput{
		Enabled: true, AdPlatform: "meta", AttributionMode: "dynamic", TimeSpentThreshold: 10,
	}
	if decodeAudioNovelJSON(c, &input) != nil {
		runtime.Bad(c, "投放链接数据格式无效")
		return
	}
	if input.Code == "" {
		input.Code = runtime.Token()[:8]
	}
	NormalizeDistributionInput(&input)
	if err := ValidateDistributionInput(input, true); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).CreateDistributionLink(ctx, input, a.Config.AdminUser)
	a.writeDistributionLink(c, item, err)
}

func (a *Handler) UpdateDistributionLink(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	var input DistributionInput
	if decodeAudioNovelJSON(c, &input) != nil {
		runtime.Bad(c, "投放链接数据格式无效")
		return
	}
	NormalizeDistributionInput(&input)
	if err := ValidateDistributionInput(input, false); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).UpdateDistributionLink(ctx, id, input, a.Config.AdminUser)
	a.writeDistributionLink(c, item, err)
}

func (a *Handler) DeleteDistributionLink(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	err := (Repository{DB: a.DB}).DeleteDistributionLink(ctx, id, a.Config.AdminUser)
	switch {
	case errors.Is(err, ErrDistributionHasVisits):
		c.JSON(409, gin.H{"error": "该链接已有访问记录，只能停用，不能删除"})
	case errors.Is(err, pgx.ErrNoRows):
		c.Status(404)
	case err != nil:
		runtime.ServerError(c, err)
	default:
		c.Status(204)
	}
}

func (a *Handler) writeDistributionLink(c *gin.Context, item DistributionLink, err error) {
	switch {
	case errors.Is(err, ErrDistributionBindingLocked):
		c.JSON(409, gin.H{"error": "该链接已有访问记录，语音小说、平台和 Pixel 已锁定；请新建链接"})
	case errors.Is(err, ErrDistributionContentInvalid):
		runtime.Bad(c, "语音小说必须已启用并上传 MP3")
	case errors.Is(err, ErrDistributionPixelInvalid):
		runtime.Bad(c, "广告平台或 Pixel 配置无效")
	case errors.Is(err, pgx.ErrNoRows):
		c.Status(404)
	case err != nil:
		if strings.Contains(err.Error(), "23505") {
			c.JSON(409, gin.H{"error": "短码已存在"})
		} else if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "23514") {
			runtime.Bad(c, "语音小说、广告平台或 Pixel 配置无效")
		} else {
			runtime.ServerError(c, err)
		}
	default:
		a.setDistributionPublicURL(&item)
		c.JSON(200, item)
	}
}

func (a *Handler) setDistributionPublicURL(item *DistributionLink) {
	item.PublicURL = strings.TrimRight(a.Config.PublicURL, "/") + "/audio-novel/" + item.Code
	if item.AdPlatform == "tiktok" {
		item.TikTokTemplateURL = AudioTikTokTemplate(a.Config.PublicURL, item.Code)
	}
}
