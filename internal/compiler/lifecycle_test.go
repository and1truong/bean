package compiler_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestLifecycleCompilesCanonicalSemanticModel(t *testing.T) {
	definitions := lifecycleDefinitions()
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	lifecycle, exists := result.App.Lifecycles["candidate_pipeline"]
	if !exists || lifecycle.Entity != "candidate" || lifecycle.StateField != "stage" || lifecycle.Initial != "applied" {
		t.Fatalf("lifecycle=%+v", lifecycle)
	}
	want := map[string][]string{"applied": {"interview", "rejected"}, "interview": {"hired", "rejected"}}
	if !reflect.DeepEqual(lifecycle.Transitions, want) {
		t.Fatalf("transitions=%v", lifecycle.Transitions)
	}
	action := result.App.Actions["move_candidate"]
	if action.Lifecycle != "candidate_pipeline" || action.StateField != "" || action.Transitions != nil {
		t.Fatalf("action=%+v", action)
	}
	if result.App.Actions["candidate_create"].Input["stage"].Required {
		t.Fatal("Lifecycle initial state remained a required create input")
	}
	if _, exposed := result.App.Actions["candidate_update"].Input["stage"]; exposed {
		t.Fatal("generic update exposes the Lifecycle state field")
	}

	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", "bean.agent/v1alpha1")
	if !containsString(capabilities.DefinitionKinds, "Lifecycle") {
		t.Fatalf("definition kinds=%v", capabilities.DefinitionKinds)
	}
	if !reflect.DeepEqual(capabilities.SemanticPrimitives, []string{"Lifecycle", "Rule"}) {
		t.Fatalf("semantic primitives=%v", capabilities.SemanticPrimitives)
	}
	if compiler.DefinitionSchemas()["Lifecycle"] == nil {
		t.Fatal("canonical Lifecycle schema missing")
	}
}

func TestLifecycleDiagnosticsAreStable(t *testing.T) {
	tests := []struct {
		name string
		edit func([]definition.Definition)
		kind string
		path string
		code string
	}{
		{"missing entity", func(defs []definition.Definition) { defs[1].Spec["entity"] = "missing" }, "Lifecycle", "spec.entity", "BEAN-E2202"},
		{"missing state field", func(defs []definition.Definition) { defs[1].Spec["stateField"] = "state" }, "Lifecycle", "spec.stateField", "BEAN-E2202"},
		{"non enum state field", func(defs []definition.Definition) {
			defs[0].Spec["fields"].([]any)[0].(map[string]any)["type"] = "string"
		}, "Lifecycle", "spec.stateField", "BEAN-E2202"},
		{"unknown initial", func(defs []definition.Definition) { defs[1].Spec["initial"] = "unknown" }, "Lifecycle", "spec.initial", "BEAN-E2202"},
		{"unknown source", func(defs []definition.Definition) {
			defs[1].Spec["transitions"].(map[string]any)["unknown"] = []any{"hired"}
		}, "Lifecycle", "spec.transitions.unknown", "BEAN-E2202"},
		{"unknown target", func(defs []definition.Definition) {
			defs[1].Spec["transitions"].(map[string]any)["applied"] = []any{"unknown"}
		}, "Lifecycle", "spec.transitions.applied.0", "BEAN-E2202"},
		{"duplicate edge", func(defs []definition.Definition) {
			defs[1].Spec["transitions"].(map[string]any)["applied"] = []any{"interview", "interview"}
		}, "Lifecycle", "spec.transitions.applied.1", "BEAN-E2202"},
		{"unreachable state", func(defs []definition.Definition) {
			defs[1].Spec["transitions"].(map[string]any)["interview"] = []any{"rejected"}
		}, "Lifecycle", "spec.transitions", "BEAN-E2202"},
		{"missing lifecycle", func(defs []definition.Definition) { defs[2].Spec["lifecycle"] = "missing" }, "Action", "spec.lifecycle", "BEAN-E2201"},
		{"unbound transition", func(defs []definition.Definition) { delete(defs[2].Spec, "lifecycle") }, "Action", "spec.lifecycle", "BEAN-E2201"},
		{"duplicated state field", func(defs []definition.Definition) { defs[2].Spec["stateField"] = "stage" }, "Action", "spec.stateField", "BEAN-E2201"},
		{"derived state field", func(defs []definition.Definition) {
			defs[2].Spec["operation"] = "create"
			delete(defs[2].Spec, "lifecycle")
			defs[2].Spec["derive"] = map[string]any{"stage": "initial_stage"}
		}, "Action", "spec.derive.stage", "BEAN-E2201"},
		{"transaction update bypass", func(defs []definition.Definition) {
			defs[2].Spec = map[string]any{
				"entity": "candidate", "operation": "transaction", "lifecycle": "candidate_pipeline",
				"input": map[string]any{"id": map[string]any{"type": "uuid", "required": true}},
				"steps": []any{map[string]any{"op": "update", "values": map[string]any{"id": "$input.id", "stage": "hired"}}},
			}
		}, "Action", "spec.steps.0.values.stage", "BEAN-E2201"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := lifecycleDefinitions()
			test.edit(definitions)
			diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
			for _, diagnostic := range diagnostics {
				if diagnostic.Kind == test.kind && diagnostic.Path == test.path && diagnostic.Code == test.code {
					return
				}
			}
			t.Fatalf("missing %s %s %s: %v", test.kind, test.path, test.code, diagnostics)
		})
	}
}

