package bootstrap_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/job"
)

func TestInspectionOpenDoesNotInitializeDatabase(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "empty.db")
	database, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	database.Close()

	if runtime, openErr := bootstrap.OpenInspection(ctx, path); openErr == nil {
		runtime.DB.Close()
		t.Fatal("inspection unexpectedly initialized an empty database")
	}
	database, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tables, err := database.Tables(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 0 {
		t.Fatalf("inspection created tables: %v", tables)
	}
}

func TestRuntimeOutboxRunnerDeliversCommittedEvents(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "outbox.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		return event.Emit(ctx, tx, "test.created", map[string]any{"id": "1"})
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Outbox.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Columns: []string{"status"}, Limit: 1})
	if err != nil || len(rows) != 1 || rows[0]["status"] != "delivered" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestJobRunnerRestoresTenantAndRejectsMissingDefinition(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "jobs.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.DB.ExecuteMigration(ctx, []string{`CREATE TABLE task (id TEXT PRIMARY KEY,status TEXT NOT NULL,tenant_id TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	tenantID := "00000000-0000-4000-8000-00000000000a"
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if _, err = runtime.DB.Insert(ctx, dbal.Insert{Table: "task", Values: map[string]dbal.Value{"id": "00000000-0000-4000-8000-000000000001", "status": "pending", "tenant_id": tenantID, "created_at": now, "updated_at": now, "version": 1}}); err != nil {
		t.Fatal(err)
	}
	app := appir.Empty()
	app.ReleaseID = "test-release"
	app.Entities["task"] = appir.Entity{Name: "task", Policy: "tenant", Tenant: true, Fields: []appir.Field{{Name: "status", Type: "enum", Required: true, Options: []string{"pending", "done"}}}}
	app.Policies["tenant"] = appir.Policy{Name: "tenant", Tenant: true}
	app.Actions["finish_task"] = appir.Action{
		Name: "finish_task", Entity: "task", Operation: "transition", Policy: "tenant", StateField: "status",
		Input:       map[string]appir.Field{"id": {Name: "id", Type: "uuid", Required: true}, "status": {Name: "status", Type: "enum", Required: true, Options: []string{"pending", "done"}}},
		Output:      map[string]appir.Field{"id": {Name: "id", Type: "uuid"}, "status": {Name: "status", Type: "enum", Options: []string{"pending", "done"}}, "tenant_id": {Name: "tenant_id", Type: "uuid"}, "created_at": {Name: "created_at", Type: "datetime"}, "updated_at": {Name: "updated_at", Type: "datetime"}, "version": {Name: "version", Type: "integer"}},
		Transitions: map[string][]string{"pending": {"done"}},
	}
	app.Jobs["finish"] = appir.Job{Name: "finish", Action: "finish_task"}
	if err = runtime.Kernel.Activate(app); err != nil {
		t.Fatal(err)
	}
	if err = runtime.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		return job.Schedule(ctx, tx, "finish", time.Now().UTC(), map[string]any{"id": "00000000-0000-4000-8000-000000000001", "status": "done", job.TenantIDPayloadKey: tenantID})
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Jobs.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "task", Columns: []string{"status"}, Limit: 1})
	if err != nil || len(rows) != 1 || rows[0]["status"] != "done" {
		t.Fatalf("tenant job rows=%v err=%v", rows, err)
	}

	if err = runtime.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		return job.Schedule(ctx, tx, "removed", time.Now().UTC(), map[string]any{})
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Jobs.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	missing := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "name", Value: "removed"}, dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "pending"})
	rows, err = runtime.DB.Select(ctx, dbal.Select{Table: "bean_job", Columns: []string{"last_error"}, Where: &missing, Limit: 1})
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0]["last_error"].(string), "not defined") {
		t.Fatalf("missing job rows=%v err=%v", rows, err)
	}
}
