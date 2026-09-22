package novel

import (
	"strings"
	"testing"
)

func TestRenderMarkdownKeepsReadingBlocksAndRemovesUnsafeContent(t *testing.T) {
	got := RenderMarkdown("# Chapter\n\nParagraph with **weight**.\n\n- one\n- two\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(2)) ![image](https://evil.test/a.png)")
	for _, expected := range []string{"<h1>", "<p>", "<strong>", "<ul>"} {
		if !strings.Contains(got, expected) {
			t.Fatalf("missing %s in %q", expected, got)
		}
	}
	for _, unsafe := range []string{"<script", "javascript:", "<img"} {
		if strings.Contains(strings.ToLower(got), unsafe) {
			t.Fatalf("unsafe output %q", got)
		}
	}
}
