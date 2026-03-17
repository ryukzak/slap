package util

import (
	"html/template"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/microcosm-cc/bluemonday"
)

var policy = bluemonday.UGCPolicy()

func RenderMarkdown(input string) template.HTML {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock | parser.Autolink
	p := parser.NewWithExtensions(extensions)

	doc := p.Parse([]byte(input))

	opts := html.RendererOptions{
		Flags:          html.CommonFlags,
		RenderNodeHook: nil,
	}
	renderer := html.NewRenderer(opts)

	raw := markdown.Render(doc, renderer)
	return template.HTML(policy.SanitizeBytes(raw)) //nolint:gosec
}
