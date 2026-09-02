package agentprotocol_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/rule"
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
	value, references, exists := compiler.InspectDefinition(compiled.App, "Lifecycle", "order_fulfillment")
	if !exists || value == nil || len(references) != 1 || references[0].Kind != "Entity" || references[0].Name != "order" {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	_, actionReferences, exists := compiler.InspectDefinition(compiled.App, "Action", "advance_order")
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

func TestRuleInspectionReferencesRedactionAndSemanticDiff(t *testing.T) {
	app := compiler.Compile("test", 1, ruleProtocolDefinitions(t)).App
	value, references, exists := compiler.InspectDefinition(app, "Rule", "minimum_total")
	if !exists || value == nil || !hasReference(references, "entity", "Entity", "invoice") {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	_, actionReferences, exists := compiler.InspectDefinition(app, "Action", "update_invoice")
	if !exists || !hasReference(actionReferences, "when", "Rule", "minimum_total") {
		t.Fatalf("action references=%+v", actionReferences)
	}
	redacted, _ := json.Marshal(agentprotocol.RedactedApp(app))
	if strings.Contains(string(redacted), "1234567") || !strings.Contains(string(redacted), "REDACTED") {
		t.Fatalf("redacted=%s", redacted)
	}
	candidate, _ := app.Clone()
	candidate.Rules["minimum_total"] = appir.Rule{Name: "minimum_total", Entity: "invoice", Result: rule.Boolean, Expression: rule.Expression{Source: "literal", Literal: json.RawMessage("false")}}
	changes := agentprotocol.SemanticDiff(app, candidate)
	found := false
	for _, change := range changes {
		found = found || strings.HasPrefix(change.Path, "rules.minimum_total.expression")
	}
	if !found {
		t.Fatalf("changes=%+v", changes)
	}
}

func TestTestSuiteInspectionReferencesRedactionAndSemanticDiff(t *testing.T) {
	definitions := ruleProtocolDefinitions(t)
	definitions = append(definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "minimum_total_contract"}, Spec: map[string]any{
		"target": map[string]any{"kind": "Rule", "name": "minimum_total"},
		"tests":  []any{map[string]any{"name": "allows_large_total", "this": map[string]any{"total": 1234568}, "context": map[string]any{"time": "2026-09-01T10:00:00Z", "seed": 42}, "expect": map[string]any{"result": true}}},
	}})
	app := compiler.Compile("test", 1, definitions).App
	suiteWithProvider := app.TestSuites["minimum_total_contract"]
	suiteWithProvider.Tests[0].Providers = map[string][]appir.TestProviderResult{"provider": {{Output: map[string]any{"token": "provider-secret"}}}}
	suiteWithProvider.Tests[0].Expect.ProviderCalls = []appir.TestProviderCall{{Extension: "provider", InvocationID: "provider-invocation", IdempotencyKey: "provider-invocation", Input: map[string]any{"token": "provider-input"}}}
	app.TestSuites["minimum_total_contract"] = suiteWithProvider
	value, references, exists := compiler.InspectDefinition(app, "TestSuite", "minimum_total_contract")
	if !exists || value == nil || !hasReference(references, "target.name", "Rule", "minimum_total") {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	redactedApp := agentprotocol.RedactedApp(app)
	redactedCase := redactedApp.TestSuites["minimum_total_contract"].Tests[0]
	if redactedCase.Context.Time == "2026-09-01T10:00:00Z" || redactedCase.Context.Seed == nil || *redactedCase.Context.Seed == 42 {
		t.Fatalf("redacted context=%+v", redactedCase.Context)
	}
	if redactedCase.Providers["provider"][0].Output["token"] == "provider-secret" || redactedCase.Expect.ProviderCalls[0].Input["token"] == "provider-input" || redactedCase.Expect.ProviderCalls[0].InvocationID == "provider-invocation" {
		t.Fatalf("redacted provider case=%+v", redactedCase)
	}
	redacted, _ := json.Marshal(redactedApp)
	if strings.Contains(string(redacted), "1234568") || strings.Contains(string(redacted), "provider-secret") || strings.Contains(string(redacted), "provider-input") || !strings.Contains(string(redacted), "REDACTED") {
		t.Fatalf("redacted=%s", redacted)
	}
	for _, mutate := range []func(*appir.TestCase){
		func(test *appir.TestCase) { test.Context.Time = "2026-09-02T10:00:00Z" },
		func(test *appir.TestCase) {
			seed := int64(43)
			test.Context.Seed = &seed
		},
	} {
		candidate, _ := app.Clone()
		suite := candidate.TestSuites["minimum_total_contract"]
		mutate(&suite.Tests[0])
		candidate.TestSuites["minimum_total_contract"] = suite
		changes := agentprotocol.SemanticDiff(app, candidate)
		if len(changes) != 1 || !strings.HasPrefix(changes[0].Path, "testSuites.minimum_total_contract.tests") {
			t.Fatalf("changes=%+v", changes)
		}
	}
}

func ruleProtocolDefinitions(t *testing.T) []definition.Definition {
	t.Helper()
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "invoice"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "total", "type": "money"}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "minimum_total"}, Spec: map[string]any{"entity": "invoice", "result": "boolean", "expression": map[string]any{"op": "gt", "args": []any{map[string]any{"source": "this", "path": "total"}, map[string]any{"source": "literal", "literal": 1234567}}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "update_invoice"}, Spec: map[string]any{"entity": "invoice", "operation": "update", "when": "minimum_total"}},
	}
}

func hasReference(references []compiler.DefinitionReference, path, kind, name string) bool {
	for _, reference := range references {
		if reference.Path == path && reference.Kind == kind && reference.Name == name {
			return true
		}
	}
	return false
}
