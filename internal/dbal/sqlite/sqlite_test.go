package sqlite_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
)

func TestContract(t *testing.T) {
	ctx := context.Background()
	d, e := sqlite.Open(filepath.Join(t.TempDir(), "db.sqlite"))
	if e != nil {
		t.Fatal(e)
	}
	defer d.Close()
	if e = d.ExecuteMigration(ctx, []string{`CREATE TABLE item (id TEXT PRIMARY KEY, name TEXT NOT NULL UNIQUE, amount INTEGER NOT NULL)`}); e != nil {
		t.Fatal(e)
	}
	if _, e = d.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": "1", "name": "alpha", "amount": 2}}); e != nil {
		t.Fatal(e)
	}
	rows, e := d.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id", "name"}, Where: &dbal.Predicate{Op: dbal.OpStartsWith, Column: "name", Value: "a"}, Limit: 50})
	if e != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, e)
	}
	if _, e = d.Update(ctx, dbal.Update{Table: "item", Values: map[string]dbal.Value{"amount": 3}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: "1"}, ExpectedRows: 1}); e != nil {
		t.Fatal(e)
	}
	e = d.Transaction(ctx, func(tx dbal.Transaction) error {
		_, e := tx.Insert(ctx, dbal.Insert{Table: "item", Values: map[string]dbal.Value{"id": "2", "name": "beta", "amount": 4}})
		return e
	})
	if e != nil {
		t.Fatal(e)
	}
	if _, e = d.Delete(ctx, dbal.Delete{Table: "item", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: "missing"}, ExpectedRows: 1}); !dbal.IsCode(e, dbal.Conflict) {
		t.Fatalf("want conflict, got %v", e)
	}
	cols, e := d.Columns(ctx, "item")
	if e != nil || len(cols) != 3 {
		t.Fatalf("columns=%v err=%v", cols, e)
	}
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
