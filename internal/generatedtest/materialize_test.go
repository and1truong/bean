package generatedtest_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/generatedtest"
)

func TestMaterializeReplaysExplicitExpectationsInCanonicalSuites(t *testing.T) {
	bundle := definition.Bundle{Name: "Generated", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "double"}, Spec: map[string]any{
			"result": "number", "input": map[string]any{"value": map[string]any{"type": "integer", "required": true}},
			"expression": map[string]any{"op": "multiply", "args": []any{map[string]any{"source": "input", "path": "value"}, map[string]any{"source": "literal", "literal": 2}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "z_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Rule", "name": "double"}, "tests": []any{
				map[string]any{"name": "z_case", "input": map[string]any{"value": 3}, "expect": map[string]any{"result": 6}},
				map[string]any{"name": "a_case", "input": map[string]any{"value": 0}, "expect": map[string]any{"result": 0}},
			},
		}},
	}}

	first, firstOrigins, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	second, secondOrigins, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 || !reflect.DeepEqual(first, second) || !reflect.DeepEqual(firstOrigins, secondOrigins) {
		t.Fatalf("materialization is not deterministic: diagnostics=%v", diagnostics)
	}
	if len(first.Definitions) != 3 {
		t.Fatalf("definitions=%d", len(first.Definitions))
	}
	origin, exists := firstOrigins["generated_replay_z_contract"]
	if !exists || origin.Family != "replay" || origin.Source.Kind != "Rule" || origin.Source.Name != "double" || origin.Suite != "z_contract" {
		t.Fatalf("origin=%+v exists=%v", origin, exists)
	}
	compiled := compiler.Compile("test", 1, first.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	generated := compiled.App.TestSuites["generated_replay_z_contract"]
	if generated.Target.Kind != "Rule" || generated.Target.Name != "double" || len(generated.Tests) != 2 || generated.Tests[0].Name != "replay_a_case" || generated.Tests[1].Name != "replay_z_case" {
		t.Fatalf("generated=%+v", generated)
	}
	if string(generated.Tests[1].Expect.Result) != "6" {
		t.Fatalf("generated expectation=%s", generated.Tests[1].Expect.Result)
	}
}

func TestMaterializeRejectsReservedGeneratedSuiteNames(t *testing.T) {
	bundle := definition.Bundle{Name: "Collision", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "always"}, Spec: map[string]any{"result": "boolean", "expression": map[string]any{"source": "literal", "literal": true}}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "generated_replay_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Rule", "name": "always"}, "tests": []any{map[string]any{"name": "works", "expect": map[string]any{"result": true}}},
		}},
	}}

	_, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 1 || diagnostics[0].Code != "BEAN-T1101" || diagnostics[0].Kind != "TestSuite" || diagnostics[0].Name != "generated_replay_contract" || diagnostics[0].Path != "metadata.name" {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}

