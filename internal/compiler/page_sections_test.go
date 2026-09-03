package compiler_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestPageSectionsCompileInDeterministicOrderWithCrossSectionFilters(t *testing.T) {
	definitions := pageSectionDefinitions()
	first := compiler.Compile("sections", 1, definitions)
	second := compiler.Compile("sections", 1, definitions)
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", first.Diagnostics)
	}
	if !reflect.DeepEqual(first.App, second.App) || first.App.FormatVersion != appir.CurrentFormat {
		t.Fatalf("Page sections did not compile deterministically: first=%+v second=%+v", first.App.Pages["home"], second.App.Pages["home"])
	}
	page := first.App.Pages["home"]
	if page.Panel != "" || !reflect.DeepEqual(page.PanelNames(), []string{"hero", "body"}) || page.Sections[0].Identity != "@section/home/introduction" || page.Sections[1].Identity != "@section/home/1" {
		t.Fatalf("page=%+v panels=%v", page, page.PanelNames())
	}
	_, references, exists := compiler.InspectDefinition(first.App, "Page", "home")
	want := []compiler.DefinitionReference{
		{Path: "filters.status.targets.0.block", Kind: "Block", Name: "candidate_list"},
		{Path: "sections.0.panel", Kind: "Panel", Name: "hero"},
		{Path: "sections.1.panel", Kind: "Panel", Name: "body"},
	}
	if !exists || !reflect.DeepEqual(references, want) {
		t.Fatalf("references=%+v exists=%v", references, exists)
	}
}

func TestPageSectionsRejectAmbiguousEmptyAndMissingComposition(t *testing.T) {
	definitions := pageSectionDefinitions()
	page := &definitions[len(definitions)-1]
	page.Spec["panel"] = "hero"
	page.Spec["sections"] = []any{}
	diagnostics := compiler.Compile("sections", 1, definitions).Diagnostics
	for _, path := range []string{"spec.sections"} {
		if !hasDiagnosticPath(diagnostics, "Page", "home", path) {
			t.Fatalf("missing %s in %v", path, diagnostics)
		}
	}

	delete(page.Spec, "panel")
	page.Spec["sections"] = []any{map[string]any{"id": "duplicate", "panel": ""}, map[string]any{"id": "duplicate", "panel": "missing"}}
	diagnostics = compiler.Compile("sections", 1, definitions).Diagnostics
	for _, path := range []string{"spec.sections.0.panel", "spec.sections.1.id", "spec.sections.1.panel"} {
		if !hasDiagnosticPath(diagnostics, "Page", "home", path) {
			t.Errorf("missing %s in %v", path, diagnostics)
		}
	}
}

func TestLegacySinglePanelPageAppIRRemainsUnchanged(t *testing.T) {
	definitions := pageSectionDefinitions()
	page := &definitions[len(definitions)-1]
	delete(page.Spec, "sections")
	page.Spec["panel"] = "body"
	result := compiler.Compile("legacy", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	compiled := result.App.Pages["home"]
	if compiled.Panel != "body" || compiled.Sections != nil || !reflect.DeepEqual(compiled.PanelNames(), []string{"body"}) {
		t.Fatalf("legacy Page changed: %+v", compiled)
	}
}

func pageSectionDefinitions() []definition.Definition {
	item := func(kind, name string, spec map[string]any) definition.Definition {
		return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
	}
	return []definition.Definition{
		item("Entity", "candidate", map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "options": []any{"active", "hired"}}}}),
		item("View", "candidates", map[string]any{"entity": "candidate", "fields": []any{"id", "status"}, "exposedFilters": map[string]any{"status": map[string]any{"field": "status", "operator": "eq"}}}),
		item("Block", "introduction", map[string]any{"type": "text", "text": "Candidates"}),
		item("Block", "candidate_list", map[string]any{"type": "view", "view": "candidates"}),
		item("Panel", "hero", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"introduction"}}}}),
		item("Panel", "body", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"candidate_list"}}}}),
		item("Page", "home", map[string]any{"route": "/", "sections": []any{map[string]any{"id": "introduction", "panel": "hero"}, map[string]any{"panel": "body"}}, "filters": map[string]any{"status": map[string]any{"targets": []any{map[string]any{"block": "candidate_list", "filter": "status"}}}}}),
	}
}

func hasDiagnosticPath(diagnostics []definition.Diagnostic, kind, name, path string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind && diagnostic.Name == name && diagnostic.Path == path {
			return true
		}
	}
	return false
}
