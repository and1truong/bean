package filter_test

import (
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	contentfilter "github.com/beanruntime/bean/internal/filter"
)

func TestMarkdownProducesSafeHTML(t *testing.T) {
	definition := appir.Filter{Name: "markdown", Steps: []appir.FilterStep{{Type: "markdown"}}}
	source := "# Hello\n\nSafe **body** with [docs](https://example.test).\n\n<script>alert(1)</script>\n\n[bad](javascript:alert(2))\n\n<img src=x onerror=alert(3)>"
	output, err := contentfilter.Apply(definition, source)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"<h1>Hello</h1>", "<strong>body</strong>", `href="https://example.test"`} {
		if !strings.Contains(output, expected) {
			t.Fatalf("safe Markdown missing %q from %q", expected, output)
		}
	}
	lower := strings.ToLower(output)
	for _, unsafe := range []string{"<script", "javascript:", "onerror", "<img"} {
		if strings.Contains(lower, unsafe) {
			t.Fatalf("unsafe output contains %q: %q", unsafe, output)
		}
	}
}

func TestSanitizeHTMLCleansLegacyRichTextWithoutDoubleEscaping(t *testing.T) {
	source := `<p onclick="alert(1)">Safe<img src=x onerror="alert(2)"></p>&lt;img src=x onerror=alert(3)&gt;`
	output := contentfilter.SanitizeHTML(source)
	lower := strings.ToLower(output)
	for _, unsafe := range []string{"onclick=", "<img"} {
		if strings.Contains(lower, unsafe) {
			t.Fatalf("unsafe legacy HTML contains %q: %q", unsafe, output)
		}
	}
	if !strings.Contains(output, "<p>Safe</p>") || !strings.Contains(output, "&lt;img src=x onerror=alert(3)&gt;") {
		t.Fatalf("legacy HTML was not safely preserved: %q", output)
	}
	if again := contentfilter.SanitizeHTML(output); again != output {
		t.Fatalf("sanitization is not idempotent: first=%q second=%q", output, again)
	}
}

func TestUnknownStepFailsClosed(t *testing.T) {
	_, err := contentfilter.Apply(appir.Filter{Steps: []appir.FilterStep{{Type: "unknown"}}}, "source")
	if err == nil {
		t.Fatal("unknown filter step accepted")
	}
}
