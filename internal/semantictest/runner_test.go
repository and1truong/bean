package semantictest_test

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/generatedtest"
	"github.com/beanruntime/bean/internal/rule"
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

func TestActionSuiteUsesOrderedOfflineExtensionMocks(t *testing.T) {
	const firstInvocationID = "00000000-0000-4000-8000-000000000020"
	const secondInvocationID = "00000000-0000-4000-8000-000000000021"
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Extension", Metadata: definition.Metadata{Name: "order_notification"}, Spec: map[string]any{
			"transport": "http", "endpoint": "https://provider.example/orders",
			"input": map[string]any{"message": map[string]any{"type": "string", "required": true}}, "output": map[string]any{"accepted": map[string]any{"type": "boolean", "required": true}},
			"permissions": []any{"network"}, "sideEffects": []any{"external_write"}, "authentication": "none", "timeoutSeconds": 5,
			"retry": map[string]any{"maxAttempts": 3, "delaySeconds": 60}, "idempotency": "required", "transaction": "after_commit", "failure": "retry_then_fail",
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction", "input": map[string]any{"first": map[string]any{"type": "string", "required": true}, "second": map[string]any{"type": "string", "required": true}},
			"steps": []any{
				map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"message": "$input.first"}},
				map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"message": "$input.second"}},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "notification_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "notify_order"}, "tests": []any{map[string]any{
				"name": "delivers_after_commit", "context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow, "ids": []any{firstInvocationID, secondInvocationID}},
				"input": map[string]any{"first": "ready", "second": "sent"}, "providers": map[string]any{"order_notification": []any{map[string]any{"output": map[string]any{"accepted": true}}, map[string]any{"output": map[string]any{"accepted": false}}}},
				"expect": map[string]any{"providerCalls": []any{
					map[string]any{"extension": "order_notification", "invocationId": firstInvocationID, "idempotencyKey": firstInvocationID, "input": map[string]any{"message": "ready"}},
					map[string]any{"extension": "order_notification", "invocationId": secondInvocationID, "idempotencyKey": secondInvocationID, "input": map[string]any{"message": "sent"}},
				}},
			}},
		}},
	}
	actionSteps := definitions[2].Spec["steps"].([]any)
	testCase := definitions[3].Spec["tests"].([]any)[0].(map[string]any)
	contextIDs := testCase["context"].(map[string]any)["ids"].([]any)
	providerResults := testCase["providers"].(map[string]any)["order_notification"].([]any)
	providerCalls := testCase["expect"].(map[string]any)["providerCalls"].([]any)
	for index := 2; index < 21; index++ {
		invocationID := fmt.Sprintf("00000000-0000-4000-8000-%012d", 20+index)
		actionSteps = append(actionSteps, map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"message": "$input.second"}})
		contextIDs = append(contextIDs, invocationID)
		providerResults = append(providerResults, map[string]any{"output": map[string]any{"accepted": true}})
		providerCalls = append(providerCalls, map[string]any{"extension": "order_notification", "invocationId": invocationID, "idempotencyKey": invocationID, "input": map[string]any{"message": "sent"}})
	}
	definitions[2].Spec["steps"] = actionSteps
	testCase["context"].(map[string]any)["ids"] = contextIDs
	testCase["providers"].(map[string]any)["order_notification"] = providerResults
	testCase["expect"].(map[string]any)["providerCalls"] = providerCalls
	bundle := definition.Bundle{Name: "Extensions", Definitions: definitions}
	results, diagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || len(diagnostics) != 0 || len(results) != 1 || results[0].Status != "passed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
	repeated, repeatedDiagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || len(repeatedDiagnostics) != 0 || !reflect.DeepEqual(results, repeated) {
		t.Fatalf("first=%+v repeated=%+v diagnostics=%v err=%v", results, repeated, repeatedDiagnostics, err)
	}
	testCase["providers"] = map[string]any{"order_notification": []any{map[string]any{"output": map[string]any{"accepted": true}}}}
	failed, failureDiagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || len(failureDiagnostics) != 1 || failureDiagnostics[0].Code != "BEAN-T1001" || failureDiagnostics[0].Path != "tests.delivers_after_commit.providers" || failed[0].Status != "failed" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", failed, failureDiagnostics, err)
	}
	repeatedFailed, repeatedFailureDiagnostics, err := semantictest.Run(context.Background(), bundle, t.TempDir())
	if err != nil || !reflect.DeepEqual(failed, repeatedFailed) || !reflect.DeepEqual(failureDiagnostics, repeatedFailureDiagnostics) {
		t.Fatalf("first=%+v repeated=%+v first diagnostics=%v repeated diagnostics=%v err=%v", failed, repeatedFailed, failureDiagnostics, repeatedFailureDiagnostics, err)
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

func TestActionSuiteAssertionDiagnosticsAreSortedByPath(t *testing.T) {
	const createdID = "00000000-0000-4000-8000-000000000010"
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "note_create_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "note_create"},
			"tests": []any{map[string]any{
				"name": "reports_in_path_order", "context": map[string]any{"actor": map[string]any{"roles": []any{"administrator"}}, "time": fixedNow, "ids": []any{createdID}}, "input": map[string]any{"title": "actual"},
				"expect": map[string]any{
					"result":  map[string]any{"id": "00000000-0000-4000-8000-000000000099"},
					"changes": []any{map[string]any{"entity": "note", "id": createdID, "values": map[string]any{"title": "expected"}}},
					"events":  []any{map[string]any{"topic": "note.expected", "payload": map[string]any{}}},
					"audit":   []any{map[string]any{"action": "unexpected_action"}},
				},
			}},
		}},
	}
	_, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Ordered failures", Definitions: definitions}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	paths := make([]string, len(diagnostics))
	for index := range diagnostics {
		paths[index] = diagnostics[index].Path
	}
	want := []string{
		"tests.reports_in_path_order.expect.audit",
		"tests.reports_in_path_order.expect.changes.0",
		"tests.reports_in_path_order.expect.events",
		"tests.reports_in_path_order.expect.result",
	}
	if !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths=%v diagnostics=%v", paths, diagnostics)
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

