package migration_test

import (
	"github.com/beanruntime/bean/internal/migration"
	"reflect"
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

func TestPlanIsDeterministic(t *testing.T) {
	schema := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "title", Type: "string"}}, Indexes: [][]string{{"title"}}}}}
	a, e := migration.Build(migration.Schema{}, schema)
	if e != nil {
		t.Fatal(e)
	}
	b, e := migration.Build(migration.Schema{}, schema)
	if e != nil || !reflect.DeepEqual(a, b) {
		t.Fatalf("plans differ: %#v %#v err=%v", a, b, e)
	}
}

func TestNewRelationsCreateTargetsBeforeDependentsAndJoinTables(t *testing.T) {
	schema := migration.Schema{Entities: []migration.Entity{
		{Name: "comment", Fields: []migration.Field{{Name: "post_id", Type: "relation", RelationEntity: "post", RelationKind: "many-to-one", TargetField: "id"}}},
		{Name: "post", Fields: []migration.Field{{Name: "tags", Type: "relation", RelationEntity: "tag", RelationKind: "many-to-many", TargetField: "id"}}},
		{Name: "tag"},
	}}
	plan, err := migration.Build(migration.Schema{}, schema)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Descriptions, ",")
	if strings.Index(joined, "create entity post") > strings.Index(joined, "create entity comment") || strings.Index(joined, "create entity tag") > strings.Index(joined, "create many-to-many relation post_tags") || strings.Index(joined, "create entity post") > strings.Index(joined, "create many-to-many relation post_tags") {
		t.Fatalf("unsafe relation order=%v", plan.Descriptions)
	}
}

func TestUnsafeConstraintAdditionIsRejected(t *testing.T) {
	old := migration.Schema{Entities: []migration.Entity{{Name: "book"}}}
	next := migration.Schema{Entities: []migration.Entity{{Name: "book", Fields: []migration.Field{{Name: "author_id", Type: "relation", RelationEntity: "author", RelationKind: "many-to-one", TargetField: "id"}}}, {Name: "author"}}}
	if _, e := migration.Build(old, next); e == nil {
		t.Fatal("unsafe relation addition accepted")
	}
}

func TestBooleanUsesPortableSQLType(t *testing.T) {
	plan, err := migration.Build(migration.Schema{}, migration.Schema{Entities: []migration.Entity{{Name: "feature", Fields: []migration.Field{{Name: "enabled", Type: "boolean"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Statements) != 1 || !strings.Contains(plan.Statements[0], `"enabled" BOOLEAN`) {
		t.Fatalf("statements=%v", plan.Statements)
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
