package agentprotocol

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	beanctx "github.com/beanruntime/bean/internal/context"
)

const protocolFixtureSource = `apiVersion: bean/v1alpha1
name: Agent Protocol Test
---
kind: Entity
name: item
label: Item
owner: true
tenant: true
policy: member_scope
fields:
  - {label: Title, name: title, required: true, type: string}
---
kind: Policy
name: member_scope
owner: true
tenant: true
readRoles: [member]
writeRoles: [member]
---
kind: View
name: items
entity: item
fields: [id, title, owner_id, tenant_id]
policy: member_scope
---
kind: Action
name: create_item
entity: item
operation: transaction
policy: member_scope
input:
  title: {name: title, required: true, type: string}
steps:
  - op: create
    values: {title: $input.title}
`

func TestRegistryDefinesStableProviderNeutralOperations(t *testing.T) {
	service := New()
	operations := service.Operations(Principal{Planes: AllPlanes()})
	names := make([]string, len(operations))
	planes := map[Plane]int{}
	for index, operation := range operations {
		names[index] = operation.Name
		planes[operation.Plane]++
		if operation.Description == "" || operation.InputSchema["additionalProperties"] != false {
			t.Fatalf("operation=%+v", operation)
		}
	}
	want := []string{
		"bean.application.execute", "bean.application.query",
		"bean.definition.capabilities", "bean.definition.inspect", "bean.definition.schema", "bean.definition.validate",
		"bean.release.diff", "bean.release.plan", "bean.release.publish", "bean.release.test",
	}
	if !reflect.DeepEqual(names, want) || planes[DefinitionPlane] != 4 || planes[ReleasePlane] != 4 || planes[ApplicationPlane] != 2 {
		t.Fatalf("names=%v planes=%v", names, planes)
	}
}

func TestAuthorizationFiltersDiscoveryAndRunsBeforeHandler(t *testing.T) {
	service := New()
	called := false
	service.Register("bean.release.publish", func(context.Context, json.RawMessage, Principal) Outcome {
		called = true
		return success(map[string]any{"published": true})
	})
	principal := Principal{Planes: map[Plane]bool{DefinitionPlane: true}}
	for _, operation := range service.Operations(principal) {
		if operation.Plane != DefinitionPlane {
			t.Fatalf("unauthorized operation discovered: %+v", operation)
		}
	}
	outcome := service.Call(context.Background(), "bean.release.publish", json.RawMessage(`{"file":"missing","target":"missing"}`), principal)
	if outcome.OK || outcome.Error == nil || outcome.Error.Code != "BEAN-P1002" || called {
		t.Fatalf("outcome=%+v called=%v", outcome, called)
	}
}

func TestParsePlanesRejectsUnknownConfiguration(t *testing.T) {
	planes, err := ParsePlanes("definition, application")
	if err != nil || !planes[DefinitionPlane] || !planes[ApplicationPlane] || planes[ReleasePlane] {
		t.Fatalf("planes=%v err=%v", planes, err)
	}
	if _, err = ParsePlanes("definition,root"); err == nil {
		t.Fatal("unknown plane accepted")
	}
}

func TestAllOperationsUseSharedRuntimeBoundaries(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	database := filepath.Join(directory, "app.db")
	if err := os.WriteFile(manifest, []byte(protocolFixtureSource), 0o600); err != nil {
		t.Fatal(err)
	}
	service := New()
	member := Principal{
		Planes:  AllPlanes(),
		Request: beanctx.Request{User: &beanctx.User{ID: "11111111-1111-4111-8111-111111111111", Roles: []string{"member"}}, TenantID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestID: "test"},
	}
	calls := []struct {
		name  string
		input map[string]any
	}{
		{"bean.definition.capabilities", map[string]any{}},
		{"bean.definition.schema", map[string]any{"kind": "View"}},
		{"bean.definition.validate", map[string]any{"file": manifest}},
		{"bean.definition.inspect", map[string]any{"file": manifest, "kind": "View", "name": "items"}},
		{"bean.release.plan", map[string]any{"file": manifest}},
		{"bean.release.diff", map[string]any{"file": manifest}},
		{"bean.release.test", map[string]any{"file": manifest}},
		{"bean.release.publish", map[string]any{"file": manifest, "target": database}},
		{"bean.application.execute", map[string]any{"target": database, "action": "create_item", "input": map[string]any{"title": "First"}}},
		{"bean.application.query", map[string]any{"target": database, "view": "items", "params": map[string]any{}}},
	}
	for _, call := range calls {
		raw, _ := json.Marshal(call.input)
		outcome := service.Call(context.Background(), call.name, raw, member)
		if !outcome.OK {
			t.Fatalf("%s: diagnostics=%+v error=%+v", call.name, outcome.Diagnostics, outcome.Error)
		}
	}

	query := service.Call(context.Background(), "bean.application.query", rawInput(map[string]any{"target": database, "view": "items", "params": map[string]any{}}), member)
	encoded, _ := json.Marshal(query.Result)
	if !query.OK || !bytes.Contains(encoded, []byte(`"First"`)) {
		t.Fatalf("query=%+v result=%s", query, encoded)
	}

	otherOwner := member
	otherOwner.Request.User = &beanctx.User{ID: "22222222-2222-4222-8222-222222222222", Roles: []string{"member"}}
	other := service.Call(context.Background(), "bean.application.query", rawInput(map[string]any{"target": database, "view": "items", "params": map[string]any{}}), otherOwner)
	encoded, _ = json.Marshal(other.Result)
	if !other.OK || bytes.Contains(encoded, []byte(`"First"`)) {
		t.Fatalf("owner boundary=%+v result=%s", other, encoded)
	}

	otherTenant := member
	otherTenant.Request.TenantID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	other = service.Call(context.Background(), "bean.application.query", rawInput(map[string]any{"target": database, "view": "items", "params": map[string]any{}}), otherTenant)
	encoded, _ = json.Marshal(other.Result)
	if !other.OK || bytes.Contains(encoded, []byte(`"First"`)) {
		t.Fatalf("tenant boundary=%+v result=%s", other, encoded)
	}

	for _, boundary := range []struct {
		name  string
		input map[string]any
	}{
		{"bean.application.query", map[string]any{"target": database, "view": "item", "params": map[string]any{}}},
		{"bean.application.execute", map[string]any{"target": database, "action": "item", "input": map[string]any{"title": "Bypass"}}},
	} {
		outcome := service.Call(context.Background(), boundary.name, rawInput(boundary.input), member)
		if outcome.OK || outcome.Error == nil || outcome.Error.Code != "BEAN-P3001" {
			t.Fatalf("boundary %s accepted: %+v", boundary.name, outcome)
		}
	}
}

