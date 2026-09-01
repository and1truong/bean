package semantictest_test

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/semantictest"
)

const (
	ticketID = "00000000-0000-4000-8000-000000000001"
	actorID  = "00000000-0000-4000-8000-000000000002"
	fixedNow = "2026-09-01T10:00:00Z"
)

func TestRuleSuitesUseProductionEvaluatorAndStableErrors(t *testing.T) {
	bundle := definition.Bundle{Name: "Rules", Definitions: ruleSuiteDefinitions()}
	results, diagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
	if len(results) != 1 || results[0].Status != "passed" || len(results[0].Cases) != 2 || results[0].Cases[0].ID != "TestSuite/division_contract/divide_by_zero" || results[0].Cases[1].ID != "TestSuite/division_contract/quotient" {
		t.Fatalf("results=%+v", results)
	}
}

func TestRuleSuiteUsesExplicitActorTenantTimeAndRequestContext(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "observer"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "context_stamp"}, Spec: map[string]any{"result": "boolean", "expression": map[string]any{"op": "and", "args": []any{
			map[string]any{"op": "eq", "args": []any{map[string]any{"source": "user", "path": "id"}, map[string]any{"source": "literal", "literal": actorID}}},
			map[string]any{"op": "eq", "args": []any{map[string]any{"source": "tenant", "path": "id"}, map[string]any{"source": "literal", "literal": ticketID}}},
			map[string]any{"op": "eq", "args": []any{map[string]any{"source": "context", "path": "now"}, map[string]any{"source": "context", "path": "now"}}},
			map[string]any{"op": "eq", "args": []any{map[string]any{"source": "context", "path": "request_id"}, map[string]any{"source": "literal", "literal": "request-1"}}},
		}}}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "context_contract"}, Spec: map[string]any{"target": map[string]any{"kind": "Rule", "name": "context_stamp"}, "tests": []any{map[string]any{
			"name":    "uses_only_injected_context",
			"context": map[string]any{"actor": map[string]any{"id": actorID, "roles": []any{"observer"}}, "tenant": ticketID, "time": fixedNow, "requestId": "request-1"},
			"expect":  map[string]any{"result": true},
		}}}},
	}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Context", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) != 0 || len(results) != 1 || results[0].Status != "passed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func TestActionSuitesUseProductionPolicyMutationEventAuditAndIsolation(t *testing.T) {
	bundle := definition.Bundle{Name: "Actions", Definitions: actionSuiteDefinitions()}
	results, diagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
	if len(results) != 1 || results[0].Status != "passed" || len(results[0].Cases) != 4 {
		t.Fatalf("results=%+v", results)
	}
}

func TestActionSuiteSeedProducesReplayableIDsAndLeavesNoState(t *testing.T) {
	const seededID = "7d6fdf57-4d6b-4211-a1cd-5cf4d28f2ae4"
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "note_create"}, Spec: map[string]any{"entity": "note", "operation": "create"}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "seed_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "note_create"},
			"tests": []any{map[string]any{
				"name": "creates_replayable_id", "context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow, "seed": 42}, "input": map[string]any{"title": "Deterministic"},
				"expect": map[string]any{"result": map[string]any{"id": seededID}, "changes": []any{map[string]any{"entity": "note", "id": seededID, "values": map[string]any{"title": "Deterministic"}}}},
			}},
		}},
	}
	bundle := definition.Bundle{Name: "Seed", Definitions: definitions}
	directory := t.TempDir()
	first, firstDiagnostics, err := semantictest.Run(context.Background(), bundle, directory)
	if err != nil || len(firstDiagnostics) != 0 {
		t.Fatalf("first=%+v diagnostics=%v err=%v", first, firstDiagnostics, err)
	}
	second, secondDiagnostics, err := semantictest.Run(context.Background(), bundle, directory)
	if err != nil || len(secondDiagnostics) != 0 || !reflect.DeepEqual(first, second) {
		t.Fatalf("first=%+v second=%+v diagnostics=%v err=%v", first, second, secondDiagnostics, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("semantic suite retained case state: entries=%v err=%v", entries, err)
	}
}

