package compiler_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/rule"
)

func TestRuleDefinitionCompilesIntoTypedAppIR(t *testing.T) {
	definitions := ruleDefinitions()
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	if result.App.FormatVersion != appir.CurrentFormat || appir.CurrentFormat != "bean/appir/v5" {
		t.Fatalf("format=%q", result.App.FormatVersion)
	}
	compiled := result.App.Rules["positive_total"]
	if compiled.Entity != "invoice" || compiled.Result != rule.Boolean || len(compiled.Input) != 0 || compiled.Expression.Op != "gt" {
		t.Fatalf("rule=%+v", compiled)
	}
	if result.App.Actions["update_invoice"].When != "positive_total" || result.App.Actions["update_invoice"].Derive["total"] != "calculated_total" {
		t.Fatalf("action=%+v", result.App.Actions["update_invoice"])
	}
	if result.App.Entities["invoice"].Validations["positive_total"] != "positive_total" {
		t.Fatalf("entity=%+v", result.App.Entities["invoice"])
	}
}

func TestRuleCapabilitiesAndSchemaExposeClosedContract(t *testing.T) {
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", "bean.agent/v1alpha1")
	if !reflect.DeepEqual(capabilities.SemanticPrimitives, []string{"Lifecycle", "Rule"}) {
		t.Fatalf("semantic primitives=%v", capabilities.SemanticPrimitives)
	}
	if !reflect.DeepEqual(capabilities.RuleOperators, rule.Operators()) || !reflect.DeepEqual(capabilities.RuleSources, rule.Sources()) {
		t.Fatalf("operators=%v sources=%v", capabilities.RuleOperators, capabilities.RuleSources)
	}
	if capabilities.MaxRuleNodes != rule.MaxNodes || capabilities.MaxRuleDepth != rule.MaxDepth || capabilities.MaxRuleLiteralBytes != rule.MaxLiteralBytes || capabilities.MaxRuleValueBytes != rule.MaxValueBytes {
		t.Fatalf("limits=%+v", capabilities)
	}
	schema := compiler.DefinitionSchemas()["Rule"]
	properties, ok := compiler.SchemaProperties(schema)
	if !ok || properties["entity"] == nil || properties["result"] == nil || properties["input"] == nil || properties["expression"] == nil {
		t.Fatalf("Rule properties=%v", properties)
	}
	resultSchema := properties["result"].(map[string]any)
	if !reflect.DeepEqual(resultSchema["enum"], []string{"boolean", "date", "datetime", "integer", "number", "string", "strings"}) {
		t.Fatalf("Rule result schema=%v", resultSchema)
	}
}

