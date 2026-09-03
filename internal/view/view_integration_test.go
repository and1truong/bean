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
	app.Views["totals"] = appir.View{Name: "totals", Entity: "book", ResultShape: "groups", Fields: []string{"category.name"}, Relationships: []appir.ViewRelationship{{Name: "category", Entity: "category", Type: "inner", LocalField: "category_id", TargetField: "id"}}, GroupBy: []appir.ViewGroup{{Field: "category.name", As: "category_name"}}, Aggregates: []appir.Aggregate{{Function: "sum", Field: "book.price", Alias: "total"}}, Sort: []appir.Sort{{Field: "category_name"}}, DefaultLimit: 10, MaxLimit: 10}
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
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "project"}, Spec: map[string]any{"tenant": true, "fields": []any{map[string]any{"name": "name", "type": "string", "required": true}}}}, {APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "tenant_member"}, Spec: map[string]any{"readRoles": []any{"member"}, "writeRoles": []any{"member"}, "tenant": true}}, {APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "projects"}, Spec: map[string]any{"entity": "project", "fields": []any{"id", "name", "tenant_id"}, "policy": "tenant_member", "exposedFilters": map[string]any{"tenant": map[string]any{"field": "tenant_id"}}}}}
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
	if app.Views["projects"].ExposedFilters["tenant"].Type != "uuid" {
		t.Fatalf("tenant filter=%+v", app.Views["projects"].ExposedFilters["tenant"])
	}
	rows, e = views.Run(ctx, app, "projects", view.Params{Filter: map[string]any{"tenant": "00000000-0000-4000-8000-00000000000a"}}, member("00000000-0000-4000-8000-00000000000a"))
	if e != nil || len(rows) != 1 {
		t.Fatalf("tenant filter rows=%v err=%v", rows, e)
	}
	_ = appir.Empty()
}

func TestPolicyIsAppliedBeforeGroupingAndAggregation(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "aggregate-policy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, []string{`CREATE TABLE deal (id TEXT PRIMARY KEY, status TEXT NOT NULL, amount INTEGER NOT NULL, owner_id TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	for _, insert := range []dbal.Insert{
		{Table: "deal", Values: map[string]dbal.Value{"id": "a1", "status": "lead", "amount": 100, "owner_id": "alice"}},
		{Table: "deal", Values: map[string]dbal.Value{"id": "a2", "status": "won", "amount": 300, "owner_id": "alice"}},
		{Table: "deal", Values: map[string]dbal.Value{"id": "b1", "status": "lead", "amount": 900, "owner_id": "bob"}},
	} {
		if _, err = db.Insert(ctx, insert); err != nil {
			t.Fatal(err)
		}
	}
	app := appir.Empty()
	app.Entities["deal"] = appir.Entity{Name: "deal", Owner: true, Policy: "owned_records", Fields: []appir.Field{{Name: "status", Type: "enum", Options: []string{"lead", "won"}}, {Name: "amount", Type: "money"}}}
	app.Policies["owned_records"] = appir.Policy{Name: "owned_records", Owner: true, ReadRoles: []string{"salesperson", "manager"}, BypassOwnerRoles: []string{"manager"}}
	app.Views["pipeline"] = appir.View{Name: "pipeline", Entity: "deal", Policy: "owned_records", ResultShape: "groups", Fields: []string{"status"}, GroupBy: []appir.ViewGroup{{Field: "status"}}, Aggregates: []appir.Aggregate{{Function: "count", Field: "id", Alias: "deal_count"}, {Function: "sum", Field: "amount", Alias: "pipeline_amount"}}, Sort: []appir.Sort{{Field: "status"}}, MaxLimit: 20}
	app.Views["deal_records"] = appir.View{Name: "deal_records", Entity: "deal", Policy: "owned_records", ResultShape: "records", Fields: []string{"id", "status", "amount"}, ExposedFilters: map[string]appir.ViewFilter{"stage": {Field: "status", Operator: "eq", Type: "enum", Options: []string{"lead", "won"}}}, Sort: []appir.Sort{{Field: "id"}}, DefaultLimit: 20, MaxLimit: 20}
	service := view.Service{DB: db}
	salesperson := beanctx.Request{User: &beanctx.User{ID: "alice", Roles: []string{"salesperson"}}}
	result, err := service.RunPage(ctx, app, "pipeline", view.Params{}, salesperson)
	if err != nil || len(result.Rows) != 2 || result.Rows[0]["deal_count"] != int64(1) || result.Rows[0]["pipeline_amount"] != int64(100) {
		t.Fatalf("salesperson aggregate=%v err=%v", result, err)
	}
	drill, err := service.RunPage(ctx, app, "deal_records", view.Params{Filter: map[string]any{"stage": "lead"}}, salesperson)
	if err != nil || len(drill.Rows) != 1 || drill.Rows[0]["id"] != "a1" {
		t.Fatalf("salesperson drill=%v err=%v", drill, err)
	}
	manager := beanctx.Request{User: &beanctx.User{ID: "manager", Roles: []string{"manager"}}}
	result, err = service.RunPage(ctx, app, "pipeline", view.Params{}, manager)
	if err != nil || len(result.Rows) != 2 || result.Rows[0]["deal_count"] != int64(2) || result.Rows[0]["pipeline_amount"] != int64(1000) {
		t.Fatalf("manager aggregate=%v err=%v", result, err)
	}
	drill, err = service.RunPage(ctx, app, "deal_records", view.Params{Filter: map[string]any{"stage": "lead"}}, manager)
	if err != nil || len(drill.Rows) != 2 {
		t.Fatalf("manager drill=%v err=%v", drill, err)
	}
}