func TestMaterializeGeneratesPolicyDenialAndInvalidTransitionCases(t *testing.T) {
	const ticketID = "00000000-0000-4000-8000-000000000001"
	bundle := definition.Bundle{Name: "Workflow", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "manager"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "manager_write"}, Spec: map[string]any{"writeRoles": []any{"manager"}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "ticket"}, Spec: map[string]any{
			"policy": "manager_write", "fields": []any{
				map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"open", "closed"}},
				map[string]any{"name": "note", "type": "string"},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "ticket_flow"}, Spec: map[string]any{"entity": "ticket", "initial": "open", "transitions": map[string]any{"open": []any{"closed"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "close_ticket"}, Spec: map[string]any{
			"entity": "ticket", "operation": "transition", "lifecycle": "ticket_flow", "policy": "manager_write",
			"input": map[string]any{"note": map[string]any{"type": "string"}},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "close_ticket_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "close_ticket"}, "tests": []any{map[string]any{
				"name": "closes_ticket", "fixtures": map[string]any{"ticket": []any{map[string]any{"id": ticketID, "status": "open"}}},
				"context": map[string]any{"actor": map[string]any{"id": "00000000-0000-4000-8000-000000000002", "roles": []any{"manager"}}, "time": "2026-09-01T10:00:00Z"},
				"input":   map[string]any{"id": ticketID, "status": "closed"}, "expect": map[string]any{"result": map[string]any{"status": "closed"}},
			}},
		}},
	}}

	materialized, origins, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	policySuite := compiled.App.TestSuites["generated_policy_close_ticket_contract"]
	if origins[policySuite.Name].Family != "policy" || len(policySuite.Tests) != 1 || policySuite.Tests[0].Name != "deny_closes_ticket" || policySuite.Tests[0].Context.Actor != nil || policySuite.Tests[0].Expect.Error != "conflict" || !policySuite.Tests[0].Expect.NoChanges || !policySuite.Tests[0].Expect.NoEvents {
		t.Fatalf("policy suite=%+v origin=%+v", policySuite, origins[policySuite.Name])
	}
	transitionSuite := compiled.App.TestSuites["generated_transition_close_ticket_contract"]
	if origins[transitionSuite.Name].Family != "transition" || len(transitionSuite.Tests) != 1 || transitionSuite.Tests[0].Name != "invalid_closes_ticket" || transitionSuite.Tests[0].Input["status"] != "open" || transitionSuite.Tests[0].Expect.Error != "conflict" {
		t.Fatalf("transition suite=%+v origin=%+v", transitionSuite, origins[transitionSuite.Name])
	}

	bundle.Definitions = append(bundle.Definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "actor_present"}, Spec: map[string]any{
		"entity": "ticket", "result": "boolean", "expression": map[string]any{"op": "is_not_null", "args": []any{map[string]any{"source": "user", "path": "id"}}},
	}})
	bundle.Definitions[2].Spec["validations"] = map[string]any{"actor_present": "actor_present"}
	materialized, _, diagnostics = generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("validated diagnostics=%v", diagnostics)
	}
	compiled = compiler.Compile("test", 1, materialized.Definitions)
	if _, exists := compiled.App.TestSuites["generated_policy_close_ticket_contract"]; exists {
		t.Fatal("Policy-negative case was generated with a confounding Entity validation")
	}
	if _, exists := compiled.App.TestSuites["generated_transition_close_ticket_contract"]; exists {
		t.Fatal("transition-negative case was generated with a confounding Entity validation")
	}

	delete(bundle.Definitions[2].Spec, "validations")
	bundle.Definitions[4].Spec["when"] = "actor_present"
	materialized, _, diagnostics = generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("guarded diagnostics=%v", diagnostics)
	}
	compiled = compiler.Compile("test", 1, materialized.Definitions)
	if _, exists := compiled.App.TestSuites["generated_policy_close_ticket_contract"]; exists {
		t.Fatal("Policy-negative case was generated with a confounding Action guard")
	}
	if _, exists := compiled.App.TestSuites["generated_transition_close_ticket_contract"]; exists {
		t.Fatal("transition-negative case was generated with a confounding Action guard")
	}

	delete(bundle.Definitions[4].Spec, "when")
	bundle.Definitions = append(bundle.Definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "derived_note"}, Spec: map[string]any{
		"entity": "ticket", "result": "string", "expression": map[string]any{"source": "literal", "literal": "derived"},
	}})
	bundle.Definitions[4].Spec["derive"] = map[string]any{"note": "derived_note"}
	materialized, _, diagnostics = generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("derived diagnostics=%v", diagnostics)
	}
	compiled = compiler.Compile("test", 1, materialized.Definitions)
	if _, exists := compiled.App.TestSuites["generated_transition_close_ticket_contract"]; exists {
		t.Fatal("transition-negative case was generated with an unproven Action derivation")
	}
}

func TestMaterializeSkipsTransitionNegativeForValidatedEntity(t *testing.T) {
	const ticketID = "00000000-0000-4000-8000-000000000001"
	bundle := definition.Bundle{Name: "Validated workflow", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "status_not_open"}, Spec: map[string]any{
			"entity": "ticket", "result": "boolean", "expression": map[string]any{"op": "ne", "args": []any{
				map[string]any{"source": "this", "path": "status"}, map[string]any{"source": "literal", "literal": "open"},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "ticket"}, Spec: map[string]any{
			"fields":      []any{map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"open", "closed"}}},
			"validations": map[string]any{"status_not_open": "status_not_open"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "ticket_flow"}, Spec: map[string]any{"entity": "ticket", "initial": "open", "transitions": map[string]any{"open": []any{"closed"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "close_ticket"}, Spec: map[string]any{"entity": "ticket", "operation": "transition", "lifecycle": "ticket_flow"}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "close_ticket_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "close_ticket"}, "tests": []any{map[string]any{
				"name": "closes_ticket", "fixtures": map[string]any{"ticket": []any{map[string]any{"id": ticketID, "status": "open"}}},
				"context": map[string]any{"time": "2026-09-01T10:00:00Z"},
				"input":   map[string]any{"id": ticketID, "status": "closed"}, "expect": map[string]any{"result": map[string]any{"status": "closed"}},
			}},
		}},
	}}

	materialized, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if _, exists := compiled.App.TestSuites["generated_transition_close_ticket_contract"]; exists {
		t.Fatal("transition-negative case was generated with a confounding Entity validation")
	}
}

