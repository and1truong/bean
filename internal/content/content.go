// Package content defines Bean's bounded semantic content vocabulary.
package content

import (
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/appir"
)

const (
	MaxElements     = 12
	MaxBulletItems  = 6
	MaxDiagramItems = 8
	MaxCodeLines    = 120
)

func Types() []string {
	return []string{"bullets", "callout", "code", "diagram", "heading", "image", "paragraph", "quote"}
}

func Tones() []string { return []string{"info", "success", "warning"} }

func Directions() []string { return []string{"horizontal", "vertical"} }

func Normalize(elements []appir.ContentElement) {
	for index := range elements {
		if elements[index].Type == "callout" && elements[index].Tone == "" {
			elements[index].Tone = "info"
		}
		if elements[index].Type == "diagram" && elements[index].Direction == "" {
			elements[index].Direction = "horizontal"
		}
	}
}

func Weight(elements []appir.ContentElement) int {
	total := 0
	for _, element := range elements {
		total += utf8.RuneCountInString(element.Text) + utf8.RuneCountInString(element.Attribution)
		for _, item := range element.Items {
			total += utf8.RuneCountInString(item)
		}
		switch element.Type {
		case "bullets":
			total += len(element.Items) * 20
		case "code":
			total += strings.Count(element.Text, "\n") * 12
		case "image":
			total += 180
		case "diagram":
			total += 60 + len(element.Items)*40
		}
	}
	return total
}

func ValidImageSource(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if parsed.Scheme == "https" {
		return parsed.Host != "" && parsed.Hostname() != ""
	}
	return parsed.Scheme == "" && parsed.Host == "" && strings.HasPrefix(parsed.Path, "/") && !strings.HasPrefix(parsed.Path, "//") && !strings.Contains(parsed.Path, "..") && !strings.Contains(parsed.Path, "\\")
}
