package patterns

import (
	"fmt"
	"sort"

	"github.com/beanruntime/bean/internal/definition"
)

type Pattern struct {
	Name                 string                  `json:"name"`
	Description          string                  `json:"description"`
	RequiredCapabilities []string                `json:"requiredCapabilities"`
	Definitions          []definition.Definition `json:"definitions"`
}

func Names() []string {
	names := make([]string, 0, len(catalog()))
	for name := range catalog() {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func Inspect(name string) (Pattern, error) {
	pattern, exists := catalog()[name]
	if !exists {
		return Pattern{}, fmt.Errorf("unknown pattern %q", name)
	}
	return pattern, nil
}

func catalog() map[string]Pattern {
	return map[string]Pattern{
		"activity_history": pattern("activity_history", "Timeline of immutable business activity.", []string{"Entity", "View", "Block", "timeline"},
			entity("activity", fields(field("subject", "string", true), field("detail", "text", false), field("occurred_at", "datetime", true))),
			view("activity_history", "activity", []any{"id", "subject", "detail", "occurred_at"}),
			block("activity_timeline", "activity_history", map[string]any{"mode": "timeline", "titleField": "subject", "bodyField": "detail", "timeField": "occurred_at"})),
		"approval_workflow": pattern("approval_workflow", "Draft, submitted, approved, and rejected request flow.", []string{"Entity", "Action", "transition"},
			entity("approval_request", fields(field("title", "string", true), enumField("status", "draft", "submitted", "approved", "rejected"))),
			transition("review_request", "approval_request", map[string]any{"draft": []any{"submitted"}, "submitted": []any{"approved", "rejected"}})),
		"assignment": pattern("assignment", "Assign work records to a related person.", []string{"Entity", "Relation"},
			entity("assignee", fields(field("name", "string", true))),
			entity("assigned_work", fields(field("title", "string", true), relationField("assignee_id", "assignee", false)))),
		"comments": pattern("comments", "Parent-bound comments as ordinary related records.", []string{"Entity", "Relation", "View"},
			entity("subject", fields(field("title", "string", true))),
			entity("comment", fields(relationField("subject_id", "subject", true), field("body", "text", true))),
			view("comments", "comment", []any{"id", "subject_id", "body", "created_at"})),
		"crud_resource": pattern("crud_resource", "Small browsable and editable resource.", []string{"Entity", "View", "Action"},
			entity("resource", fields(field("name", "string", true), field("description", "text", false))),
			view("resources", "resource", []any{"id", "name", "description", "created_at"})),
		"dashboard": pattern("dashboard", "Metric dashboard composed from View, Block, Panel, and Page.", []string{"View", "Block", "Panel", "Page", "metric"},
			entity("dashboard_item", fields(field("name", "string", true))),
			definitionOf("View", "dashboard_total", map[string]any{"entity": "dashboard_item", "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "item_count"}}}),
			block("dashboard_metric", "dashboard_total", map[string]any{"mode": "metric", "metricField": "item_count", "metricLabel": "Items"}),
			definitionOf("Panel", "dashboard_panel", map[string]any{"layout": "grid", "regions": []any{map[string]any{"name": "main", "blocks": []any{"dashboard_metric"}}}}),
			definitionOf("Page", "dashboard", map[string]any{"route": "/dashboard", "title": "Dashboard", "panel": "dashboard_panel"})),
		"many_to_many_tagging": pattern("many_to_many_tagging", "Attach reusable tags through a many-to-many relation.", []string{"Entity", "Relation"},
			entity("tag", fields(field("name", "string", true))),
			entity("tagged_item", fields(field("title", "string", true), toManyField("tag_ids", "tag")))),
		"ownership": pattern("ownership", "Owner-scoped records with authenticated access.", []string{"Role", "Policy", "Entity", "ownership"},
			definitionOf("Role", "member", map[string]any{"permissions": []any{}}),
			definitionOf("Policy", "owned_records", map[string]any{"authenticated": true, "owner": true, "readRoles": []any{"member"}, "writeRoles": []any{"member"}}),
			definitionOf("Entity", "owned_record", map[string]any{"owner": true, "policy": "owned_records", "fields": fields(field("title", "string", true))})),
		"parent_child": pattern("parent_child", "Parent and child resources with a required relation.", []string{"Entity", "Relation"},
			entity("parent", fields(field("name", "string", true))),
			entity("child", fields(relationField("parent_id", "parent", true), field("name", "string", true)))),
		"workflow_resource": pattern("workflow_resource", "Enum-backed workflow with a guarded transition Action.", []string{"Entity", "Action", "Board", "transition"},
			entity("workflow_item", fields(field("title", "string", true), enumField("status", "todo", "in_progress", "done"))),
			transition("move_workflow_item", "workflow_item", map[string]any{"todo": []any{"in_progress"}, "in_progress": []any{"todo", "done"}, "done": []any{"in_progress"}}),
			view("workflow_items", "workflow_item", []any{"id", "title", "status"}),
			block("workflow_board", "workflow_items", map[string]any{"mode": "board", "titleField": "title", "groupField": "status", "moveAction": "move_workflow_item", "columns": []any{"todo", "in_progress", "done"}})),
	}
}

func pattern(name, description string, capabilities []string, definitions ...definition.Definition) Pattern {
	return Pattern{Name: name, Description: description, RequiredCapabilities: capabilities, Definitions: definitions}
}

func definitionOf(kind, name string, spec map[string]any) definition.Definition {
	return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
}

func entity(name string, fieldDefinitions []any) definition.Definition {
	return definitionOf("Entity", name, map[string]any{"fields": fieldDefinitions})
}

func view(name, entityName string, selected []any) definition.Definition {
	return definitionOf("View", name, map[string]any{"entity": entityName, "fields": selected})
}

func block(name, viewName string, presentation map[string]any) definition.Definition {
	return definitionOf("Block", name, map[string]any{"type": "view", "view": viewName, "presentation": presentation})
}

func transition(name, entityName string, transitions map[string]any) definition.Definition {
	return definitionOf("Action", name, map[string]any{"entity": entityName, "operation": "transition", "transitions": transitions})
}

func fields(items ...map[string]any) []any {
	out := make([]any, len(items))
	for index := range items {
		out[index] = items[index]
	}
	return out
}

func field(name, fieldType string, required bool) map[string]any {
	return map[string]any{"name": name, "label": name, "type": fieldType, "required": required}
}

func enumField(name string, options ...string) map[string]any {
	values := make([]any, len(options))
	for index := range options {
		values[index] = options[index]
	}
	return map[string]any{"name": name, "label": name, "type": "enum", "required": true, "options": values}
}

func relationField(name, entityName string, required bool) map[string]any {
	return map[string]any{"name": name, "label": name, "type": "relation", "required": required, "relation": map[string]any{"entity": entityName, "kind": "many-to-one", "targetField": "id"}}
}

func toManyField(name, entityName string) map[string]any {
	return map[string]any{"name": name, "label": name, "type": "relation", "relation": map[string]any{"entity": entityName, "kind": "many-to-many", "targetField": "id"}}
}
