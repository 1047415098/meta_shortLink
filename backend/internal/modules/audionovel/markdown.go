package audionovel

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

var markdownCommentPattern = regexp.MustCompile(`(?s)<!--.*?-->`)

// RenderMarkdown 使用 goldmark 的安全默认渲染，并在渲染前移除所有图片节点。
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
	for _, image := range images {
		if parent := image.Parent(); parent != nil {
			parent.RemoveChild(parent, image)
		}
	}
	var output bytes.Buffer
	if err := markdown.Renderer().Render(&output, []byte(source), document); err != nil {
		return ""
	}
	// 默认渲染器用注释替代原始 HTML；公开输出不保留这些实现细节。
	return strings.TrimSpace(markdownCommentPattern.ReplaceAllString(output.String(), ""))
}
