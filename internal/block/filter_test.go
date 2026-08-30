package block_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestViewBlockOnlyTrustsCompiledOrSanitizedFields(t *testing.T) {
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "body", Type: "text"}, {Name: "legacy", Type: "richtext"}}}
	app.Views["article"] = appir.View{Name: "article", Entity: "article", Fields: []string{"id", "body", "legacy"}, FieldFilters: map[string]string{"body": "markdown"}}
	definition := appir.Block{Name: "article", Type: "view", View: "article", Presentation: appir.ViewPresentation{RichTextFields: []string{"body", "legacy"}}}
	node, allowed, err := block.Node(app, definition, map[string]any{}, beanctx.Request{})
	if err != nil || !allowed {
		t.Fatalf("node=%+v allowed=%v err=%v", node, allowed, err)
	}
	if fields := node.Props["formattedFields"]; !reflect.DeepEqual(fields, []string{"body", "legacy"}) {
		t.Fatalf("formatted fields=%v", fields)
	}

	app.Views["article"] = appir.View{Name: "article", Entity: "article", Fields: []string{"id", "body"}}
	definition.Presentation.RichTextFields = []string{"body"}
	node, _, _ = block.Node(app, definition, map[string]any{}, beanctx.Request{})
	if fields := node.Props["formattedFields"]; !reflect.DeepEqual(fields, []string{}) {
		t.Fatalf("unsanitized text trusted as HTML: %v", fields)
	}
}
