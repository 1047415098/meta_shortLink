package novel

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

type Novel struct {
	ID             int64      `json:"id"`
	Title          string     `json:"title"`
	Slug           string     `json:"slug"`
	Author         string     `json:"author"`
	Category       string     `json:"category"`
	Excerpt        string     `json:"excerpt"`
	CoverPath      string     `json:"cover_path"`
	PublishedAt    string     `json:"published_at"`
	Enabled        bool       `json:"enabled"`
	Featured       bool       `json:"featured"`
	SortOrder      int        `json:"sort_order"`
	SourceRevision int64      `json:"source_revision"`
	ChapterCount   int        `json:"chapter_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

type NovelInput struct {
	Title       string `json:"title"`
	Slug        string `json:"slug"`
	Author      string `json:"author"`
	Category    string `json:"category"`
	Excerpt     string `json:"excerpt"`
	CoverPath   string `json:"cover_path"`
	PublishedAt string `json:"published_at"`
	Enabled     bool   `json:"enabled"`
	Featured    bool   `json:"featured"`
	SortOrder   int    `json:"sort_order"`
}

type Chapter struct {
	ID            int64      `json:"id"`
	NovelID       int64      `json:"novel_id,omitempty"`
	ChapterNumber int        `json:"chapter_number"`
	Title         string     `json:"title"`
	BodyMarkdown  string     `json:"body_markdown,omitempty"`
	BodyHTML      string     `json:"body_html,omitempty"`
	Enabled       bool       `json:"enabled"`
	CreatedAt     time.Time  `json:"created_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at,omitempty"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

type ChapterInput struct {
	ChapterNumber int    `json:"chapter_number"`
	Title         string `json:"title"`
	BodyMarkdown  string `json:"body_markdown"`
	Enabled       bool   `json:"enabled"`
}

type ListFilter struct {
	Query, Status  string
	Page, PageSize int
}

type NovelList struct {
	Items    []Novel `json:"items"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	Total    int64   `json:"total"`
	Pages    int     `json:"pages"`
}

var slugPattern = regexp.MustCompile(`^[\p{L}\p{N}]+(?:-[\p{L}\p{N}]+)*$`)
var coverPattern = regexp.MustCompile(`^/novel-uploads/[a-f0-9]{32}\.(jpg|png|webp)$`)

func ValidateNovelInput(input NovelInput) error {
	if strings.TrimSpace(input.Title) == "" || len([]rune(input.Title)) > 160 {
		return errors.New("标题不能为空且不能超过 160 个字符")
	}
	if !slugPattern.MatchString(input.Slug) || input.Slug != strings.ToLower(input.Slug) || len([]rune(input.Slug)) > 120 {
		return errors.New("slug 只能包含中文、小写字母、数字和单横线，且不能超过 120 个字符")
	}
	if len([]rune(input.Author)) > 100 || len([]rune(input.Category)) > 80 {
		return errors.New("作者或分类过长")
	}
	if strings.TrimSpace(input.Excerpt) == "" || len([]rune(input.Excerpt)) > 500 {
		return errors.New("摘要不能为空且不能超过 500 个字符")
	}
	if input.CoverPath != "" && !coverPattern.MatchString(input.CoverPath) {
		return errors.New("封面路径无效")
	}
	if _, err := time.Parse("2006-01-02", input.PublishedAt); err != nil {
		return errors.New("发布日期必须使用 YYYY-MM-DD 格式")
	}
	return nil
}

func ValidateChapterInput(input ChapterInput) error {
	if input.ChapterNumber < 1 {
		return errors.New("章节序号必须大于 0")
	}
	if len([]rune(input.Title)) > 160 {
		return errors.New("章节标题不能超过 160 个字符")
	}
	length := len([]rune(strings.TrimSpace(input.BodyMarkdown)))
	if length < 1 || length > 200000 {
		return errors.New("章节正文不能为空且不能超过 200000 个字符")
	}
	return nil
}
