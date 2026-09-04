package agenttest

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestPresentationPromptRubricUsesOrdinaryDeterministicDefinitions(t *testing.T) {
	bundle, diagnostics := definition.LoadFile(filepath.Join("..", "..", "examples", "presentation", "app.yaml"))
	if len(diagnostics) != 0 {
		t.Fatalf("load diagnostics=%v", diagnostics)
	}
	first := compiler.Compile("default", 1, bundle.Definitions)
	second := compiler.Compile("default", 1, bundle.Definitions)
	if len(first.Diagnostics) != 0 || len(second.Diagnostics) != 0 {
		t.Fatalf("compile diagnostics=%v / %v", first.Diagnostics, second.Diagnostics)
	}
	if !reflect.DeepEqual(first.App, second.App) {
		t.Fatal("the same presentation definitions produced different AppIR")
	}

	sequence := first.App.Sequences["bean_introduction"]
	if sequence.Route != "/presentations/bean" || sequence.Profile != "presentation" || len(sequence.Frames) != 10 {
		t.Fatalf("sequence=%+v", sequence)
	}
	directions := map[string]int{}
	for _, frame := range sequence.Frames {
		directions[frame.Direction]++
		panel := first.App.Panels[frame.Panel]
		if panel.Name == "" || len(panel.Regions) == 0 {
			t.Fatalf("frame %s does not compose an inspectable Panel", frame.Name)
		}
	}
	if directions["next"] != 5 || directions["down"] != 5 {
		t.Fatalf("directions=%v", directions)
	}
	view := first.App.Views["capabilities_by_area"]
	if view.ResultShape != "groups" || view.Displays["chart"].Renderer.Type != "chart" {
		t.Fatalf("data-backed frame view=%+v", view)
	}
	for _, target := range [][2]string{{"Sequence", "bean_introduction"}, {"View", "capabilities_by_area"}, {"Panel", "frame_architecture"}, {"Block", "product_statement"}} {
		if _, _, exists := compiler.InspectDefinition(first.App, target[0], target[1]); !exists {
			t.Fatalf("missing inspectable %s/%s", target[0], target[1])
		}
	}
}

func TestPresentationPromptCanRepairStableDiagnosedFields(t *testing.T) {
	bundle, diagnostics := definition.LoadFile(filepath.Join("..", "..", "examples", "presentation", "app.yaml"))
	if len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	var sequenceDefinition, openingDefinition *definition.Definition
	for index := range bundle.Definitions {
		item := &bundle.Definitions[index]
		if item.Kind == "Sequence" && item.Metadata.Name == "bean_introduction" {
			sequenceDefinition = item
		}
		if item.Kind == "Panel" && item.Metadata.Name == "frame_opening" {
			openingDefinition = item
		}
	}
	if sequenceDefinition == nil || openingDefinition == nil {
		t.Fatal("repair fixture definitions are missing")
	}
	regions := openingDefinition.Spec["regions"].([]any)
	region := regions[0].(map[string]any)
	items := region["items"].([]any)
	inline := items[0].(map[string]any)
	originalProfile, originalContent := sequenceDefinition.Spec["profile"], inline["content"]
	sequenceDefinition.Spec["profile"] = "slides"
	inline["content"] = []any{map[string]any{"type": "image", "source": "javascript:alert(1)"}}

	first := compiler.Compile("default", 1, bundle.Definitions).Diagnostics
	second := compiler.Compile("default", 1, bundle.Definitions).Diagnostics
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("identical broken presentation produced different diagnostics: %v / %v", first, second)
	}
	want := map[string]bool{
		"Panel/frame_opening/spec.regions.0.items.0.content.0.alt":    false,
		"Panel/frame_opening/spec.regions.0.items.0.content.0.source": false,
		"Sequence/bean_introduction/spec.profile":                     false,
	}
	for _, diagnostic := range first {
		key := strings.Join([]string{diagnostic.Kind, diagnostic.Name, diagnostic.Path}, "/")
		if _, expected := want[key]; expected && diagnostic.Code == "BEAN-E2881" {
			want[key] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("missing stable repair diagnostic %s in %v", path, first)
		}
	}

	sequenceDefinition.Spec["profile"] = originalProfile
	inline["content"] = originalContent
	if repaired := compiler.Compile("default", 1, bundle.Definitions); len(repaired.Diagnostics) != 0 {
		t.Fatalf("repairing only diagnosed fields did not validate: %v", repaired.Diagnostics)
	}
}
