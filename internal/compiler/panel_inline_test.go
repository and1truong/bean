package compiler_test

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestPanelInlineContentCompilesToDeterministicOrderedAppIR(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "shared"}, Spec: map[string]any{
			"type": "content", "content": []any{map[string]any{"type": "paragraph", "text": "Shared"}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "frame"}, Spec: map[string]any{
			"layout": "single-column",
			"regions": []any{map[string]any{"name": "main", "items": []any{
				map[string]any{"id": "opening", "content": []any{map[string]any{"type": "heading", "text": "Inline"}}},
				map[string]any{"block": "shared"},
				map[string]any{"content": []any{map[string]any{"type": "callout", "text": "Default tone"}}},
			}}},
		}},
	}
	first := compiler.Compile("inline", 1, definitions)
	second := compiler.Compile("inline", 1, definitions)
	if len(first.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", first.Diagnostics)
	}
	if !reflect.DeepEqual(first.App, second.App) {
		t.Fatal("identical inline source did not produce identical AppIR")
	}
	region := first.App.Panels["frame"].Regions[0]
	if region.Blocks != nil || len(region.Items) != 3 {
		t.Fatalf("region=%+v", region)
	}
	if region.Items[0].Identity != "@inline/frame/main/id/opening" || region.Items[1].Block != "shared" || region.Items[2].Identity != "@inline/frame/main/item/2" {
		t.Fatalf("ordered items=%+v", region.Items)
	}
	if region.Items[2].Content[0].Tone != "info" {
		t.Fatalf("inline defaults were not normalized: %+v", region.Items[2])
	}
	if len(first.App.Blocks) != 1 {
		t.Fatalf("inline identities leaked into global Blocks: %+v", first.App.Blocks)
	}
	regions := definitions[1].Spec["regions"].([]any)
	items := regions[0].(map[string]any)["items"].([]any)
	items[0], items[1] = items[1], items[0]
	reordered := compiler.Compile("inline", 1, definitions)
	if len(reordered.Diagnostics) != 0 || reordered.App.Panels["frame"].Regions[0].Items[1].Identity != "@inline/frame/main/id/opening" {
		t.Fatalf("explicit local identity changed after reorder: diagnostics=%v items=%+v", reordered.Diagnostics, reordered.App.Panels["frame"].Regions[0].Items)
	}
	_, references, exists := compiler.InspectDefinition(first.App, "Panel", "frame")
	if !exists || !reflect.DeepEqual(references, []compiler.DefinitionReference{{Path: "regions.0.items.1.block", Kind: "Block", Name: "shared"}}) {
		t.Fatalf("references=%+v exists=%v", references, exists)
	}
}

func TestPanelCollapseWhenEmptyCompilesIntoAppIR(t *testing.T) {
	definitions := []definition.Definition{{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{
		"layout": "sidebar-main",
		"regions": []any{
			map[string]any{"name": "sidebar", "collapseWhenEmpty": true, "blocks": []any{}},
			map[string]any{"name": "main", "blocks": []any{}},
		},
	}}}
	result := compiler.Compile("collapse", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	regions := result.App.Panels["article"].Regions
	if len(regions) != 2 || !regions[0].CollapseWhenEmpty || regions[1].CollapseWhenEmpty {
		t.Fatalf("regions=%+v", regions)
	}
}

func TestPanelInlineContentDiagnosticsUseExactSourcePathsAndLocations(t *testing.T) {
	filesystem := fstest.MapFS{
		"app.yaml": {Data: []byte("apiVersion: bean/v1alpha1\nname: Inline\nresources: [panel.yaml]\n")},
		"panel.yaml": {Data: []byte(`kind: Panel
name: frame
layout: single-column
regions:
  - name: main
    items:
      - content:
          - {type: image, source: "javascript:alert(1)"}
      - {block: "@inline/frame/main/item/0"}
      - {block: missing}
      - {content: []}
`)},
	}
	bundle, loadDiagnostics := definition.LoadFS(filesystem, "app.yaml")
	if len(loadDiagnostics) != 0 {
		t.Fatalf("load diagnostics=%v", loadDiagnostics)
	}
	result := compiler.Compile("inline", 1, bundle.Definitions)
	for _, expected := range []string{
		"spec.regions.0.items.0.content.0.alt",
		"spec.regions.0.items.0.content.0.source",
		"spec.regions.0.items.1.block",
		"spec.regions.0.items.2.block",
		"spec.regions.0.items.3.content",
	} {
		diagnostic, found := diagnosticAt(result.Diagnostics, expected)
		if !found {
			t.Errorf("missing %s in %v", expected, result.Diagnostics)
			continue
		}
		if diagnostic.Source.Path != "panel.yaml" || diagnostic.Source.Line == 0 || diagnostic.Source.Column == 0 {
			t.Errorf("%s source=%+v", expected, diagnostic.Source)
		}
	}
}

func TestInlineContentUsesContentElementBudget(t *testing.T) {
	elements := make([]any, 13)
	for index := range elements {
		elements[index] = map[string]any{"type": "paragraph", "text": "bounded"}
	}
	definitions := []definition.Definition{{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "frame"}, Spec: map[string]any{
		"layout": "single-column", "regions": []any{map[string]any{"name": "main", "items": []any{map[string]any{"content": elements}}}},
	}}}
	diagnostics := compiler.Compile("inline", 1, definitions).Diagnostics
	if _, found := diagnosticAt(diagnostics, "spec.regions.0.items.0.content"); !found {
		t.Fatalf("inline element budget was not enforced: %v", diagnostics)
	}
}

func TestSequenceCountsAndBudgetsInlineRegionItems(t *testing.T) {
	definitions := validSequenceDefinitions()
	definitions[2].Spec["regions"] = []any{map[string]any{"name": "main", "items": []any{
		map[string]any{"content": []any{map[string]any{"type": "paragraph", "text": strings.Repeat("dense ", 60)}}},
		map[string]any{"block": "opening"},
	}}}
	diagnostics := compiler.Compile("inline", 1, definitions).Diagnostics
	if !hasSequenceDiagnostic(diagnostics, "Sequence", "bean_intro", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("inline density was not included: %v", diagnostics)
	}

	items := make([]any, 13)
	for index := range items {
		items[index] = map[string]any{"content": []any{map[string]any{"type": "paragraph", "text": "small"}}}
	}
	definitions[2].Spec["regions"] = []any{map[string]any{"name": "main", "items": items}}
	diagnostics = compiler.Compile("inline", 1, definitions).Diagnostics
	if !hasSequenceDiagnostic(diagnostics, "Sequence", "bean_intro", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("inline Block count was not included: %v", diagnostics)
	}
}

func TestLegacyPanelBlocksRemainUnchanged(t *testing.T) {
	result := compiler.Compile("legacy", 1, validSequenceDefinitions())
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	region := result.App.Panels["opening_panel"].Regions[0]
	if !reflect.DeepEqual(region.Blocks, []string{"opening"}) || region.Items != nil {
		t.Fatalf("legacy region changed: %+v", region)
	}
}

func diagnosticAt(diagnostics []definition.Diagnostic, path string) (definition.Diagnostic, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == "Panel" && diagnostic.Name == "frame" && diagnostic.Path == path {
			return diagnostic, true
		}
	}
	return definition.Diagnostic{}, false
}
