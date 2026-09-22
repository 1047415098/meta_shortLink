package audionovel

import (
	"strings"
	"testing"
)

func TestValidateAudioNovelInput(t *testing.T) {
	valid := AudioNovelInput{Title: "The Glass Orchard", Slug: "the-glass-orchard", Category: "Fantasy", Excerpt: "A short introduction.", BodyMarkdown: "# Chapter one\n\nStory.", CoverPath: "/audio-novel-uploads/0123456789abcdef0123456789abcdef.webp", PublishedAt: "2026-09-20", Enabled: true}
	if err := ValidateAudioNovelInput(valid); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	chineseSlug := valid
	chineseSlug.Slug = "玻璃果园-第一章"
	if err := ValidateAudioNovelInput(chineseSlug); err != nil {
		t.Fatalf("Chinese slug rejected: %v", err)
	}
	cases := []AudioNovelInput{
		{Slug: valid.Slug, Category: valid.Category, Excerpt: valid.Excerpt, BodyMarkdown: valid.BodyMarkdown, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: "Bad Slug", Category: valid.Category, Excerpt: valid.Excerpt, BodyMarkdown: valid.BodyMarkdown, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Excerpt: valid.Excerpt, BodyMarkdown: valid.BodyMarkdown, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Category: valid.Category, Excerpt: strings.Repeat("x", 501), BodyMarkdown: valid.BodyMarkdown, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Category: valid.Category, Excerpt: valid.Excerpt, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Category: valid.Category, Excerpt: valid.Excerpt, BodyMarkdown: valid.BodyMarkdown, PublishedAt: "20-09-2026"},
		{Title: valid.Title, Slug: valid.Slug, Category: valid.Category, Excerpt: valid.Excerpt, BodyMarkdown: valid.BodyMarkdown, CoverPath: "/tmp/a.webp", PublishedAt: valid.PublishedAt},
	}
	for index, input := range cases {
		if err := ValidateAudioNovelInput(input); err == nil {
			t.Fatalf("case %d should be rejected", index)
		}
	}
}

func TestValidateAudioNovelInputRequiresCompleteAudioMetadata(t *testing.T) {
	base := AudioNovelInput{Title: "The Glass Orchard", Slug: "the-glass-orchard", Category: "Fantasy", Excerpt: "A short introduction.", BodyMarkdown: "Story.", PublishedAt: "2026-09-20", Enabled: true}
	valid := base
	valid.AudioPath = "/audio-novel-audio/0123456789abcdef0123456789abcdef.mp3"
	valid.AudioDuration = "32:05"
	valid.AudioSizeBytes = 23_100_419
	if err := ValidateAudioNovelInput(valid); err != nil {
		t.Fatalf("valid audio metadata rejected: %v", err)
	}

	cases := []AudioNovelInput{base}
	missingDuration := valid
	missingDuration.AudioDuration = ""
	cases = append(cases, missingDuration)
	wrongPrefix := valid
	wrongPrefix.AudioPath = "/novel-audio/0123456789abcdef0123456789abcdef.mp3"
	cases = append(cases, wrongPrefix)
	invalidSeconds := valid
	invalidSeconds.AudioDuration = "32:60"
	cases = append(cases, invalidSeconds)
	zeroSize := valid
	zeroSize.AudioSizeBytes = 0
	cases = append(cases, zeroSize)

	if err := ValidateAudioNovelInput(cases[0]); err != nil {
		t.Fatalf("empty audio metadata group rejected: %v", err)
	}
	for index, input := range cases[1:] {
		if err := ValidateAudioNovelInput(input); err == nil {
			t.Fatalf("invalid audio case %d accepted", index)
		}
	}
}

func TestRenderMarkdownSupportsEditorialBlocks(t *testing.T) {
	got := RenderMarkdown("# Heading\n\nParagraph with **weight**.\n\n- one\n- two\n\n> quote\n\n---")
	for _, expected := range []string{"<h1>", "<p>", "<strong>", "<ul>", "<blockquote>", "<hr"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("missing %s in %q", expected, got)
		}
	}
}

func TestRenderMarkdownRemovesUnsafeContent(t *testing.T) {
	got := RenderMarkdown("<script onclick=\"bad()\">alert(1)</script>\n\n[bad](javascript:alert(2)) ![image](https://evil.test/a.png)")
	for _, unsafe := range []string{"<script", "javascript:", "onclick", "<img"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("unsafe output %q", got)
		}
	}
}
