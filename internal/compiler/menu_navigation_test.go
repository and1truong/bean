package compiler_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestHierarchicalMenusCompileTypedStaticAndRecordTargets(t *testing.T) {
	result := compiler.Compile("menus", 1, menuNavigationDefinitions())
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	contents := result.App.Menus["book_contents"]
	if contents.Profile != "workspace" || contents.Variant != "default" || contents.MaxDepth != 3 || contents.Owner == nil || contents.Owner.Entity != "book" || len(contents.Items) != 0 {
		t.Fatalf("contents=%+v", contents)
	}
	main := result.App.Menus["main_navigation"]
	if main.Profile != "workspace" || main.Variant != "line" || main.MaxDepth != 3 || len(main.Items) != 2 || main.Items[0].ID != "home" || main.Items[1].Parent != "home" || main.Items[1].Target.View != "book_pages" {
		t.Fatalf("main=%+v", main)
	}
	navigation := result.App.Entities["book_page"].Navigation
	if navigation == nil || navigation.LabelField != "title" || navigation.Destination.View != "book_pages" || navigation.Destination.Display != "detail" || !reflect.DeepEqual(navigation.Menus, []string{"book_contents"}) {
		t.Fatalf("navigation=%+v", navigation)
	}
	_, references, exists := compiler.InspectDefinition(result.App, "Menu", "main_navigation")
	if !exists || !menuContainsReference(references, "items.0.target.page", "Page", "home") || !menuContainsReference(references, "items.1.target.view", "View", "book_pages") {
		t.Fatalf("references=%+v", references)
	}
}

func TestHierarchicalMenusRejectInvalidScopeTargetsHierarchyAndEntityDestination(t *testing.T) {
	definitions := menuNavigationDefinitions()
	for index := range definitions {
		switch definitions[index].Kind {
		case "Menu":
			if definitions[index].Metadata.Name == "book_contents" {
				definitions[index].Spec["owner"] = map[string]any{"entity": "missing"}
				definitions[index].Spec["items"] = []any{map[string]any{"id": "scoped", "target": map[string]any{"page": "home"}}}
			}
			if definitions[index].Metadata.Name == "main_navigation" {
				definitions[index].Spec["variant"] = "cards"
				definitions[index].Spec["items"] = []any{
					map[string]any{"id": "first", "parent": "second", "weight": 1001, "target": map[string]any{"page": "home"}},
					map[string]any{"id": "second", "parent": "first", "target": map[string]any{"page": "home"}},
				}
			}
		case "Entity":
			if definitions[index].Metadata.Name == "book_page" {
				navigation := definitions[index].Spec["navigation"].(map[string]any)
				navigation["labelField"] = "missing"
				navigation["destination"] = map[string]any{"view": "books", "display": "list"}
				navigation["menus"] = []any{"main_navigation"}
			}
		}
	}
	result := compiler.Compile("invalid-menus", 1, definitions)
	for _, expected := range []struct{ kind, name, path string }{
		{"Menu", "book_contents", "spec.owner.entity"},
		{"Menu", "book_contents", "spec.items"},
		{"Menu", "main_navigation", "spec.variant"},
		{"Menu", "main_navigation", "spec.items.0.weight"},
		{"Menu", "main_navigation", "spec.items.0.parent"},
		{"Menu", "main_navigation", "spec.items.1.target"},
		{"Entity", "book_page", "spec.navigation.labelField"},
		{"Entity", "book_page", "spec.navigation.destination.view"},
		{"Entity", "book_page", "spec.navigation.menus.0"},
	} {
		if !hasDiagnosticPath(result.Diagnostics, expected.kind, expected.name, expected.path) {
			t.Errorf("missing %s/%s %s in %v", expected.kind, expected.name, expected.path, result.Diagnostics)
		}
	}
}

func menuContainsReference(references []compiler.DefinitionReference, path, kind, name string) bool {
	for _, reference := range references {
		if reference.Path == path && reference.Kind == kind && reference.Name == name {
			return true
		}
	}
	return false
}

func menuNavigationDefinitions() []definition.Definition {
	item := func(kind, name string, spec map[string]any) definition.Definition {
		return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
	}
	return []definition.Definition{
		item("Entity", "book", map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}),
		item("Entity", "book_page", map[string]any{
			"fields":     []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "book_id", "type": "relation", "required": true, "relation": map[string]any{"entity": "book", "kind": "many-to-one", "targetField": "id"}}},
			"navigation": map[string]any{"labelField": "title", "destination": map[string]any{"view": "book_pages", "display": "detail"}, "menus": []any{"book_contents"}},
		}),
		item("View", "books", map[string]any{
			"entity": "book", "fields": []any{"id", "title"},
			"displays": map[string]any{"list": map[string]any{
				"type": "page", "route": "/books",
				"renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title"}}},
			}},
		}),
		item("View", "book_pages", map[string]any{
			"entity": "book_page", "fields": []any{"id", "title", "book_id"},
			"exposedFilters": map[string]any{"id": map[string]any{"field": "id", "operator": "eq"}, "book_id": map[string]any{"field": "book_id", "operator": "eq"}},
			"displays": map[string]any{
				"detail": map[string]any{"type": "page", "route": "/books/:book_id/pages/:id", "bindings": map[string]any{"id": map[string]any{"source": "route", "name": "id", "required": true}, "book_id": map[string]any{"source": "route", "name": "book_id", "required": true}}, "renderer": map[string]any{"type": "detail"}},
				"directory": map[string]any{
					"type": "page", "route": "/pages",
					"renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title"}}},
				},
			},
		}),
		item("Block", "home_text", map[string]any{"type": "text", "text": "Home"}),
		item("Panel", "home_panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"home_text"}}}}),
		item("Page", "home", map[string]any{"route": "/", "panel": "home_panel"}),
		item("Menu", "book_contents", map[string]any{"profile": "workspace", "owner": map[string]any{"entity": "book"}}),
		item("Menu", "main_navigation", map[string]any{"profile": "workspace", "variant": "line", "items": []any{
			map[string]any{"id": "home", "target": map[string]any{"page": "home"}, "weight": 0},
			map[string]any{"id": "directory", "label": "Pages", "parent": "home", "target": map[string]any{"view": "book_pages", "display": "directory"}, "weight": 10},
		}}),
	}
}
