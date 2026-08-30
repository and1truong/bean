package action_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func runtime(t *testing.T, name string) (*sqlite.DB, *appir.App) {
	t.Helper()
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), name+".db"))
	if e != nil {
		t.Fatal(e)
	}
	k := kernel.New()
	s := &release.Store{DB: db, Migrations: db, Kernel: k, OpenAPI: openapi.Generate}
	if e = s.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	f, e := os.Open(filepath.Join("..", "..", "examples", name, "app.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	bundle, e := definition.Decode(f)
	if e != nil {
		t.Fatal(e)
	}
	if e = s.SaveBundle(ctx, "default", bundle); e != nil {
		t.Fatal(e)
	}
	_, ds, e := s.Publish(ctx, "default")
	if e != nil || len(ds) > 0 {
		t.Fatalf("publish err=%v diagnostics=%v", e, ds)
	}
	a, _ := k.Active()
	return db, a
}

func TestManyToManyActionWriteAndViewTraversal(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "relations.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Kernel: k, OpenAPI: openapi.Generate}
	if e = store.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "tag"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "tags", "type": "relation", "relation": map[string]any{"entity": "tag", "kind": "many-to-many"}}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "article_tags"}, Spec: map[string]any{"entity": "article", "fields": []any{"id", "tags.name"}, "relationships": []any{map[string]any{"name": "tags", "relationField": "tags", "type": "inner"}}, "sort": []any{map[string]any{"field": "id"}}}},
	}
	if e = store.SaveBundle(ctx, "default", definition.Bundle{Name: "relations", Definitions: defs}); e != nil {
		t.Fatal(e)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	tag, e := engine.Execute(ctx, app, "tag_create", map[string]any{"name": "Go"}, admin())
	if e != nil {
		t.Fatal(e)
	}
	article, e := engine.Execute(ctx, app, "article_create", map[string]any{"title": "Bean", "tags": []any{tag["id"]}}, admin())
	if e != nil {
		t.Fatal(e)
	}
	rows, e := (view.Service{DB: db}).Run(ctx, app, "article_tags", view.Params{}, admin())
	if e != nil || len(rows) != 1 || rows[0]["id"] != article["id"] || rows[0]["name"] != "Go" {
		t.Fatalf("rows=%v err=%v cause=%v", rows, e, errors.Unwrap(e))
	}
}

func TestScalarRelationRejectsCrossTenantTarget(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "scalar-relation.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Inspector: db, Kernel: k, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "tenant"}, Spec: map[string]any{"tenant": true}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "category"}, Spec: map[string]any{"tenant": true, "policy": "tenant", "fields": []any{map[string]any{"name": "name", "type": "string", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"tenant": true, "policy": "tenant", "fields": []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "category_id", "type": "relation", "required": true, "relation": map[string]any{"entity": "category", "kind": "many-to-one", "targetField": "id"}}}}},
	}
	if err = store.SaveBundle(ctx, "default", definition.Bundle{Name: "scalar relation", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	tenantA := beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000001"}, TenantID: "00000000-0000-4000-8000-00000000000a", RequestID: "tenant-a"}
	tenantB := beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000002"}, TenantID: "00000000-0000-4000-8000-00000000000b", RequestID: "tenant-b"}
	category, err := engine.Execute(ctx, app, "category_create", map[string]any{"name": "Tenant A"}, tenantA)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(ctx, app, "article_create", map[string]any{"title": "Cross tenant", "category_id": category["id"]}, tenantB); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatalf("want hidden cross-tenant target, got %v", err)
	}
	rows, err := db.Select(ctx, dbal.Select{Table: "article", Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("cross-tenant relation committed: rows=%v err=%v", rows, err)
	}
}

func TestAnonymousCannotReadOwnerScopedGeneratedView(t *testing.T) {
	ctx := context.Background()
	db, app := runtime(t, "community")
	defer db.Close()
	engine := action.Service{DB: db}
	if _, err := engine.Execute(ctx, app, "profile_create", map[string]any{"display_name": "Owner"}, admin()); err != nil {
		t.Fatal(err)
	}
	views := view.Service{DB: db}
	if _, err := views.Run(ctx, app, "profile_list", view.Params{}, beanctx.Request{}); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatalf("anonymous owner view error=%v", err)
	}
	rows, err := views.Run(ctx, app, "profile_list", view.Params{}, admin())
	if err != nil || len(rows) != 1 {
		t.Fatalf("owner rows=%v err=%v", rows, err)
	}
}