func TestApplicationPlanePostgresParity(t *testing.T) {
	target := os.Getenv("BEAN_TEST_AGENT_POSTGRES_URL")
	if target == "" {
		t.Skip("set BEAN_TEST_AGENT_POSTGRES_URL to run Agent Protocol PostgreSQL parity")
	}
	manifest := filepath.Join(t.TempDir(), "app.yaml")
	if err := os.WriteFile(manifest, []byte(protocolFixtureSource), 0o600); err != nil {
		t.Fatal(err)
	}
	principal := Principal{
		Planes:  AllPlanes(),
		Request: beanctx.Request{User: &beanctx.User{ID: "11111111-1111-4111-8111-111111111111", Roles: []string{"member"}}, TenantID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", RequestID: "postgres-test"},
	}
	service := New()
	for _, call := range []struct {
		name  string
		input map[string]any
	}{
		{"bean.release.publish", map[string]any{"file": manifest, "target": target}},
		{"bean.application.execute", map[string]any{"target": target, "action": "create_item", "input": map[string]any{"title": "PostgreSQL"}}},
		{"bean.application.query", map[string]any{"target": target, "view": "items", "params": map[string]any{}}},
	} {
		outcome := service.Call(context.Background(), call.name, rawInput(call.input), principal)
		if !outcome.OK {
			t.Fatalf("%s diagnostics=%+v error=%+v", call.name, outcome.Diagnostics, outcome.Error)
		}
	}
}

func TestStrictInputsRejectModelControlledAuthority(t *testing.T) {
	service := New()
	principal := Principal{Planes: AllPlanes()}
	for _, input := range []json.RawMessage{
		json.RawMessage(`{"target":"app.db","view":"items","params":{},"roles":["administrator"]}`),
		json.RawMessage(`{"target":"app.db","action":"create_item","input":{},"tenantId":"other"}`),
		json.RawMessage(`{"target":"app.db","view":"items","params":{},"sql":"select * from item"}`),
	} {
		operation := "bean.application.query"
		if bytesContain(input, []byte(`"action"`)) {
			operation = "bean.application.execute"
		}
		outcome := service.Call(context.Background(), operation, input, principal)
		if outcome.OK || outcome.Error == nil || outcome.Error.Code != "BEAN-P1003" {
			t.Fatalf("input=%s outcome=%+v", input, outcome)
		}
	}
}

func TestApplicationQueryDoesNotInitializeMissingTarget(t *testing.T) {
	target := filepath.Join(t.TempDir(), "missing.db")
	outcome := New().Call(context.Background(), "bean.application.query", rawInput(map[string]any{
		"target": target,
		"view":   "items",
		"params": map[string]any{},
	}), Principal{Planes: map[Plane]bool{ApplicationPlane: true}})
	if outcome.OK || outcome.Error == nil {
		t.Fatalf("outcome=%+v", outcome)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("query created target: %v", err)
	}
}

func TestRuntimeErrorsRedactDatabaseCredentials(t *testing.T) {
	outcome := operationError(errors.New("connect postgres://agent:super-secret@db/bean?password=also-secret"))
	if outcome.Error == nil || bytes.Contains([]byte(outcome.Error.Message), []byte("super-secret")) || bytes.Contains([]byte(outcome.Error.Message), []byte("also-secret")) {
		t.Fatalf("outcome=%+v", outcome.Error)
	}
}

func rawInput(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func bytesContain(value, part []byte) bool {
	return bytes.Contains(value, part)
}
