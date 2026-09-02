package view

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestMatchPageDisplayPrefersStaticRouteAndBuildsRenderNode(t *testing.T) {
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "attachment", Type: "file"}}}
	app.Views["articles"] = appir.View{
		Name: "articles", Entity: "article", Fields: []string{"id", "attachment"},
		ExposedFilters: map[string]appir.ViewFilter{"id": {Field: "id"}},
		Displays: map[string]appir.Display{
			"detail":  {Type: "page", Route: "/articles/:id", Renderer: appir.ViewRenderer{Type: "detail"}},
			"archive": {Type: "page", Route: "/articles/archive", Renderer: appir.ViewRenderer{Type: "table"}},
		},
	}
	match, found := MatchPageDisplay(app, "/articles/archive")
	if !found || match.Name != "archive" || len(match.Params) != 0 {
		t.Fatalf("match=%+v found=%v", match, found)
	}
	node := DisplayPageNode(app, match)
	if len(node.Children) != 1 || node.Children[0].Component != "ViewBlock" || node.Children[0].Props["display"] == nil {
		t.Fatalf("node=%+v", node)
	}
}

func TestResolveDisplayBindingsUsesTrustedRouteValues(t *testing.T) {
	display := appir.Display{Bindings: map[string]appir.ContextBinding{
		"id": {Source: "route", Name: "article", Required: true},
	}}
	bound, err := ResolveDisplayBindings(display, map[string]string{"article": "trusted"}, map[string]string{"id": "forged"}, beanctx.Request{})
	if err != nil || bound["id"] != "trusted" {
		t.Fatalf("bound=%v err=%v", bound, err)
	}
	if _, err = ResolveDisplayBindings(display, nil, nil, beanctx.Request{}); err == nil {
		t.Fatal("missing required route binding was accepted")
	}
}

func TestFieldTypesIncludesEnabledSyntheticFields(t *testing.T) {
	app := appir.Empty()
	app.Entities["record"] = appir.Entity{Name: "record", Owner: true, Tenant: true, SoftDelete: true}
	types := FieldTypes(app, appir.View{Entity: "record", Fields: []string{"owner_id", "tenant_id", "deleted_at"}})
	if types["owner_id"] != "uuid" || types["tenant_id"] != "uuid" || types["deleted_at"] != "datetime" {
		t.Fatalf("types=%v", types)
	}
}
