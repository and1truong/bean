package bootstrap_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
)

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
