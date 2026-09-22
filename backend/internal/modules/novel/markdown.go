package novel

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var markdownCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

func RenderMarkdown(source string) string {
	markdown := goldmark.New()
	document := markdown.Parser().Parse(text.NewReader([]byte(source)))
	var images []ast.Node
	_ = ast.Walk(document, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering && node.Kind() == ast.KindImage {
			images = append(images, node)
		}
		return ast.WalkContinue, nil
	})
	// 正文图片只允许通过运营封面流程进入，避免 Markdown 绕过资源与协议检查。
	for _, image := range images {
		if parent := image.Parent(); parent != nil {
			parent.RemoveChild(parent, image)
		}
	}
	var output bytes.Buffer
	if err := markdown.Renderer().Render(&output, []byte(source), document); err != nil {
		return ""
	}
	return strings.TrimSpace(markdownCommentPattern.ReplaceAllString(output.String(), ""))
}
