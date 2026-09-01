package generatedtest_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
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
	app.Views["notes"] = appir.View{Name: "notes", Entity: "note", Displays: map[string]appir.Display{"api": {Type: "json", Route: "/api/notes"}}}
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
}
