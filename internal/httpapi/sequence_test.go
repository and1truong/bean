package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/render"
)

func TestSequenceRouteReturnsOrdinaryCompositionTree(t *testing.T) {
	runtime, err := bootstrap.Open(context.Background(), filepath.Join(t.TempDir(), "sequence.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	item := func(kind, name string, spec map[string]any) definition.Definition {
		return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
	}
	bundle := definition.Bundle{Name: "presentation", Definitions: []definition.Definition{
		item("Entity", "capability", map[string]any{"fields": []any{map[string]any{"name": "area", "type": "enum", "required": true, "options": []any{"application", "safety"}}}}),
		item("View", "capabilities_by_area", map[string]any{"entity": "capability", "fields": []any{"area"}, "groupBy": []any{map[string]any{"field": "area"}}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "capability_count"}}, "displays": map[string]any{"chart": map[string]any{"type": "block", "renderer": map[string]any{"type": "chart", "groupField": "area", "metricField": "capability_count"}, "pager": map[string]any{"type": "none", "pageSize": 20}}}}),
		item("Block", "opening", map[string]any{"type": "content", "content": []any{map[string]any{"type": "heading", "text": "Bean"}}}),
		item("Block", "capability_chart", map[string]any{"type": "view", "view": "capabilities_by_area", "display": "chart"}),
		item("Panel", "opening", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"opening", "capability_chart"}}}}),
		item("Sequence", "bean", map[string]any{"route": "/presentations/bean", "title": "Introducing Bean", "frames": []any{map[string]any{"name": "opening", "title": "Bean", "layout": "title", "panel": "opening"}}}),
	}}
	if err = runtime.Store.SaveBundle(context.Background(), "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(context.Background(), "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Kernel.Active()
	if _, err = runtime.HTTP.Actions.Execute(context.Background(), app, "capability_create", map[string]any{"area": "safety"}, beanctx.Request{User: &beanctx.User{ID: "admin", Roles: []string{"administrator"}}, RequestID: "sequence-test"}); err != nil {
		t.Fatal(err)
	}
	response := serve(t, runtime.HTTP.Handler(), http.MethodGet, "/api/system/page?path="+url.QueryEscape("/presentations/bean"), nil, nil, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var body struct{ Tree render.Node }
	decodeResponse(t, response, &body)
	if body.Tree.Component != "Sequence" || len(body.Tree.Children) != 1 || body.Tree.Children[0].Component != "SequenceFrame" || body.Tree.Children[0].Children[0].Children[0].Children[0].Component != "ContentBlock" {
		t.Fatalf("tree=%+v", body.Tree)
	}
	viewResponse := serve(t, runtime.HTTP.Handler(), http.MethodGet, "/api/views/capabilities_by_area?_page="+url.QueryEscape("/presentations/bean")+"&_block=capability_chart", nil, nil, "")
	if viewResponse.Code != http.StatusOK || !strings.Contains(viewResponse.Body.String(), `"area":"safety"`) || !strings.Contains(viewResponse.Body.String(), `"capability_count":1`) {
		t.Fatalf("status=%d body=%s", viewResponse.Code, viewResponse.Body.String())
	}
}
