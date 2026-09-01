package action_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/rule"
)

func TestRulesDeriveSimultaneouslyUseInjectedNowAndRejectClientOverride(t *testing.T) {
	db, app := publishRuleApp(t, invoiceRuleDefinitions())
	defer db.Close()
	now := time.Date(2026, 9, 1, 2, 3, 4, 0, time.UTC)
	currentTime := now
	engine := action.Service{DB: db, Now: func() time.Time { return currentTime }}
	input := map[string]any{"quantity": 3, "unit_price": 125, "_idempotencyKey": "invoice-1"}
	created, err := engine.Execute(context.Background(), app, "invoice_create", input, admin())
	if err != nil || created["total"] != int64(375) && created["total"] != 375 || created["stamped_at"] != now.Format(time.RFC3339Nano) {
		t.Fatalf("created=%v err=%v", created, err)
	}
	currentTime = now.Add(time.Hour)
	replayed, err := engine.Execute(context.Background(), app, "invoice_create", input, admin())
	if err != nil || replayed["id"] != created["id"] || replayed["stamped_at"] != created["stamped_at"] {
		t.Fatalf("replayed=%v err=%v", replayed, err)
	}
	if _, err = engine.Execute(context.Background(), app, "invoice_create", map[string]any{"quantity": 1, "unit_price": 1, "total": 1}, admin()); !dbal.IsCode(err, dbal.InvalidQuery) {
		t.Fatalf("client override error=%v", err)
	}
}

func TestRuleGuardRunsAfterRecordPolicyAndBeforeMutation(t *testing.T) {
	db, app := publishRuleApp(t, guardedDocumentDefinitions())
	defer db.Close()
	engine := action.Service{DB: db}
	owner := beanctx.Request{User: &beanctx.User{ID: "11111111-1111-4111-8111-111111111111", Roles: []string{"member"}}}
	other := beanctx.Request{User: &beanctx.User{ID: "22222222-2222-4222-8222-222222222222", Roles: []string{"member"}}}
	document, err := engine.Execute(context.Background(), app, "document_create", map[string]any{"title": "before", "locked": true}, owner)
	if err != nil {
		t.Fatal(err)
	}
	_, err = engine.Execute(context.Background(), app, "explosive_document_update", map[string]any{"id": document["id"], "title": "other"}, other)
	var ruleError *rule.Error
	if !dbal.IsCode(err, dbal.NotFound) || errors.As(err, &ruleError) {
		t.Fatalf("record Policy did not fail before guard: %v", err)
	}
	_, err = engine.Execute(context.Background(), app, "explosive_document_update", map[string]any{"id": document["id"], "title": "after"}, owner)
	if !dbal.IsCode(err, dbal.InvalidQuery) || !errors.As(err, &ruleError) || ruleError.Code != rule.CodeDivideByZero {
		t.Fatalf("authorized guard failure=%v cause=%+v", err, ruleError)
	}
	_, err = engine.Execute(context.Background(), app, "document_update", map[string]any{"id": document["id"], "title": "after"}, owner)
	if !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("guard denial error=%v", err)
	}
	rows, err := db.Select(context.Background(), dbal.Select{Table: "document", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: document["id"]}, Limit: 1})
	if err != nil || len(rows) != 1 || rows[0]["title"] != "before" {
		t.Fatalf("guarded mutation leaked: rows=%v err=%v", rows, err)
	}
}

func TestEntityRuleInvariantRollsBackCRUDAndTransactionSteps(t *testing.T) {
	db, app := publishRuleApp(t, invariantDefinitions())
	defer db.Close()
	engine := action.Service{DB: db}
	valid, err := engine.Execute(context.Background(), app, "balance_create", map[string]any{"amount": 10}, admin())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(context.Background(), app, "balance_update", map[string]any{"id": valid["id"], "amount": -1}, admin()); !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("update invariant error=%v", err)
	}
	if _, err = engine.Execute(context.Background(), app, "balance_batch", map[string]any{"first": 5, "second": -2}, admin()); !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("transaction invariant error=%v", err)
	}
	rows, err := db.Select(context.Background(), dbal.Select{Table: "balance", OrderBy: []dbal.Order{{Column: "amount"}}})
	if err != nil || len(rows) != 1 || rows[0]["amount"] != int64(10) {
		t.Fatalf("invariant rollback failed: rows=%v err=%v", rows, err)
	}
}

