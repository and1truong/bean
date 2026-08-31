package field_test

import (
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/field"
	"reflect"
	"strings"
	"testing"
)

func TestStorageEncodingPreservesJSONAndBooleanTypes(t *testing.T) {
	jsonField := appir.Field{Name: "settings", Type: "json"}
	want := map[string]any{"enabled": true, "labels": []any{"one", "two"}}
	stored, err := field.Encode(jsonField, want)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := stored.(string); !ok {
		t.Fatalf("stored JSON type=%T", stored)
	}
	if got := field.Decode(jsonField, stored); !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded=%#v want=%#v", got, want)
	}
	if got := field.Decode(appir.Field{Name: "enabled", Type: "boolean"}, int64(1)); got != true {
		t.Fatalf("decoded boolean=%#v", got)
	}
}

func TestFileUploadRequiresContentWithinBound(t *testing.T) {
	definition := appir.Field{Name: "file", Type: "file", Required: true}
	if err := field.Validate(definition, field.Upload{Name: "plan.txt", ContentType: "text/plain", Data: []byte("ok")}); err != nil {
		t.Fatal(err)
	}
	if err := field.Validate(definition, field.Upload{Name: "large.bin", Data: make([]byte, field.MaxFileBytes+1)}); err == nil {
		t.Fatal("oversized file accepted")
	}
	if err := field.Validate(definition, map[string]any{"name": "forged"}); err == nil {
		t.Fatal("forged JSON file accepted")
	}
}

func TestMoneyRejectsFloatLikeStringAndRichTextSanitizes(t *testing.T) {
	if e := field.Validate(appir.Field{Name: "price", Type: "money"}, "1.50"); e == nil {
		t.Fatal("money string accepted")
	}
	out := field.SanitizeRichText(`<p>ok</p><script>alert(1)</script>`)
	if strings.Contains(out, "script") {
		t.Fatalf("unsafe output %q", out)
	}
	for _, payload := range []string{`<img src=x onerror=alert(1)>`, `<a href="javascript:alert(1)">click</a>`, `<svg><script>alert(1)</script></svg>`} {
		out = field.SanitizeRichText(payload)
		if strings.Contains(out, "<img") || strings.Contains(out, "<a ") || strings.Contains(out, "<svg") || strings.Contains(out, "<script") || strings.Contains(strings.ToLower(out), "javascript:") && !strings.Contains(out, "&") {
			t.Fatalf("unsafe rich text output %q", out)
		}
	}
	if out = field.SanitizeRichText(`<p><strong>safe</strong></p>`); out != `<p><strong>safe</strong></p>` {
		t.Fatalf("safe formatting lost: %q", out)
	}
}
