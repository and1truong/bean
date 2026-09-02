package dbaltest

import (
	"context"
	"testing"

	"github.com/beanruntime/bean/internal/dbal"
)

type Backend interface {
	dbal.Database
	dbal.SchemaInspector
	ExecuteMigration(context.Context, []string) error
}

func Contract(t *testing.T, database Backend) {
	t.Helper()
	ctx := context.Background()
	if err := database.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, amount INTEGER NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": "1", "name": "alpha", "amount": 2}}); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id", "name"}, Where: &dbal.Predicate{Op: dbal.OpStartsWith, Column: "name", Value: "a"}, Limit: 50})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if _, err = database.Update(ctx, dbal.Update{Table: "item", Values: map[string]dbal.Value{"amount": 3}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: "1"}, ExpectedRows: 1}); err != nil {
		t.Fatal(err)
	}
	err = database.Transaction(ctx, func(tx dbal.Transaction) error {
		_, transactionErr := tx.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": "2", "name": "beta", "amount": 4}})
		return transactionErr
	})
	if err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		predicate dbal.Predicate
		rows      int
	}{
		"contains": {dbal.Predicate{Op: dbal.OpContains, Column: "name", Value: "ph"}, 1},
		"gte":      {dbal.Predicate{Op: dbal.OpGTE, Column: "amount", Value: 3}, 2},
		"lte":      {dbal.Predicate{Op: dbal.OpLTE, Column: "amount", Value: 3}, 1},
	} {
		rows, selectErr := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id"}, Where: &test.predicate, Limit: 50})
		if selectErr != nil || len(rows) != test.rows {
			t.Fatalf("%s rows=%v err=%v", name, rows, selectErr)
		}
	}
	if _, err = database.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": "3", "name": "alpha", "amount": 1}}); !dbal.IsCode(err, dbal.UniqueViolation) {
		t.Fatalf("want unique violation, got %v", err)
	}
	if _, err = database.Delete(ctx, dbal.Delete{Table: "item", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: "missing"}, ExpectedRows: 1}); !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("want conflict, got %v", err)
	}
	columns, err := database.Columns(ctx, "item")
	if err != nil || len(columns) != 3 {
		t.Fatalf("columns=%v err=%v", columns, err)
	}
}
