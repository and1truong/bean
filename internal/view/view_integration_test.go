package view_test

import (
	"context"
	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
	"path/filepath"
	"testing"
)

func TestCompiledQueryPlanAndOpaqueCursor(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "views.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = db.ExecuteMigration(ctx, []string{`CREATE TABLE category (id TEXT PRIMARY KEY, name TEXT NOT NULL, title TEXT NOT NULL)`, `CREATE TABLE book (id TEXT PRIMARY KEY, title TEXT NOT NULL, price INTEGER NOT NULL, category_id TEXT NOT NULL)`}); e != nil {
		t.Fatal(e)
	}
	for _, insert := range []dbal.Insert{
		{Table: "category", Values: map[string]dbal.Value{"id": "c1", "name": "Technical", "title": "Reference"}},
		{Table: "book", Values: map[string]dbal.Value{"id": "b1", "title": "Alpha", "price": 2, "category_id": "c1"}},
		{Table: "book", Values: map[string]dbal.Value{"id": "b2", "title": "Beta", "price": 3, "category_id": "c1"}},
	} {
		if _, e = db.Insert(ctx, insert); e != nil {
			t.Fatal(e)
		}
	}
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Fields: []appir.Field{{Name: "title", Type: "string"}, {Name: "price", Type: "integer"}, {Name: "category_id", Type: "relation"}}}
	app.Entities["category"] = appir.Entity{Name: "category", Fields: []appir.Field{{Name: "name", Type: "string"}, {Name: "title", Type: "string"}}}
	app.Views["totals"] = appir.View{Name: "totals", Entity: "book", Fields: []string{"category.name"}, Relationships: []appir.ViewRelationship{{Name: "category", Entity: "category", Type: "inner", LocalField: "category_id", TargetField: "id"}}, GroupBy: []string{"category.name"}, Aggregates: []appir.Aggregate{{Function: "sum", Field: "book.price", Alias: "total"}}, Sort: []appir.Sort{{Field: "category.name"}}, DefaultLimit: 10, MaxLimit: 10}
	service := view.Service{DB: db}
	rows, e := service.Run(ctx, app, "totals", view.Params{}, beanctx.Request{})
	if e != nil || len(rows) != 1 || rows[0]["total"] != int64(5) {
		t.Fatalf("rows=%v err=%v", rows, e)
	}
	app.Views["book_detail"] = appir.View{Name: "book_detail", Entity: "book", Fields: []string{"id", "title", "category.name"}, Relationships: []appir.ViewRelationship{{Name: "category", Entity: "category", Type: "inner", LocalField: "category_id", TargetField: "id"}}, DefaultLimit: 10, MaxLimit: 10}
	detail, e := service.RunPage(ctx, app, "book_detail", view.Params{RecordID: "b2"}, beanctx.Request{})
	if e != nil || len(detail.Rows) != 1 || detail.Rows[0]["id"] != "b2" {
		t.Fatalf("joined record lookup=%v err=%v", detail, e)
	}
	detail, e = service.RunPage(ctx, app, "book_detail", view.Params{Search: "bet", SearchFields: []string{"title"}}, beanctx.Request{})
	if e != nil || len(detail.Rows) != 1 || detail.Rows[0]["id"] != "b2" {
		t.Fatalf("joined base-field search=%v err=%v", detail, e)
	}
	app.Views["books"] = appir.View{Name: "books", Entity: "book", Fields: []string{"id", "title", "price"}, ExposedFilters: map[string]appir.ViewFilter{
		"title": {Field: "title", Operator: "contains", Type: "string"},
		"min":   {Field: "price", Operator: "gte", Type: "integer"},
		"max":   {Field: "price", Operator: "lte", Type: "integer"},
	}, Sort: []appir.Sort{{Field: "title"}}, DefaultLimit: 1, MaxLimit: 2}
	first, e := service.RunPage(ctx, app, "books", view.Params{}, beanctx.Request{})
	if e != nil || len(first.Rows) != 1 || first.NextCursor == "" {
		t.Fatalf("first=%v err=%v", first, e)
	}
	second, e := service.RunPage(ctx, app, "books", view.Params{Cursor: first.NextCursor}, beanctx.Request{})
	if e != nil || len(second.Rows) != 1 || second.Rows[0]["id"] == first.Rows[0]["id"] {
		t.Fatalf("second=%v err=%v", second, e)
	}
	if _, e = service.RunPage(ctx, app, "books", view.Params{Cursor: "not-a-cursor"}, beanctx.Request{}); !dbal.IsCode(e, dbal.InvalidQuery) {
		t.Fatalf("malformed cursor accepted: %v", e)
	}
	for name, filter := range map[string]map[string]any{"contains": {"title": "et"}, "gte": {"min": "3"}, "lte": {"max": "2"}} {
		result, filterErr := service.RunPage(ctx, app, "books", view.Params{Filter: filter}, beanctx.Request{})
		if filterErr != nil || len(result.Rows) != 1 || result.Rows[0]["title"] == nil {
			t.Fatalf("%s filter rows=%v err=%v", name, result.Rows, filterErr)
		}
	}
	if _, e = service.RunPage(ctx, app, "books", view.Params{Filter: map[string]any{"title": "alpha"}, Cursor: first.NextCursor}, beanctx.Request{}); !dbal.IsCode(e, dbal.InvalidQuery) {
		t.Fatalf("cursor survived a filter-state change: %v", e)
	}
	filtered, e := service.RunPage(ctx, app, "books", view.Params{Search: "bet", SearchFields: []string{"title"}, ExactFilters: map[string]any{"title": "Beta"}, Sort: []appir.Sort{{Field: "title", Desc: true}}}, beanctx.Request{})
	if e != nil || len(filtered.Rows) != 1 || filtered.Rows[0]["id"] != "b2" {
		t.Fatalf("admin query rows=%v err=%v", filtered.Rows, e)
	}
}

