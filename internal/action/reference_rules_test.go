package action_test

import (
	"context"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/dbal"
)

func TestCommerceRuleDerivesOrderItemTotal(t *testing.T) {
	db, app := runtime(t, "commerce")
	defer db.Close()
	engine := action.Service{DB: db}
	product, err := engine.Execute(context.Background(), app, "product_create", map[string]any{"name": "Keyboard", "price": 125, "inventory": 10}, admin())
	if err != nil {
		t.Fatal(err)
	}
	order, err := engine.Execute(context.Background(), app, "order_create", map[string]any{}, admin())
	if err != nil {
		t.Fatal(err)
	}
	item, err := engine.Execute(context.Background(), app, "order_item_create", map[string]any{"order_id": order["id"], "product_id": product["id"], "quantity": 3, "unit_price": 125}, admin())
	if err != nil || item["line_total"] != int64(375) && item["line_total"] != 375 {
		t.Fatalf("item=%v err=%v", item, err)
	}
}

func TestATSRuleGuardsUnnamedCandidate(t *testing.T) {
	db, app := runtime(t, "ats")
	defer db.Close()
	engine := action.Service{DB: db}
	job, err := engine.Execute(context.Background(), app, "job_create", map[string]any{"title": "Engineer", "department": "Platform"}, admin())
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := engine.Execute(context.Background(), app, "candidate_create", map[string]any{"job_id": job["id"], "name": "   ", "email": "unnamed@example.test", "applied_at": "2030-01-01T00:00:00Z"}, admin())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = engine.Execute(context.Background(), app, "move_candidate", map[string]any{"id": candidate["id"], "stage": "screen"}, admin()); !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("unnamed candidate guard error=%v", err)
	}
}

func TestBookingRulesDeriveRequestedAtAndValidateInterval(t *testing.T) {
	db, app := runtime(t, "booking")
	defer db.Close()
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	engine := action.Service{DB: db, Now: func() time.Time { return now }}
	resource, err := engine.Execute(context.Background(), app, "resource_create", map[string]any{"name": "Room"}, admin())
	if err != nil {
		t.Fatal(err)
	}
	created, err := engine.Execute(context.Background(), app, "book_resource", map[string]any{"resource_id": resource["id"], "start_at": "2030-01-02T10:00:00Z", "end_at": "2030-01-02T11:00:00Z"}, admin())
	if err != nil || created["requested_at"] != now.Format(time.RFC3339Nano) {
		t.Fatalf("booking=%v err=%v", created, err)
	}
	_, err = engine.Execute(context.Background(), app, "book_resource", map[string]any{"resource_id": resource["id"], "start_at": "2030-01-03T11:00:00Z", "end_at": "2030-01-03T10:00:00Z"}, admin())
	if !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("invalid interval error=%v", err)
	}
}
