package audionovel

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/modules/links"
	"whatsapp-analytics/internal/platform/runtime"
)

func (a *Handler) PublicHome(c *gin.Context) {
	if !a.validPublicCode(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).PublicHome(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(200, gin.H{"featured": nil})
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	item.BodyMarkdown = ""
	c.JSON(200, gin.H{"featured": item})
}
func (a *Handler) PublicList(c *gin.Context) {
	if !a.validPublicCode(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "6"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := (Repository{DB: a.DB}).PublicList(ctx, page, pageSize)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, result)
}
func (a *Handler) PublicStory(c *gin.Context) {
	if !a.validPublicCode(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: a.DB}
	item, err := repo.PublicBySlug(ctx, c.Param("slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	related, err := repo.Related(ctx, item, 3)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"story": item, "related": related})
}

func (a *Handler) PublicAudioList(c *gin.Context) {
	if !a.validPublicCode(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "6"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := (Repository{DB: a.DB}).PublicAudioList(ctx, page, pageSize)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, result)
}

func (a *Handler) PublicAudioStory(c *gin.Context) {
	if !a.validPublicCode(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).PublicAudioBySlug(ctx, c.Param("slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"audio": item})
}

// 公开内容沿用同一条短链接配置，但读取语音小说时不新增访问事件。
func (a *Handler) validPublicCode(c *gin.Context) bool {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: a.DB}).ByCode(ctx, c.Param("code"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return false
	}
	if err != nil {
		runtime.ServerError(c, err)
		return false
	}
	if !link.Enabled {
		c.Status(410)
		return false
	}
	return true
}
