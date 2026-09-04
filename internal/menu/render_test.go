package menu_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	beanmenu "github.com/beanruntime/bean/internal/menu"
)

func TestStaticTreeResolvesTypedTargetsOrdersSiblingsAndMarksActiveTrail(t *testing.T) {
	app := appir.Empty()
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Title: "Home"}
	app.Pages["activity"] = appir.Page{Name: "activity", Route: "/activity", Title: "Activity"}
	app.Views["records"] = appir.View{Name: "records", Entity: "record", Displays: map[string]appir.Display{"list": {Type: "page", Route: "/records", Title: appir.DisplayTitle{Text: "Records"}}}}
	app.Entities["record"] = appir.Entity{Name: "record"}
	definition := appir.Menu{Name: "main", Profile: "workspace", MaxDepth: 3, Items: []appir.MenuItem{
		{ID: "activity", Weight: 20, Target: appir.MenuTarget{Page: "activity"}},
		{ID: "home", Weight: 0, Target: appir.MenuTarget{Page: "home"}},
		{ID: "records", Parent: "home", Weight: 10, Target: appir.MenuTarget{View: "records", Display: "list"}},
	}}
	tree := beanmenu.StaticTree(app, definition, beanctx.Request{Route: "/records"})
	if len(tree) != 2 || tree[0].ID != "home" || !tree[0].Active || tree[0].Current || len(tree[0].Children) != 1 || !tree[0].Children[0].Current || tree[0].Children[0].Label != "Records" || tree[1].ID != "activity" {
		t.Fatalf("tree=%+v", tree)
	}
}

func TestStaticTreePreservesLegacyFlatMenuOrder(t *testing.T) {
	app := appir.Empty()
	definition := appir.Menu{Name: "legacy", Items: []appir.MenuItem{{Label: "Second", Route: "/second"}, {Label: "First", Route: "/first"}}}
	tree := beanmenu.StaticTree(app, definition, beanctx.Request{Route: "/first"})
	if len(tree) != 2 || tree[0].Label != "Second" || tree[1].Label != "First" || !tree[1].Current {
		t.Fatalf("tree=%+v", tree)
	}
}