func TestTenantIsolationInjectedIntoView(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "tenant.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	k := kernel.New()
	s := &release.Store{DB: db, Migrations: db, Kernel: k, OpenAPI: openapi.Generate}
	if e = s.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "project"}, Spec: map[string]any{"tenant": true, "fields": []any{map[string]any{"name": "name", "type": "string", "required": true}}}}, {APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "tenant_member"}, Spec: map[string]any{"readRoles": []any{"member"}, "writeRoles": []any{"member"}, "tenant": true}}, {APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "projects"}, Spec: map[string]any{"entity": "project", "fields": []any{"id", "name", "tenant_id"}, "policy": "tenant_member"}}}
	if e = s.SaveBundle(ctx, "default", definition.Bundle{Name: "test", Definitions: defs}); e != nil {
		t.Fatal(e)
	}
	_, ds, e := s.Publish(ctx, "default")
	if e != nil || len(ds) > 0 {
		t.Fatalf("publish=%v ds=%v", e, ds)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	admin := func(tenant string) beanctx.Request {
		return beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000001", Roles: []string{"administrator"}}, TenantID: tenant}
	}
	if _, e = engine.Execute(ctx, app, "project_create", map[string]any{"name": "secret"}, admin("00000000-0000-4000-8000-00000000000a")); e != nil {
		t.Fatal(e)
	}
	views := view.Service{DB: db}
	member := func(tenant string) beanctx.Request {
		return beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000002", Roles: []string{"member"}}, TenantID: tenant}
	}
	rows, e := views.Run(ctx, app, "projects", view.Params{}, member("00000000-0000-4000-8000-00000000000b"))
	if e != nil || len(rows) != 0 {
		t.Fatalf("tenant B rows=%v err=%v", rows, e)
	}
	rows, e = views.Run(ctx, app, "projects", view.Params{}, member("00000000-0000-4000-8000-00000000000a"))
	if e != nil || len(rows) != 1 {
		t.Fatalf("tenant A rows=%v err=%v", rows, e)
	}
	_ = appir.Empty()
}
