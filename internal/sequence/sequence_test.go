package sequence_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/sequence"
)

func TestNodeBuildsDeterministicPolicyVisibleComposition(t *testing.T) {
	app := appir.Empty()
	app.Policies["managers"] = appir.Policy{Name: "managers", ReadRoles: []string{"manager"}}
	app.Blocks["public"] = appir.Block{Name: "public", Type: "content", Content: []appir.ContentElement{{Type: "heading", Text: "Bean"}}}
	app.Blocks["private"] = appir.Block{Name: "private", Type: "content", Content: []appir.ContentElement{{Type: "paragraph", Text: "Hidden"}}}
	app.Blocks["block_private"] = appir.Block{Name: "block_private", Type: "content", Policy: "managers", Content: []appir.ContentElement{{Type: "paragraph", Text: "Block hidden"}}}
	app.Panels["opening"] = appir.Panel{Name: "opening", Layout: "single-column", Regions: []appir.Region{{Name: "main", Blocks: []string{"public"}}}}
	app.Panels["internal"] = appir.Panel{Name: "internal", Layout: "single-column", Policy: "managers", Regions: []appir.Region{{Name: "main", Blocks: []string{"private"}}}}
	app.Panels["block_internal"] = appir.Panel{Name: "block_internal", Layout: "single-column", Regions: []appir.Region{{Name: "main", Blocks: []string{"block_private"}}}}
	item := appir.Sequence{Name: "bean", Route: "/presentations/bean", Title: "Bean", Profile: "presentation", AspectRatio: "wide", Frames: []appir.SequenceFrame{
		{Name: "opening", Title: "Bean", Layout: "title", Panel: "opening", Notes: "Public note"},
		{Name: "internal", Title: "Internal", Layout: "section", Panel: "internal", Notes: "Secret note"},
		{Name: "block_internal", Title: "Block internal", Layout: "section", Panel: "block_internal", Notes: "Block secret note"},
	}}
	app.Sequences[item.Name] = item

	first, allowed, err := sequence.Node(app, item, beanctx.Request{})
	second, _, _ := sequence.Node(app, item, beanctx.Request{})
	if err != nil || !allowed || !reflect.DeepEqual(first, second) {
		t.Fatalf("allowed=%v err=%v first=%+v second=%+v", allowed, err, first, second)
	}
	if first.Component != "Sequence" || len(first.Children) != 1 || first.Children[0].Props["name"] != "opening" {
		t.Fatalf("tree=%+v", first)
	}
	if first.Children[0].Children[0].Children[0].Children[0].Component != "ContentBlock" {
		t.Fatalf("content composition=%+v", first.Children[0])
	}

	manager := beanctx.Request{User: &beanctx.User{Roles: []string{"manager"}}}
	tree, allowed, err := sequence.Node(app, item, manager)
	if err != nil || !allowed || len(tree.Children) != 3 || !sequence.Protected(app, item) {
		t.Fatalf("manager tree=%+v allowed=%v err=%v", tree, allowed, err)
	}
}

func TestNodeIncludesInlineContentInSequenceFrameTree(t *testing.T) {
	app := appir.Empty()
	app.Blocks["named"] = appir.Block{Name: "named", Type: "text", Text: "Named"}
	app.Panels["mixed"] = appir.Panel{Name: "mixed", Layout: "single-column", Regions: []appir.Region{{Name: "main", Items: []appir.RegionItem{
		{Identity: "@inline/mixed/main/item/0", Content: []appir.ContentElement{{Type: "heading", Text: "Inline"}}},
		{Block: "named"},
	}}}}
	item := appir.Sequence{Name: "mixed", Title: "Mixed", Profile: "presentation", AspectRatio: "wide", Frames: []appir.SequenceFrame{{Name: "one", Title: "One", Layout: "bullets", Panel: "mixed"}}}
	node, allowed, err := sequence.Node(app, item, beanctx.Request{})
	if err != nil || !allowed {
		t.Fatalf("allowed=%v err=%v", allowed, err)
	}
	blocks := node.Children[0].Children[0].Children[0].Children
	if len(blocks) != 2 || blocks[0].Component != "ContentBlock" || blocks[1].Component != "TextBlock" {
		t.Fatalf("sequence blocks=%+v", blocks)
	}
}

func TestMatchAndRootPolicyFailClosed(t *testing.T) {
	app := appir.Empty()
	app.Policies["private"] = appir.Policy{Name: "private", Authenticated: true}
	app.Panels["opening"] = appir.Panel{Name: "opening", Layout: "single-column", Regions: []appir.Region{{Name: "main"}}}
	item := appir.Sequence{Name: "bean", Route: "/presentations/bean", Policy: "private", Frames: []appir.SequenceFrame{{Name: "opening", Panel: "opening"}}}
	app.Sequences[item.Name] = item
	matched, found := sequence.Match(app, "/presentations/bean")
	if !found || matched.Name != "bean" {
		t.Fatalf("matched=%+v found=%v", matched, found)
	}
	if _, allowed, err := sequence.Node(app, item, beanctx.Request{}); err != nil || allowed {
		t.Fatalf("protected Sequence allowed=%v err=%v", allowed, err)
	}
}
