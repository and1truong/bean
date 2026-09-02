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
	if !reflect.DeepEqual(capabilities.ViewDisplayTypes, []string{"block", "csv", "json", "page", "rss"}) || !reflect.DeepEqual(capabilities.ViewRenderers, []string{"board", "detail", "list", "metric", "table", "timeline", "tree"}) || !reflect.DeepEqual(capabilities.ViewFilterOperators, []string{"contains", "eq", "gte", "lte"}) || !reflect.DeepEqual(capabilities.ViewControlWidgets, []string{"auto", "checkbox", "date", "number", "select", "text"}) || !reflect.DeepEqual(capabilities.ViewPagers, []string{"cursor", "none"}) {
		t.Fatalf("capabilities=%+v", capabilities)
	}
	if contains(capabilities.Presentations, "table") {
		t.Fatalf("legacy presentations=%v", capabilities.Presentations)
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
	definitions[2].Spec = map[string]any{"type": "view", "view": "articles", "presentation": map[string]any{"mode": "table"}}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "Block", "articles_list", "spec.presentation.mode") {
		t.Fatalf("legacy table diagnostics=%v", diagnostics)
	}
}

func TestViewDisplayRejectsOverlappingRoutes(t *testing.T) {
	entity := definition.Definition{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}}
	t.Run("between displays", func(t *testing.T) {
		view := definition.Definition{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title"}, "displays": map[string]any{
				"by_id":   map[string]any{"type": "page", "route": "/articles/:id", "renderer": map[string]any{"type": "list"}},
				"by_slug": map[string]any{"type": "page", "route": "/articles/:slug", "renderer": map[string]any{"type": "list"}},
			},
		}}
		diagnostics := compiler.Compile("test", 1, []definition.Definition{entity, view}).Diagnostics
		if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.by_slug.route") {
			t.Fatalf("overlapping display diagnostics=%v", diagnostics)
		}
	})
	t.Run("with an existing Page", func(t *testing.T) {
		view := definition.Definition{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title"}, "displays": map[string]any{
				"new": map[string]any{"type": "page", "route": "/articles/new", "renderer": map[string]any{"type": "list"}},
			},
		}}
		panel := definition.Definition{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"layout": "single-column", "regions": []any{}}}
		page := definition.Definition{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"route": "/articles/:id", "panel": "article"}}
		diagnostics := compiler.Compile("test", 1, []definition.Definition{entity, view, panel, page}).Diagnostics
		if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.new.route") {
			t.Fatalf("shadowed display diagnostics=%v", diagnostics)
		}
	})
}

func TestViewExposedFiltersDeriveScopedSystemFieldTypes(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "record"}, Spec: map[string]any{"owner": true, "tenant": true, "softDelete": true, "fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "records"}, Spec: map[string]any{
			"entity": "record", "fields": []any{"id", "owner_id", "tenant_id", "deleted_at"}, "exposedFilters": map[string]any{
				"owner": map[string]any{"field": "owner_id"}, "tenant": map[string]any{"field": "tenant_id"}, "deleted": map[string]any{"field": "deleted_at"},
			},
		}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	filters := result.App.Views["records"].ExposedFilters
	if filters["owner"].Type != "uuid" || filters["tenant"].Type != "uuid" || filters["deleted"].Type != "datetime" {
		t.Fatalf("filters=%+v", filters)
	}
}

func TestViewExposedFilterDerivesTypeThroughShorthandRelationship(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "category"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "category_id", "type": "relation", "relation": map[string]any{"entity": "category", "kind": "many-to-one", "targetField": "id"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "category.name"},
			"relationships":  []any{map[string]any{"name": "category", "relationField": "category_id"}},
			"exposedFilters": map[string]any{"category_name": map[string]any{"field": "category.name", "operator": "contains"}},
		}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	filter := result.App.Views["articles"].ExposedFilters["category_name"]
	if filter.Type != "string" || filter.Field != "category.name" {
		t.Fatalf("filter=%+v", filter)
	}
}