func TestDateBucketsAndGroupOverflow(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "date-groups.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, []string{`CREATE TABLE event (id TEXT PRIMARY KEY, occurred_at TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	for _, insert := range []dbal.Insert{
		{Table: "event", Values: map[string]dbal.Value{"id": "1", "occurred_at": "2026-07-01T10:00:00Z"}},
		{Table: "event", Values: map[string]dbal.Value{"id": "2", "occurred_at": "2026-08-01T10:00:00Z"}},
		{Table: "event", Values: map[string]dbal.Value{"id": "3", "occurred_at": "2026-09-01T10:00:00Z"}},
	} {
		if _, err = db.Insert(ctx, insert); err != nil {
			t.Fatal(err)
		}
	}
	app := appir.Empty()
	app.Entities["event"] = appir.Entity{Name: "event", Fields: []appir.Field{{Name: "occurred_at", Type: "datetime"}}}
	app.Views["events_by_month"] = appir.View{Name: "events_by_month", Entity: "event", ResultShape: "groups", Fields: []string{"occurred_at"}, GroupBy: []appir.ViewGroup{{Field: "occurred_at", As: "month", Bucket: "month"}}, Aggregates: []appir.Aggregate{{Function: "count", Field: "id", Alias: "event_count"}}, Sort: []appir.Sort{{Field: "month"}}, MaxLimit: 3}
	service := view.Service{DB: db}
	result, err := service.RunPage(ctx, app, "events_by_month", view.Params{}, beanctx.Request{})
	if err != nil || len(result.Rows) != 3 || result.Rows[0]["month"] != "2026-07-01" {
		t.Fatalf("date groups=%v err=%v", result, err)
	}
	grouped := app.Views["events_by_month"]
	grouped.MaxLimit = 2
	app.Views["events_by_month"] = grouped
	if _, err = service.RunPage(ctx, app, "events_by_month", view.Params{}, beanctx.Request{}); !dbal.IsCode(err, dbal.ResultLimitExceeded) {
		t.Fatalf("group overflow accepted: %v", err)
	}
}

func TestDateBucketCarriesTheSelectedFieldTypeToDBAL(t *testing.T) {
	database := &capturingDatabase{}
	app := appir.Empty()
	app.Entities["event"] = appir.Entity{Name: "event", Fields: []appir.Field{{Name: "occurred_at", Type: "datetime"}}}
	app.Views["events_by_month"] = appir.View{Name: "events_by_month", Entity: "event", ResultShape: "groups", Fields: []string{"occurred_at"}, GroupBy: []appir.ViewGroup{{Field: "occurred_at", As: "month", Bucket: "month"}}, Aggregates: []appir.Aggregate{{Function: "count", Field: "id", Alias: "event_count"}}, MaxLimit: 3}
	if _, err := (view.Service{DB: database}).RunPage(context.Background(), app, "events_by_month", view.Params{}, beanctx.Request{}); err != nil {
		t.Fatal(err)
	}
	if len(database.query.GroupBy) != 1 || database.query.GroupBy[0].Type != "datetime" {
		t.Fatalf("groups=%+v", database.query.GroupBy)
	}
}

type capturingDatabase struct{ query dbal.Select }

func (d *capturingDatabase) Select(_ context.Context, query dbal.Select) ([]dbal.Row, error) {
	d.query = query
	return []dbal.Row{{"month": "2026-09-01", "event_count": int64(1)}}, nil
}
func (*capturingDatabase) Insert(context.Context, dbal.Insert) (dbal.Result, error) {
	panic("unexpected insert")
}
func (*capturingDatabase) Update(context.Context, dbal.Update) (dbal.Result, error) {
	panic("unexpected update")
}
func (*capturingDatabase) Delete(context.Context, dbal.Delete) (dbal.Result, error) {
	panic("unexpected delete")
}
func (*capturingDatabase) Transaction(context.Context, func(dbal.Transaction) error) error {
	panic("unexpected transaction")
}
func (*capturingDatabase) Close() error { return nil }
