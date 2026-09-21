package audionovel

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

// AudioNovel 是管理端和公开接口共享的语音小说数据，不包含作者字段。
type AudioNovel struct {
	ID           int64      `json:"id"`
	Title        string     `json:"title"`
	Slug         string     `json:"slug"`
	Category     string     `json:"category"`
	Excerpt      string     `json:"excerpt"`
	BodyMarkdown string     `json:"body_markdown,omitempty"`
	BodyHTML     string     `json:"body_html,omitempty"`
	CoverPath    string     `json:"cover_path"`
	PublishedAt  string     `json:"published_at"`
	Enabled      bool       `json:"enabled"`
	Featured     bool       `json:"featured"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"deleted_at,omitempty"`
}

type AudioNovelInput struct {
	Title        string `json:"title"`
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	Excerpt      string `json:"excerpt"`
	BodyMarkdown string `json:"body_markdown"`
	CoverPath    string `json:"cover_path"`
	PublishedAt  string `json:"published_at"`
	Enabled      bool   `json:"enabled"`
	Featured     bool   `json:"featured"`
}

type ListFilter struct {
	Query    string
	Status   string
	Page     int
	PageSize int
}

type AudioNovelList struct {
	Items    []AudioNovel `json:"items"`
	Page     int          `json:"page"`
	PageSize int          `json:"page_size"`
	Total    int64        `json:"total"`
	Pages    int          `json:"pages"`
}

// slug 支持中文等 Unicode 字母，同时继续限制为小写、数字和单横线结构。
var audioNovelSlugPattern = regexp.MustCompile(`^[\p{L}\p{N}]+(?:-[\p{L}\p{N}]+)*$`)
var audioNovelCoverPattern = regexp.MustCompile(`^/audio-novel-uploads/[a-f0-9]{32}\.(jpg|png|webp)$`)

func ValidateAudioNovelInput(input AudioNovelInput) error {
	input.Title = strings.TrimSpace(input.Title)
	input.Category = strings.TrimSpace(input.Category)
	input.Excerpt = strings.TrimSpace(input.Excerpt)
	input.BodyMarkdown = strings.TrimSpace(input.BodyMarkdown)
	if input.Title == "" || len([]rune(input.Title)) > 160 {
		return errors.New("标题不能为空且不能超过 160 个字符")
	}
	if !audioNovelSlugPattern.MatchString(input.Slug) || input.Slug != strings.ToLower(input.Slug) || len([]rune(input.Slug)) > 120 {
		return errors.New("slug 只能包含中文、小写字母、数字和单横线，且不能超过 120 个字符")
	}
	if input.Category == "" || len([]rune(input.Category)) > 80 {
		return errors.New("分类不能为空且不能超过 80 个字符")
	}
	if input.Excerpt == "" || len([]rune(input.Excerpt)) > 500 {
		return errors.New("摘要不能为空且不能超过 500 个字符")
	}
	if input.BodyMarkdown == "" {
		return errors.New("正文不能为空")
	}
	if input.CoverPath != "" && !audioNovelCoverPattern.MatchString(input.CoverPath) {
		return errors.New("封面路径无效")
	}
	if _, err := time.Parse("2006-01-02", input.PublishedAt); err != nil {
		return errors.New("发布日期必须使用 YYYY-MM-DD 格式")
	}
	return nil
}