func TestActionSuiteCanAssertSoftDeletedRows(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{"softDelete": true}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "document_delete_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "document_delete"},
			"tests": []any{map[string]any{
				"name": "records_injected_delete_time", "fixtures": map[string]any{"document": []any{map[string]any{"id": ticketID}}},
				"context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow}, "input": map[string]any{"id": ticketID},
				"expect": map[string]any{"changes": []any{map[string]any{"entity": "document", "id": ticketID, "values": map[string]any{"deleted_at": fixedNow}}}},
			}},
		}},
	}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Soft delete", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) != 0 || len(results) != 1 || results[0].Status != "passed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func TestActionSuiteCanAssertHardDeletedRows(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "document_delete_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "document_delete"},
			"tests": []any{map[string]any{
				"name": "removes_record", "fixtures": map[string]any{"document": []any{map[string]any{"id": ticketID}}},
				"context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow}, "input": map[string]any{"id": ticketID},
				"expect": map[string]any{"changes": []any{map[string]any{"entity": "document", "id": ticketID, "absent": true}}},
			}},
		}},
	}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Hard delete", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) != 0 || len(results) != 1 || results[0].Status != "passed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func TestActionSuiteChangeRequiresAssertedValueDelta(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "document_update_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "document_update"},
			"tests": []any{map[string]any{
				"name": "same_value", "fixtures": map[string]any{"document": []any{map[string]any{"id": ticketID, "title": "unchanged"}}},
				"context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow}, "input": map[string]any{"id": ticketID, "title": "unchanged"},
				"expect": map[string]any{"changes": []any{map[string]any{"entity": "document", "id": ticketID, "values": map[string]any{"title": "unchanged"}}}},
			}},
		}},
	}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "No-op update", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "BEAN-T1001" || diagnostics[0].Path != "tests.same_value.expect.changes.0" || len(results) != 1 || results[0].Status != "failed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func TestAssertionFailuresAreStableAndDoNotExposeValues(t *testing.T) {
	definitions := ruleSuiteDefinitions()
	tests := definitions[1].Spec["tests"].([]any)
	tests[0].(map[string]any)["expect"].(map[string]any)["result"] = 99
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Broken", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) != 1 || diagnostics[0].Code != "BEAN-T1001" || diagnostics[0].Path != "tests.quotient.expect.result" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
	value, ok := diagnostics[0].Value.(map[string]any)
	if !ok || value["expectedDigest"] == nil || value["actualDigest"] == nil {
		t.Fatalf("diagnostic value=%#v", diagnostics[0].Value)
	}
}

func TestMaintainedSuitesCatchSeededBehaviorDefects(t *testing.T) {
	tests := []struct {
		name        string
		application string
		mutate      func([]definition.Definition)
	}{
		{"rule calculation", "commerce", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Rule", "order_item_total").Spec["expression"].(map[string]any)["op"] = "add"
		}},
		{"guard", "ats", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Action", "move_candidate").Spec["when"] = ""
		}},
		{"invariant", "booking", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Entity", "booking").Spec["validations"] = map[string]any{}
		}},
		{"event", "commerce", func(definitions []definition.Definition) {
			action := findDefinition(t, definitions, "Action", "place_order")
			steps := action.Spec["steps"].([]any)
			action.Spec["steps"] = append(steps[:2], steps[3:]...)
		}},
		{"mutation", "commerce", func(definitions []definition.Definition) {
			action := findDefinition(t, definitions, "Action", "place_order")
			steps := action.Spec["steps"].([]any)
			action.Spec["steps"] = append([]any{}, steps[1:]...)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle, err := examples.Load(test.application)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(bundle.Definitions)
			results, diagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
			if err != nil || len(diagnostics) == 0 || diagnostics[0].Code != "BEAN-T1001" {
				t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
			}
		})
	}
}

func TestActionSuiteCatchesSeededPermissionDefect(t *testing.T) {
	definitions := actionSuiteDefinitions()
	findDefinition(t, definitions, "Policy", "manager_write").Spec["writeRoles"] = []any{"viewer"}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Broken permission", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) == 0 || diagnostics[0].Code != "BEAN-T1001" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func findDefinition(t *testing.T, definitions []definition.Definition, kind, name string) *definition.Definition {
	t.Helper()
	for index := range definitions {
		if definitions[index].Kind == kind && definitions[index].Metadata.Name == name {
			return &definitions[index]
		}
	}
	t.Fatalf("missing %s/%s", kind, name)
	return nil
}

func ruleSuiteDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "divide"}, Spec: map[string]any{
			"result":     "number",
			"input":      map[string]any{"left": map[string]any{"type": "money", "required": true}, "right": map[string]any{"type": "money", "required": true}},
			"expression": map[string]any{"op": "divide", "args": []any{map[string]any{"source": "input", "path": "left"}, map[string]any{"source": "input", "path": "right"}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "division_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Rule", "name": "divide"},
			"tests": []any{
				map[string]any{"name": "quotient", "input": map[string]any{"left": 12, "right": 3}, "expect": map[string]any{"result": 4}},
				map[string]any{"name": "divide_by_zero", "input": map[string]any{"left": 12, "right": 0}, "expect": map[string]any{"error": "RULE_DIVIDE_BY_ZERO"}},
			},
		}},
	}
}

func actionSuiteDefinitions() []definition.Definition {
	success := true
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "manager"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "viewer"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "manager_write"}, Spec: map[string]any{"writeRoles": []any{"manager"}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "ticket"}, Spec: map[string]any{
			"policy": "manager_write",
			"fields": []any{
				map[string]any{"name": "title", "type": "string", "required": true},
				map[string]any{"name": "status", "type": "enum", "options": []any{"open", "closed"}, "required": true},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "ticket_close"}, Spec: map[string]any{
			"entity": "ticket", "operation": "transaction", "policy": "manager_write",
			"input":  map[string]any{"id": map[string]any{"type": "uuid", "required": true}},
			"output": map[string]any{"id": map[string]any{"type": "uuid", "required": true}, "status": map[string]any{"type": "enum", "options": []any{"open", "closed"}, "required": true}},
			"steps": []any{
				map[string]any{"op": "update", "entity": "ticket", "values": map[string]any{"id": "$input.id", "status": "closed"}},
				map[string]any{"op": "emit", "event": "ticket.closed", "values": map[string]any{"ticket_id": "$input.id"}},
				map[string]any{"op": "return", "values": map[string]any{"id": "$input.id", "status": "closed"}},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "ticket_close_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "ticket_close"},
			"tests": []any{
				actionCase("closes_again_in_fresh_state", "manager", map[string]any{"result": map[string]any{"id": ticketID, "status": "closed"}, "changes": []any{map[string]any{"entity": "ticket", "id": ticketID, "values": map[string]any{"status": "closed", "version": 2}}}, "events": []any{map[string]any{"topic": "ticket.closed", "payload": map[string]any{"ticket_id": ticketID}}}, "audit": []any{map[string]any{"action": "ticket_close", "actorId": actorID, "entity": "ticket", "entityId": ticketID, "success": success}}}),
				actionCase("manager_closes_ticket", "manager", map[string]any{"result": map[string]any{"id": ticketID, "status": "closed"}, "changes": []any{map[string]any{"entity": "ticket", "id": ticketID, "values": map[string]any{"status": "closed", "version": 2}}}, "events": []any{map[string]any{"topic": "ticket.closed", "payload": map[string]any{"ticket_id": ticketID}}}, "audit": []any{map[string]any{"action": "ticket_close", "actorId": actorID, "entity": "ticket", "entityId": ticketID, "success": success}}}),
				actionCase("side_effect_assertions_need_no_result", "manager", map[string]any{
					"changes": []any{map[string]any{"entity": "ticket", "id": ticketID, "values": map[string]any{"status": "closed"}}},
					"events":  []any{map[string]any{"topic": "ticket.closed", "payload": map[string]any{"ticket_id": ticketID}}},
				}),
				actionCase("viewer_is_denied", "viewer", map[string]any{"error": "conflict", "noChanges": true, "noEvents": true}),
			},
		}},
	}
}

func actionCase(name, role string, expect map[string]any) map[string]any {
	return map[string]any{
		"name":     name,
		"fixtures": map[string]any{"ticket": []any{map[string]any{"id": ticketID, "title": "Example", "status": "open"}}},
		"context":  map[string]any{"actor": map[string]any{"id": actorID, "roles": []any{role}}, "time": fixedNow, "requestId": "request-1"},
		"input":    map[string]any{"id": ticketID},
		"expect":   expect,
	}
}
