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
