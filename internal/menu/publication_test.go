package menu_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	"github.com/beanruntime/bean/internal/migration"
)

func TestPublicationRejectsOrphaningDynamicPlacementContracts(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "publication.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	current := appir.Empty()
	current.Entities["book"] = appir.Entity{Name: "book"}
	current.Entities["page"] = appir.Entity{Name: "page", Navigation: &appir.EntityNavigation{LabelField: "title", Destination: appir.NavigationDestination{View: "pages", Display: "detail"}, Menus: []string{"contents"}}}
	current.Menus["contents"] = appir.Menu{Name: "contents", Profile: "workspace", MaxDepth: 3, Owner: &appir.MenuOwner{Entity: "book"}}
	next, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Insert(ctx, dbal.Insert{Table: beanmenu.PlacementTable, Values: map[string]dbal.Value{
		"id": "placement", "menu_name": "contents", "owner_entity": "book", "owner_id": "book-1", "target_entity": "page", "target_id": "page-1", "weight": 0,
		"created_at": "2026-01-01T00:00:00Z", "updated_at": "2026-01-01T00:00:00Z",
	}}); err != nil {
		t.Fatal(err)
	}
	delete(next.Menus, "contents")
	if err = beanmenu.ValidatePublication(ctx, db, current, next); err == nil || !strings.Contains(err.Error(), "Menu contents") {
		t.Fatalf("Menu removal err=%v", err)
	}
	next, _ = current.Clone()
	page := next.Entities["page"]
	page.Navigation = nil
	next.Entities["page"] = page
	if err = beanmenu.ValidatePublication(ctx, db, current, next); err == nil || !strings.Contains(err.Error(), "Entity page") {
		t.Fatalf("Entity navigation removal err=%v", err)
	}
	if _, err = db.Delete(ctx, dbal.Delete{Table: beanmenu.PlacementTable, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: "placement"}, ExpectedRows: 1}); err != nil {
		t.Fatal(err)
	}
	if err = beanmenu.ValidatePublication(ctx, db, current, next); err != nil {
		t.Fatalf("clean publication rejected: %v", err)
	}
}