func TestTransactionQueryEnforcesImplicitOwnerScope(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "transaction-owner.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Inspector: db, Kernel: k, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "public_action"}, Spec: map[string]any{}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "profile"}, Spec: map[string]any{"owner": true, "fields": []any{map[string]any{"name": "display_name", "type": "string", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "profiles"}, Spec: map[string]any{"entity": "profile", "fields": []any{"id", "display_name"}}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "query_profiles"}, Spec: map[string]any{"entity": "profile", "operation": "transaction", "policy": "public_action", "input": map[string]any{"request": map[string]any{"type": "string", "required": true}}, "output": map[string]any{"first_id": map[string]any{"type": "uuid", "required": true}}, "steps": []any{
			map[string]any{"op": "query", "view": "profiles", "result": "profiles"},
			map[string]any{"op": "return", "values": map[string]any{"first_id": "$result.profiles.0.id"}},
		}}},
	}
	if err = store.SaveBundle(ctx, "default", definition.Bundle{Name: "transaction owner", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	first := admin()
	second := admin()
	second.User.ID = "00000000-0000-4000-8000-000000000002"
	firstProfile, err := engine.Execute(ctx, app, "profile_create", map[string]any{"display_name": "First"}, first)
	if err != nil {
		t.Fatal(err)
	}
	secondProfile, err := engine.Execute(ctx, app, "profile_create", map[string]any{"display_name": "Second"}, second)
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"request": "profiles"}
	if _, err = engine.Execute(ctx, app, "query_profiles", input, beanctx.Request{}); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatalf("anonymous transaction query error=%v", err)
	}
	for _, test := range []struct {
		request beanctx.Request
		want    any
	}{{first, firstProfile["id"]}, {second, secondProfile["id"]}} {
		result, executeErr := engine.Execute(ctx, app, "query_profiles", input, test.request)
		if executeErr != nil || result["first_id"] != test.want {
			t.Fatalf("result=%v want=%v err=%v", result, test.want, executeErr)
		}
	}
}

