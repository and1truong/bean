package action_test

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
	rows, e := db.Select(context.Background(), dbal.Select{Table: "booking", Columns: []string{"id"}, Limit: 50})
	if e != nil || len(rows) != 1 {
		t.Fatalf("bookings=%v err=%v", rows, e)
	}
}
