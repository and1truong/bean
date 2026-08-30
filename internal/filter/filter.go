// Package filter applies compiled, deterministic content-formatting pipelines.
package filter

import (
	"bytes"
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
)

var (
	markdown  = goldmark.New(goldmark.WithExtensions(extension.GFM))
	sanitizer = newPolicy()
)

// Apply runs a compiled Filter and returns safe HTML. Sanitization is mandatory
// and cannot be disabled by application metadata.
func Apply(definition appir.Filter, source string) (string, error) {
	value := source
	for _, step := range definition.Steps {
		switch step.Type {
		case "markdown":
			var output bytes.Buffer
			if err := markdown.Convert([]byte(value), &output); err != nil {
				return "", fmt.Errorf("markdown filter: %w", err)
			}
			value = output.String()
		default:
			return "", fmt.Errorf("unsupported filter step %q", step.Type)
		}
	}
	return sanitizer.Sanitize(value), nil
}

func newPolicy() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("p", "br", "strong", "em", "del", "ul", "ol", "li", "blockquote", "code", "pre", "h1", "h2", "h3", "h4", "h5", "h6", "hr", "table", "thead", "tbody", "tr", "th", "td", "a")
	p.AllowAttrs("href", "title").OnElements("a")
	p.AllowURLSchemes("http", "https", "mailto")
	p.RequireNoFollowOnLinks(true)
	p.RequireNoReferrerOnLinks(true)
	return p
}