func TestWritePolicyReceivesHydratedBooleanRecord(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "policy-hydration.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Inspector: db, Kernel: k, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	condition := map[string]any{"op": "eq", "left": map[string]any{"source": "record", "name": "featured"}, "right": map[string]any{"source": "literal", "literal": true}}
	definitions := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "featured_only"}, Spec: map[string]any{"condition": condition}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "document"}, Spec: map[string]any{"policy": "featured_only", "fields": []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "featured", "type": "boolean", "required": true}, map[string]any{"name": "settings", "type": "json", "required": true}}}},
	}
	if err = store.SaveBundle(ctx, "default", definition.Bundle{Name: "policy hydration", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	document, err := engine.Execute(ctx, app, "document_create", map[string]any{"title": "before", "featured": true, "settings": map[string]any{"scope": "private"}}, admin())
	if err != nil {
		t.Fatal(err)
	}
	request := admin()
	request.RequestID = "policy-update"
	updated, err := engine.Execute(ctx, app, "document_update", map[string]any{"id": document["id"], "title": "after"}, request)
	if err != nil || updated["title"] != "after" || updated["featured"] != true {
		t.Fatalf("updated=%v err=%v", updated, err)
	}
}

func TestEveryTransactionStepHasRuntimeSemantics(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "steps.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Kernel: k, OpenAPI: openapi.Generate}
	if e = store.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	value := func(source, name string, literal any) map[string]any {
		return map[string]any{"source": source, "name": name, "literal": literal}
	}
	condition := func(field string, literal any) map[string]any {
		return map[string]any{"op": "eq", "left": value("record", field, nil), "right": value("literal", "", literal)}
	}
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string", "required": true}, map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"draft", "done"}}, map[string]any{"name": "count", "type": "integer", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{"entity": "item", "fields": []any{"id", "name", "status", "count"}, "sort": []any{map[string]any{"field": "id"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "flow"}, Spec: map[string]any{"entity": "item", "operation": "transaction", "input": map[string]any{"id": map[string]any{"type": "uuid", "required": true}, "name": map[string]any{"type": "string", "required": true}}, "output": map[string]any{"message": map[string]any{"type": "string"}}, "stateField": "status", "transitions": map[string]any{"draft": []any{"done"}}, "steps": []any{
			map[string]any{"op": "load", "result": "loaded", "values": map[string]any{"id": "$input.id"}},
			map[string]any{"op": "assert", "condition": map[string]any{"op": "ne", "left": value("input", "name", nil), "right": value("literal", "", "")}},
			map[string]any{"op": "update", "result": "updated", "values": map[string]any{"id": "$input.id", "name": "$input.name"}},
			map[string]any{"op": "conditional_update", "values": map[string]any{"id": "$input.id", "count": 2}, "condition": condition("status", "draft")},
			map[string]any{"op": "transition", "values": map[string]any{"id": "$input.id", "status": "done"}},
			map[string]any{"op": "query", "view": "items", "result": "queried", "where": map[string]any{"op": "eq", "left": value("record", "id", nil), "right": value("input", "id", nil)}},
			map[string]any{"op": "emit", "event": "item_changed"},
			map[string]any{"op": "schedule", "job": "flow_job"},
			map[string]any{"op": "return", "values": map[string]any{"message": "$result.updated.name"}},
		}}},
		{APIVersion: "bean/v1alpha1", Kind: "Job", Metadata: definition.Metadata{Name: "flow_job"}, Spec: map[string]any{"action": "flow"}},
	}
	if e = store.SaveBundle(ctx, "default", definition.Bundle{Name: "steps", Definitions: defs}); e != nil {
		t.Fatal(e)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := k.Active()
	engine := action.Service{DB: db}
	item, e := engine.Execute(ctx, app, "item_create", map[string]any{"name": "before", "status": "draft", "count": 1}, admin())
	if e != nil {
		t.Fatal(e)
	}
	request := admin()
	request.TenantID = "00000000-0000-4000-8000-00000000000a"
	result, e := engine.Execute(ctx, app, "flow", map[string]any{"id": item["id"], "name": "after", job.TenantIDPayloadKey: "attacker-tenant"}, request)
	if e != nil || result["message"] != "after" || result["status"] != "done" || result["count"] != int64(2) {
		t.Fatalf("result=%v err=%v", result, e)
	}
	for _, table := range []string{"bean_outbox", "bean_job"} {
		rows, er := db.Select(ctx, dbal.Select{Table: table, Limit: 10})
		if er != nil || len(rows) != 1 {
			t.Fatalf("table=%s rows=%v err=%v", table, rows, er)
		}
		if table == "bean_job" {
			var payload map[string]any
			if er = json.Unmarshal([]byte(fmt.Sprint(rows[0]["payload"])), &payload); er != nil || payload[job.TenantIDPayloadKey] != request.TenantID {
				t.Fatalf("scheduled payload=%v err=%v", payload, er)
			}
		}
	}
	second, e := engine.Execute(ctx, app, "item_create", map[string]any{"name": "second", "status": "draft", "count": 1}, admin())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = engine.Execute(ctx, app, "flow", map[string]any{"id": second["id"], "name": "second-after", job.TenantIDPayloadKey: "attacker-tenant"}, admin()); e != nil {
		t.Fatal(e)
	}
	jobs, e := db.Select(ctx, dbal.Select{Table: "bean_job", Columns: []string{"payload"}, Limit: 10})
	if e != nil || len(jobs) != 2 {
		t.Fatalf("jobs=%v err=%v", jobs, e)
	}
	foundSecond := false
	for _, row := range jobs {
		var payload map[string]any
		if e = json.Unmarshal([]byte(fmt.Sprint(row["payload"])), &payload); e != nil {
			t.Fatal(e)
		}
		if payload["id"] == second["id"] {
			foundSecond = true
			if _, exists := payload[job.TenantIDPayloadKey]; exists {
				t.Fatalf("untrusted tenant context persisted: %v", payload)
			}
		}
	}
	if !foundSecond {
		t.Fatalf("second scheduled payload not found: %v", jobs)
	}
	if _, e = engine.Execute(ctx, app, "item_delete", map[string]any{"id": item["id"]}, admin()); e != nil {
		t.Fatal(e)
	}
}