func TestMaterializeGeneratesDemoSeedCRUDSuites(t *testing.T) {
	bundle := definition.Bundle{Name: "CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}

	materialized, origins, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	for _, action := range []string{"note_create", "note_delete", "note_update"} {
		suite := compiled.App.TestSuites["generated_crud_"+action]
		if origins[suite.Name].Family != "crud" || suite.Target.Kind != "Action" || suite.Target.Name != action || len(suite.Tests) != 1 || suite.Tests[0].Name == "" {
			t.Fatalf("action=%s suite=%+v origin=%+v", action, suite, origins[suite.Name])
		}
	}
}

func TestMaterializeSkipsCRUDWithUnsatisfiedCustomActionInput(t *testing.T) {
	bundle := definition.Bundle{Name: "Custom CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "note_create"}, Spec: map[string]any{"entity": "note", "operation": "create", "input": map[string]any{"token": map[string]any{"type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "note_update"}, Spec: map[string]any{"entity": "note", "operation": "update", "input": map[string]any{"reason": map[string]any{"type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}

	materialized, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	if _, exists := compiled.App.TestSuites["generated_crud_note_create"]; exists {
		t.Fatal("generated create case cannot satisfy required token input")
	}
	if _, exists := compiled.App.TestSuites["generated_crud_note_update"]; exists {
		t.Fatal("generated update case cannot satisfy required reason input")
	}
	if _, exists := compiled.App.TestSuites["generated_crud_note_delete"]; !exists {
		t.Fatal("compatible delete case was not generated")
	}
}

func TestMaterializeSkipsCRUDWhenGeneratedPrincipalIsUnauthorized(t *testing.T) {
	bundle := definition.Bundle{Name: "Restricted CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "specific_user"}, Spec: map[string]any{"condition": map[string]any{
			"op": "eq", "left": map[string]any{"source": "record", "name": "title"}, "right": map[string]any{"source": "user", "name": "email"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"policy": "specific_user", "fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}

	materialized, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	for _, action := range []string{"note_create", "note_delete", "note_update"} {
		if _, exists := compiled.App.TestSuites["generated_crud_"+action]; exists {
			t.Fatalf("unauthorized generated case exists for %s", action)
		}
	}
}

func TestMaterializeSkipsGuardedCRUD(t *testing.T) {
	bundle := definition.Bundle{Name: "Guarded CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "demo_user"}, Spec: map[string]any{
			"entity": "note", "result": "boolean", "expression": map[string]any{"op": "eq", "args": []any{
				map[string]any{"source": "user", "path": "email"}, map[string]any{"source": "literal", "literal": "demo@bean.local"},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "note_update"}, Spec: map[string]any{"entity": "note", "operation": "update", "when": "demo_user"}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}

	materialized, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	if _, exists := compiled.App.TestSuites["generated_crud_note_update"]; exists {
		t.Fatal("guarded update case was generated without a proven allow result")
	}
	for _, action := range []string{"note_create", "note_delete"} {
		if _, exists := compiled.App.TestSuites["generated_crud_"+action]; !exists {
			t.Fatalf("compatible generated case is missing for %s", action)
		}
	}

	delete(bundle.Definitions[2].Spec, "when")
	bundle.Definitions = append(bundle.Definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "derived_title"}, Spec: map[string]any{
		"entity": "note", "result": "string", "expression": map[string]any{"source": "literal", "literal": "derived"},
	}})
	bundle.Definitions[2].Spec["derive"] = map[string]any{"title": "derived_title"}
	materialized, _, diagnostics = generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("derived diagnostics=%v", diagnostics)
	}
	compiled = compiler.Compile("test", 1, materialized.Definitions)
	if _, exists := compiled.App.TestSuites["generated_crud_note_update"]; exists {
		t.Fatal("derived update case was generated without a proven derivation result")
	}
	for _, action := range []string{"note_create", "note_delete"} {
		if _, exists := compiled.App.TestSuites["generated_crud_"+action]; !exists {
			t.Fatalf("independent generated case is missing for %s", action)
		}
	}
}

func TestMaterializeSkipsValidatedCreateAndUpdateCRUD(t *testing.T) {
	bundle := definition.Bundle{Name: "Validated CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "demo_user"}, Spec: map[string]any{
			"entity": "note", "result": "boolean", "expression": map[string]any{"op": "eq", "args": []any{
				map[string]any{"source": "user", "path": "email"}, map[string]any{"source": "literal", "literal": "demo@bean.local"},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{
			"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}, "validations": map[string]any{"demo_user": "demo_user"},
		}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}

	materialized, _, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	compiled := compiler.Compile("test", 1, materialized.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", compiled.Diagnostics)
	}
	for _, action := range []string{"note_create", "note_update"} {
		if _, exists := compiled.App.TestSuites["generated_crud_"+action]; exists {
			t.Fatalf("unproven validated case exists for %s", action)
		}
	}
	if _, exists := compiled.App.TestSuites["generated_crud_note_delete"]; !exists {
		t.Fatal("delete case was omitted even though delete does not run Entity validations")
	}
}
