package compiler_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestFilterDefinitionAndViewFieldReferenceCompile(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "body", "type": "text"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Filter", Metadata: definition.Metadata{Name: "markdown"}, Spec: map[string]any{"steps": []any{map[string]any{"type": "markdown"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "published_articles"}, Spec: map[string]any{"entity": "article", "fields": []any{"id", "body"}, "fieldFilters": map[string]any{"body": "markdown"}}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("valid Filter diagnostics=%v", result.Diagnostics)
	}
	if result.App.Filters["markdown"].Steps[0].Type != "markdown" || result.App.Views["published_articles"].FieldFilters["body"] != "markdown" {
		t.Fatalf("compiled Filter or View reference missing: filters=%+v view=%+v", result.App.Filters, result.App.Views["published_articles"])
	}

	definitions[2].Spec["fieldFilters"] = map[string]any{"missing": "markdown"}
	if invalid := compiler.Compile("test", 1, definitions); len(invalid.Diagnostics) == 0 {
		t.Fatal("unselected Filter field accepted")
	}
	definitions[2].Spec["fieldFilters"] = map[string]any{"body": "missing"}
	if invalid := compiler.Compile("test", 1, definitions); len(invalid.Diagnostics) == 0 {
		t.Fatal("missing Filter reference accepted")
	}
	definitions[2].Spec["fieldFilters"] = map[string]any{"body": "markdown"}
	definitions[1].Spec["steps"] = []any{map[string]any{"type": "unknown"}}
	if invalid := compiler.Compile("test", 1, definitions); len(invalid.Diagnostics) == 0 {
		t.Fatal("unknown Filter step accepted")
	}
}
