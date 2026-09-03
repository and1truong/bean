package panel_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/panel"
)

func TestNodeRendersInlineAndNamedBlocksInDeclaredOrder(t *testing.T) {
	app := appir.Empty()
	app.Blocks["chart"] = appir.Block{Name: "chart", Type: "text", Text: "chart"}
	definition := appir.Panel{Name: "frame", Layout: "single-column", Regions: []appir.Region{{Name: "main", Items: []appir.RegionItem{
		{ID: "intro", Identity: "@inline/frame/main/id/intro", Content: []appir.ContentElement{{Type: "paragraph", Text: "Before"}}},
		{Block: "chart"},
		{Identity: "@inline/frame/main/item/2", Content: []appir.ContentElement{{Type: "paragraph", Text: "After"}}},
	}}}}

	node, allowed, err := panel.Node(app, definition, map[string]any{}, beanctx.Request{})
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	children := node.Children[0].Children
	if len(children) != 3 || children[0].Component != "ContentBlock" || children[1].Component != "TextBlock" || children[2].Component != "ContentBlock" {
		t.Fatalf("children=%+v", children)
	}
	if children[0].Props["name"] != "@inline/frame/main/id/intro" || children[1].Props["name"] != "chart" || children[2].Props["name"] != "@inline/frame/main/item/2" {
		t.Fatalf("ordered identities=%+v", children)
	}
}

func TestInlineContentInheritsPanelPolicyWhileNamedBlockPolicyRemainsIndependent(t *testing.T) {
	app := appir.Empty()
	app.Policies["members"] = appir.Policy{Name: "members", Authenticated: true}
	app.Blocks["restricted"] = appir.Block{Name: "restricted", Type: "text", Text: "restricted", Policy: "members"}
	inline := appir.RegionItem{Identity: "@inline/frame/main/item/0", Content: []appir.ContentElement{{Type: "paragraph", Text: "Visible with panel"}}}
	definition := appir.Panel{Name: "frame", Layout: "single-column", Regions: []appir.Region{{Name: "main", Items: []appir.RegionItem{inline, appir.RegionItem{Block: "restricted"}}}}}

	node, allowed, err := panel.Node(app, definition, nil, beanctx.Request{})
	if err != nil || !allowed || len(node.Children[0].Children) != 1 || node.Children[0].Children[0].Component != "ContentBlock" {
		t.Fatalf("anonymous tree=%+v allowed=%v err=%v", node, allowed, err)
	}
	definition.Policy = "members"
	if _, allowed, err = panel.Node(app, definition, nil, beanctx.Request{}); err != nil || allowed {
		t.Fatalf("Panel policy did not hide inline content: allowed=%v err=%v", allowed, err)
	}
}