func TestTransactionRollbackAndIdempotentReplay(t *testing.T) {
	db, app := runtime(t, "commerce")
	defer db.Close()
	engine := action.Service{DB: db}
	product, e := engine.Execute(context.Background(), app, "product_create", map[string]any{"name": "Replay", "price": 100, "inventory": 2}, admin())
	if e != nil {
		t.Fatal(e)
	}
	input := map[string]any{"product_id": product["id"], "quantity": 1, "_idempotencyKey": "checkout-1"}
	first, e := engine.Execute(context.Background(), app, "place_order", input, admin())
	if e != nil {
		t.Fatal(e)
	}
	second, e := engine.Execute(context.Background(), app, "place_order", input, admin())
	if e != nil || fmt.Sprint(first["id"]) != fmt.Sprint(second["id"]) {
		t.Fatalf("first=%v second=%v err=%v", first, second, e)
	}
	products, _ := db.Select(context.Background(), dbal.Select{Table: "product", Columns: []string{"inventory"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: product["id"]}, Limit: 1})
	if products[0]["inventory"] != int64(1) {
		t.Fatalf("idempotent action ran twice: %v", products)
	}
	app.Actions["rollback"] = appir.Action{Name: "rollback", Entity: "order", Operation: "transaction", Steps: []appir.Step{
		{Op: "create", Values: []appir.Assignment{{Field: "status", Value: appir.ValueBinding{Source: "literal", Literal: json.RawMessage(`"pending_payment"`)}}}},
		{Op: "assert", Condition: &expr.Expr{Op: "eq", Left: &expr.Value{Source: "literal", Literal: 1}, Right: &expr.Value{Source: "literal", Literal: 2}}},
	}}
	if _, e = engine.Execute(context.Background(), app, "rollback", map[string]any{}, admin()); !dbal.IsCode(e, dbal.Conflict) {
		t.Fatalf("want rollback conflict, got %v", e)
	}
	orders, _ := db.Select(context.Background(), dbal.Select{Table: "order", Columns: []string{"id"}, Limit: 20})
	if len(orders) != 1 {
		t.Fatalf("failed transaction leaked a row: %v", orders)
	}
}

func TestIdempotencyKeyRejectsDifferentInput(t *testing.T) {
	db, app := runtime(t, "commerce")
	defer db.Close()
	engine := action.Service{DB: db}
	product, err := engine.Execute(context.Background(), app, "product_create", map[string]any{"name": "Fingerprint", "price": 100, "inventory": 3}, admin())
	if err != nil {
		t.Fatal(err)
	}
	key := "checkout-fingerprint"
	if _, err = engine.Execute(context.Background(), app, "place_order", map[string]any{"product_id": product["id"], "quantity": 1, "_idempotencyKey": key}, admin()); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(context.Background(), app, "place_order", map[string]any{"product_id": product["id"], "quantity": 2, "_idempotencyKey": key}, admin()); !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("want conflict for reused key with different input, got %v", err)
	}
	orders, err := db.Select(context.Background(), dbal.Select{Table: "order", Columns: []string{"id"}, Limit: 10})
	if err != nil || len(orders) != 1 {
		t.Fatalf("orders=%v err=%v", orders, err)
	}
}