func TestGeneratedReplaysCatchSeededRuleAndConsumerDefects(t *testing.T) {
	tests := []struct {
		name        string
		application string
		mutate      func([]definition.Definition)
	}{
		{"calculation", "commerce", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Rule", "order_item_total").Spec["expression"].(map[string]any)["op"] = "add"
		}},
		{"guard", "ats", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Action", "move_candidate").Spec["when"] = ""
		}},
		{"validation", "booking", func(definitions []definition.Definition) {
			findDefinition(t, definitions, "Entity", "booking").Spec["validations"] = map[string]any{}
		}},
		{"context", "booking", func(definitions []definition.Definition) {
			tests := findDefinition(t, definitions, "TestSuite", "book_resource_contract").Spec["tests"].([]any)
			tests[0].(map[string]any)["context"].(map[string]any)["time"] = "2026-09-02T10:00:00Z"
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundle, err := examples.Load(test.application)
			if err != nil {
				t.Fatal(err)
			}
			test.mutate(bundle.Definitions)
			_, diagnostics, err := semantictest.RunGenerated(context.Background(), bundle, t.TempDir())
			if err != nil || !hasGeneratedAssertionFailure(diagnostics) {
				t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
			}
		})
	}

	t.Run("resource limit", func(t *testing.T) {
		piece := strings.Repeat("x", rule.MaxLiteralBytes-2)
		args := make([]any, 5)
		for index := range args {
			args[index] = map[string]any{"source": "literal", "literal": piece}
		}
		definitions := []definition.Definition{
			{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "bounded_concat"}, Spec: map[string]any{"result": "string", "expression": map[string]any{"op": "concat", "args": args}}},
			{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "bounded_concat_contract"}, Spec: map[string]any{
				"target": map[string]any{"kind": "Rule", "name": "bounded_concat"}, "tests": []any{map[string]any{"name": "rejects_large_result", "expect": map[string]any{"error": rule.CodeLimit}}},
			}},
		}
		findDefinition(t, definitions, "Rule", "bounded_concat").Spec["expression"] = args[0]
		_, diagnostics, err := semantictest.RunGenerated(context.Background(), definition.Bundle{Name: "Limit", Definitions: definitions}, t.TempDir())
		if err != nil || !hasGeneratedAssertionFailure(diagnostics) {
			t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
		}
	})
}

