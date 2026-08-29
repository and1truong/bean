package compiler_test

import (
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"testing"
)

func TestDiagnosticsAreActionable(t *testing.T) {
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "broken"}, Spec: map[string]any{"entity": "missing", "fields": []any{"title"}}}}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%v", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if d.Kind != "View" || d.Name != "broken" || d.Path != "spec.entity" {
		t.Fatalf("diagnostic=%+v", d)
	}
}
func TestGeneratedCRUDUsesActionsAndViews(t *testing.T) {
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}}}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) > 0 {
		t.Fatal(r.Diagnostics)
	}
	if _, ok := r.App.Views["book_list"]; !ok {
		t.Fatal("generated View missing")
	}
	if _, ok := r.App.Actions["book_create"]; !ok {
		t.Fatal("generated Action missing")
	}
}
