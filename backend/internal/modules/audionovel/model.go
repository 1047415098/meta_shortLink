package audionovel

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// AudioNovel 是管理端和公开接口共享的语音小说数据，不包含作者字段。
type AudioNovel struct {
	ID                   int64      `json:"id"`
	Title                string     `json:"title"`
	Slug                 string     `json:"slug"`
	Category             string     `json:"category"`
	Excerpt              string     `json:"excerpt"`
	BodyMarkdown         string     `json:"body_markdown,omitempty"`
	BodyHTML             string     `json:"body_html,omitempty"`
	CoverPath            string     `json:"cover_path"`
	AudioPath            string     `json:"audio_path"`
	AudioDuration        string     `json:"audio_duration"`
	AudioDurationSeconds int        `json:"audio_duration_seconds"`
	AudioSizeBytes       int64      `json:"audio_size_bytes"`
	PublishedAt          string     `json:"published_at"`
	Enabled              bool       `json:"enabled"`
	Featured             bool       `json:"featured"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	DeletedAt            *time.Time `json:"deleted_at,omitempty"`
}

type AudioNovelInput struct {
	Title                string `json:"title"`
	Slug                 string `json:"slug"`
	Category             string `json:"category"`
	Excerpt              string `json:"excerpt"`
	BodyMarkdown         string `json:"body_markdown"`
	CoverPath            string `json:"cover_path"`
	AudioPath            string `json:"audio_path"`
	AudioDuration        string `json:"audio_duration"`
	AudioDurationSeconds int    `json:"audio_duration_seconds"`
	AudioSizeBytes       int64  `json:"audio_size_bytes"`
	PublishedAt          string `json:"published_at"`
	Enabled              bool   `json:"enabled"`
	Featured             bool   `json:"featured"`
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
var audioNovelAudioPattern = regexp.MustCompile(`^/audio-novel-audio/[a-f0-9]{32}\.mp3$`)
var audioDurationPattern = regexp.MustCompile(`^(?:[0-9]+:)?[0-5][0-9]:[0-5][0-9]$`)

func parseAudioDurationSeconds(value string) (int, bool) {
	if !audioDurationPattern.MatchString(value) {
		return 0, false
	}
	parts := strings.Split(value, ":")
	values := make([]int, len(parts))
	for index, part := range parts {
		parsed, err := strconv.Atoi(part)
		if err != nil {
			return 0, false
		}
		values[index] = parsed
	}
	seconds := values[0]*60 + values[1]
	if len(values) == 3 {
		seconds = values[0]*3600 + values[1]*60 + values[2]
	}
	// The integer value drives completion statistics, so reject empty and
	// implausibly long metadata even when its display text is well formed.
	return seconds, seconds >= 1 && seconds <= 86400
}

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
	// Podcast 元数据必须成组出现，防止播放地址和统计时长互相矛盾。
	if input.AudioPath == "" && input.AudioDuration == "" && input.AudioDurationSeconds == 0 && input.AudioSizeBytes == 0 {
		// 没有 Podcast 的纯文字内容仍然有效。
	} else if !audioNovelAudioPattern.MatchString(input.AudioPath) || input.AudioDurationSeconds <= 0 || input.AudioSizeBytes <= 0 {
		return errors.New("音频路径、时长和文件大小必须完整且有效")
	} else if parsedSeconds, ok := parseAudioDurationSeconds(input.AudioDuration); !ok {
		return errors.New("音频时长格式无效")
	} else {
		difference := parsedSeconds - input.AudioDurationSeconds
		if difference < 0 {
			difference = -difference
		}
		if difference > 1 {
			return errors.New("音频显示时长与秒数不一致")
		}
	}
	if _, err := time.Parse("2006-01-02", input.PublishedAt); err != nil {
		return errors.New("发布日期必须使用 YYYY-MM-DD 格式")
	}
	return nil
}
