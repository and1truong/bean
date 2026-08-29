package migration_test

import (
	"github.com/beanruntime/bean/internal/migration"
	"strings"
	"testing"
)

func TestSafeAdditivePlan(t *testing.T) {
	old := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "title", Type: "string"}}}}}
	next := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "title", Type: "string"}, {Name: "isbn", Type: "string"}}}}}
	p, e := migration.Build(old, next)
	if e != nil || len(p.Statements) != 1 || !strings.Contains(p.Statements[0], "ADD COLUMN") {
		t.Fatalf("plan=%v err=%v", p, e)
	}
}
func TestDestructiveChangesBlocked(t *testing.T) {
	old := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "title", Type: "string"}}}}}
	if _, e := migration.Build(old, migration.Schema{}); e == nil {
		t.Fatal("entity deletion accepted")
	}
	next := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "title", Type: "integer"}}}}}
	if _, e := migration.Build(old, next); e == nil {
		t.Fatal("type change accepted")
	}
}