func TestIdempotencyKeyIsScopedToPrincipalAndTenant(t *testing.T) {
	db, app := runtime(t, "commerce")
	defer db.Close()
	ctx := context.Background()
	engine := action.Service{DB: db}
	product, err := engine.Execute(ctx, app, "product_create", map[string]any{"name": "Scoped", "price": 100, "inventory": 3}, admin())
	if err != nil {
		t.Fatal(err)
	}
	first := admin()
	first.TenantID = "00000000-0000-4000-8000-00000000000a"
	second := admin()
	second.User.ID = "00000000-0000-4000-8000-000000000002"
	second.TenantID = "00000000-0000-4000-8000-00000000000b"
	input := map[string]any{"product_id": product["id"], "quantity": 1, "_idempotencyKey": "shared-client-key"}
	if _, err = engine.Execute(ctx, app, "place_order", input, first); err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(ctx, app, "place_order", input, second); err != nil {
		t.Fatal(err)
	}
	orders, err := db.Select(ctx, dbal.Select{Table: "order", Columns: []string{"id"}, Limit: 10})
	if err != nil || len(orders) != 2 {
		t.Fatalf("orders=%v err=%v", orders, err)
	}
	replays, err := db.Select(ctx, dbal.Select{Table: "bean_idempotency", Columns: []string{"key"}, Limit: 10})
	if err != nil || len(replays) != 2 || replays[0]["key"] == replays[1]["key"] {
		t.Fatalf("idempotency rows=%v err=%v", replays, err)
	}
}

func admin() beanctx.Request {
	return beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000001", Email: "admin@example.test", Roles: []string{"administrator"}}, RequestID: "test"}
}
func TestConcurrentInventoryCannotGoNegative(t *testing.T) {
	db, app := runtime(t, "commerce")
	defer db.Close()
	engine := action.Service{DB: db}
	product, e := engine.Execute(context.Background(), app, "product_create", map[string]any{"name": "One", "price": 100, "inventory": 1}, admin())
	if e != nil {
		t.Fatal(e)
	}
	id := product["id"]
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := engine.Execute(context.Background(), app, "place_order", map[string]any{"product_id": id, "quantity": 1}, admin())
			errs <- e
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful orders=%d", success)
	}
	rows, e := db.Select(context.Background(), dbal.Select{Table: "product", Columns: []string{"inventory"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if e != nil || rows[0]["inventory"].(int64) != 0 {
		t.Fatalf("rows=%v err=%v", rows, e)
	}
	orders, e := db.Select(context.Background(), dbal.Select{Table: "order", Columns: []string{"id"}, Limit: 50})
	if e != nil || len(orders) != 1 {
		t.Fatalf("orders=%v err=%v", orders, e)
	}
}
func TestConcurrentBookingsDoNotOverlap(t *testing.T) {
	db, app := runtime(t, "booking")
	defer db.Close()
	engine := action.Service{DB: db}
	resource, e := engine.Execute(context.Background(), app, "resource_create", map[string]any{"name": "Room"}, admin())
	if e != nil {
		t.Fatal(e)
	}
	input := map[string]any{"resource_id": resource["id"], "start_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339), "end_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)}
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, e := engine.Execute(context.Background(), app, "book_resource", input, admin())
			errs <- e
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	success := 0
	for e := range errs {
		if e == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("successful bookings=%d", success)
	}
	rows, e := db.Select(context.Background(), dbal.Select{Table: "booking", Columns: []string{"id", "status"}, Limit: 50})
	if e != nil || len(rows) != 1 {
		t.Fatalf("bookings=%v err=%v", rows, e)
	}
	runner := job.Runner{DB: db, Handle: func(ctx context.Context, name string, payload map[string]any) error {
		definition := app.Jobs[name]
		_, runErr := engine.Execute(ctx, app, definition.Action, payload, admin())
		return runErr
	}}
	if e = runner.RunOnce(context.Background()); e != nil {
		t.Fatal(e)
	}
	rows, e = db.Select(context.Background(), dbal.Select{Table: "booking", Columns: []string{"id", "status"}, Limit: 50})
	if e != nil || rows[0]["status"] != "cancelled" {
		t.Fatalf("booking reminder did not receive transition payload: rows=%v err=%v", rows, e)
	}
}
