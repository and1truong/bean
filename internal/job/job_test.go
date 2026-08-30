package job_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/migration"
)

func TestRunnerRetriesAndRecoversStaleClaim(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "jobs.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 30, 10, 0, 0, 0, time.UTC)
	if err = db.Transaction(ctx, func(tx dbal.Transaction) error {
		return job.Schedule(ctx, tx, "send", now, map[string]any{"id": "1"})
	}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	runner := job.Runner{DB: db, Now: func() time.Time { return now }, Lease: time.Minute, Handle: func(context.Context, string, map[string]any) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		return nil
	}}
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	row := oneJob(t, db)
	if row["status"] != "pending" || row["attempts"] != int64(1) {
		t.Fatalf("after failure: %v", row)
	}
	now = now.Add(61 * time.Second)
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	row = oneJob(t, db)
	if row["status"] != "complete" || row["attempts"] != int64(2) || calls != 2 {
		t.Fatalf("after retry: row=%v calls=%d", row, calls)
	}
	if _, err = db.Update(ctx, dbal.Update{Table: "bean_job", Values: map[string]dbal.Value{"status": "running", "claim_token": "dead", "claimed_at": now.Add(-2 * time.Minute).Format(time.RFC3339Nano), "completed_at": nil}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: row["id"]}, ExpectedRows: 1}); err != nil {
		t.Fatal(err)
	}
	if err = runner.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if row = oneJob(t, db); row["status"] != "complete" || calls != 3 {
		t.Fatalf("stale claim was not recovered: row=%v calls=%d", row, calls)
	}
}

func oneJob(t *testing.T, db *sqlite.DB) dbal.Row {
	t.Helper()
	rows, err := db.Select(context.Background(), dbal.Select{Table: "bean_job", Limit: 2})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	return rows[0]
}
