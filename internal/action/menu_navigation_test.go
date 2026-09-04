package action_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/kernel"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestEntityActionsCommitNavigationPlacementsAtomicallyAndCleanUp(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "menu.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &release.Store{DB: db, Migrations: db, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.ExecuteMigration(ctx, []string{
		`CREATE TABLE book (id TEXT PRIMARY KEY,title TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL)`,
		`CREATE TABLE book_page (id TEXT PRIMARY KEY,title TEXT NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL)`,
	}); err != nil {
		t.Fatal(err)
	}
	app := menuActionApp()
	counter := 0
	service := action.Service{DB: db, CreateID: func(appir.Entity, map[string]any) string {
		counter++
		return fmt.Sprintf("00000000-0000-4000-8000-%012d", counter)
	}}
	request := beanctx.Request{User: &beanctx.User{ID: "admin", Roles: []string{"administrator"}}}
	book, err := service.Execute(ctx, app, "book_create", map[string]any{"title": "Building Bean"}, request)
	if err != nil {
		t.Fatal(err)
	}
	root, err := service.Execute(ctx, app, "page_create", map[string]any{
		"title":                 "Architecture",
		beanmenu.ActionInputKey: map[string]any{"placements": []any{map[string]any{"menu": "book_contents", "ownerId": book["id"], "weight": 10}}},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	placements := menuPlacements(t, db)
	if len(placements) != 1 || placements[0]["target_id"] != root["id"] || placements[0]["owner_id"] != book["id"] || placements[0]["weight"] != int64(10) {
		t.Fatalf("root placements=%+v", placements)
	}
	rootPlacement := placements[0]["id"]
	child, err := service.Execute(ctx, app, "page_create", map[string]any{
		"title":                 "Modules",
		beanmenu.ActionInputKey: map[string]any{"placements": []any{map[string]any{"menu": "book_contents", "ownerId": book["id"], "parentId": rootPlacement, "weight": 20, "labelOverride": "Deep modules"}}},
	}, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(menuPlacements(t, db)) != 2 {
		t.Fatal("child placement was not committed")
	}
	_, err = service.Execute(ctx, app, "page_update", map[string]any{
		"id": root["id"], "version": root["version"], "title": "Changed",
		beanmenu.ActionInputKey: map[string]any{"placements": []any{}},
	}, request)
	if !dbal.IsCode(err, dbal.Conflict) {
		t.Fatalf("parent placement removal err=%v", err)
	}
	rows, err := db.Select(ctx, dbal.Select{Table: "book_page", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: root["id"]}, Limit: 1})
	if err != nil || len(rows) != 1 || rows[0]["title"] != "Architecture" || rows[0]["version"] != int64(1) {
		t.Fatalf("record update did not roll back: rows=%+v err=%v", rows, err)
	}
	if _, err = service.Execute(ctx, app, "page_delete", map[string]any{"id": child["id"], "version": child["version"]}, request); err != nil {
		t.Fatal(err)
	}
	if len(menuPlacements(t, db)) != 1 {
		t.Fatal("target deletion did not remove its placement")
	}
	if _, err = service.Execute(ctx, app, "book_delete", map[string]any{"id": book["id"], "version": book["version"]}, request); err != nil {
		t.Fatal(err)
	}
	if placements = menuPlacements(t, db); len(placements) != 0 {
		t.Fatalf("owner deletion retained placements=%+v", placements)
	}
}

func menuPlacements(t *testing.T, db *sqlite.DB) []dbal.Row {
	t.Helper()
	rows, err := db.Select(context.Background(), dbal.Select{Table: beanmenu.PlacementTable, OrderBy: []dbal.Order{{Column: "created_at"}}, Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

func menuActionApp() *appir.App {
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Fields: []appir.Field{{Name: "title", Type: "string", Required: true}}}
	app.Entities["book_page"] = appir.Entity{Name: "book_page", Fields: []appir.Field{{Name: "title", Type: "string", Required: true}}, Navigation: &appir.EntityNavigation{LabelField: "title", Destination: appir.NavigationDestination{View: "book_pages", Display: "detail"}, Menus: []string{"book_contents"}}}
	app.Menus["book_contents"] = appir.Menu{Name: "book_contents", Profile: "workspace", MaxDepth: 3, Owner: &appir.MenuOwner{Entity: "book"}}
	app.Views["book_pages"] = appir.View{Name: "book_pages", Entity: "book_page", Fields: []string{"id", "title"}, Displays: map[string]appir.Display{"detail": {Type: "page", Route: "/pages/:id"}}}
	output := func(entity string) map[string]appir.Field {
		return map[string]appir.Field{
			"id": {Name: "id", Type: "uuid"}, "title": {Name: "title", Type: "string"},
			"created_at": {Name: "created_at", Type: "datetime"}, "updated_at": {Name: "updated_at", Type: "datetime"}, "version": {Name: "version", Type: "integer"},
		}
	}
	for _, entity := range []string{"book", "book_page"} {
		prefix := entity
		if entity == "book_page" {
			prefix = "page"
		}
		app.Actions[prefix+"_create"] = appir.Action{Name: prefix + "_create", Entity: entity, Operation: "create", Input: map[string]appir.Field{"title": {Name: "title", Type: "string", Required: true}}, Output: output(entity)}
		app.Actions[prefix+"_update"] = appir.Action{Name: prefix + "_update", Entity: entity, Operation: "update", Input: map[string]appir.Field{"id": {Name: "id", Type: "uuid", Required: true}, "title": {Name: "title", Type: "string"}, "version": {Name: "version", Type: "integer", Required: true}}, Output: output(entity)}
		app.Actions[prefix+"_delete"] = appir.Action{Name: prefix + "_delete", Entity: entity, Operation: "delete", Input: map[string]appir.Field{"id": {Name: "id", Type: "uuid", Required: true}, "version": {Name: "version", Type: "integer", Required: true}}, Output: output(entity)}
	}
	return app
}