func TestResultTitleRequiresMandatoryUniqueBinding(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "slug", "type": "string"}}, "unique": []any{[]any{"slug"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "slug"}, "exposedFilters": map[string]any{"slug": map[string]any{"field": "slug"}},
			"displays": map[string]any{"detail": map[string]any{
				"type": "page", "route": "/articles/:slug", "bindings": map[string]any{"slug": map[string]any{"source": "route", "name": "slug"}},
				"title": map[string]any{"field": "title", "fallback": "Article"}, "renderer": map[string]any{"type": "detail", "titleField": "title"},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.detail.title.field") {
		t.Fatalf("optional unique binding diagnostics=%v", diagnostics)
	}
	definitions[1].Spec["displays"].(map[string]any)["detail"].(map[string]any)["bindings"].(map[string]any)["slug"].(map[string]any)["required"] = true
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("mandatory unique binding diagnostics=%v", diagnostics)
	}
	definitions[1].Spec["exposedFilters"].(map[string]any)["slug"].(map[string]any)["operator"] = "contains"
	diagnostics = compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.detail.title.field") {
		t.Fatalf("non-equality unique binding diagnostics=%v", diagnostics)
	}
	definitions[0].Spec["unique"] = []any{[]any{"slug", "title"}}
	definitions[1].Spec["exposedFilters"].(map[string]any)["slug"].(map[string]any)["operator"] = "eq"
	diagnostics = compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.detail.title.field") {
		t.Fatalf("partially bound composite unique diagnostics=%v", diagnostics)
	}
	definitions[1].Spec["exposedFilters"].(map[string]any)["title"] = map[string]any{"field": "title"}
	display := definitions[1].Spec["displays"].(map[string]any)["detail"].(map[string]any)
	display["route"] = "/articles/:slug/:title"
	display["bindings"].(map[string]any)["title"] = map[string]any{"source": "route", "name": "title", "required": true}
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("fully bound composite unique diagnostics=%v", diagnostics)
	}
}

func TestResultTitleBlockRequiresMandatoryInput(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "slug", "type": "string", "unique": true}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "slug"}, "exposedFilters": map[string]any{"slug": map[string]any{"field": "slug"}},
			"displays": map[string]any{"detail": map[string]any{
				"type": "block", "title": map[string]any{"field": "title", "fallback": "Article"}, "renderer": map[string]any{"type": "detail", "titleField": "title"},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "article_detail"}, Spec: map[string]any{
			"type": "view", "view": "articles", "display": "detail", "inputs": map[string]any{"slug": map[string]any{"type": "string"}},
			"bindings": map[string]any{"slug": map[string]any{"source": "context", "name": "slug"}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "Block", "article_detail", "spec.display") {
		t.Fatalf("optional Block input diagnostics=%v", diagnostics)
	}
	definitions[2].Spec["inputs"].(map[string]any)["slug"].(map[string]any)["required"] = true
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("mandatory Block input diagnostics=%v", diagnostics)
	}
}

func TestResultTitleRejectsRowMultiplyingRelationshipField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "tag"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "tags", "type": "relation", "relation": map[string]any{"entity": "tag", "kind": "many-to-many", "targetField": "id"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "tags.name"},
			"relationships":  []any{map[string]any{"name": "tags", "relationField": "tags"}},
			"exposedFilters": map[string]any{"id": map[string]any{"field": "id"}},
			"displays": map[string]any{"detail": map[string]any{
				"type": "page", "route": "/articles/:id", "bindings": map[string]any{"id": map[string]any{"source": "route", "name": "id", "required": true}},
				"title": map[string]any{"field": "tags.name", "fallback": "Article"}, "renderer": map[string]any{"type": "detail", "titleField": "title"},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays.detail.title.field") {
		t.Fatalf("row-multiplying title diagnostics=%v", diagnostics)
	}
}

func TestUserAuthoredPrivateViewDisplayPrefixIsRejected(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title"}, "displays": map[string]any{
				"_unvalidated": map[string]any{"type": "page", "route": "/hidden", "renderer": map[string]any{"type": "future"}},
			},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "articles", "spec.displays._unvalidated") {
		t.Fatalf("diagnostics=%v", diagnostics)
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
				"index_copy":     map[string]any{"type": "page", "route": "/items", "renderer": map[string]any{"type": "list"}},
				"unknown":        map[string]any{"type": "screen"},
				"missing_feed":   map[string]any{"type": "rss"},
				"relative_api":   map[string]any{"type": "json", "route": "items.json"},
				"dynamic_csv":    map[string]any{"type": "csv", "route": "/items/:id.csv"},
				"query_api":      map[string]any{"type": "json", "route": "/healthz?format=json"},
				"fragment_feed":  map[string]any{"type": "rss", "route": "/items.xml#latest"},
				"builtin_api":    map[string]any{"type": "json", "route": "/api/system/page"},
				"builtin_health": map[string]any{"type": "csv", "route": "/healthz"},
				"unsafe_rich": map[string]any{
					"type": "block", "renderer": map[string]any{"type": "detail", "titleField": "title", "richTextFields": []any{"title"}},
				},
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
		"View/items/spec.displays.missing_feed.route",
		"View/items/spec.displays.relative_api.route",
		"View/items/spec.displays.dynamic_csv.route",
		"View/items/spec.displays.query_api.route",
		"View/items/spec.displays.fragment_feed.route",
		"View/items/spec.displays.builtin_api.route",
		"View/items/spec.displays.builtin_health.route",
		"View/items/spec.displays.unsafe_rich.renderer.richTextFields.0",
		"Block/items/spec.display",
	} {
		if !paths[path] {
			t.Errorf("missing %s: %v", path, diagnostics)
		}
	}
}

func hasViewDisplayDiagnostic(diagnostics []definition.Diagnostic, kind, name, path string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind && diagnostic.Name == name && diagnostic.Path == path {
			return true
		}
	}
	return false
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