func TestRuleDiagnosticsAreStableAndFailClosed(t *testing.T) {
	tests := []struct {
		name string
		edit func([]definition.Definition)
		kind string
		path string
		code string
	}{
		{"missing entity", func(defs []definition.Definition) { defs[1].Spec["entity"] = "missing" }, "Rule", "spec.entity", "BEAN-E2001"},
		{"unknown operator", func(defs []definition.Definition) { defs[1].Spec["expression"].(map[string]any)["op"] = "execute" }, "Rule", "spec.expression.op", "BEAN-E2351"},
		{"unknown input", func(defs []definition.Definition) { defs[2].Spec["expression"].(map[string]any)["path"] = "missing" }, "Rule", "spec.expression.path", "BEAN-E2351"},
		{"empty leaf op", func(defs []definition.Definition) { defs[2].Spec["expression"].(map[string]any)["op"] = "" }, "Rule", "spec.expression.op", "BEAN-E2351"},
		{"null leaf op", func(defs []definition.Definition) { defs[2].Spec["expression"].(map[string]any)["op"] = nil }, "Rule", "spec.expression.op", "BEAN-E2351"},
		{"empty operator source", func(defs []definition.Definition) { defs[1].Spec["expression"].(map[string]any)["source"] = "" }, "Rule", "spec.expression.source", "BEAN-E2351"},
		{"null operator source", func(defs []definition.Definition) { defs[1].Spec["expression"].(map[string]any)["source"] = nil }, "Rule", "spec.expression.source", "BEAN-E2351"},
		{"empty literal path", func(defs []definition.Definition) {
			defs[1].Spec["expression"].(map[string]any)["args"].([]any)[1].(map[string]any)["path"] = ""
		}, "Rule", "spec.expression.args.1.path", "BEAN-E2351"},
		{"null literal path", func(defs []definition.Definition) {
			defs[1].Spec["expression"].(map[string]any)["args"].([]any)[1].(map[string]any)["path"] = nil
		}, "Rule", "spec.expression.args.1.path", "BEAN-E2351"},
		{"empty leaf args", func(defs []definition.Definition) { defs[2].Spec["expression"].(map[string]any)["args"] = []any{} }, "Rule", "spec.expression", "BEAN-E2351"},
		{"null leaf args", func(defs []definition.Definition) { defs[2].Spec["expression"].(map[string]any)["args"] = nil }, "Rule", "spec.expression", "BEAN-E2351"},
		{"result mismatch", func(defs []definition.Definition) { defs[1].Spec["result"] = "string" }, "Rule", "spec.result", "BEAN-E2351"},
		{"null boolean result", func(defs []definition.Definition) {
			defs[1].Spec["expression"] = map[string]any{"source": "literal", "literal": nil}
		}, "Rule", "spec.result", "BEAN-E2351"},
		{"conditionally null boolean result", func(defs []definition.Definition) {
			defs[1].Spec["expression"] = map[string]any{"op": "if", "args": []any{
				map[string]any{"op": "eq", "args": []any{
					map[string]any{"source": "this", "path": "total"},
					map[string]any{"source": "literal", "literal": 0},
				}},
				map[string]any{"source": "literal", "literal": nil},
				map[string]any{"source": "literal", "literal": true},
			}}
		}, "Rule", "spec.result", "BEAN-E2351"},
		{"forbidden input", func(defs []definition.Definition) {
			defs[2].Spec["input"].(map[string]any)["amount"].(map[string]any)["type"] = "password"
		}, "Rule", "spec.input.amount.type", "BEAN-E2351"},
		{"sensitive Action input", func(defs []definition.Definition) {
			defs[3].Spec["input"].(map[string]any)["amount"].(map[string]any)["sensitive"] = true
		}, "Action", "spec.derive.total", "BEAN-E2351"},
		{"missing guard", func(defs []definition.Definition) { defs[3].Spec["when"] = "missing" }, "Action", "spec.when", "BEAN-E2001"},
		{"wrong derive result", func(defs []definition.Definition) {
			defs[3].Spec["derive"].(map[string]any)["note"] = "calculated_total"
		}, "Action", "spec.derive.note", "BEAN-E2351"},
		{"wrong invariant entity", func(defs []definition.Definition) {
			defs[0].Spec["validations"] = map[string]any{"positive_total": "other_entity"}
		}, "Entity", "spec.validations.positive_total", "BEAN-E2351"},
		{"derive dependency", func(defs []definition.Definition) {
			defs[3].Spec["derive"] = map[string]any{"amount": "calculated_total", "total": "calculated_total"}
		}, "Action", "spec.derive.total", "BEAN-E2351"},
		{"derived identifier", func(defs []definition.Definition) {
			defs[2].Spec["result"] = "string"
			defs[2].Spec["input"] = map[string]any{}
			defs[2].Spec["expression"] = map[string]any{"source": "literal", "literal": "00000000-0000-4000-8000-000000000001"}
			defs[3].Spec["derive"] = map[string]any{"id": "calculated_total"}
		}, "Action", "spec.derive.id", "BEAN-E2351"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := ruleDefinitions()
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

func ruleDefinitions() []definition.Definition {
	literalZero, _ := json.Marshal(0)
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "invoice"}, Spec: map[string]any{
			"fields": []any{
				map[string]any{"name": "total", "type": "money"},
				map[string]any{"name": "note", "type": "string"},
			},
			"validations": map[string]any{"positive_total": "positive_total"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "positive_total"}, Spec: map[string]any{
			"entity": "invoice", "result": "boolean",
			"expression": map[string]any{"op": "gt", "args": []any{
				map[string]any{"source": "this", "path": "total"},
				map[string]any{"source": "literal", "literal": json.RawMessage(literalZero)},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "calculated_total"}, Spec: map[string]any{
			"entity": "invoice", "result": "number",
			"input":      map[string]any{"amount": map[string]any{"type": "money"}},
			"expression": map[string]any{"source": "input", "path": "amount"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "update_invoice"}, Spec: map[string]any{
			"entity": "invoice", "operation": "update", "when": "positive_total",
			"input": map[string]any{
				"amount": map[string]any{"type": "money"},
				"note":   map[string]any{"type": "string"},
				"total":  map[string]any{"type": "money"},
			},
			"derive": map[string]any{"total": "calculated_total"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "other_entity"}, Spec: map[string]any{
			"result": "boolean", "expression": map[string]any{"source": "literal", "literal": true},
		}},
	}
}
