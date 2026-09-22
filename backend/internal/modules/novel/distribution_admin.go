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

func (h *Handler) ListDistributionLinks(c *gin.Context) {
	novelID, _ := strconv.ParseInt(c.Query("novel_id"), 10, 64)
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := (Repository{DB: h.DB}).ListDistributionLinks(ctx, novelID)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	for i := range items {
		h.setDistributionPublicURL(&items[i])
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *Handler) CreateDistributionLink(c *gin.Context) {
	input := DistributionInput{AttributionMode: "dynamic", Enabled: true}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "投放链接数据格式无效")
		return
	}
	if input.Code == "" {
		input.Code = runtime.Token()[:8]
	}
	if err := ValidateDistributionInput(input, true); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: h.DB}).CreateDistributionLink(ctx, input, h.Config.AdminUser)
	h.writeDistributionLink(c, item, err)
}

func (h *Handler) UpdateDistributionLink(c *gin.Context) {
	id, ok := positiveID(c, "id", "投放链接编号无效")
	if !ok {
		return
	}
	var input DistributionInput
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "投放链接数据格式无效")
		return
	}
	if err := ValidateDistributionInput(input, false); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: h.DB}).UpdateDistributionLink(ctx, id, input, h.Config.AdminUser)
	h.writeDistributionLink(c, item, err)
}

func (h *Handler) DeleteDistributionLink(c *gin.Context) {
	id, ok := positiveID(c, "id", "投放链接编号无效")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	err := (Repository{DB: h.DB}).DeleteDistributionLink(ctx, id, h.Config.AdminUser)
	if errors.Is(err, ErrDistributionHasVisits) {
		c.JSON(409, gin.H{"error": "该链接已有访问记录，只能停用，不能删除"})
	} else if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		c.Status(204)
	}
}

func (h *Handler) writeDistributionLink(c *gin.Context, item DistributionLink, err error) {
	if errors.Is(err, ErrDistributionBindingLocked) {
		c.JSON(409, gin.H{"error": "该链接已有访问记录，小说绑定已锁定；请新建链接"})
	} else if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		if strings.Contains(err.Error(), "23505") {
			c.JSON(409, gin.H{"error": "短码已存在"})
		} else if strings.Contains(err.Error(), "23503") || strings.Contains(err.Error(), "23514") {
			runtime.Bad(c, "小说或 Meta Pixel 配置无效")
		} else {
			runtime.ServerError(c, err)
		}
	} else {
		h.setDistributionPublicURL(&item)
		c.JSON(200, item)
	}
}

func (h *Handler) setDistributionPublicURL(item *DistributionLink) {
	item.PublicURL = strings.TrimRight(h.Config.PublicURL, "/") + "/novel/" + item.Code
}
