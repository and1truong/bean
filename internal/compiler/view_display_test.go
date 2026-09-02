package compiler_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestFirstClassViewDisplayCompilesCanonicalContract(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "label": "Title", "type": "string", "required": true},
			map[string]any{"name": "status", "label": "Status", "type": "enum", "options": []any{"draft", "published"}},
			map[string]any{"name": "published_at", "label": "Published", "type": "datetime"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "status", "published_at"},
			"exposedFilters": map[string]any{
				"id":              map[string]any{"field": "id", "operator": "eq"},
				"status":          map[string]any{"field": "status", "operator": "eq"},
				"published_after": map[string]any{"field": "published_at", "operator": "gte"},
			},
			"displays": map[string]any{
				"index": map[string]any{
					"type": "page", "route": "/articles", "title": map[string]any{"text": "Articles"},
					"renderer": map[string]any{"type": "table", "fields": []any{
						map[string]any{"field": "title", "label": "Article", "linkRoute": "/articles/:id"},
						map[string]any{"field": "status", "label": "Status"},
					}},
					"controls": []any{map[string]any{"filter": "status", "label": "Publication status", "widget": "select"}},
					"pager":    map[string]any{"type": "cursor", "pageSize": 25},
				},
				"detail": map[string]any{
					"type": "page", "route": "/articles/:id",
					"bindings": map[string]any{"id": map[string]any{"source": "route", "name": "id", "required": true}},
					"title":    map[string]any{"field": "title", "fallback": "Article"},
					"renderer": map[string]any{"type": "detail", "titleField": "title"},
				},
				"recent": map[string]any{"type": "block", "renderer": map[string]any{"type": "list", "titleField": "title"}},
				"feed":   map[string]any{"type": "rss", "route": "/articles.rss"},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "recent_articles"}, Spec: map[string]any{"type": "view", "view": "articles", "display": "recent"}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	if result.App.FormatVersion != appir.CurrentFormat || appir.CurrentFormat != "bean/appir/v6" {
		t.Fatalf("format=%q", result.App.FormatVersion)
	}
	view := result.App.Views["articles"]
	if view.ExposedFilters["status"].Type != "enum" || len(view.ExposedFilters["status"].Options) != 2 {
		t.Fatalf("derived filter=%+v", view.ExposedFilters["status"])
	}
	if view.Displays["index"].Renderer.Type != "table" || view.Displays["index"].Pager.PageSize != 25 {
		t.Fatalf("display=%+v", view.Displays["index"])
	}
	if result.App.Blocks["recent_articles"].Display != "recent" {
		t.Fatalf("block=%+v", result.App.Blocks["recent_articles"])
	}
	capabilities := compiler.AgentCapabilities("test")
	if !reflect.DeepEqual(capabilities.ViewDisplayTypes, []string{"block", "csv", "json", "page", "rss"}) || !reflect.DeepEqual(capabilities.ViewFilterOperators, []string{"contains", "eq", "gte", "lte"}) || !reflect.DeepEqual(capabilities.ViewControlWidgets, []string{"auto", "checkbox", "date", "number", "select", "text"}) || !reflect.DeepEqual(capabilities.ViewPagers, []string{"cursor", "none"}) {
		t.Fatalf("capabilities=%+v", capabilities)
	}
}

func TestLegacyBlockPresentationNormalizesToPrivateViewDisplay(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{"entity": "article", "fields": []any{"id", "title"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "articles_list"}, Spec: map[string]any{"type": "view", "view": "articles", "presentation": map[string]any{"mode": "list", "titleField": "title"}}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	block := result.App.Blocks["articles_list"]
	display, exists := result.App.Views["articles"].Displays[block.Display]
	if !exists || block.Display != "_block_articles_list" || display.Type != "block" || display.Renderer.Type != "list" {
		t.Fatalf("block=%+v display=%+v", block, display)
	}
}

func TestFirstClassViewDisplayRejectsUnsafeContracts(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "redacted"}, Spec: map[string]any{"redact": []any{"status"}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string", "unique": true},
			map[string]any{"name": "status", "type": "enum", "options": []any{"open", "closed"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{
			"entity": "item", "fields": []any{"id", "title", "status"}, "policy": "redacted",
			"exposedFilters": map[string]any{"title": map[string]any{"field": "title"}, "status": map[string]any{"field": "status", "operator": "contains"}},
			"displays": map[string]any{
				"index": map[string]any{
					"type": "page", "route": "/items", "bindings": map[string]any{"title": map[string]any{"source": "route", "name": "title"}},
					"title": map[string]any{"field": "status", "fallback": "Item"},
					"renderer": map[string]any{"type": "table", "fields": []any{
						map[string]any{"field": "missing"}, map[string]any{"field": "status"}, map[string]any{"field": "title", "linkRoute": "javascript:alert(1)"},
					}},
					"controls": []any{map[string]any{"filter": "title", "widget": "checkbox"}, map[string]any{"filter": "missing", "widget": "future"}},
					"pager":    map[string]any{"type": "offset", "pageSize": 999},
				},
				"index_copy": map[string]any{"type": "page", "route": "/items", "renderer": map[string]any{"type": "list"}},
				"unknown":    map[string]any{"type": "screen"},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{"type": "view", "view": "items", "display": "missing"}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	paths := map[string]bool{}
	for _, diagnostic := range diagnostics {
		paths[diagnostic.Kind+"/"+diagnostic.Name+"/"+diagnostic.Path] = true
	}
	for _, path := range []string{
		"View/items/spec.exposedFilters.status.operator",
		"View/items/spec.displays.index.renderer.fields.0.field",
		"View/items/spec.displays.index.renderer.fields.1.field",
		"View/items/spec.displays.index.renderer.fields.2.linkRoute",
		"View/items/spec.displays.index.title.field",
		"View/items/spec.displays.index.controls.0.filter",
		"View/items/spec.displays.index.controls.0.widget",
		"View/items/spec.displays.index.controls.1.filter",
		"View/items/spec.displays.index.pager.type",
		"View/items/spec.displays.index.pager.pageSize",
		"View/items/spec.displays.index_copy.route",
		"View/items/spec.displays.unknown.type",
		"Block/items/spec.display",
	} {
		if !paths[path] {
			t.Errorf("missing %s: %v", path, diagnostics)
		}
	}
}
