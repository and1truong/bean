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

func TestMoneyRejectsFloatLikeStringAndRichTextSanitizes(t *testing.T) {
	if e := field.Validate(appir.Field{Name: "price", Type: "money"}, "1.50"); e == nil {
		t.Fatal("money string accepted")
	}
	out := field.SanitizeRichText(`<p>ok</p><script>alert(1)</script>`)
	if strings.Contains(out, "script") {
		t.Fatalf("unsafe output %q", out)
	}
}
