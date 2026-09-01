package agentprotocol

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/expr"
)

type InspectedReference struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func InspectedDefinition(app *appir.App, kind, name string) (any, []InspectedReference, bool) {
	var value any
	var exists bool
	switch kind {
	case "Entity":
		value, exists = app.Entities[name]
	case "View":
		value, exists = app.Views[name]
	case "Action":
		value, exists = app.Actions[name]
	case "Lifecycle":
		value, exists = app.Lifecycles[name]
	case "Policy":
		value, exists = app.Policies[name]
	case "Filter":
		value, exists = app.Filters[name]
	case "Webform":
		value, exists = app.Webforms[name]
	case "Block":
		value, exists = app.Blocks[name]
	case "Panel":
		value, exists = app.Panels[name]
	case "Page":
		value, exists = app.Pages[name]
	case "Role":
		value, exists = app.Roles[name]
	case "Menu":
		value, exists = app.Menus[name]
	case "Job":
		value, exists = app.Jobs[name]
	case "AdminResource":
		value, exists = app.AdminResources[name]
	case "LocalRegistration":
		if app.LocalRegistration != nil {
			value, exists = *app.LocalRegistration, true
		}
	case "Theme":
		if app.Theme != nil && app.Theme.Name == name {
			value, exists = *app.Theme, true
		}
	case "DemoSeed":
		if app.DemoSeed != nil && app.DemoSeed.Name == name {
			value, exists = *app.DemoSeed, true
		}
	}
	if !exists {
		return nil, nil, false
	}
	return value, definitionReferences(app, kind, name), true
}

func DefinitionNames(app *appir.App, kind string) []string {
	names := []string{}
	appendMap := func(values any) {
		value := reflect.ValueOf(values)
		for _, key := range value.MapKeys() {
			names = append(names, key.String())
		}
	}
	switch kind {
	case "Entity":
		appendMap(app.Entities)
	case "View":
		appendMap(app.Views)
	case "Action":
		appendMap(app.Actions)
	case "Lifecycle":
		appendMap(app.Lifecycles)
	case "Policy":
		appendMap(app.Policies)
	case "Filter":
		appendMap(app.Filters)
	case "Webform":
		appendMap(app.Webforms)
	case "Block":
		appendMap(app.Blocks)
	case "Panel":
		appendMap(app.Panels)
	case "Page":
		appendMap(app.Pages)
	case "Role":
		appendMap(app.Roles)
	case "Menu":
		appendMap(app.Menus)
	case "Job":
		appendMap(app.Jobs)
	case "AdminResource":
		appendMap(app.AdminResources)
	case "Theme":
		if app.Theme != nil {
			names = append(names, app.Theme.Name)
		}
	case "DemoSeed":
		if app.DemoSeed != nil {
			names = append(names, app.DemoSeed.Name)
		}
	}
	sort.Strings(names)
	return names
}

func definitionReferences(app *appir.App, kind, name string) []InspectedReference {
	references := []InspectedReference{}
	add := func(path, targetKind, targetName string) {
		if targetName != "" {
			references = append(references, InspectedReference{Path: path, Kind: targetKind, Name: targetName})
		}
	}
	switch kind {
	case "Entity":
		entity := app.Entities[name]
		add("policy", "Policy", entity.Policy)
		for index, field := range entity.Fields {
			if field.Relation != nil {
				add(fmt.Sprintf("fields.%d.relation.entity", index), "Entity", field.Relation.Entity)
			}
		}
	case "View":
		item := app.Views[name]
		add("entity", "Entity", item.Entity)
		add("policy", "Policy", item.Policy)
		for path, filter := range item.FieldFilters {
			add("fieldFilters."+path, "Filter", filter)
		}
		for index, relationship := range item.Relationships {
			add(fmt.Sprintf("relationships.%d.entity", index), "Entity", relationship.Entity)
		}
	case "Action":
		item := app.Actions[name]
		add("entity", "Entity", item.Entity)
		add("lifecycle", "Lifecycle", item.Lifecycle)
		add("policy", "Policy", item.Policy)
		add("defaultRole", "Role", item.DefaultRole)
		for index, step := range item.Steps {
			add(fmt.Sprintf("steps.%d.entity", index), "Entity", step.Entity)
			add(fmt.Sprintf("steps.%d.view", index), "View", step.View)
			add(fmt.Sprintf("steps.%d.job", index), "Job", step.Job)
		}
	case "Lifecycle":
		add("entity", "Entity", app.Lifecycles[name].Entity)
	case "Webform":
		add("action", "Action", app.Webforms[name].Action)
	case "Block":
		item := app.Blocks[name]
		add("view", "View", item.View)
		add("entity", "Entity", item.Entity)
		add("webform", "Webform", item.Webform)
		add("action", "Action", item.Action)
		add("menu", "Menu", item.Menu)
		add("policy", "Policy", item.Policy)
		add("resource", "AdminResource", item.Resource)
		add("presentation.moveAction", "Action", item.Presentation.MoveAction)
	case "Panel":
		item := app.Panels[name]
		add("policy", "Policy", item.Policy)
		for regionIndex, region := range item.Regions {
			for blockIndex, block := range region.Blocks {
				add(fmt.Sprintf("regions.%d.blocks.%d", regionIndex, blockIndex), "Block", block)
			}
		}
	case "Page":
		item := app.Pages[name]
		add("panel", "Panel", item.Panel)
		add("policy", "Policy", item.Policy)
	case "Menu":
		for index, item := range app.Menus[name].Items {
			add(fmt.Sprintf("items.%d.policy", index), "Policy", item.Policy)
		}
	case "Job":
		add("action", "Action", app.Jobs[name].Action)
	case "AdminResource":
		item := app.AdminResources[name]
		add("entity", "Entity", item.Entity)
		add("view", "View", item.View)
		add("createAction", "Action", item.CreateAction)
		add("updateAction", "Action", item.UpdateAction)
		add("deleteAction", "Action", item.DeleteAction)
		for index, action := range item.Actions {
			add(fmt.Sprintf("actions.%d", index), "Action", action)
		}
	case "LocalRegistration":
		if app.LocalRegistration != nil {
			add("action", "Action", app.LocalRegistration.Action)
		}
	}
	sort.Slice(references, func(left, right int) bool {
		if references[left].Path != references[right].Path {
			return references[left].Path < references[right].Path
		}
		if references[left].Kind != references[right].Kind {
			return references[left].Kind < references[right].Kind
		}
		return references[left].Name < references[right].Name
	})
	return references
}

type SemanticChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func SemanticDiff(current, candidate *appir.App) []SemanticChange {
	if current == nil {
		current = appir.Empty()
	}
	left := RedactedApp(current)
	right := RedactedApp(candidate)
	for _, app := range []*appir.App{left, right} {
		app.ReleaseID = ""
		app.AppID = ""
		app.Version = 0
		app.OpenAPI = nil
	}
	changes := []SemanticChange{}
	diffValue("", normalizedJSON(left), normalizedJSON(right), &changes)
	return changes
}

func RedactedApp(source *appir.App) *appir.App {
	redacted, _ := source.Clone()
	for name, item := range redacted.Views {
		redactExpression(item.Filter)
		redactExpression(item.ContextFilter)
		redacted.Views[name] = item
	}
	for name, item := range redacted.Actions {
		for stepIndex := range item.Steps {
			redactExpression(item.Steps[stepIndex].Where)
			redactExpression(item.Steps[stepIndex].Condition)
			for valueIndex := range item.Steps[stepIndex].Values {
				value := &item.Steps[stepIndex].Values[valueIndex].Value
				if value.Source == "literal" {
					value.Literal = json.RawMessage(`"[REDACTED]"`)
				}
			}
		}
		redacted.Actions[name] = item
	}
	for name, item := range redacted.Policies {
		redactExpression(item.Condition)
		redacted.Policies[name] = item
	}
	for name, item := range redacted.Webforms {
		redactFormExpressions(item.Elements)
		redacted.Webforms[name] = item
	}
	return redacted
}

func redactExpression(expression *expr.Expr) {
	if expression == nil {
		return
	}
	for _, value := range []*expr.Value{expression.Left, expression.Right} {
		if value != nil && value.Source == "literal" {
			value.Literal = "[REDACTED]"
		}
	}
	for index := range expression.Args {
		redactExpression(&expression.Args[index])
	}
}

func redactFormExpressions(elements []appir.FormElement) {
	for index := range elements {
		redactExpression(elements[index].Visible)
		redactExpression(elements[index].RequiredWhen)
		redactFormExpressions(elements[index].Children)
	}
}

func normalizedJSON(value any) any {
	encoded, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(encoded, &decoded)
	return normalizeJSONKeys(decoded)
}

func normalizeJSONKeys(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			runes := []rune(key)
			if len(runes) > 0 {
				runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
			}
			out[string(runes)] = normalizeJSONKeys(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = normalizeJSONKeys(typed[index])
		}
		return out
	default:
		return value
	}
}

func diffValue(path string, before, after any, changes *[]SemanticChange) {
	leftMap, leftIsMap := before.(map[string]any)
	rightMap, rightIsMap := after.(map[string]any)
	if leftIsMap && rightIsMap {
		keys := map[string]bool{}
		for key := range leftMap {
			keys[key] = true
		}
		for key := range rightMap {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			left, leftExists := leftMap[key]
			right, rightExists := rightMap[key]
			switch {
			case !leftExists:
				*changes = append(*changes, SemanticChange{Operation: "add", Path: childPath, After: right})
			case !rightExists:
				*changes = append(*changes, SemanticChange{Operation: "remove", Path: childPath, Before: left})
			default:
				diffValue(childPath, left, right, changes)
			}
		}
		return
	}
	if !reflect.DeepEqual(before, after) {
		*changes = append(*changes, SemanticChange{Operation: "change", Path: path, Before: before, After: after})
	}
}
