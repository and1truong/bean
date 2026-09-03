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
	if result.App.FormatVersion != appir.CurrentFormat || appir.CurrentFormat != "bean/appir/v11" {
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
	if !reflect.DeepEqual(capabilities.ViewDisplayTypes, []string{"block", "csv", "json", "page", "rss"}) || !reflect.DeepEqual(capabilities.ViewRenderers, []string{"board", "calendar", "cards", "chart", "detail", "list", "metric", "table", "timeline", "tree"}) || !reflect.DeepEqual(capabilities.ViewFilterOperators, []string{"contains", "eq", "gte", "lte"}) || !reflect.DeepEqual(capabilities.ViewControlWidgets, []string{"auto", "checkbox", "date", "number", "select", "text"}) || !reflect.DeepEqual(capabilities.ViewPagers, []string{"cursor", "none"}) {
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

func TestCalendarDisplayAllowsStartWithoutEnd(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "event"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "starts_at", "type": "datetime"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "events"}, Spec: map[string]any{
			"entity": "event", "fields": []any{"id", "title", "starts_at"},
			"displays": map[string]any{"calendar": map[string]any{
				"type": "block", "renderer": map[string]any{"type": "calendar", "titleField": "title", "timeField": "starts_at"},
			}},
		}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
}

func TestBoardDisplayRejectsUnsupportedSelectionActions(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "issue"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "status", "type": "enum", "options": []any{"todo", "done"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_issue"}, Spec: map[string]any{
			"entity": "issue", "operation": "transition", "transitions": map[string]any{"todo": []any{"done"}},
		}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "issues"}, Spec: map[string]any{
			"entity": "issue", "fields": []any{"id", "title", "status"},
			"displays": map[string]any{"board": map[string]any{
				"type": "block", "selection": "multiple", "actions": []any{"move_issue"},
				"renderer": map[string]any{"type": "board", "titleField": "title", "groupField": "status", "columns": []any{"todo", "done"}, "moveAction": "move_issue"},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasViewDisplayDiagnostic(diagnostics, "View", "issues", "spec.displays.board.actions") {
		t.Fatalf("board actions diagnostics=%v", diagnostics)
	}
}

func TestViewDisplayRejectsBucketedGroupEqualityDrill(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "event"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "occurred_on", "type": "date"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "event_records"}, Spec: map[string]any{
			"entity": "event", "fields": []any{"id", "title", "occurred_on"},
			"exposedFilters": map[string]any{"occurred_on": map[string]any{"field": "occurred_on", "operator": "eq"}},
			"displays": map[string]any{"page": map[string]any{
				"type": "page", "route": "/events", "renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title"}}},
			}},
		}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "events_by_month"}, Spec: map[string]any{
			"entity": "event", "fields": []any{"occurred_on"},
			"groupBy":    []any{map[string]any{"field": "occurred_on", "as": "month", "bucket": "month"}},
			"aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "event_count"}},
			"displays": map[string]any{"chart": map[string]any{
				"type": "block", "renderer": map[string]any{"type": "chart", "groupField": "month", "metricField": "event_count"},
				"drill": map[string]any{"view": "event_records", "display": "page", "bindings": []any{map[string]any{"source": "group", "name": "month", "filter": "occurred_on"}}},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_month", "spec.displays.chart.drill.bindings.0.name", "bucketed groups cannot bind equality drills") {
		t.Fatalf("bucketed drill diagnostics=%v", diagnostics)
	}
}

func TestChartDisplayValidatesSearchFieldsAndGroupAxis(t *testing.T) {
	definitions := drillDefinitions("eq", "eq", "/events")
	entityFields := definitions[0].Spec["fields"].([]any)
	definitions[0].Spec["fields"] = append(entityFields, map[string]any{"name": "category", "type": "string"})
	source := definitions[2].Spec
	source["fields"] = []any{"status", "category"}
	source["groupBy"] = []any{"status", "category"}
	renderer := source["displays"].(map[string]any)["chart"].(map[string]any)["renderer"].(map[string]any)
	renderer["searchFields"] = []any{"event_count"}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.renderer.groupField", "chart requires exactly one grouped output field") {
		t.Fatalf("multiple chart groups accepted: %v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.renderer.searchFields.0", "must reference a selected View source field") {
		t.Fatalf("aggregate alias chart search accepted: %v", diagnostics)
	}

	definitions = drillDefinitions("eq", "eq", "/events")
	source = definitions[2].Spec
	source["groupBy"] = []any{map[string]any{"field": "status", "as": "category"}}
	display := source["displays"].(map[string]any)["chart"].(map[string]any)
	delete(display, "drill")
	renderer = display["renderer"].(map[string]any)
	renderer["groupField"] = "category"
	renderer["searchFields"] = []any{"status"}
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("aliased group source search rejected: %v", diagnostics)
	}
}

