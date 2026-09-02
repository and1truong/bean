package view

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestMatchPageDisplayPrefersStaticRouteAndBuildsRenderNode(t *testing.T) {
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "attachment", Type: "file"}, {Name: "body", Type: "richtext"}}}
	app.Views["articles"] = appir.View{
		Name: "articles", Entity: "article", Fields: []string{"id", "attachment", "body"},
		ExposedFilters: map[string]appir.ViewFilter{"id": {Field: "id"}},
		Displays: map[string]appir.Display{
			"detail":  {Type: "page", Route: "/articles/:id", Renderer: appir.ViewRenderer{Type: "detail"}},
			"archive": {Type: "page", Route: "/articles/archive", Renderer: appir.ViewRenderer{Type: "detail", RichTextFields: []string{"body"}}},
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
	formatted, ok := node.Children[0].Props["formattedFields"].([]string)
	if !ok || len(formatted) != 1 || formatted[0] != "body" {
		t.Fatalf("formattedFields=%v", node.Children[0].Props["formattedFields"])
	}
	match, found = MatchPageDisplay(app, "/articles/Hello%20world%2B")
	if !found || match.Name != "detail" || match.Params["id"] != "Hello world+" {
		t.Fatalf("encoded match=%+v found=%v", match, found)
	}
	if _, found = MatchPageDisplay(app, "/articles/%zz"); found {
		t.Fatal("invalid escaped route parameter matched")
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

func TestDisplayPageNodeDoesNotTrustStringAsRichText(t *testing.T) {
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "title", Type: "string"}}}
	app.Views["articles"] = appir.View{Name: "articles", Entity: "article", Fields: []string{"id", "title"}}
	match := DisplayMatch{View: "articles", Name: "detail", Display: appir.Display{Type: "page", Renderer: appir.ViewRenderer{Type: "detail", RichTextFields: []string{"title"}}}}
	formatted := DisplayPageNode(app, match).Children[0].Props["formattedFields"].([]string)
	if len(formatted) != 0 {
		t.Fatalf("formattedFields=%v", formatted)
	}
}
