package block_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/expr"
)

func TestEveryAcceptedBlockTypeHasRenderer(t *testing.T) {
	app := appir.Empty()
	app.Menus["main"] = appir.Menu{Name: "main", Items: []appir.MenuItem{{Label: "Home", Route: "/"}}}
	tests := map[string]string{"text": "TextBlock", "view": "ViewBlock", "entity": "EntityBlock", "webform": "WebformBlock", "action": "ActionBlock", "menu": "MenuBlock", "resource-list": "ResourceListBlock"}
	wantNames := []string{"action", "entity", "menu", "resource-list", "text", "view", "webform"}
	if got := block.Names(); !reflect.DeepEqual(got, wantNames) {
		t.Fatalf("registered Block types=%v want=%v", got, wantNames)
	}
	for kind, component := range tests {
		specification, registered := block.Lookup(kind)
		if !registered || specification.Component != component {
			t.Fatalf("type=%s specification=%+v registered=%v", kind, specification, registered)
		}
		node, allowed, e := block.Node(app, appir.Block{Name: kind, Type: kind, Text: "text", View: "view", Entity: "entity", Webform: "form", Action: "action", Menu: "main"}, map[string]any{}, beanctx.Request{})
		if e != nil || !allowed || node.Component != component {
			t.Fatalf("type=%s node=%+v allowed=%v err=%v", kind, node, allowed, e)
		}
	}
}

func TestWebformConditionsBindServerContextForBrowserEvaluation(t *testing.T) {
	app := appir.Empty()
	app.Webforms["form"] = appir.Webform{Name: "form", Elements: []appir.FormElement{{Name: "detail", Visible: &expr.Expr{Op: "eq", Left: &expr.Value{Source: "tenant"}, Right: &expr.Value{Source: "input", Name: "tenant"}}}}}
	node, allowed, err := block.Node(app, appir.Block{Name: "form_block", Type: "webform", Webform: "form"}, nil, beanctx.Request{TenantID: "tenant-1"})
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	form := node.Props["form"].(appir.Webform)
	condition := form.Elements[0].Visible
	if condition.Left.Source != "literal" || condition.Left.Literal != "tenant-1" || condition.Right.Source != "input" {
		t.Fatalf("condition was not safely bound: %+v", condition)
	}
}

func TestRequiredTypedInputFailsSafely(t *testing.T) {
	app := appir.Empty()
	definition := appir.Block{Name: "record", Type: "text", Inputs: map[string]appir.Field{"id": {Name: "id", Type: "uuid", Required: true}}, Bindings: map[string]appir.ContextBinding{"id": {Source: "context", Name: "id"}}}
	if _, _, e := block.Node(app, definition, map[string]any{}, beanctx.Request{}); e == nil {
		t.Fatal("missing required input accepted")
	}
}

func TestViewOwnedSearchIsAvailableOnEverySwitchableDisplay(t *testing.T) {
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "title", Type: "string"}}}
	app.Views["articles"] = appir.View{Name: "articles", Entity: "article", Search: appir.ViewSearch{Fields: []string{"title"}}, Displays: map[string]appir.Display{
		"list":  {Type: "block", Renderer: appir.ViewRenderer{Type: "list"}},
		"cards": {Type: "block", Renderer: appir.ViewRenderer{Type: "cards"}},
	}}
	node, allowed, err := block.Node(app, appir.Block{Name: "articles", Type: "view", View: "articles", Display: "list"}, nil, beanctx.Request{})
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	for name, display := range node.Props["displays"].(map[string]appir.Display) {
		if !reflect.DeepEqual(display.Renderer.SearchFields, []string{"title"}) {
			t.Fatalf("display %s search fields=%v", name, display.Renderer.SearchFields)
		}
	}
}
