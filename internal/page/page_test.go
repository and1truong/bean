package page_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/page"
)

func TestMatchPrefersStaticSegmentsDeterministically(t *testing.T) {
	app := appir.Empty()
	app.Pages["dynamic"] = appir.Page{Name: "dynamic", Route: "/posts/:slug"}
	app.Pages["static"] = appir.Page{Name: "static", Route: "/posts/new"}
	for attempt := 0; attempt < 100; attempt++ {
		matched, params, ok := page.Match(app, "/posts/new")
		if !ok || matched.Name != "static" || len(params) != 0 {
			t.Fatalf("attempt=%d matched=%+v params=%v ok=%v", attempt, matched, params, ok)
		}
	}
	matched, params, ok := page.Match(app, "/posts/Hello%20world%2B")
	if !ok || matched.Name != "dynamic" || params["slug"] != "Hello world+" {
		t.Fatalf("encoded match=%+v params=%v ok=%v", matched, params, ok)
	}
	if _, _, ok = page.Match(app, "/posts/%zz"); ok {
		t.Fatal("invalid escaped route parameter matched")
	}
}

func TestPageNodeExposesWhetherRouteMetadataIsProtected(t *testing.T) {
	app := appir.Empty()
	app.Policies["members"] = appir.Policy{Name: "members", Authenticated: true}
	app.Panels["public"] = appir.Panel{Name: "public"}
	app.Panels["members"] = appir.Panel{Name: "members", Policy: "members"}
	request := beanctx.Request{User: &beanctx.User{ID: "member"}}
	for _, definition := range []appir.Page{{Panel: "public"}, {Panel: "public", Policy: "members"}, {Panel: "members"}} {
		node, allowed, err := page.Node(app, definition, nil, request)
		if err != nil || !allowed {
			t.Fatalf("node=%+v allowed=%v err=%v", node, allowed, err)
		}
		expected := definition.Policy != "" || app.Panels[definition.Panel].Policy != ""
		if protected := node.Props["protected"]; protected != expected {
			t.Fatalf("protected=%v pagePolicy=%q panelPolicy=%q", protected, definition.Policy, app.Panels[definition.Panel].Policy)
		}
	}
}

func TestPageNodeRendersOrderedPolicyVisiblePanelSectionsAndAppliesFilters(t *testing.T) {
	app := appir.Empty()
	app.Policies["members"] = appir.Policy{Name: "members", Authenticated: true}
	app.Blocks["hero"] = appir.Block{Name: "hero", Type: "text", Text: "Hero"}
	app.Blocks["results"] = appir.Block{Name: "results", Type: "view", View: "results"}
	app.Blocks["private"] = appir.Block{Name: "private", Type: "text", Text: "Private"}
	app.Panels["hero"] = appir.Panel{Name: "hero", Layout: "single-column", Regions: []appir.Region{{Name: "main", Blocks: []string{"hero"}}}}
	app.Panels["body"] = appir.Panel{Name: "body", Layout: "grid", Regions: []appir.Region{{Name: "main", Blocks: []string{"results"}}}}
	app.Panels["private"] = appir.Panel{Name: "private", Layout: "two-column", Policy: "members", Regions: []appir.Region{{Name: "left", Blocks: []string{"private"}}}}
	definition := appir.Page{Sections: []appir.PageSection{{ID: "introduction", Panel: "hero", Identity: "@section/home/introduction"}, {Panel: "body", Identity: "@section/home/1"}, {Panel: "private", Identity: "@section/home/2"}}, Filters: map[string]appir.PageFilter{"status": {Targets: []appir.PageFilterTarget{{Block: "results", Filter: "state"}}}}}

	node, allowed, err := page.Node(app, definition, nil, beanctx.Request{})
	if err != nil || !allowed || len(node.Children) != 2 || node.Children[0].Props["layout"] != "single-column" || node.Children[0].Props["pageSection"] != "@section/home/introduction" || node.Children[1].Props["layout"] != "grid" || node.Children[1].Props["pageSection"] != "@section/home/1" {
		t.Fatalf("anonymous node=%+v allowed=%v err=%v", node, allowed, err)
	}
	result := node.Children[1].Children[0].Children[0]
	if result.Props["pageFilters"].(map[string]string)["state"] != "status" || node.Props["protected"] != true {
		t.Fatalf("filtered node=%+v", node)
	}

	member := beanctx.Request{User: &beanctx.User{ID: "member"}}
	node, allowed, err = page.Node(app, definition, nil, member)
	if err != nil || !allowed || len(node.Children) != 3 || node.Children[2].Props["layout"] != "two-column" {
		t.Fatalf("member node=%+v allowed=%v err=%v", node, allowed, err)
	}

	definition.Sections = []appir.PageSection{{Panel: "private", Identity: "@section/home/private"}}
	if _, allowed, err = page.Node(app, definition, nil, beanctx.Request{}); err != nil || allowed {
		t.Fatalf("fully denied Page allowed=%v err=%v", allowed, err)
	}
}

func TestTypedContextResolution(t *testing.T) {
	definition := appir.Page{Context: map[string]appir.ContextBinding{"record": {Source: "route", Name: "id", Required: true}, "tenant": {Source: "tenant", Required: true}}}
	context, e := page.ResolveContext(definition, map[string]string{"id": "record-1"}, nil, beanctx.Request{TenantID: "tenant-1"})
	if e != nil || context["record"] != "record-1" || context["tenant"] != "tenant-1" {
		t.Fatalf("context=%v err=%v", context, e)
	}
	if _, e = page.ResolveContext(definition, nil, nil, beanctx.Request{}); e == nil {
		t.Fatal("missing required context accepted")
	}
}
