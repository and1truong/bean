package view_test

import (
	"context"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/view"
)

type countingReader struct{ calls int }

func (r *countingReader) Select(_ context.Context, query dbal.Select) ([]dbal.Row, error) {
	r.calls++
	stored := dbal.Row{"id": "book-1", "title": "Building Bean", "metadata": map[string]any{"chapter": "one"}, "secret": "hidden", "version": 1}
	row := dbal.Row{}
	for _, column := range query.Columns {
		row[column] = stored[column]
	}
	return []dbal.Row{row}, nil
}

func TestScopeReusesOnlyCompatibleEntityReadProofs(t *testing.T) {
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Policy: "entity_read", Fields: []appir.Field{{Name: "title", Type: "string"}, {Name: "secret", Type: "string"}}}
	app.Policies["entity_read"] = appir.Policy{Name: "entity_read"}
	app.Policies["admin_read"] = appir.Policy{Name: "admin_read"}
	app.Views["compatible"] = appir.View{Name: "compatible", Entity: "book", Fields: []string{"id", "title"}, Policy: "entity_read", DefaultLimit: 1, MaxLimit: 1}
	app.Views["different_policy"] = appir.View{Name: "different_policy", Entity: "book", Fields: []string{"id", "title"}, Policy: "admin_read", DefaultLimit: 1, MaxLimit: 1}
	app.Views["contextual"] = appir.View{Name: "contextual", Entity: "book", Fields: []string{"id", "title"}, Policy: "entity_read", ContextFilter: &expr.Expr{Op: "eq", Left: &expr.Value{Source: "record", Name: "id"}, Right: &expr.Value{Source: "literal", Literal: "book-1"}}, DefaultLimit: 1, MaxLimit: 1}

	for _, test := range []struct {
		name      string
		view      string
		wantCalls int
	}{{"same effective contract", "compatible", 1}, {"different policy", "different_policy", 2}, {"contextual predicate", "contextual", 2}} {
		t.Run(test.name, func(t *testing.T) {
			reader := &countingReader{}
			scope := view.NewScope(app, reader, beanctx.Request{})
			if _, err := scope.Resolve(context.Background(), test.view, "book-1"); err != nil {
				t.Fatal(err)
			}
			if err := scope.AuthorizeEntity(context.Background(), "book", "book-1"); err != nil {
				t.Fatal(err)
			}
			if reader.calls != test.wantCalls {
				t.Fatalf("Select calls=%d, want %d", reader.calls, test.wantCalls)
			}
		})
	}
}

func TestScopeCachesExactImmutableProjectionPerRequest(t *testing.T) {
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Fields: []appir.Field{{Name: "title", Type: "string"}, {Name: "metadata", Type: "json"}, {Name: "secret", Type: "string"}}}
	app.Views["admin"] = appir.View{Name: "admin", Entity: "book", Fields: []string{"id", "title", "metadata"}, DefaultLimit: 1, MaxLimit: 1}
	app.Views["secret"] = appir.View{Name: "secret", Entity: "book", Fields: []string{"id", "secret"}, DefaultLimit: 1, MaxLimit: 1}
	reader := &countingReader{}
	scope := view.NewScope(app, reader, beanctx.Request{})

	first, err := scope.Resolve(context.Background(), "admin", "book-1")
	if err != nil {
		t.Fatal(err)
	}
	row := first.Row()
	row["title"] = "mutated"
	row["metadata"].(map[string]any)["chapter"] = "mutated"
	second, err := scope.Resolve(context.Background(), "admin", "book-1")
	if err != nil {
		t.Fatal(err)
	}
	if second.Row()["title"] != "Building Bean" || second.Row()["metadata"].(map[string]any)["chapter"] != "one" || second.Row()["secret"] != nil {
		t.Fatalf("cached projection was mutable or leaked fields: %#v", second.Row())
	}
	if reader.calls != 1 {
		t.Fatalf("Select calls=%d, want 1", reader.calls)
	}
	secret, err := scope.Resolve(context.Background(), "secret", "book-1")
	if err != nil {
		t.Fatal(err)
	}
	if secret.Row()["secret"] != "hidden" || secret.Row()["title"] != nil || reader.calls != 2 {
		t.Fatalf("one View projection satisfied another: row=%v calls=%d", secret.Row(), reader.calls)
	}
	if _, err = view.NewScope(app, reader, beanctx.Request{}).Resolve(context.Background(), "admin", "book-1"); err != nil {
		t.Fatal(err)
	}
	if reader.calls != 3 {
		t.Fatalf("cache crossed request scopes: calls=%d", reader.calls)
	}
}

func TestScopeDoesNotReuseAViewThatBroadensEntityVisibility(t *testing.T) {
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Policy: "authenticated", Fields: []appir.Field{{Name: "title", Type: "string"}}}
	app.Policies["authenticated"] = appir.Policy{Name: "authenticated", Authenticated: true}
	app.Policies["public_admin"] = appir.Policy{Name: "public_admin"}
	app.Views["admin"] = appir.View{Name: "admin", Entity: "book", Policy: "public_admin", Fields: []string{"id", "title"}, DefaultLimit: 1, MaxLimit: 1}
	reader := &countingReader{}
	scope := view.NewScope(app, reader, beanctx.Request{})
	if _, err := scope.Resolve(context.Background(), "admin", "book-1"); err != nil {
		t.Fatal(err)
	}
	if err := scope.AuthorizeEntity(context.Background(), "book", "book-1"); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatalf("broader View seeded Entity proof: %v", err)
	}
	if reader.calls != 1 {
		t.Fatalf("denied Entity authorization unexpectedly queried: calls=%d", reader.calls)
	}
}

func TestScopeCopiesAuthorizationContext(t *testing.T) {
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Policy: "editors", Fields: []appir.Field{{Name: "title", Type: "string"}}}
	app.Policies["editors"] = appir.Policy{Name: "editors", ReadRoles: []string{"editor"}}
	app.Views["admin"] = appir.View{Name: "admin", Entity: "book", Fields: []string{"id", "title"}, DefaultLimit: 1, MaxLimit: 1}
	reader := &countingReader{}
	request := beanctx.Request{User: &beanctx.User{ID: "user-1", Roles: []string{"editor"}}, Values: map[string]any{"nested": map[string]any{"value": "original"}}}
	scope := view.NewScope(app, reader, request)
	request.User.Roles[0] = "manager"
	request.Values["nested"].(map[string]any)["value"] = "changed"
	if _, err := scope.Resolve(context.Background(), "admin", "book-1"); err != nil {
		t.Fatalf("scope did not preserve its authorization context: %v", err)
	}
	copy := scope.Request()
	copy.User.Roles[0] = "manager"
	if scope.Request().User.Roles[0] != "editor" || scope.Request().Values["nested"].(map[string]any)["value"] != "original" {
		t.Fatalf("scope exposed mutable authorization context: %+v", scope.Request())
	}
}
