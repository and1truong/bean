package page_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/page"
)

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
