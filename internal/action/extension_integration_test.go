package action_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/migration"
)

func TestExtensionIntentPreservesAuthorizationTransactionAuditAndIdempotency(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "extension-action.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	app := extensionActionApp()
	fixed := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	invocations := []string{"00000000-0000-4000-8000-000000000201", "00000000-0000-4000-8000-000000000202"}
	invocationIndex := 0
	service := action.Service{DB: database, Now: func() time.Time { return fixed }, CreateInvocationID: func() string {
		id := invocations[invocationIndex]
		invocationIndex++
		return id
	}}
	denied := beanctx.Request{User: &beanctx.User{ID: "member", Roles: []string{"member"}}, RequestID: "denied"}
	if _, err = service.Execute(ctx, app, "notify", map[string]any{"message": "hello"}, denied); err == nil {
		t.Fatal("Policy-denied extension Action succeeded")
	}
	assertOutboxCount(t, ctx, database, 0)

	manager := beanctx.Request{User: &beanctx.User{ID: "manager", Roles: []string{"manager"}}, RequestID: "allowed"}
	input := map[string]any{"message": "hello", "_idempotencyKey": "notify-1"}
	for attempt := 0; attempt < 2; attempt++ {
		result, executeErr := service.Execute(ctx, app, "notify", input, manager)
		if executeErr != nil || result["status"] != "queued" {
			t.Fatalf("attempt=%d result=%v err=%v", attempt, result, executeErr)
		}
	}
	if invocationIndex != 1 {
		t.Fatalf("idempotent replay allocated %d invocation IDs", invocationIndex)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 10})
	if err != nil || len(rows) != 1 || rows[0]["id"] != invocations[0] || rows[0]["topic"] != "bean.extension/notify_provider" || rows[0]["retry_delay"] != int64(7) || rows[0]["max_attempts"] != int64(4) || rows[0]["created_at"] != fixed.Format(time.RFC3339Nano) {
		t.Fatalf("outbox=%v err=%v", rows, err)
	}
	payload := map[string]any{}
	if err = json.Unmarshal([]byte(rows[0]["payload"].(string)), &payload); err != nil || payload["invocationId"] != invocations[0] || payload["idempotencyKey"] != invocations[0] || payload["extension"] != "notify_provider" || payload["input"].(map[string]any)["message"] != "hello" {
		t.Fatalf("payload=%v err=%v", payload, err)
	}
	audits, err := database.Select(ctx, dbal.Select{Table: "bean_audit", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "action", Value: "notify"}})
	if err != nil || len(audits) != 1 || audits[0]["success"] != int64(1) {
		t.Fatalf("audits=%v err=%v", audits, err)
	}

	failed := app.Actions["notify"]
	failed.Name = "notify_then_fail"
	failed.Steps = append(failed.Steps[:1], appir.Step{Op: "assert", Condition: &expr.Expr{Op: "eq", Left: &expr.Value{Source: "literal", Literal: false}, Right: &expr.Value{Source: "literal", Literal: true}}})
	app.Actions[failed.Name] = failed
	if _, err = service.Execute(ctx, app, failed.Name, map[string]any{"message": "rollback"}, manager); err == nil {
		t.Fatal("failing Action succeeded")
	}
	assertOutboxCount(t, ctx, database, 1)
	failureAudits, err := database.Select(ctx, dbal.Select{Table: "bean_audit", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "action", Value: failed.Name}})
	if err != nil || len(failureAudits) != 1 || failureAudits[0]["success"] != int64(0) {
		t.Fatalf("failure audits=%v err=%v", failureAudits, err)
	}
}

func extensionActionApp() *appir.App {
	app := appir.Empty()
	app.Entities["order"] = appir.Entity{Name: "order"}
	app.Policies["managers"] = appir.Policy{Name: "managers", WriteRoles: []string{"manager"}}
	app.Extensions["notify_provider"] = appir.Extension{
		Name: "notify_provider", Transport: "http", Endpoint: "https://provider.example/notify",
		Input:       map[string]appir.Field{"message": {Name: "message", Type: "string", Required: true}},
		Output:      map[string]appir.Field{"accepted": {Name: "accepted", Type: "boolean", Required: true}},
		Permissions: []string{"network"}, SideEffects: []string{"external_write"}, Authentication: "none", TimeoutSeconds: 5,
		Retry: appir.ExtensionRetry{MaxAttempts: 4, DelaySeconds: 7}, Idempotency: "required", Transaction: "after_commit", Failure: "retry_then_fail",
	}
	app.Actions["notify"] = appir.Action{
		Name: "notify", Entity: "order", Operation: "transaction", Policy: "managers",
		Input:  map[string]appir.Field{"message": {Name: "message", Type: "string", Required: true}},
		Output: map[string]appir.Field{"status": {Name: "status", Type: "string", Required: true}},
		Steps: []appir.Step{
			{Op: "extension", Extension: "notify_provider", Values: []appir.Assignment{{Field: "message", Value: appir.ValueBinding{Source: "input", Path: "message"}}}},
			{Op: "return", Values: []appir.Assignment{{Field: "status", Value: appir.ValueBinding{Source: "literal", Literal: json.RawMessage(`"queued"`)}}}},
		},
	}
	return app
}

func assertOutboxCount(t *testing.T, ctx context.Context, database dbal.Database, expected int) {
	t.Helper()
	rows, err := database.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 10})
	if err != nil || len(rows) != expected {
		t.Fatalf("outbox=%v err=%v", rows, err)
	}
}
