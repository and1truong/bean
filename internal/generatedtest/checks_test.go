package generatedtest_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/generatedtest"
)

func TestStructuralChecksHaveStableSourceTracedOrder(t *testing.T) {
	bundle := definition.Bundle{Name: "Checks", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "always"}, Spec: map[string]any{"result": "boolean", "expression": map[string]any{"source": "literal", "literal": true}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{}},
	}}
	compiled := compiler.Compile("test", 1, bundle.Definitions)
	checks := generatedtest.StructuralChecks(bundle, compiled.App)
	ids := make([]string, len(checks))
	for index, check := range checks {
		ids[index] = check.ID
		if check.Status != "passed" || check.Source.Kind == "" || check.Source.Name == "" || len(check.Evidence) == 0 {
			t.Fatalf("check=%+v", check)
		}
	}
	want := []string{"generated/rule/Rule/always", "generated/schema/Entity/note", "generated/schema/Rule/always"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%v want=%v", ids, want)
	}
}

func TestJourneyChecksExerciseStaticPagesAndViewRoutes(t *testing.T) {
	app := appir.Empty()
	app.Entities["note"] = appir.Entity{Name: "note"}
	app.Views["notes"] = appir.View{Name: "notes", Entity: "note", Displays: map[string]appir.Display{
		"api": {Type: "json", Route: "/api/notes"}, "inline": {Type: "json"},
	}}
	app.Panels["home"] = appir.Panel{Name: "home", Regions: []appir.Region{{Name: "main"}}}
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home"}
	handler := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) { response.WriteHeader(http.StatusOK) })

	checks, diagnostics := generatedtest.JourneyChecks(context.Background(), app, handler)
	if len(diagnostics) != 0 || len(checks) != 2 || checks[0].ID != "generated/journey/Page/home" || checks[1].ID != "generated/journey/View/notes/api" {
		t.Fatalf("checks=%+v diagnostics=%v", checks, diagnostics)
	}
	failing := http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusInternalServerError)
	})
	checks, diagnostics = generatedtest.JourneyChecks(context.Background(), app, failing)
	if len(diagnostics) != 2 || diagnostics[0].Code != "BEAN-T1201" || checks[0].Status != "failed" || checks[1].Status != "failed" {
		t.Fatalf("checks=%+v diagnostics=%v", checks, diagnostics)
	}

	app.Blocks["filtered_notes"] = appir.Block{
		Name: "filtered_notes", Type: "view", View: "notes",
		Inputs:   map[string]appir.Field{"id": {Name: "id", Type: "uuid"}},
		Bindings: map[string]appir.ContextBinding{"id": {Source: "context", Name: "id"}},
	}
	app.Panels["home"] = appir.Panel{Name: "home", Regions: []appir.Region{{Name: "main", Blocks: []string{"filtered_notes"}}}}
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home", Context: map[string]appir.ContextBinding{"id": {Source: "query", Name: "id"}}}
	checks, diagnostics = generatedtest.JourneyChecks(context.Background(), app, handler)
	if len(diagnostics) != 0 || len(checks) != 1 || checks[0].ID != "generated/journey/View/notes/api" {
		t.Fatalf("typed Block input page was not omitted: checks=%+v diagnostics=%v", checks, diagnostics)
	}

	condition := expr.Expr{Op: "is_null", Left: &expr.Value{Source: "context", Name: "filter"}}
	app.Policies["empty_filter"] = appir.Policy{Name: "empty_filter", Condition: &condition}
	app.Blocks["filtered_notes"] = appir.Block{Name: "filtered_notes", Type: "view", View: "notes"}
	app.Panels["home"] = appir.Panel{Name: "home", Policy: "empty_filter", Regions: []appir.Region{{Name: "main", Blocks: []string{"filtered_notes"}}}}
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home", Context: map[string]appir.ContextBinding{"filter": {Source: "query", Name: "filter"}}}
	checks, diagnostics = generatedtest.JourneyChecks(context.Background(), app, handler)
	if len(diagnostics) != 0 || len(checks) != 1 || checks[0].ID != "generated/journey/View/notes/api" {
		t.Fatalf("context-dependent Policy page was not omitted: checks=%+v diagnostics=%v", checks, diagnostics)
	}

	pathCondition := expr.Expr{Op: "eq", Left: &expr.Value{Source: "context", Name: "path"}, Right: &expr.Value{Source: "literal", Literal: ""}}
	app.Policies["empty_path"] = appir.Policy{Name: "empty_path", Condition: &pathCondition}
	app.Panels["home"] = appir.Panel{Name: "home", Policy: "empty_path", Regions: []appir.Region{{Name: "main", Blocks: []string{"filtered_notes"}}}}
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home", Context: map[string]appir.ContextBinding{"path": {Source: "query", Name: "path"}}}
	checks, diagnostics = generatedtest.JourneyChecks(context.Background(), app, handler)
	if len(diagnostics) != 0 || len(checks) != 1 || checks[0].ID != "generated/journey/View/notes/api" {
		t.Fatalf("routing-query Policy page was not omitted: checks=%+v diagnostics=%v", checks, diagnostics)
	}
}
