package compiler_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestSequenceCompilesAsInspectablePanelComposition(t *testing.T) {
	definitions := validSequenceDefinitions()
	first := compiler.Compile("presentation", 1, definitions)
	second := compiler.Compile("presentation", 1, definitions)
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", first.Diagnostics)
	}
	if !reflect.DeepEqual(first.App, second.App) {
		t.Fatal("identical Sequence source did not produce identical AppIR")
	}
	value, references, exists := compiler.InspectDefinition(first.App, "Sequence", "bean_intro")
	if !exists || value == nil {
		t.Fatal("Sequence is not inspectable")
	}
	want := []compiler.DefinitionReference{
		{Path: "frames.0.panel", Kind: "Panel", Name: "opening_panel"},
		{Path: "frames.1.panel", Kind: "Panel", Name: "architecture_panel"},
	}
	if !reflect.DeepEqual(references, want) {
		t.Fatalf("references=%v want=%v", references, want)
	}
	if first.App.FormatVersion != "bean/appir/v12" {
		t.Fatalf("format=%q", first.App.FormatVersion)
	}
}

func TestSequenceDiagnosticsAreStableAndRepairable(t *testing.T) {
	definitions := validSequenceDefinitions()
	definitions[0].Spec["content"] = []any{map[string]any{"type": "image", "source": "javascript:alert(1)"}}
	definitions[4].Spec["frames"] = []any{
		map[string]any{"name": "opening", "title": strings.Repeat("x", 81), "layout": "title", "panel": "opening_panel"},
		map[string]any{"name": "opening", "title": "Architecture", "layout": "comparison", "panel": "architecture_panel"},
	}
	first := compiler.Compile("presentation", 1, definitions).Diagnostics
	second := compiler.Compile("presentation", 1, definitions).Diagnostics
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("diagnostics changed between identical runs:\n%v\n%v", first, second)
	}
	for _, expected := range []struct{ kind, name, path, code string }{
		{"Block", "opening", "spec.content.0.alt", "BEAN-E2881"},
		{"Block", "opening", "spec.content.0.source", "BEAN-E2881"},
		{"Sequence", "bean_intro", "spec.frames.0.title", "BEAN-E2881"},
		{"Sequence", "bean_intro", "spec.frames.1.name", "BEAN-E2881"},
		{"Sequence", "bean_intro", "spec.frames.1.layout", "BEAN-E2881"},
	} {
		if !hasSequenceDiagnostic(first, expected.kind, expected.name, expected.path, expected.code) {
			t.Errorf("missing %+v in %v", expected, first)
		}
	}
}

func TestSequenceRejectsRouteConflictsAndDenseFrames(t *testing.T) {
	definitions := validSequenceDefinitions()
	definitions[0].Spec["content"] = []any{map[string]any{"type": "paragraph", "text": strings.Repeat("dense ", 140)}}
	definitions = append(definitions, definition.Definition{
		APIVersion: definition.APIVersion,
		Kind:       "Page",
		Metadata:   definition.Metadata{Name: "conflict"},
		Spec:       map[string]any{"route": "/presentations/bean", "panel": "opening_panel", "title": "Conflict"},
	})
	diagnostics := compiler.Compile("presentation", 1, definitions).Diagnostics
	if !hasSequenceDiagnostic(diagnostics, "Sequence", "bean_intro", "spec.route", "BEAN-E2881") {
		t.Fatalf("route conflict accepted: %v", diagnostics)
	}
	if !hasSequenceDiagnostic(diagnostics, "Sequence", "bean_intro", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("dense frame accepted: %v", diagnostics)
	}
}

func TestSequenceRejectsPanelWithRequiredContextInput(t *testing.T) {
	definitions := validSequenceDefinitions()
	definitions[0].Spec["inputs"] = map[string]any{"record_id": map[string]any{"type": "uuid", "required": true}}
	definitions[0].Spec["bindings"] = map[string]any{"record_id": map[string]any{"source": "context", "name": "record_id"}}
	diagnostics := compiler.Compile("presentation", 1, definitions).Diagnostics
	if !hasSequenceDiagnostic(diagnostics, "Sequence", "bean_intro", "spec.frames.0.panel", "BEAN-E2881") {
		t.Fatalf("required context-bound Sequence Block accepted: %v", diagnostics)
	}
}

func validSequenceDefinitions() []definition.Definition {
	item := func(kind, name string, spec map[string]any) definition.Definition {
		return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
	}
	return []definition.Definition{
		item("Block", "opening", map[string]any{"type": "content", "content": []any{map[string]any{"type": "heading", "text": "Bean"}, map[string]any{"type": "paragraph", "text": "The deterministic application runtime for agents."}}}),
		item("Block", "architecture", map[string]any{"type": "content", "content": []any{map[string]any{"type": "diagram", "items": []any{"Intent", "Definitions", "Compiler", "Runtime"}, "direction": "horizontal"}}}),
		item("Panel", "opening_panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"opening"}}}}),
		item("Panel", "architecture_panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"architecture"}}}}),
		item("Sequence", "bean_intro", map[string]any{
			"route": "/presentations/bean", "title": "Introducing Bean", "profile": "presentation", "aspectRatio": "wide",
			"frames": []any{
				map[string]any{"name": "opening", "title": "Introducing Bean", "layout": "title", "panel": "opening_panel", "notes": "Open with the product thesis."},
				map[string]any{"name": "architecture", "title": "Deterministic architecture", "layout": "architecture", "panel": "architecture_panel"},
			},
		}),
	}
}

func hasSequenceDiagnostic(diagnostics []definition.Diagnostic, kind, name, path, code string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind && diagnostic.Name == name && diagnostic.Path == path && diagnostic.Code == code {
			return true
		}
	}
	return false
}
