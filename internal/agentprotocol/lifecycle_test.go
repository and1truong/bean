package agentprotocol_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestLifecycleInspectionReferencesAndSemanticDiff(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"pending", "paid", "fulfilled"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "order_fulfillment"}, Spec: map[string]any{"entity": "order", "initial": "pending", "transitions": map[string]any{"pending": []any{"paid"}, "paid": []any{"fulfilled"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "advance_order"}, Spec: map[string]any{"entity": "order", "operation": "transition", "lifecycle": "order_fulfillment"}},
	}
	compiled := compiler.Compile("test", 1, definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	value, references, exists := agentprotocol.InspectedDefinition(compiled.App, "Lifecycle", "order_fulfillment")
	if !exists || value == nil || len(references) != 1 || references[0].Kind != "Entity" || references[0].Name != "order" {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	_, actionReferences, exists := agentprotocol.InspectedDefinition(compiled.App, "Action", "advance_order")
	if !exists || !hasReference(actionReferences, "lifecycle", "Lifecycle", "order_fulfillment") {
		t.Fatalf("action references=%+v", actionReferences)
	}

	candidate := compiled.App
	current := compiler.Compile("test", 1, definitions[:1]).App
	changes := agentprotocol.SemanticDiff(current, candidate)
	found := false
	for _, change := range changes {
		found = found || change.Operation == "add" && change.Path == "lifecycles.order_fulfillment"
	}
	if !found {
		t.Fatalf("changes=%+v", changes)
	}
}

func hasReference(references []agentprotocol.InspectedReference, path, kind, name string) bool {
	for _, reference := range references {
		if reference.Path == path && reference.Kind == kind && reference.Name == name {
			return true
		}
	}
	return false
}
