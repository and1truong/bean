package block_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestEveryAcceptedBlockTypeHasRenderer(t *testing.T) {
	app := appir.Empty()
	app.Menus["main"] = appir.Menu{Name: "main", Items: []appir.MenuItem{{Label: "Home", Route: "/"}}}
	tests := map[string]string{"text": "TextBlock", "view": "ViewBlock", "entity": "EntityBlock", "webform": "WebformBlock", "action": "ActionBlock", "menu": "MenuBlock"}
	for kind, component := range tests {
		node, allowed, e := block.Node(app, appir.Block{Name: kind, Type: kind, Text: "text", View: "view", Entity: "entity", Webform: "form", Action: "action", Menu: "main"}, map[string]any{}, beanctx.Request{})
		if e != nil || !allowed || node.Component != component {
			t.Fatalf("type=%s node=%+v allowed=%v err=%v", kind, node, allowed, e)
		}
	}
}

func TestRequiredTypedInputFailsSafely(t *testing.T) {
	app := appir.Empty()
	definition := appir.Block{Name: "record", Type: "text", Inputs: map[string]appir.Field{"id": {Name: "id", Type: "uuid", Required: true}}, Bindings: map[string]appir.ContextBinding{"id": {Source: "context", Name: "id"}}}
	if _, _, e := block.Node(app, definition, map[string]any{}, beanctx.Request{}); e == nil {
		t.Fatal("missing required input accepted")
	}
}
