package novel

import (
	"strings"
	"testing"
)

func TestValidateNovelAndChapterInput(t *testing.T) {
	valid := NovelInput{Title: "Story", Slug: "玻璃果园-one", Author: "Nine", Excerpt: "Excerpt", CoverPath: "/novel-uploads/0123456789abcdef0123456789abcdef.webp", PublishedAt: "2026-09-21", Enabled: true}
	if err := ValidateNovelInput(valid); err != nil {
		t.Fatalf("valid novel rejected: %v", err)
	}
	invalid := []NovelInput{
		{Slug: valid.Slug, Excerpt: valid.Excerpt, PublishedAt: valid.PublishedAt},
		{Title: strings.Repeat("x", 161), Slug: valid.Slug, Excerpt: valid.Excerpt, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: "Bad Slug", Excerpt: valid.Excerpt, PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Excerpt: strings.Repeat("x", 501), PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Excerpt: valid.Excerpt, CoverPath: "/tmp/a.webp", PublishedAt: valid.PublishedAt},
		{Title: valid.Title, Slug: valid.Slug, Excerpt: valid.Excerpt, PublishedAt: "21-09-2026"},
	}
	for i, input := range invalid {
		if ValidateNovelInput(input) == nil {
			t.Fatalf("novel case %d should fail", i)
		}
	}
	if ValidateChapterInput(ChapterInput{ChapterNumber: 1, Title: "Prologue", BodyMarkdown: "Body", Enabled: true}) != nil {
		t.Fatal("valid chapter rejected")
	}
	for _, input := range []ChapterInput{{ChapterNumber: 0, BodyMarkdown: "Body"}, {ChapterNumber: 1}, {ChapterNumber: 1, BodyMarkdown: strings.Repeat("x", 200001)}} {
		if ValidateChapterInput(input) == nil {
			t.Fatalf("invalid chapter accepted: %#v", input)
		}
	}
}

func TestNovelCarriesEnglishSourceRevision(t *testing.T) {
	item := Novel{SourceRevision: 7}
	if item.SourceRevision != 7 {
		t.Fatalf("source revision = %d, want 7", item.SourceRevision)
	}
}
