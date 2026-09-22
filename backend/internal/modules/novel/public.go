package novel

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

func (h *Handler) PublicHome(c *gin.Context) {
	if !h.validPublicCode(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	featured, err := repo.PublicHome(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(200, gin.H{"featured": nil, "ranking": []Novel{}, "items": []Novel{}})
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	list, err := repo.PublicList(ctx, "", 1, 20)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	ranking := list.Items
	if len(ranking) > 6 {
		ranking = ranking[:6]
	}
	c.JSON(200, gin.H{"featured": featured, "ranking": ranking, "items": list.Items})
}
func (h *Handler) PublicList(c *gin.Context) {
	if !h.validPublicCode(c) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := (Repository{DB: h.DB}).PublicList(ctx, c.Query("q"), page, pageSize)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, result)
}
func (h *Handler) PublicStory(c *gin.Context) {
	if !h.validPublicCode(c) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	item, err := repo.PublicBySlug(ctx, c.Param("slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	chapters, err := repo.ListChapters(ctx, item.ID, true)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	related, err := repo.Related(ctx, item, 3)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"story": item, "chapters": chapters, "related": related})
}
func (h *Handler) PublicChapter(c *gin.Context) {
	if !h.validPublicCode(c) {
		return
	}
	number, err := strconv.Atoi(c.Param("number"))
	if err != nil || number < 1 {
		c.Status(404)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	item, err := repo.PublicBySlug(ctx, c.Param("slug"))
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	chapter, err := repo.PublicChapter(ctx, item.ID, number)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	chapters, err := repo.ListChapters(ctx, item.ID, true)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	var previous, next any
	for i, current := range chapters {
		if current.ChapterNumber == number {
			if i > 0 {
				previous = chapters[i-1]
			}
			if i+1 < len(chapters) {
				next = chapters[i+1]
			}
			break
		}
	}
	c.JSON(200, gin.H{"chapter": chapter, "previous": previous, "next": next})
}
func (h *Handler) validPublicCode(c *gin.Context) bool {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	link, err := (links.Repository{DB: h.DB}).ByCode(ctx, c.Param("code"))
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
	if link.ProductType == "novel" {
		var available bool
		// Every API call follows the bound novel's live status, so re-enabling restores the link.
		if link.NovelID == nil || h.DB.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM novels WHERE id=$1 AND enabled AND deleted_at IS NULL)", link.NovelID).Scan(&available) != nil {
			c.Status(503)
			return false
		}
		if !available {
			c.Status(410)
			return false
		}
	}
	return true
}
