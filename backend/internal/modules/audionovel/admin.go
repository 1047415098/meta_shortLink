package audionovel

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

func (a *Handler) ListAdmin(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := (Repository{DB: a.DB}).ListAdmin(ctx, ListFilter{Query: c.Query("q"), Status: c.Query("status"), Page: page, PageSize: pageSize})
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, result)
}

func (a *Handler) GetAdmin(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).ByID(ctx, id)
	writeAudioNovel(c, item, err)
}

func (a *Handler) CreateAdmin(c *gin.Context) {
	var input AudioNovelInput
	if err := decodeAudioNovelJSON(c, &input); err != nil {
		// 管理端错误提示统一使用当前“语音小说”产品名称。
		runtime.Bad(c, "语音小说数据格式无效")
		return
	}
	if err := ValidateAudioNovelInput(input); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).Create(ctx, input, a.Config.AdminUser)
	writeAudioNovel(c, item, err)
}

func (a *Handler) UpdateAdmin(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	var input AudioNovelInput
	if err := decodeAudioNovelJSON(c, &input); err != nil {
		// 编辑接口与新增接口保持相同的产品命名。
		runtime.Bad(c, "语音小说数据格式无效")
		return
	}
	if err := ValidateAudioNovelInput(input); err != nil {
		runtime.Bad(c, err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).Update(ctx, id, input, a.Config.AdminUser)
	writeAudioNovel(c, item, err)
}

func (a *Handler) DeleteAdmin(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	err := (Repository{DB: a.DB}).SoftDelete(ctx, id, a.Config.AdminUser)
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		runtime.ServerError(c, err)
		return
	}
	c.Status(204)
}

func (a *Handler) SetEnabled(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	var input struct {
		Enabled *bool `json:"enabled"`
	}
	if decodeAudioNovelJSON(c, &input) != nil || input.Enabled == nil {
		runtime.Bad(c, "请提交 enabled")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).SetEnabled(ctx, id, *input.Enabled, a.Config.AdminUser)
	writeAudioNovel(c, item, err)
}
func (a *Handler) SetFeatured(c *gin.Context) {
	id, ok := audioNovelID(c)
	if !ok {
		return
	}
	var input struct {
		Featured *bool `json:"featured"`
	}
	if decodeAudioNovelJSON(c, &input) != nil || input.Featured == nil {
		runtime.Bad(c, "请提交 featured")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	item, err := (Repository{DB: a.DB}).SetFeatured(ctx, id, *input.Featured, a.Config.AdminUser)
	writeAudioNovel(c, item, err)
}

func (a *Handler) Preview(c *gin.Context) {
	var input struct {
		BodyMarkdown string `json:"body_markdown"`
	}
	if decodeAudioNovelJSON(c, &input) != nil {
		runtime.Bad(c, "Markdown 格式无效")
		return
	}
	c.JSON(200, gin.H{"body_html": RenderMarkdown(input.BodyMarkdown)})
}

func audioNovelID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id < 1 {
		// 参数错误也不能回退到已经停用的旧产品名称。
		runtime.Bad(c, "语音小说编号无效")
		return 0, false
	}
	return id, true
}
func decodeAudioNovelJSON(c *gin.Context, target any) error {
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
func writeAudioNovel(c *gin.Context, item AudioNovel, err error) {
	if errors.Is(err, pgx.ErrNoRows) {
		c.Status(404)
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "23505") {
			c.JSON(409, gin.H{"error": "slug 已存在或推荐状态冲突"})
			return
		}
		runtime.ServerError(c, err)
		return
	}
	c.JSON(200, item)
}
