package field_test

import (
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/field"
	"strings"
	"testing"
)

func TestMoneyRejectsFloatLikeStringAndRichTextSanitizes(t *testing.T) {
	if e := field.Validate(appir.Field{Name: "price", Type: "money"}, "1.50"); e == nil {
		t.Fatal("money string accepted")
	}
	out := field.SanitizeRichText(`<p>ok</p><script>alert(1)</script>`)
	if strings.Contains(out, "script") {
		t.Fatalf("unsafe output %q", out)
	}
}