func TestViewDisplayRejectsParameterizedDrillTarget(t *testing.T) {
	definitions := drillDefinitions("eq", "eq", "/events/:status")
	targetDisplay := definitions[1].Spec["displays"].(map[string]any)["page"].(map[string]any)
	targetDisplay["bindings"] = map[string]any{"status": map[string]any{"source": "route", "name": "status", "required": true}}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.drill.display", "must reference a page Display with a static route") {
		t.Fatalf("parameterized drill diagnostics=%v", diagnostics)
	}
}

func TestViewDisplayRequiresDrillTargetControl(t *testing.T) {
	definitions := drillDefinitions("eq", "eq", "/events")
	targetDisplay := definitions[1].Spec["displays"].(map[string]any)["page"].(map[string]any)
	delete(targetDisplay, "controls")
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.drill.bindings.0.filter", "must reference an unbound target Display control") {
		t.Fatalf("unconsumed drill diagnostics=%v", diagnostics)
	}
}

func TestViewDisplayRejectsDrillOperatorMismatch(t *testing.T) {
	t.Run("group requires equality", func(t *testing.T) {
		diagnostics := compiler.Compile("test", 1, drillDefinitions("eq", "contains", "/events")).Diagnostics
		if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.drill.bindings.0", "source and target filter operators must match") {
			t.Fatalf("group operator diagnostics=%v", diagnostics)
		}
	})
	t.Run("filters preserve their operator", func(t *testing.T) {
		definitions := drillDefinitions("contains", "eq", "/events")
		source := definitions[2].Spec
		source["exposedFilters"] = map[string]any{"status": map[string]any{"field": "status", "operator": "contains"}}
		drill := source["displays"].(map[string]any)["chart"].(map[string]any)["drill"].(map[string]any)
		drill["bindings"] = []any{map[string]any{"source": "filter", "name": "status", "filter": "status"}}
		diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
		if !hasDiagnosticMessage(diagnostics, "View", "events_by_status", "spec.displays.chart.drill.bindings.0", "source and target filter operators must match") {
			t.Fatalf("filter operator diagnostics=%v", diagnostics)
		}
	})
}

func drillDefinitions(sourceOperator, targetOperator, targetRoute string) []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "event"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "event_records"}, Spec: map[string]any{
			"entity": "event", "fields": []any{"id", "status"}, "exposedFilters": map[string]any{"status": map[string]any{"field": "status", "operator": targetOperator}},
			"displays": map[string]any{"page": map[string]any{"type": "page", "route": targetRoute, "renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "status"}}}, "controls": []any{map[string]any{"filter": "status", "widget": "text"}}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "events_by_status"}, Spec: map[string]any{
			"entity": "event", "fields": []any{"status"}, "groupBy": []any{"status"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "event_count"}},
			"exposedFilters": map[string]any{"status": map[string]any{"field": "status", "operator": sourceOperator}},
			"displays": map[string]any{"chart": map[string]any{
				"type": "block", "renderer": map[string]any{"type": "chart", "groupField": "status", "metricField": "event_count"},
				"drill": map[string]any{"view": "event_records", "display": "page", "bindings": []any{map[string]any{"source": "group", "name": "status", "filter": "status"}}},
			}},
		}},
	}
}

func TestTableDisplayRejectsActionsWithoutRecordID(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "create_article"}, Spec: map[string]any{
			"entity": "article", "operation": "create",
		}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title"},
			"displays": map[string]any{"table": map[string]any{
				"type": "block", "selection": "multiple", "actions": []any{"create_article"},
				"renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title"}}},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "articles", "spec.displays.table.actions.0", "record Action must accept a UUID id input") {
		t.Fatalf("record id diagnostics=%v", diagnostics)
	}
}

