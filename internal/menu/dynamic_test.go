package menu_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/view"
)

type countingReader struct {
	reader view.Reader
	tables map[string]int
}

func (r *countingReader) Select(ctx context.Context, query dbal.Select) ([]dbal.Row, error) {
	r.tables[query.Table]++
	return r.reader.Select(ctx, query)
}

func TestDynamicTreeResolvesRecordRoutesAndHierarchy(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "menu.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	plan, err := migration.Build(migration.Schema{}, migration.Schema{Entities: []migration.Entity{
		{Name: "book", Fields: []migration.Field{{Name: "title", Type: "string"}}},
		{Name: "page", Fields: []migration.Field{{Name: "title", Type: "string"}}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err = db.ExecuteMigration(ctx, append(migration.MetadataSchema(), plan.Statements...)); err != nil {
		t.Fatal(err)
	}
	insert := func(table, id, title string) {
		t.Helper()
		if _, insertErr := db.Insert(ctx, dbal.Insert{Table: table, Values: map[string]dbal.Value{"id": id, "title": title, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z", "version": 1}}); insertErr != nil {
			t.Fatalf("%#v", insertErr)
		}
	}
	insert("book", "book-1", "A book")
	insert("page", "page-1", "Introduction")
	insert("page", "page-2", "Details")
	for _, values := range []map[string]dbal.Value{
		{"id": "b", "menu_name": "contents", "owner_entity": "book", "owner_id": "book-1", "target_entity": "page", "target_id": "page-1", "weight": 20, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
		{"id": "a", "menu_name": "contents", "owner_entity": "book", "owner_id": "book-1", "target_entity": "page", "target_id": "page-2", "parent_id": "b", "weight": 0, "created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z"},
	} {
		if _, err = db.Insert(ctx, dbal.Insert{Table: beanmenu.PlacementTable, Values: values}); err != nil {
			t.Fatal(err)
		}
	}
	app := appir.Empty()
	app.Entities["book"] = appir.Entity{Name: "book", Fields: []appir.Field{{Name: "title", Type: "string"}}}
	app.Entities["page"] = appir.Entity{Name: "page", Fields: []appir.Field{{Name: "title", Type: "string"}}, Navigation: &appir.EntityNavigation{LabelField: "title", Destination: appir.NavigationDestination{View: "pages", Display: "detail"}, Menus: []string{"contents"}}}
	app.Views["pages"] = appir.View{Name: "pages", Entity: "page", Fields: []string{"id", "title"}, Displays: map[string]appir.Display{"detail": {Type: "page", Route: "/pages/:id"}}}
	app.Menus["contents"] = appir.Menu{Name: "contents", Profile: "workspace", MaxDepth: 3, Owner: &appir.MenuOwner{Entity: "book"}}
	tree, err := beanmenu.DynamicTree(ctx, db, app, "contents", "book-1", beanctx.Request{Route: "/pages/page-2"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 1 || tree[0].Label != "Introduction" || tree[0].Route != "/pages/page-1?_menu=contents&_owner=book-1" || !tree[0].Active || len(tree[0].Children) != 1 || tree[0].Children[0].Label != "Details" || !tree[0].Children[0].Current {
		t.Fatalf("tree=%+v", tree)
	}
	app.Views["book_admin"] = appir.View{Name: "book_admin", Entity: "book", Fields: []string{"id", "title"}, DefaultLimit: 1, MaxLimit: 1}
	reader := &countingReader{reader: db, tables: map[string]int{}}
	scope := view.NewScope(app, reader, beanctx.Request{})
	if _, err = scope.Resolve(ctx, "book_admin", "book-1"); err != nil {
		t.Fatal(err)
	}
	if _, err = beanmenu.DynamicTreeScoped(ctx, scope, "contents", "book-1"); err != nil {
		t.Fatal(err)
	}
	if reader.tables["book"] != 1 {
		t.Fatalf("Book owner lookups=%d, want 1 for composed request", reader.tables["book"])
	}
}