func TestLifecycleActionSubsetMustBelongToCanonicalGraph(t *testing.T) {
	definitions := lifecycleDefinitions()
	definitions[2].Spec["transitions"] = map[string]any{"applied": []any{"hired"}}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	found := false
	for _, diagnostic := range diagnostics {
		found = found || diagnostic.Kind == "Action" && diagnostic.Name == "move_candidate" && diagnostic.Path == "spec.transitions.applied.0" && diagnostic.Code == "BEAN-E2201"
	}
	if !found {
		t.Fatalf("out-of-graph Action subset accepted: %v", diagnostics)
	}
}

func TestLifecycleSupportsPolicySpecificActionSubsetsAndTransactionSteps(t *testing.T) {
	definitions := lifecycleDefinitions()
	definitions[2].Spec["transitions"] = map[string]any{"applied": []any{"rejected"}}
	definitions = append(definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "reject_candidate"}, Spec: map[string]any{
		"entity": "candidate", "operation": "transaction", "lifecycle": "candidate_pipeline",
		"input":       map[string]any{"id": map[string]any{"type": "uuid", "required": true}},
		"transitions": map[string]any{"applied": []any{"rejected"}},
		"steps":       []any{map[string]any{"op": "transition", "values": map[string]any{"id": "$input.id", "stage": "rejected"}}},
	}})
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	if result.App.Actions["reject_candidate"].Lifecycle != "candidate_pipeline" {
		t.Fatalf("action=%+v", result.App.Actions["reject_candidate"])
	}
}

func TestLegacyTransitionStateCannotBeDerivedByUpdate(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "stage", "type": "enum", "options": []any{"applied", "hired"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_candidate"}, Spec: map[string]any{
			"entity": "candidate", "operation": "transition", "stateField": "stage", "transitions": map[string]any{"applied": []any{"hired"}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "update_candidate"}, Spec: map[string]any{
			"entity": "candidate", "operation": "update", "derive": map[string]any{"stage": "initial_stage"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "initial_stage"}, Spec: map[string]any{
			"result": "string", "expression": map[string]any{"source": "literal", "literal": "applied"},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == "Action" && diagnostic.Name == "update_candidate" && diagnostic.Path == "spec.derive.stage" && diagnostic.Code == "BEAN-E2201" {
			return
		}
	}
	t.Fatalf("legacy transition state derivation accepted: %v", diagnostics)
}

func lifecycleDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "stage", "type": "enum", "required": true, "options": []any{"applied", "interview", "hired", "rejected"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "candidate_pipeline"}, Spec: map[string]any{
			"entity": "candidate", "stateField": "stage", "initial": "applied",
			"transitions": map[string]any{"applied": []any{"interview", "rejected"}, "interview": []any{"hired", "rejected"}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_candidate"}, Spec: map[string]any{
			"entity": "candidate", "operation": "transition", "lifecycle": "candidate_pipeline",
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "initial_stage"}, Spec: map[string]any{
			"result": "string", "expression": map[string]any{"source": "literal", "literal": "applied"},
		}},
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