func hasGeneratedAssertionFailure(diagnostics []definition.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "BEAN-T1001" && strings.HasPrefix(diagnostic.Name, "generated_replay_") {
			return true
		}
	}
	return false
}

func TestActionSuiteCatchesSeededPermissionDefect(t *testing.T) {
	definitions := actionSuiteDefinitions()
	findDefinition(t, definitions, "Policy", "manager_write").Spec["writeRoles"] = []any{"viewer"}
	results, diagnostics, err := semantictest.Run(context.Background(), definition.Bundle{Name: "Broken permission", Definitions: definitions}, t.TempDir())
	if err != nil || len(diagnostics) == 0 || diagnostics[0].Code != "BEAN-T1001" {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
}

func TestGeneratedNegativeCasesCatchSeededPolicyAndTransitionDefects(t *testing.T) {
	t.Run("policy", func(t *testing.T) {
		bundle := definition.Bundle{Name: "Policy", Definitions: actionSuiteDefinitions()}
		materialized, origins, diagnostics := generatedtest.Materialize(bundle)
		if len(diagnostics) != 0 || origins["generated_policy_ticket_close_contract"].Family != "policy" {
			t.Fatalf("origins=%v diagnostics=%v", origins, diagnostics)
		}
		findDefinition(t, materialized.Definitions, "Policy", "manager_write").Spec["writeRoles"] = []any{}
		_, diagnostics, err := semantictest.Run(context.Background(), materialized, t.TempDir())
		if err != nil || !hasGeneratedFamilyFailure(diagnostics, "generated_policy_") {
			t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
		}
	})

	t.Run("transition", func(t *testing.T) {
		bundle, err := examples.Load("commerce")
		if err != nil {
			t.Fatal(err)
		}
		materialized, origins, diagnostics := generatedtest.Materialize(bundle)
		if len(diagnostics) != 0 || origins["generated_transition_advance_order_contract"].Family != "transition" {
			t.Fatalf("origins=%v diagnostics=%v", origins, diagnostics)
		}
		lifecycle := findDefinition(t, materialized.Definitions, "Lifecycle", "order_fulfillment")
		lifecycle.Spec["transitions"].(map[string]any)["pending_payment"] = []any{"paid", "pending_payment"}
		_, diagnostics, err = semantictest.Run(context.Background(), materialized, t.TempDir())
		if err != nil || !hasGeneratedFamilyFailure(diagnostics, "generated_transition_") {
			t.Fatalf("diagnostics=%v err=%v", diagnostics, err)
		}
	})
}

func TestGeneratedCRUDSuitesUseProductionActions(t *testing.T) {
	bundle := definition.Bundle{Name: "CRUD", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"note": map[string]any{"count": 1}}}},
	}}
	results, diagnostics, err := semantictest.RunGenerated(context.Background(), bundle, t.TempDir())
	if err != nil || len(diagnostics) != 0 || len(results) != 3 {
		t.Fatalf("results=%+v diagnostics=%v err=%v", results, diagnostics, err)
	}
	for _, result := range results {
		if result.Status != "passed" || result.Evidence == nil || result.Evidence.Family != "crud" || len(result.Cases) != 1 || result.Cases[0].Status != "passed" {
			t.Fatalf("result=%+v", result)
		}
	}
}

func hasGeneratedFamilyFailure(diagnostics []definition.Diagnostic, prefix string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "BEAN-T1001" && strings.HasPrefix(diagnostic.Name, prefix) {
			return true
		}
	}
	return false
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
