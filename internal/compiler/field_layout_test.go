package compiler_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func layoutDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "body", "type": "richtext"}}}},
		{APIVersion: definition.APIVersion, Kind: "AdminResource", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"entity": "article", "form": map[string]any{"fields": []any{"title", "body"}, "layout": map[string]any{"groups": []any{map[string]any{"name": "content", "label": "Content", "fields": []any{map[string]any{"field": "title"}, map[string]any{"field": "body", "span": "full"}}}}}}}},
	}
}

func TestFieldLayoutCanonicalAndCompatibility(t *testing.T) {
	result := compiler.Compile("test", 1, layoutDefinitions())
	if len(result.Diagnostics) > 0 {
		t.Fatal(result.Diagnostics)
	}
	layout := result.App.AdminResources["article"].Form.Layout
	if layout == nil || layout.Groups[0].Columns != 1 || layout.Groups[0].Fields[0].Span != "single" {
		t.Fatalf("layout=%+v", layout)
	}
	encoded, err := json.Marshal(result.App)
	if err != nil {
		t.Fatal(err)
	}
	var restored appir.App
	if err = json.Unmarshal(encoded, &restored); err != nil {
		t.Fatal(err)
	}
	if err = restored.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(layout, restored.AdminResources["article"].Form.Layout) {
		t.Fatal("round trip changed layout")
	}
	clone, err := restored.Clone()
	if err != nil {
		t.Fatal(err)
	}
	clone.AdminResources["article"].Form.Layout.Groups[0].Label = "Changed"
	if layout.Groups[0].Label != "Content" || restored.AdminResources["article"].Form.Layout.Groups[0].Label != "Content" {
		t.Fatal("layout aliases immutable snapshots")
	}
	restored.FormatVersion = appir.MenuVariantFormat
	if restored.ValidateFormat() == nil {
		t.Fatal("legacy format accepted layout")
	}
	resource := restored.AdminResources["article"]
	resource.Form.Layout = nil
	restored.AdminResources["article"] = resource
	if err = restored.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
}

func TestFieldLayoutRejectsInvalidContracts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any, map[string]any)
	}{
		{"columns", func(_ map[string]any, g map[string]any) { g["columns"] = 3 }},
		{"empty label", func(_ map[string]any, g map[string]any) { g["label"] = " " }},
		{"invalid name", func(_ map[string]any, g map[string]any) { g["name"] = "bad name" }},
		{"missing field", func(_ map[string]any, g map[string]any) { g["fields"] = []any{map[string]any{"field": "title"}} }},
		{"unknown field", func(_ map[string]any, g map[string]any) {
			g["fields"] = []any{map[string]any{"field": "secret"}, map[string]any{"field": "body"}}
		}},
		{"duplicate field", func(_ map[string]any, g map[string]any) {
			g["fields"] = []any{map[string]any{"field": "title"}, map[string]any{"field": "title"}, map[string]any{"field": "body"}}
		}},
		{"invalid span", func(_ map[string]any, g map[string]any) { g["fields"].([]any)[0].(map[string]any)["span"] = "8px" }},
		{"empty groups", func(l map[string]any, _ map[string]any) { l["groups"] = []any{} }},
		{"duplicate groups", func(l map[string]any, g map[string]any) { l["groups"] = []any{g, g} }},
		{"too many groups", func(l map[string]any, g map[string]any) {
			groups := []any{}
			for i := 0; i < 17; i++ {
				groups = append(groups, g)
			}
			l["groups"] = groups
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defs := layoutDefinitions()
			l := defs[1].Spec["form"].(map[string]any)["layout"].(map[string]any)
			g := l["groups"].([]any)[0].(map[string]any)
			tc.change(l, g)
			a := compiler.Compile("test", 1, defs)
			b := compiler.Compile("test", 1, defs)
			if len(a.Diagnostics) == 0 {
				t.Fatal("accepted invalid layout")
			}
			if !reflect.DeepEqual(a.Diagnostics, b.Diagnostics) {
				t.Fatal("unstable diagnostics")
			}
		})
	}
}
