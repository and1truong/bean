package postgres_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
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

func TestCompilerCastsTextTimestampsBeforeDateBucketing(t *testing.T) {
	datetimeStatement, _, err := (postgres.Compiler{}).CompileSelect(dbal.Select{
		Table:   "event",
		GroupBy: []dbal.Group{{Column: "occurred_at", Alias: "month", Bucket: "month", Type: "datetime"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(datetimeStatement, `CAST(CAST(date_trunc('month', CAST("occurred_at" AS timestamptz) AT TIME ZONE 'UTC') AS date) AS text)`) {
		t.Fatalf("datetime statement=%q", datetimeStatement)
	}
	dateStatement, _, err := (postgres.Compiler{}).CompileSelect(dbal.Select{
		Table:   "event",
		GroupBy: []dbal.Group{{Column: "occurred_on", Alias: "month", Bucket: "month", Type: "date"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dateStatement, `CAST(CAST(date_trunc('month', CAST("occurred_on" AS date)) AS date) AS text)`) || strings.Contains(dateStatement, "timestamptz") {
		t.Fatalf("date statement=%q", dateStatement)
	}
}

func TestCompilerDoesNotNumberDecimalGrammarQuantifiers(t *testing.T) {
	statement, arguments, err := (postgres.Compiler{}).CompileSelect(dbal.Select{
		Table:      "item",
		Aggregates: []dbal.Aggregate{{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(arguments) != 0 || strings.Contains(statement, "$1") {
		t.Fatalf("statement=%q arguments=%v", statement, arguments)
	}
}

func TestDecimalAggregatesUseNumericStorageSemantics(t *testing.T) {
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
	if err = database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS decimal_item`, `CREATE TABLE decimal_item (id TEXT PRIMARY KEY, amount TEXT)`}); err != nil {
		t.Fatal(err)
	}
	defer database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS decimal_item`})
	for id, amount := range map[string]string{"one": "2", "two": "10"} {
		if _, err = database.Insert(ctx, dbal.Insert{Table: "decimal_item", Values: map[string]dbal.Value{"id": id, "amount": amount}}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "decimal_item", Aggregates: []dbal.Aggregate{
		{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"},
		{Function: "avg", Column: "amount", Alias: "average", Type: "decimal"},
		{Function: "min", Column: "amount", Alias: "minimum", Type: "decimal"},
		{Function: "max", Column: "amount", Alias: "maximum", Type: "decimal"},
	}})
	if err != nil || len(rows) != 1 || fmt.Sprint(rows[0]["total"]) != "12" || fmt.Sprint(rows[0]["average"]) != "6.0000000000000000" || fmt.Sprint(rows[0]["minimum"]) != "2" || fmt.Sprint(rows[0]["maximum"]) != "10" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if err = database.ExecuteMigration(ctx, []string{`DELETE FROM decimal_item`}); err != nil {
		t.Fatal(err)
	}
	for id, amount := range map[string]string{"one": "1", "two": "0", "three": "0"} {
		if _, err = database.Insert(ctx, dbal.Insert{Table: "decimal_item", Values: map[string]dbal.Value{"id": id, "amount": amount}}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err = database.Select(ctx, dbal.Select{Table: "decimal_item", Aggregates: []dbal.Aggregate{{Function: "avg", Column: "amount", Alias: "average", Type: "decimal"}}})
	if err != nil || len(rows) != 1 || fmt.Sprint(rows[0]["average"]) != "0.3333333333333333" {
		t.Fatalf("repeating average rows=%v err=%v", rows, err)
	}
	for _, invalid := range []string{"NaN", " 1.5 ", "10e4096", "0.1e4097"} {
		if err = database.ExecuteMigration(ctx, []string{`DELETE FROM decimal_item`}); err != nil {
			t.Fatal(err)
		}
		if _, err = database.Insert(ctx, dbal.Insert{Table: "decimal_item", Values: map[string]dbal.Value{"id": "legacy", "amount": invalid}}); err != nil {
			t.Fatal(err)
		}
		if _, err = database.Select(ctx, dbal.Select{Table: "decimal_item", Aggregates: []dbal.Aggregate{{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"}}}); err == nil {
			t.Fatalf("legacy decimal %q was silently aggregated", invalid)
		}
	}
}

func TestDateBucketsAreTypeAwareAcrossSessionTimezones(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL contracts")
	}
	connection, err := url.Parse(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	query := connection.Query()
	query.Set("timezone", "Australia/Brisbane")
	connection.RawQuery = query.Encode()
	database, err := postgres.Open(connection.String())
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	ctx := context.Background()
	if err = database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS bucket_event`, `CREATE TABLE bucket_event (id TEXT PRIMARY KEY, occurred_on TEXT NOT NULL, occurred_at TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	defer database.ExecuteMigration(ctx, []string{`DROP TABLE IF EXISTS bucket_event`})
	if _, err = database.Insert(ctx, dbal.Insert{Table: "bucket_event", Values: map[string]dbal.Value{"id": "1", "occurred_on": "2026-09-01", "occurred_at": "2026-08-31T16:30:00Z"}}); err != nil {
		t.Fatal(err)
	}
	for name, group := range map[string]dbal.Group{
		"date":     {Column: "occurred_on", Alias: "month", Bucket: "month", Type: "date"},
		"datetime": {Column: "occurred_at", Alias: "month", Bucket: "month", Type: "datetime"},
	} {
		rows, selectErr := database.Select(ctx, dbal.Select{Table: "bucket_event", GroupBy: []dbal.Group{group}, Aggregates: []dbal.Aggregate{{Function: "count", Column: "id", Alias: "event_count"}}, Limit: 2})
		if selectErr != nil || len(rows) != 1 {
			t.Fatalf("%s rows=%v err=%v", name, rows, selectErr)
		}
		want := "2026-09-01"
		if name == "datetime" {
			want = "2026-08-01"
		}
		if rows[0]["month"] != want {
			t.Fatalf("%s month=%v want=%s", name, rows[0]["month"], want)
		}
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
