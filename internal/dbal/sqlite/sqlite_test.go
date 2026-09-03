package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/dbaltest"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
)

func TestContract(t *testing.T) {
	d, e := sqlite.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	dbaltest.Contract(t, d)
}

func TestCompilerRejectsIdentifiersAndParameterizes(t *testing.T) {
	c := sqlite.Compiler{}
	if _, _, e := c.CompileSelect(dbal.Select{Table: "item; drop table item"}); e == nil {
		t.Fatal("unsafe identifier accepted")
	}
	s, args, e := c.CompileSelect(dbal.Select{Table: "item", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "name", Value: "' OR 1=1"}})
	if e != nil {
		t.Fatal(e)
	}
	if len(args) != 1 || s == "" {
		t.Fatalf("sql=%q args=%v", s, args)
	}
}

func TestCompilerUsesExactDecimalAggregates(t *testing.T) {
	statement, _, err := (sqlite.Compiler{}).CompileSelect(dbal.Select{Table: "sale", Aggregates: []dbal.Aggregate{{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"}}})
	if err != nil {
		t.Fatal(err)
	}
	if statement != `SELECT BEAN_DECIMAL_SUM("amount") AS "total" FROM "sale"` {
		t.Fatalf("statement=%q", statement)
	}
}

func TestDecimalAggregatesPreserveExactValues(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "decimal.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, amount TEXT)`}); err != nil {
		t.Fatal(err)
	}
	for id, amount := range map[string]string{"one": "0.1", "two": "0.2", "large": "9007199254740993.1"} {
		if _, err = database.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": id, "amount": amount}}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Aggregates: []dbal.Aggregate{
		{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"},
		{Function: "avg", Column: "amount", Alias: "average", Type: "decimal"},
		{Function: "min", Column: "amount", Alias: "minimum", Type: "decimal"},
		{Function: "max", Column: "amount", Alias: "maximum", Type: "decimal"},
	}})
	if err != nil || len(rows) != 1 || rows[0]["total"] != "9007199254740993.4" || rows[0]["average"] != "3002399751580331.1333333333333333" || rows[0]["minimum"] != "0.1" || rows[0]["maximum"] != "9007199254740993.1" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestRepeatingDecimalAverageUsesPortableScale(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "decimal-average.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, amount TEXT)`}); err != nil {
		t.Fatal(err)
	}
	for id, amount := range map[string]string{"one": "1", "two": "0", "three": "0"} {
		if _, err = database.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": id, "amount": amount}}); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Aggregates: []dbal.Aggregate{{Function: "avg", Column: "amount", Alias: "average", Type: "decimal"}}})
	if err != nil || len(rows) != 1 || rows[0]["average"] != "0.3333333333333333" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestDecimalAggregatesRejectLegacyRationalText(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "invalid-decimal.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, amount TEXT)`, `INSERT INTO item (id, amount) VALUES ('legacy', '1/2')`}); err != nil {
		t.Fatal(err)
	}
	if _, err = database.Select(ctx, dbal.Select{Table: "item", Aggregates: []dbal.Aggregate{{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"}}}); err == nil {
		t.Fatal("legacy rational decimal was silently aggregated")
	}
}

func TestDecimalAggregateOrderingIsNumeric(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "decimal-order.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, category TEXT, amount TEXT)`}); err != nil {
		t.Fatal(err)
	}
	for _, query := range []dbal.Insert{
		{Table: "item", Values: map[string]dbal.Value{"id": "one", "category": "small", "amount": "2"}},
		{Table: "item", Values: map[string]dbal.Value{"id": "two", "category": "large", "amount": "10"}},
	} {
		if _, err = database.Insert(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", GroupBy: []dbal.Group{{Column: "category", Alias: "category"}}, Aggregates: []dbal.Aggregate{{Function: "sum", Column: "amount", Alias: "total", Type: "decimal"}}, OrderBy: []dbal.Order{{Column: "total"}}})
	if err != nil || len(rows) != 2 || rows[0]["total"] != "2" || rows[1]["total"] != "10" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestQueryPlanJoinsGroupsAndAggregates(t *testing.T) {
	ctx := context.Background()
	d, e := sqlite.Open(filepath.Join(t.TempDir(), "plan.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	if e = d.ExecuteMigration(ctx, []string{`CREATE TABLE customer (id TEXT PRIMARY KEY, name TEXT NOT NULL)`, `CREATE TABLE sale (id TEXT PRIMARY KEY, customer_id TEXT NOT NULL, amount INTEGER NOT NULL)`}); e != nil {
		t.Fatal(e)
	}
	for _, q := range []dbal.Insert{
		{Table: "customer", Values: map[string]dbal.Value{"id": "c1", "name": "Ada"}},
		{Table: "sale", Values: map[string]dbal.Value{"id": "s1", "customer_id": "c1", "amount": 2}},
		{Table: "sale", Values: map[string]dbal.Value{"id": "s2", "customer_id": "c1", "amount": 3}},
	} {
		if _, e = d.Insert(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	rows, e := d.Select(ctx, dbal.Select{Table: "sale", Joins: []dbal.Join{{Table: "customer", Alias: "customer", Type: "inner", Left: "sale.customer_id", Right: "customer.id"}}, GroupBy: []dbal.Group{{Column: "customer.name", Alias: "customer_name"}}, Aggregates: []dbal.Aggregate{{Function: "count", Column: "sale.id", Alias: "sales"}, {Function: "sum", Column: "sale.amount", Alias: "total"}}, OrderBy: []dbal.Order{{Column: "customer_name"}}, Limit: 10})
	if e != nil || len(rows) != 1 || rows[0]["sales"] != int64(2) || rows[0]["total"] != int64(5) {
		t.Fatalf("rows=%v err=%v", rows, e)
	}
}

func TestForeignKeysApplyToEveryPooledConnection(t *testing.T) {
	ctx := context.Background()
	d, err := sqlite.Open(filepath.Join(t.TempDir(), "foreign-keys.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if err = d.ExecuteMigration(ctx, []string{
		`CREATE TABLE parent (id TEXT PRIMARY KEY)`,
		`CREATE TABLE child (id TEXT PRIMARY KEY, parent_id TEXT REFERENCES parent(id))`,
	}); err != nil {
		t.Fatal(err)
	}

	held := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- d.Transaction(ctx, func(dbal.Transaction) error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	_, insertErr := d.Insert(ctx, dbal.Insert{Table: "child", Values: map[string]dbal.Value{"id": "child-1", "parent_id": "missing"}})
	close(release)
	if transactionErr := <-done; transactionErr != nil {
		t.Fatal(transactionErr)
	}
	if !dbal.IsCode(insertErr, dbal.ForeignKeyViolation) {
		t.Fatalf("want foreign-key violation on second pooled connection, got %v", insertErr)
	}
}