func TestCreateRulesTreatAbsentOptionalAndSystemFieldsAsNull(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{
			"softDelete":  true,
			"fields":      []any{map[string]any{"name": "note", "type": "string"}},
			"validations": map[string]any{"not_deleted": "not_deleted"},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "optional_empty"}, Spec: map[string]any{
			"entity": "document", "result": "boolean",
			"input": map[string]any{"note": map[string]any{"type": "string"}},
			"expression": map[string]any{"op": "and", "args": []any{
				map[string]any{"op": "is_null", "args": []any{map[string]any{"source": "this", "path": "note"}}},
				map[string]any{"op": "is_null", "args": []any{map[string]any{"source": "input", "path": "note"}}},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "not_deleted"}, Spec: map[string]any{
			"entity": "document", "result": "boolean",
			"expression": map[string]any{"op": "is_null", "args": []any{map[string]any{"source": "this", "path": "deleted_at"}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "document_create"}, Spec: map[string]any{
			"entity": "document", "operation": "create", "when": "optional_empty",
		}},
	}
	db, app := publishRuleApp(t, definitions)
	defer db.Close()
	if _, err := (action.Service{DB: db}).Execute(context.Background(), app, "document_create", map[string]any{}, admin()); err != nil {
		t.Fatalf("create with absent nullable fields failed: %v", err)
	}
}

func TestRuleFailsClosedWhenRequestContextIsUnavailable(t *testing.T) {
	definitions := invoiceRuleDefinitions()
	definitions = append(definitions,
		definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "request_stamp"}, Spec: map[string]any{"result": "string", "expression": map[string]any{"source": "context", "path": "request_id"}}},
		definition.Definition{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "request_invoice_create"}, Spec: map[string]any{"entity": "invoice", "operation": "create", "derive": map[string]any{"reference": "request_stamp", "total": "subtotal", "stamped_at": "now"}}},
	)
	db, app := publishRuleApp(t, definitions)
	defer db.Close()
	request := admin()
	request.RequestID = ""
	_, err := (action.Service{DB: db}).Execute(context.Background(), app, "request_invoice_create", map[string]any{"quantity": 1, "unit_price": 1}, request)
	var ruleError *rule.Error
	if !dbal.IsCode(err, dbal.InvalidQuery) || !errors.As(err, &ruleError) || ruleError.Code != rule.CodeMissingValue {
		t.Fatalf("missing context error=%v cause=%+v", err, ruleError)
	}
}

func publishRuleApp(t *testing.T, definitions []definition.Definition) (*sqlite.DB, *appir.App) {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "rules.db"))
	if err != nil {
		t.Fatal(err)
	}
	store := &release.Store{DB: db, Migrations: db, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.SaveBundle(ctx, "default", definition.Bundle{Name: "rules", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := store.Kernel.Active()
	return db, app
}

func invoiceRuleDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "invoice"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "quantity", "type": "integer", "required": true}, map[string]any{"name": "unit_price", "type": "money", "required": true},
			map[string]any{"name": "total", "type": "money", "required": true}, map[string]any{"name": "stamped_at", "type": "datetime", "required": true}, map[string]any{"name": "reference", "type": "string"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "subtotal"}, Spec: map[string]any{"result": "number", "input": map[string]any{"quantity": map[string]any{"type": "integer"}, "unit_price": map[string]any{"type": "money"}}, "expression": map[string]any{"op": "multiply", "args": []any{map[string]any{"source": "input", "path": "quantity"}, map[string]any{"source": "input", "path": "unit_price"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "now"}, Spec: map[string]any{"result": "datetime", "expression": map[string]any{"source": "context", "path": "now"}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "invoice_create"}, Spec: map[string]any{"entity": "invoice", "operation": "create", "derive": map[string]any{"total": "subtotal", "stamped_at": "now"}}},
	}
}

func guardedDocumentDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "owned"}, Spec: map[string]any{"writeRoles": []any{"member"}, "owner": true}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{"policy": "owned", "owner": true, "fields": []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "locked", "type": "boolean", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "editable"}, Spec: map[string]any{"entity": "document", "result": "boolean", "expression": map[string]any{"op": "not", "args": []any{map[string]any{"source": "this", "path": "locked"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "explosive"}, Spec: map[string]any{"entity": "document", "result": "boolean", "expression": map[string]any{"op": "eq", "args": []any{map[string]any{"op": "divide", "args": []any{map[string]any{"source": "literal", "literal": 1}, map[string]any{"source": "literal", "literal": 0}}}, map[string]any{"source": "literal", "literal": 1}}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "document_update"}, Spec: map[string]any{"entity": "document", "operation": "update", "policy": "owned", "when": "editable"}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "explosive_document_update"}, Spec: map[string]any{"entity": "document", "operation": "update", "policy": "owned", "when": "explosive"}},
	}
}

func invariantDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "balance"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "amount", "type": "money", "required": true}}, "validations": map[string]any{"non_negative": "non_negative"}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "non_negative"}, Spec: map[string]any{"entity": "balance", "result": "boolean", "expression": map[string]any{"op": "gte", "args": []any{map[string]any{"source": "this", "path": "amount"}, map[string]any{"source": "literal", "literal": 0}}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "balance_batch"}, Spec: map[string]any{"entity": "balance", "operation": "transaction", "input": map[string]any{"first": map[string]any{"type": "money", "required": true}, "second": map[string]any{"type": "money", "required": true}}, "steps": []any{
			map[string]any{"op": "create", "entity": "balance", "values": map[string]any{"amount": "$input.first"}},
			map[string]any{"op": "create", "entity": "balance", "values": map[string]any{"amount": "$input.second"}},
		}}},
	}
}