func TestTableDisplayRejectsUnsupportedActionInputs(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "tag"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string"},
			map[string]any{"name": "attachment", "type": "file"},
			map[string]any{"name": "metadata", "type": "json"},
			map[string]any{"name": "tags", "type": "relation", "relation": map[string]any{"entity": "tag", "kind": "many-to-many", "targetField": "id"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "update_article"}, Spec: map[string]any{
			"entity": "article", "operation": "update",
		}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title"},
			"displays": map[string]any{"table": map[string]any{
				"type": "block", "selection": "multiple", "actions": []any{"update_article"},
				"renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title"}}},
			}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "articles", "spec.displays.table.actions.0", "record Action input tags is not supported for selection Actions") {
		t.Fatalf("to-many input diagnostics=%v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "View", "articles", "spec.displays.table.actions.0", "record Action input attachment is not supported for selection Actions") {
		t.Fatalf("file input diagnostics=%v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "View", "articles", "spec.displays.table.actions.0", "record Action input metadata is not supported for selection Actions") {
		t.Fatalf("json input diagnostics=%v", diagnostics)
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

func TestPageRoutesMustBeCanonicalURLPaths(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "query"}, Spec: map[string]any{"route": "/reports?format=full", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "encoded"}, Spec: map[string]any{"route": "/caf%C3%A9", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "unclean"}, Spec: map[string]any{"route": "/reports/../latest", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "unicode"}, Spec: map[string]any{"route": "/café", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "backslash"}, Spec: map[string]any{"route": "/\\evil.example/x", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "server"}, Spec: map[string]any{"route": "/healthz", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "client"}, Spec: map[string]any{"route": "/login", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "dynamic_client"}, Spec: map[string]any{"route": "/admin/:section", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "dynamic_server"}, Spec: map[string]any{"route": "/:section", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "duplicate_parameter"}, Spec: map[string]any{"route": "/projects/:id/tasks/:id", "panel": "missing"}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "binding"}, Spec: map[string]any{"route": "/reports/:slug", "panel": "missing", "context": map[string]any{"id": map[string]any{"source": "route", "name": "id"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	for _, name := range []string{"query", "encoded", "unclean", "unicode", "backslash", "server", "client", "dynamic_client", "dynamic_server", "duplicate_parameter"} {
		if !hasViewDisplayDiagnostic(diagnostics, "Page", name, "spec.route") {
			t.Errorf("missing Page/%s/spec.route: %v", name, diagnostics)
		}
	}
	if !hasViewDisplayDiagnostic(diagnostics, "Page", "binding", "spec.context.id.name") {
		t.Errorf("missing Page/binding/spec.context.id.name: %v", diagnostics)
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
			"exposedFilters": map[string]any{
				"title": map[string]any{"field": "title"}, "status": map[string]any{"field": "status", "operator": "contains"},
				"limit": map[string]any{"field": "title"}, "offset": map[string]any{"field": "title"}, "cursor": map[string]any{"field": "title"},
				"q": map[string]any{"field": "title"}, "_display": map[string]any{"field": "title"},
			},
			"displays": map[string]any{
				"index": map[string]any{
					"type": "page", "route": "/items", "bindings": map[string]any{
						"title": map[string]any{"source": "route", "name": "title"}, "q": map[string]any{"source": "context", "name": "q"},
					},
					"title": map[string]any{"field": "status", "fallback": "Item"},
					"renderer": map[string]any{"type": "table", "fields": []any{
						map[string]any{"field": "missing"}, map[string]any{"field": "status"}, map[string]any{"field": "title", "linkRoute": "javascript:alert(1)"},
						map[string]any{"field": "title", "linkRoute": "/\\evil.example/:id"},
					}},
					"controls": []any{
						map[string]any{"filter": "title", "widget": "checkbox"}, map[string]any{"filter": "missing", "widget": "future"},
						map[string]any{"filter": "limit"}, map[string]any{"filter": "offset"}, map[string]any{"filter": "cursor"},
						map[string]any{"filter": "q"}, map[string]any{"filter": "_display"},
					},
					"pager": map[string]any{"type": "offset", "pageSize": 999},
				},
				"index_copy":     map[string]any{"type": "page", "route": "/items", "renderer": map[string]any{"type": "list"}},
				"unknown":        map[string]any{"type": "screen"},
				"missing_feed":   map[string]any{"type": "rss"},
				"relative_api":   map[string]any{"type": "json", "route": "items.json"},
				"dynamic_csv":    map[string]any{"type": "csv", "route": "/items/:id.csv"},
				"query_api":      map[string]any{"type": "json", "route": "/healthz?format=json"},
				"fragment_feed":  map[string]any{"type": "rss", "route": "/items.xml#latest"},
				"encoded_api":    map[string]any{"type": "json", "route": "/caf%C3%A9.json"},
				"builtin_api":    map[string]any{"type": "json", "route": "/api/system/page"},
				"builtin_health": map[string]any{"type": "csv", "route": "/healthz"},
				"client_api":     map[string]any{"type": "json", "route": "/login"},
				"builtin_page":   map[string]any{"type": "page", "route": "/docs", "renderer": map[string]any{"type": "list"}},
				"client_page":    map[string]any{"type": "page", "route": "/admin/reports", "renderer": map[string]any{"type": "list"}},
				"dynamic_page":   map[string]any{"type": "page", "route": "/:section", "renderer": map[string]any{"type": "list"}},
				"duplicate_page": map[string]any{"type": "page", "route": "/projects/:id/tasks/:id", "renderer": map[string]any{"type": "list"}},
				"query_page":     map[string]any{"type": "page", "route": "/reports?format=full", "renderer": map[string]any{"type": "list"}},
				"fragment_page":  map[string]any{"type": "page", "route": "/reports#latest", "renderer": map[string]any{"type": "list"}},
				"unsafe_rich": map[string]any{
					"type": "block", "renderer": map[string]any{"type": "detail", "titleField": "title", "richTextFields": []any{"title"}},
				},
				"": map[string]any{"type": "page", "route": "/empty-display", "renderer": map[string]any{"type": "list"}},
			},
		}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{
			"type": "view", "view": "items", "display": "missing", "inputs": map[string]any{"q": map[string]any{"type": "string"}},
			"bindings": map[string]any{"q": map[string]any{"source": "context", "name": "q"}},
		}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	paths := map[string]bool{}
	for _, diagnostic := range diagnostics {
		paths[diagnostic.Kind+"/"+diagnostic.Name+"/"+diagnostic.Path] = true
	}
	for _, path := range []string{
		"View/items/spec.displays",
		"View/items/spec.exposedFilters.limit",
		"View/items/spec.exposedFilters.offset",
		"View/items/spec.exposedFilters.cursor",
		"View/items/spec.exposedFilters.q",
		"View/items/spec.exposedFilters._display",
		"View/items/spec.exposedFilters.status.operator",
		"View/items/spec.displays.index.renderer.fields.0.field",
		"View/items/spec.displays.index.renderer.fields.1.field",
		"View/items/spec.displays.index.renderer.fields.2.linkRoute",
		"View/items/spec.displays.index.renderer.fields.3.linkRoute",
		"View/items/spec.displays.index.title.field",
		"View/items/spec.displays.index.controls.0.filter",
		"View/items/spec.displays.index.controls.0.widget",
		"View/items/spec.displays.index.controls.1.filter",
		"View/items/spec.displays.index.controls.2.filter",
		"View/items/spec.displays.index.controls.3.filter",
		"View/items/spec.displays.index.controls.4.filter",
		"View/items/spec.displays.index.controls.5.filter",
		"View/items/spec.displays.index.controls.6.filter",
		"View/items/spec.displays.index.pager.type",
		"View/items/spec.displays.index.pager.pageSize",
		"View/items/spec.displays.index.bindings.title.name",
		"View/items/spec.displays.index.bindings.q",
		"View/items/spec.displays.index_copy.route",
		"View/items/spec.displays.unknown.type",
		"View/items/spec.displays.missing_feed.route",
		"View/items/spec.displays.relative_api.route",
		"View/items/spec.displays.dynamic_csv.route",
		"View/items/spec.displays.query_api.route",
		"View/items/spec.displays.fragment_feed.route",
		"View/items/spec.displays.encoded_api.route",
		"View/items/spec.displays.builtin_api.route",
		"View/items/spec.displays.builtin_health.route",
		"View/items/spec.displays.client_api.route",
		"View/items/spec.displays.builtin_page.route",
		"View/items/spec.displays.client_page.route",
		"View/items/spec.displays.dynamic_page.route",
		"View/items/spec.displays.duplicate_page.route",
		"View/items/spec.displays.query_page.route",
		"View/items/spec.displays.fragment_page.route",
		"View/items/spec.displays.unsafe_rich.renderer.richTextFields.0",
		"Block/items/spec.display",
		"Block/items/spec.bindings.q",
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

func hasDiagnosticMessage(diagnostics []definition.Diagnostic, kind, name, path, message string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind && diagnostic.Name == name && diagnostic.Path == path && diagnostic.Message == message {
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
