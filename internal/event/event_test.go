package event_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/migration"
)

func TestOutboxDeliveryRetriesAndBecomesTerminal(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "outbox.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	if err = db.Transaction(ctx, func(tx dbal.Transaction) error {
		return event.Emit(ctx, tx, "order.created", map[string]any{"id": "1"})
	}); err != nil {
		t.Fatal(err)
	}
	rows, _ := db.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 1})
	if _, err = db.Update(ctx, dbal.Update{Table: "bean_outbox", Values: map[string]dbal.Value{"max_attempts": 2, "next_attempt_at": now.Format(time.RFC3339Nano)}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["id"]}, ExpectedRows: 1}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := event.Runner{DB: db, Now: func() time.Time { return now }, Deliver: func(context.Context, string, map[string]any) error {
		calls++
		return errors.New("offline")
	}}
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	now = now.Add(61 * time.Second)
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err = db.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 1})
	if err != nil || rows[0]["status"] != "failed" || rows[0]["attempts"] != int64(2) || calls != 2 {
		t.Fatalf("rows=%v calls=%d err=%v", rows, calls, err)
	}
}

type permanentDeliveryError struct{}

func (permanentDeliveryError) Error() string   { return "permanent" }
func (permanentDeliveryError) Retryable() bool { return false }

func TestOutboxHonorsEnqueuePolicyAndPermanentFailure(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "outbox-policy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC)
	if err = db.Transaction(ctx, func(tx dbal.Transaction) error {
		id, enqueueErr := event.Enqueue(ctx, tx, "extension.notify", map[string]any{"message": "hello"}, event.Options{ID: "invocation-1", RetryDelay: 7 * time.Second, MaxAttempts: 4, CreatedAt: now})
		if id != "invocation-1" {
			t.Fatalf("id=%s", id)
		}
		return enqueueErr
	}); err != nil {
		t.Fatal(err)
	}
	runner := event.Runner{DB: db, Now: func() time.Time { return now }, Deliver: func(context.Context, string, map[string]any) error { return permanentDeliveryError{} }}
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 1})
	if err != nil || len(rows) != 1 || rows[0]["id"] != "invocation-1" || rows[0]["retry_delay"] != int64(7) || rows[0]["max_attempts"] != int64(4) || rows[0]["attempts"] != int64(1) || rows[0]["status"] != "failed" || rows[0]["last_error"] != "permanent" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}
