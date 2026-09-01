package postgres_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal/dbaltest"
	"github.com/beanruntime/bean/internal/dbal/postgres"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
)

func TestContract(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL contracts")
	}
	database, err := postgres.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	dbaltest.Contract(t, database)
}

func TestLegacyIntegerBooleanFailsStorageValidation(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL contracts")
	}
	database, err := postgres.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err = database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS legacy_boolean`, `CREATE TABLE legacy_boolean (id TEXT PRIMARY KEY,enabled INTEGER,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	defer database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS legacy_boolean`})
	app := appir.Empty()
	app.ReleaseID = "legacy-release"
	app.Entities["legacy_boolean"] = appir.Entity{Name: "legacy_boolean", Fields: []appir.Field{{Name: "enabled", Type: "boolean"}}}
	err = (&release.Store{Inspector: database}).ValidateStorage(ctx, app)
	if err == nil || !strings.Contains(err.Error(), "explicit data migration is required") {
		t.Fatalf("validation error=%v", err)
	}
}

func TestCompilerUsesNumberedParameters(t *testing.T) {
	if got := (postgres.Compiler{}).Placeholder(3); got != "$3" {
		t.Fatalf("placeholder=%q", got)
	}
}

func TestUngroupedAggregateViewDoesNotOrderByRecordID(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL contracts")
	}
	database, err := postgres.Open(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err = database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS metric_item`, `CREATE TABLE metric_item (id TEXT PRIMARY KEY)`}); err != nil {
		t.Fatal(err)
	}
	defer database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS metric_item`})
	app := appir.Empty()
	app.Entities["metric_item"] = appir.Entity{Name: "metric_item"}
	app.Views["metric_total"] = appir.View{Name: "metric_total", Entity: "metric_item", Aggregates: []appir.Aggregate{{Function: "count", Field: "id", Alias: "item_count"}}, DefaultLimit: 10, MaxLimit: 10}
	rows, err := (view.Service{DB: database}).Run(ctx, app, "metric_total", view.Params{}, beanctx.Request{})
	if err != nil || len(rows) != 1 || rows[0]["item_count"] != int64(0) {
		t.Fatalf("metric rows=%v err=%v", rows, err)
	}
}
