package compiler_test

import (
	"encoding/json"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
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
	admin, ok := r.App.AdminResources["book"]
	if !ok || admin.View != "book_list" || admin.LabelField != "title" || admin.List.PageSize != 25 {
		t.Fatalf("generated AdminResource=%+v", admin)
	}
}

func TestAdminResourceReferencesAreValidated(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "library"}, Spec: map[string]any{"entity": "book", "labelField": "missing", "list": map[string]any{"columns": []any{"id", "missing"}, "pageSize": 500}}},
	}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) < 2 {
		t.Fatalf("expected AdminResource diagnostics, got %v", r.Diagnostics)
	}
}
func TestRejectsUnknownFieldsAndUnimplementedSteps(t *testing.T) {
	entity := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{}, "mystery": true}}
	r := compiler.Compile("test", 1, []definition.Definition{entity})
	if len(r.Diagnostics) == 0 {
		t.Fatal("unknown field accepted")
	}
	entity.Spec = map[string]any{"fields": []any{}}
	action := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "bad"}, Spec: map[string]any{"entity": "book", "operation": "transaction", "steps": []any{map[string]any{"op": "load"}}}}
	r = compiler.Compile("test", 1, []definition.Definition{entity, action})
	if len(r.Diagnostics) == 0 {
		t.Fatal("step without executor accepted")
	}
}
func TestCompilationIsDeterministic(t *testing.T) {
	d := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{}}}
	a := compiler.Compile("test", 1, []definition.Definition{d})
	b := compiler.Compile("test", 1, []definition.Definition{d})
	aj, _ := json.Marshal(a.App)
	bj, _ := json.Marshal(b.App)
	if string(aj) != string(bj) {
		t.Fatalf("non-deterministic AppIR\n%s\n%s", aj, bj)
	}
}

func TestLocalRegistrationCompilesFixedSensitiveBoundary(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
		{APIVersion: "bean/v1alpha1", Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup"}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	action := result.App.Actions["signup"]
	if action.DefaultRole != "member" || !action.Input["password"].Sensitive || !action.Input["password_confirmation"].Sensitive {
		t.Fatalf("registration boundary=%+v", action)
	}
	if _, exposed := action.Output["password"]; exposed {
		t.Fatal("password exposed in registration output")
	}
	defs[1].Spec["defaultRole"] = "administrator"
	if invalid := compiler.Compile("test", 1, defs); len(invalid.Diagnostics) == 0 {
		t.Fatal("undefined registration role accepted")
	}
}

func TestBlockBindingsMustMatchTargetTypes(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "post"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "slug", "type": "slug"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "post_by_slug"}, Spec: map[string]any{"entity": "post", "fields": []any{"id", "slug"}, "exposedFilters": map[string]any{"slug": map[string]any{"name": "slug", "type": "slug"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "detail"}, Spec: map[string]any{"type": "view", "view": "post_by_slug", "inputs": map[string]any{"slug": map[string]any{"type": "uuid"}}, "bindings": map[string]any{"slug": map[string]any{"source": "context", "name": "slug"}}}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) == 0 {
		t.Fatal("mismatched bound input type accepted")
	}
}
