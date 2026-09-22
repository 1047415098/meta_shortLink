package novel

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"whatsapp-analytics/internal/platform/runtime"
)

func (h *Handler) ListAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := (Repository{DB: h.DB}).ListAdmin(ctx, ListFilter{Query: c.Query("q"), Status: c.Query("status"), Page: page, PageSize: pageSize})
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, result)
}

func (h *Handler) GetAdmin(c *gin.Context) {
	id, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: h.DB}).ByID(ctx, id)
	writeNovel(c, item, err)
}

func (h *Handler) CreateAdmin(c *gin.Context) { h.writeAdmin(c, 0) }
func (h *Handler) UpdateAdmin(c *gin.Context) {
	id, ok := positiveID(c, "id", "小说编号无效")
	if ok {
		h.writeAdmin(c, id)
	}
}

func (h *Handler) writeAdmin(c *gin.Context, id int64) {
	var input NovelInput
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "小说数据格式无效")
		return
	}
	if err := ValidateNovelInput(input); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	var item Novel
	var err error
	if id == 0 {
		item, err = repo.Create(ctx, input, h.Config.AdminUser)
	} else {
		item, err = repo.Update(ctx, id, input, h.Config.AdminUser)
	}
	writeNovel(c, item, err)
}

func (h *Handler) DeleteAdmin(c *gin.Context) {
	id, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := (Repository{DB: h.DB}).SoftDelete(ctx, id, h.Config.AdminUser); errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		c.Status(204)
	}
}

func (h *Handler) SetEnabled(c *gin.Context)  { h.setFlag(c, false) }
func (h *Handler) SetFeatured(c *gin.Context) { h.setFlag(c, true) }
func (h *Handler) setFlag(c *gin.Context, featured bool) {
	id, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	var input struct {
		Enabled  *bool `json:"enabled"`
		Featured *bool `json:"featured"`
	}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "请求格式无效")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	var item Novel
	var err error
	if featured {
		if input.Featured == nil {
			runtime.Bad(c, "请提交 featured")
			return
		}
		item, err = repo.SetFeatured(ctx, id, *input.Featured, h.Config.AdminUser)
	} else {
		if input.Enabled == nil {
			runtime.Bad(c, "请提交 enabled")
			return
		}
		item, err = repo.SetEnabled(ctx, id, *input.Enabled, h.Config.AdminUser)
	}
	writeNovel(c, item, err)
}

func (h *Handler) ListChaptersAdmin(c *gin.Context) {
	id, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	items, err := (Repository{DB: h.DB}).ListChapters(ctx, id, false)
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items})
}

func (h *Handler) CreateChapterAdmin(c *gin.Context) { h.writeChapter(c, 0) }
func (h *Handler) UpdateChapterAdmin(c *gin.Context) {
	chapterID, ok := positiveID(c, "chapterId", "章节编号无效")
	if ok {
		h.writeChapter(c, chapterID)
	}
}
func (h *Handler) writeChapter(c *gin.Context, chapterID int64) {
	novelID, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	var input ChapterInput
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "章节数据格式无效")
		return
	}
	if err := ValidateChapterInput(input); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	repo := Repository{DB: h.DB}
	var item Chapter
	var err error
	if chapterID == 0 {
		item, err = repo.CreateChapter(ctx, novelID, input, h.Config.AdminUser)
	} else {
		item, err = repo.UpdateChapter(ctx, novelID, chapterID, input, h.Config.AdminUser)
	}
	writeChapter(c, item, err)
}

func (h *Handler) DeleteChapterAdmin(c *gin.Context) {
	novelID, ok := positiveID(c, "id", "小说编号无效")
	if !ok {
		return
	}
	chapterID, ok := positiveID(c, "chapterId", "章节编号无效")
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := (Repository{DB: h.DB}).DeleteChapter(ctx, novelID, chapterID, h.Config.AdminUser); errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		runtime.ServerError(c, err)
	} else {
		c.Status(204)
	}
}

func (h *Handler) Preview(c *gin.Context) {
	var input struct {
		BodyMarkdown string `json:"body_markdown"`
	}
	if decodeJSON(c, &input) != nil {
		runtime.Bad(c, "Markdown 格式无效")
		return
	}
	c.JSON(200, gin.H{"body_html": RenderMarkdown(input.BodyMarkdown)})
}

func positiveID(c *gin.Context, name, message string) (int64, bool) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id < 1 {
		runtime.Bad(c, message)
		return 0, false
	}
	return id, true
}
func decodeJSON(c *gin.Context, target any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("只能提交一个 JSON 对象")
	}
	return nil
}
func writeNovel(c *gin.Context, item Novel, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		if strings.Contains(err.Error(), "23505") {
			c.JSON(409, gin.H{"error": "slug 已存在或推荐状态冲突"})
		} else {
			runtime.ServerError(c, err)
		}
	} else {
		c.JSON(200, item)
	}
}
func writeChapter(c *gin.Context, item Chapter, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
	} else if err != nil {
		if strings.Contains(err.Error(), "23505") {
			c.JSON(409, gin.H{"error": "章节序号已存在"})
		} else {
			runtime.ServerError(c, err)
		}
	} else {
		c.JSON(200, item)
	}
}
