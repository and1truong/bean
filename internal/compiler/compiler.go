package compiler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/policy"
)

type Result struct {
	App         *appir.App
	Schema      migration.Schema
	Diagnostics []definition.Diagnostic
}
type actionSource struct {
	Entity, Operation, Policy, StateField string
	DefaultRole, Confirm                  string
	Input                                 map[string]appir.Field
	Output                                map[string]appir.Field
	Steps                                 []stepSource
	Transitions                           map[string][]string
}
type stepSource struct {
	Op, Result, Entity, View, StateField, Event, Job string
	Values                                           map[string]any
	Where, Condition                                 *expr.Expr
}

func Compile(appID string, version int, defs []definition.Definition) Result {
	a := appir.Empty()
	a.AppID = appID
	a.Version = version
	r := Result{App: a}
	seen := map[string]bool{}
	for _, d := range defs {
		r.Diagnostics = append(r.Diagnostics, definition.ValidateEnvelope(d)...)
		key := d.Kind + "/" + d.Metadata.Name
		if seen[key] {
			r.Diagnostics = append(r.Diagnostics, diag(d, "metadata.name", "duplicate machine name"))
			continue
		}
		seen[key] = true
		switch d.Kind {
		case "Entity":
			var x appir.Entity
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			for i := range x.Fields {
				if x.Fields[i].Type != "relation" {
					continue
				}
				if x.Fields[i].Relation == nil {
					target := strings.TrimSuffix(x.Fields[i].Name, "_id")
					x.Fields[i].Relation = &appir.Relation{Entity: target, Kind: "many-to-one", TargetField: "id"}
				}
				if x.Fields[i].Relation.Kind == "" {
					x.Fields[i].Relation.Kind = "many-to-one"
				}
				if x.Fields[i].Relation.TargetField == "" {
					x.Fields[i].Relation.TargetField = "id"
				}
			}
			if x.Label == "" {
				x.Label = x.Name
			}
			a.Entities[x.Name] = x
		case "View":
			var x appir.View
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			for i := range x.Relationships {
				if x.Relationships[i].Type == "" {
					x.Relationships[i].Type = "inner"
				}
			}
			if x.DefaultLimit == 0 {
				x.DefaultLimit = 50
			}
			if x.MaxLimit == 0 {
				x.MaxLimit = 200
			}
			a.Views[x.Name] = x
		case "Action":
			var source actionSource
			if e := definition.DecodeSpec(d.Spec, &source); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x := appir.Action{Name: d.Metadata.Name, Entity: source.Entity, Operation: source.Operation, Policy: source.Policy, StateField: source.StateField, DefaultRole: source.DefaultRole, Confirm: source.Confirm, Input: source.Input, Output: source.Output, Transitions: source.Transitions}
			for inputName, input := range x.Input {
				if input.Name == "" {
					input.Name = inputName
					x.Input[inputName] = input
				}
			}
			for outputName, output := range x.Output {
				if output.Name == "" {
					output.Name = outputName
					x.Output[outputName] = output
				}
			}
			for _, step := range source.Steps {
				compiled := appir.Step{Op: step.Op, Result: step.Result, Entity: step.Entity, View: step.View, StateField: step.StateField, Where: step.Where, Condition: step.Condition, Event: step.Event, Job: step.Job}
				keys := make([]string, 0, len(step.Values))
				for key := range step.Values {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					compiled.Values = append(compiled.Values, appir.Assignment{Field: key, Value: compileBinding(step.Values[key])})
				}
				x.Steps = append(x.Steps, compiled)
			}
			a.Actions[x.Name] = x
		case "Policy":
			var x appir.Policy
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Policies[x.Name] = x
		case "Filter":
			var x appir.Filter
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Filters[x.Name] = x
		case "Webform":
			var x appir.Webform
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Webforms[x.Name] = x
		case "Block":
			var x appir.Block
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Blocks[x.Name] = x
		case "Panel":
			var x appir.Panel
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Panels[x.Name] = x
		case "Page":
			var x appir.Page
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Pages[x.Name] = x
		case "Role":
			var x appir.Role
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Roles[x.Name] = x
		case "Menu":
			var x appir.Menu
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Menus[x.Name] = x
		case "Job":
			var x appir.Job
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.Jobs[x.Name] = x
		case "AdminResource":
			var x appir.AdminResource
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			x.Name = d.Metadata.Name
			a.AdminResources[x.Name] = x
		case "LocalRegistration":
			var x appir.LocalRegistration
			if e := definition.DecodeSpec(d.Spec, &x); e != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "spec", e.Error()))
				continue
			}
			if a.LocalRegistration != nil {
				r.Diagnostics = append(r.Diagnostics, diag(d, "metadata.name", "only one local registration definition is allowed"))
				continue
			}
			a.LocalRegistration = &x
		}
	}
	if len(r.Diagnostics) > 0 {
		return r
	}
	for name, entity := range a.Entities {
		generate(a, name, entity)
	}
	normalizeActions(a)
	normalizeAdminResources(a)
	normalizeResourceListBlocks(a)
	r.Diagnostics = append(r.Diagnostics, validate(a)...)
	if len(r.Diagnostics) > 0 {
		return r
	}
	for _, e := range a.Entities {
		me := migration.Entity{Name: e.Name, Indexes: e.Indexes, Unique: e.Unique}
		for _, f := range e.Fields {
			mf := migration.Field{Name: f.Name, Type: f.Type, Required: f.Required, Unique: f.Unique}
			if f.Relation != nil {
				mf.RelationEntity, mf.RelationKind, mf.TargetField = f.Relation.Entity, f.Relation.Kind, f.Relation.TargetField
			}
			me.Fields = append(me.Fields, mf)
		}
		if e.Owner {
			me.Fields = append(me.Fields, migration.Field{Name: "owner_id", Type: "uuid"})
		}
		if e.Tenant {
			me.Fields = append(me.Fields, migration.Field{Name: "tenant_id", Type: "uuid"})
		}
		if e.SoftDelete {
			me.Fields = append(me.Fields, migration.Field{Name: "deleted_at", Type: "datetime"})
		}
		r.Schema.Entities = append(r.Schema.Entities, me)
	}
	sort.Slice(r.Schema.Entities, func(i, j int) bool { return r.Schema.Entities[i].Name < r.Schema.Entities[j].Name })
	return r
}
func diag(d definition.Definition, path, msg string) definition.Diagnostic {
	return definition.Diagnostic{Kind: d.Kind, Name: d.Metadata.Name, Path: path, Message: msg}
}
func compileBinding(value any) appir.ValueBinding {
	if text, ok := value.(string); ok && strings.HasPrefix(text, "$") {
		if text == "$now" {
			return appir.ValueBinding{Source: "now"}
		}
		if text == "$context.tenant" {
			return appir.ValueBinding{Source: "tenant"}
		}
		for _, source := range []string{"input", "result", "record", "context", "user"} {
			prefix := "$" + source + "."
			if strings.HasPrefix(text, prefix) {
				return appir.ValueBinding{Source: source, Path: strings.TrimPrefix(text, prefix)}
			}
		}
		return appir.ValueBinding{Source: "invalid", Path: text}
	}
	b, _ := json.Marshal(value)
	return appir.ValueBinding{Source: "literal", Literal: b}
}
func normalizeActions(a *appir.App) {
	for name, action := range a.Actions {
		if action.Input == nil {
			action.Input = map[string]appir.Field{}
		}
		if action.Output == nil {
			action.Output = map[string]appir.Field{}
		}
		if action.Operation == "register_local_user" {
			action.Entity = ""
			action.Input = map[string]appir.Field{
				"display_name":          {Name: "display_name", Type: "string", Required: true},
				"email":                 {Name: "email", Type: "email", Required: true},
				"password":              {Name: "password", Type: "password", Required: true, Sensitive: true},
				"password_confirmation": {Name: "password_confirmation", Type: "password", Required: true, Sensitive: true},
			}
			action.Output = map[string]appir.Field{
				"id":           {Name: "id", Type: "uuid"},
				"display_name": {Name: "display_name", Type: "string"},
				"email":        {Name: "email", Type: "email"},
			}
			a.Actions[name] = action
			continue
		}
		entity, exists := a.Entities[action.Entity]
		if !exists || action.Operation == "transaction" {
			if exists {
				normalizeOutput(&action, entity)
			}
			a.Actions[name] = action
			continue
		}
		if action.Operation == "create" {
			for _, field := range entity.Fields {
				action.Input[field.Name] = field
			}
		} else {
			action.Input["id"] = appir.Field{Name: "id", Type: "uuid", Required: true}
			if action.Operation == "update" {
				for _, field := range entity.Fields {
					field.Required = false
					action.Input[field.Name] = field
				}
			}
			if action.Operation == "transition" {
				stateField := action.StateField
				if stateField == "" {
					stateField = "status"
				}
				for _, field := range entity.Fields {
					if field.Name == stateField {
						field.Required = true
						action.Input[field.Name] = field
					}
				}
			}
		}
		normalizeOutput(&action, entity)
		a.Actions[name] = action
	}
}
func normalizeOutput(action *appir.Action, entity appir.Entity) {
	action.Output["id"] = appir.Field{Name: "id", Type: "uuid"}
	if action.Operation == "delete" {
		return
	}
	for _, field := range entity.Fields {
		field.Required = false
		action.Output[field.Name] = field
	}
	if entity.Owner {
		action.Output["owner_id"] = appir.Field{Name: "owner_id", Type: "uuid"}
	}
	if entity.Tenant {
		action.Output["tenant_id"] = appir.Field{Name: "tenant_id", Type: "uuid"}
	}
	if entity.SoftDelete {
		action.Output["deleted_at"] = appir.Field{Name: "deleted_at", Type: "datetime"}
	}
	action.Output["created_at"] = appir.Field{Name: "created_at", Type: "datetime"}
	action.Output["updated_at"] = appir.Field{Name: "updated_at", Type: "datetime"}
	action.Output["version"] = appir.Field{Name: "version", Type: "integer"}
}
func assignment(step appir.Step, name string) (appir.ValueBinding, bool) {
	for _, value := range step.Values {
		if value.Field == name {
			return value.Value, true
		}
	}
	return appir.ValueBinding{}, false
}
func diagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.Diagnostic{Kind: kind, Name: name, Path: path, Message: message}
}
func validate(a *appir.App) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	routes := map[string]string{}
	for name, filterDefinition := range a.Filters {
		if len(filterDefinition.Steps) == 0 {
			out = append(out, diagnostic("Filter", name, "spec.steps", "requires at least one filter step"))
		}
		for i, step := range filterDefinition.Steps {
			if step.Type != "markdown" {
				out = append(out, diagnostic("Filter", name, fmt.Sprintf("spec.steps.%d.type", i), "has no registered filter implementation"))
			}
		}
	}
	for name, v := range a.Views {
		e, ok := a.Entities[v.Entity]
		if !ok {
			out = append(out, diagnostic("View", name, "spec.entity", "references missing Entity "+v.Entity))
			continue
		}
		fields := fieldSet(e)
		if len(v.Fields) == 0 && len(v.Aggregates) == 0 {
			out = append(out, diagnostic("View", name, "spec.fields", "must select fields or aggregates"))
		}
		relationships := map[string]appir.ViewRelationship{}
		for i, relationship := range v.Relationships {
			path := fmt.Sprintf("spec.relationships.%d", i)
			if relationship.RelationField != "" {
				var relation *appir.Relation
				for _, field := range e.Fields {
					if field.Name == relationship.RelationField {
						relation = field.Relation
					}
				}
				if relation == nil {
					out = append(out, diagnostic("View", name, path+".relationField", "references a field without relation storage"))
					continue
				}
				relationship.Entity = relation.Entity
				relationship.LocalField = relationship.RelationField
				relationship.TargetField = relation.TargetField
				v.Relationships[i] = relationship
				a.Views[name] = v
			}
			if relationship.Name == "" {
				out = append(out, diagnostic("View", name, path+".name", "is required"))
			}
			if relationships[relationship.Name].Name != "" {
				out = append(out, diagnostic("View", name, path+".name", "duplicates another relationship"))
			}
			relationships[relationship.Name] = relationship
			target, exists := a.Entities[relationship.Entity]
			if !exists {
				out = append(out, diagnostic("View", name, path+".entity", "references missing Entity "+relationship.Entity))
				continue
			}
			if !fields[relationship.LocalField] {
				out = append(out, diagnostic("View", name, path+".localField", "references missing field "+relationship.LocalField))
			}
			if !fieldSet(target)[relationship.TargetField] {
				out = append(out, diagnostic("View", name, path+".targetField", "references missing target field "+relationship.TargetField))
			}
			if relationship.Type != "inner" && relationship.Type != "left" {
				out = append(out, diagnostic("View", name, path+".type", "must be inner or left"))
			}
		}
		for _, f := range v.Fields {
			if !validViewField(f, fields, relationships, a) {
				out = append(out, diagnostic("View", name, "spec.fields", "references missing field "+f))
			}
		}
		selected := map[string]bool{}
		for _, field := range v.Fields {
			selected[field] = true
		}
		for fieldName, filterName := range v.FieldFilters {
			path := "spec.fieldFilters." + fieldName
			if !selected[fieldName] {
				out = append(out, diagnostic("View", name, path, "must reference a selected View field"))
				continue
			}
			fieldType, exists := viewFieldType(fieldName, e, relationships, a)
			if !exists || !map[string]bool{"string": true, "text": true, "richtext": true}[fieldType] {
				out = append(out, diagnostic("View", name, path, "can only filter textual fields"))
			}
			if _, exists = a.Filters[filterName]; !exists {
				out = append(out, diagnostic("View", name, path, "references missing Filter "+filterName))
			}
		}
		if len(v.GroupBy) == 0 && len(v.Aggregates) == 0 && !selected["id"] {
			out = append(out, diagnostic("View", name, "spec.fields", "must include id for deterministic cursor pagination"))
		}
		for _, group := range v.GroupBy {
			if !validViewField(group, fields, relationships, a) {
				out = append(out, diagnostic("View", name, "spec.groupBy", "references missing field "+group))
			}
		}
		aliases := map[string]bool{}
		for i, aggregate := range v.Aggregates {
			path := fmt.Sprintf("spec.aggregates.%d", i)
			if !map[string]bool{"count": true, "sum": true, "min": true, "max": true, "average": true, "avg": true}[strings.ToLower(aggregate.Function)] {
				out = append(out, diagnostic("View", name, path+".function", "has no query-plan implementation"))
			}
			if !validViewField(aggregate.Field, fields, relationships, a) {
				out = append(out, diagnostic("View", name, path+".field", "references missing field "+aggregate.Field))
			}
			if aggregate.Alias == "" || aliases[aggregate.Alias] {
				out = append(out, diagnostic("View", name, path+".alias", "must be a unique machine name"))
			}
			aliases[aggregate.Alias] = true
		}
		for i, order := range v.Sort {
			if !validViewField(order.Field, fields, relationships, a) && !aliases[order.Field] {
				out = append(out, diagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), "references missing field "+order.Field))
			} else if !selected[order.Field] && !aliases[order.Field] {
				out = append(out, diagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), "must be selected so cursor state is stable"))
			}
		}
		if len(v.Aggregates) > 0 && len(v.Sort) == 0 {
			for _, group := range v.GroupBy {
				v.Sort = append(v.Sort, appir.Sort{Field: group})
			}
			a.Views[name] = v
		}
		for key, exposed := range v.ExposedFilters {
			fieldName := exposed.Name
			if fieldName == "" {
				fieldName = key
			}
			if !validViewField(fieldName, fields, relationships, a) {
				out = append(out, diagnostic("View", name, "spec.exposedFilters."+key, "references missing field "+fieldName))
			}
		}
		for path, expression := range map[string]*expr.Expr{"spec.filter": v.Filter, "spec.contextFilter": v.ContextFilter} {
			if expression != nil {
				if er := validateExpr(*expression, true); er != nil {
					out = append(out, diagnostic("View", name, path, er.Error()))
				}
			}
		}
		if v.DefaultLimit < 1 || v.MaxLimit < 1 || v.MaxLimit > 200 || v.DefaultLimit > v.MaxLimit {
			out = append(out, diagnostic("View", name, "spec.maxLimit", "must be between the default and 200"))
		}
		if v.Policy != "" {
			definition, exists := a.Policies[v.Policy]
			if !exists {
				out = append(out, diagnostic("View", name, "spec.policy", "references missing Policy "+v.Policy))
			} else {
				redacted := map[string]bool{}
				for _, field := range definition.Redact {
					redacted[field] = true
				}
				for i, order := range v.Sort {
					if redacted[order.Field] {
						out = append(out, diagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), "redacted fields cannot control ordering"))
					}
				}
				for key, exposed := range v.ExposedFilters {
					if redacted[exposed.Name] || exposed.Name == "" && redacted[key] {
						out = append(out, diagnostic("View", name, "spec.exposedFilters."+key, "redacted fields cannot be exposed as filters"))
					}
				}
				for _, expression := range []*expr.Expr{v.Filter, v.ContextFilter} {
					for _, field := range recordFields(expression) {
						if redacted[field] {
							out = append(out, diagnostic("View", name, "spec.filter", "redacted fields cannot control filtering"))
						}
					}
				}
			}
		}
		for displayName, display := range v.Displays {
			if !map[string]bool{"json": true, "csv": true, "rss": true}[display.Type] {
				out = append(out, diagnostic("View", name, "spec.displays."+displayName+".type", "has no serializer"))
			}
			if display.Route == "" {
				continue
			}
			if old := routes[display.Route]; old != "" {
				out = append(out, diagnostic("View", name, "spec.displays."+displayName+".route", "duplicates route used by "+old))
			}
			routes[display.Route] = "View/" + name
		}
	}
	allowedActions := map[string]bool{"create": true, "update": true, "delete": true, "transition": true, "transaction": true, "register_local_user": true}
	allowedSteps := map[string]bool{"load": true, "query": true, "assert": true, "assert_no_overlap": true, "create": true, "update": true, "conditional_update": true, "decrement": true, "delete": true, "transition": true, "emit": true, "schedule": true, "return": true}
	allowedRelations := map[string]bool{"one-to-one": true, "one-to-many": true, "many-to-one": true, "many-to-many": true}
	for name, entity := range a.Entities {
		if entity.Policy != "" {
			if _, ok := a.Policies[entity.Policy]; !ok {
				out = append(out, diagnostic("Entity", name, "spec.policy", "references missing Policy "+entity.Policy))
			}
		}
		for i, field := range entity.Fields {
			if field.Type != "relation" {
				continue
			}
			if field.Relation == nil {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation", i), "relation storage is required"))
				continue
			}
			if !allowedRelations[field.Relation.Kind] {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation", i), "relation kind is invalid"))
				continue
			}
			if _, ok := a.Entities[field.Relation.Entity]; !ok {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.entity", i), "references missing Entity "+field.Relation.Entity))
			}
			if field.Relation.TargetField == "" {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.targetField", i), "is required"))
			} else if target, ok := a.Entities[field.Relation.Entity]; ok && !fieldSet(target)[field.Relation.TargetField] {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.targetField", i), "references missing target field "+field.Relation.TargetField))
			}
		}
	}
	for name, action := range a.Actions {
		if action.Operation == "register_local_user" {
			if action.DefaultRole == "" {
				out = append(out, diagnostic("Action", name, "spec.defaultRole", "is required"))
			} else if _, ok := a.Roles[action.DefaultRole]; !ok {
				out = append(out, diagnostic("Action", name, "spec.defaultRole", "references missing Role "+action.DefaultRole))
			}
		} else if _, ok := a.Entities[action.Entity]; !ok {
			out = append(out, diagnostic("Action", name, "spec.entity", "references missing Entity "+action.Entity))
		}
		if !allowedActions[action.Operation] {
			out = append(out, diagnostic("Action", name, "spec.operation", "invalid Action operation"))
		}
		if action.Policy != "" {
			if _, ok := a.Policies[action.Policy]; !ok {
				out = append(out, diagnostic("Action", name, "spec.policy", "references missing Policy "+action.Policy))
			}
		}
		for inputName, input := range action.Input {
			if input.Name != inputName {
				out = append(out, diagnostic("Action", name, "spec.input."+inputName+".name", "must match its input key"))
			}
			if input.Type == "" {
				out = append(out, diagnostic("Action", name, "spec.input."+inputName+".type", "is required"))
			}
		}
		for outputName, output := range action.Output {
			if output.Sensitive {
				out = append(out, diagnostic("Action", name, "spec.output."+outputName, "sensitive fields cannot be Action outputs"))
			}
			if output.Name != outputName || output.Type == "" {
				out = append(out, diagnostic("Action", name, "spec.output."+outputName, "requires a matching name and type"))
			}
		}
		if action.Operation == "transaction" && len(action.Input) == 0 {
			out = append(out, diagnostic("Action", name, "spec.input", "transaction Action requires a typed input schema"))
		}
		if action.Operation == "transaction" && len(action.Steps) == 0 {
			out = append(out, diagnostic("Action", name, "spec.steps", "transaction requires at least one step"))
		}
		if action.Operation != "transaction" && len(action.Steps) > 0 {
			out = append(out, diagnostic("Action", name, "spec.steps", "steps are only valid for transaction Actions"))
		}
		results := map[string]bool{}
		for i, step := range action.Steps {
			path := fmt.Sprintf("spec.steps.%d", i)
			if !allowedSteps[step.Op] {
				out = append(out, diagnostic("Action", name, path+".op", "has no runtime executor"))
				continue
			}
			if step.Result != "" {
				if results[step.Result] {
					out = append(out, diagnostic("Action", name, path+".result", "duplicates a previous step result"))
				}
			}
			for _, assignment := range step.Values {
				switch assignment.Value.Source {
				case "literal", "context", "tenant", "user", "record", "now":
				case "input":
					if _, ok := action.Input[assignment.Value.Path]; !ok {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "references undeclared input "+assignment.Value.Path))
					}
				case "result":
					root := strings.Split(assignment.Value.Path, ".")[0]
					if !results[root] {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "references unavailable step result "+root))
					}
				default:
					out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "has unsupported binding "+assignment.Value.Path))
				}
			}
			if step.Result != "" {
				results[step.Result] = true
			}
			entity := step.Entity
			if entity == "" {
				if legacy, ok := assignment(step, "entity"); ok && legacy.Source == "literal" {
					_ = json.Unmarshal(legacy.Literal, &entity)
				} else {
					entity = action.Entity
				}
			}
			if usesEntity(step.Op) {
				if _, ok := a.Entities[entity]; !ok {
					out = append(out, diagnostic("Action", name, path+".entity", "references missing Entity "+entity))
				}
			}
			if target, ok := a.Entities[entity]; ok {
				allowedValues := stepValueFields(step.Op, target, action)
				for _, assignment := range step.Values {
					if !allowedValues[assignment.Field] {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "is not used by the "+step.Op+" executor"))
					}
				}
			}
			if step.Op == "query" && step.View != "" {
				if _, ok := a.Views[step.View]; !ok {
					out = append(out, diagnostic("Action", name, path+".view", "references missing View "+step.View))
				}
			}
			if step.Op == "query" && step.View == "" {
				out = append(out, diagnostic("Action", name, path+".view", "is required so reads use a compiled View"))
			}
			if (step.Op == "assert" || step.Op == "conditional_update") && step.Condition == nil {
				out = append(out, diagnostic("Action", name, path+".condition", "is required"))
			}
			if step.Condition != nil {
				if er := validateExpr(*step.Condition, false); er != nil {
					out = append(out, diagnostic("Action", name, path+".condition", er.Error()))
				}
			}
			if step.Where != nil {
				if er := validateExpr(*step.Where, true); er != nil {
					out = append(out, diagnostic("Action", name, path+".where", er.Error()))
				}
			}
			if (step.Op == "load" || step.Op == "update" || step.Op == "conditional_update" || step.Op == "delete" || step.Op == "transition") && !hasAssignment(step, "id") {
				out = append(out, diagnostic("Action", name, path+".values.id", "is required"))
			}
			if step.Op == "schedule" {
				if _, ok := a.Jobs[step.Job]; !ok {
					out = append(out, diagnostic("Action", name, path+".job", "references missing Job "+step.Job))
				}
			}
			if step.Op == "emit" && step.Event == "" {
				out = append(out, diagnostic("Action", name, path+".event", "is required"))
			}
			if step.Op == "return" {
				for _, assignment := range step.Values {
					if _, declared := action.Output[assignment.Field]; !declared {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "is not declared in the Action output schema"))
					}
				}
			}
		}
	}
	for name, form := range a.Webforms {
		action, ok := a.Actions[form.Action]
		if !ok {
			out = append(out, diagnostic("Webform", name, "spec.action", "references missing Action "+form.Action))
		} else {
			for i, element := range form.Elements {
				input, exists := action.Input[element.Name]
				if !exists {
					out = append(out, diagnostic("Webform", name, fmt.Sprintf("spec.elements.%d.name", i), "has no matching Action input"))
				} else if !compatibleFormType(element.Type, input.Type) {
					out = append(out, diagnostic("Webform", name, fmt.Sprintf("spec.elements.%d.type", i), "does not match Action input type "+input.Type))
				}
			}
		}
		out = append(out, validateForm(name, form)...)
	}
	for name, policy := range a.Policies {
		if policy.Condition != nil {
			if er := validateExpr(*policy.Condition, true); er != nil {
				out = append(out, diagnostic("Policy", name, "spec.condition", er.Error()))
			}
		}
	}
	for name, block := range a.Blocks {
		if !map[string]bool{"text": true, "view": true, "entity": true, "webform": true, "action": true, "menu": true, "resource-list": true}[block.Type] {
			out = append(out, diagnostic("Block", name, "spec.type", "has no registered renderer"))
		}
		if block.Type == "resource-list" && block.Resource == "" {
			out = append(out, diagnostic("Block", name, "spec.resource", "is required"))
		}
		refs := []struct{ kind, value string }{{"view", block.View}, {"entity", block.Entity}, {"webform", block.Webform}, {"action", block.Action}, {"resource", block.Resource}}
		for _, ref := range refs {
			if ref.value == "" {
				continue
			}
			ok := false
			switch ref.kind {
			case "view":
				_, ok = a.Views[ref.value]
			case "entity":
				_, ok = a.Entities[ref.value]
			case "webform":
				_, ok = a.Webforms[ref.value]
			case "action":
				_, ok = a.Actions[ref.value]
			case "resource":
				_, ok = a.AdminResources[ref.value]
			}
			if !ok {
				out = append(out, diagnostic("Block", name, "spec."+ref.kind, "invalid Block input reference "+ref.value))
			}
		}
		if block.Menu != "" {
			if _, ok := a.Menus[block.Menu]; !ok {
				out = append(out, diagnostic("Block", name, "spec.menu", "references missing Menu "+block.Menu))
			}
		}
		if block.Policy != "" {
			if _, ok := a.Policies[block.Policy]; !ok {
				out = append(out, diagnostic("Block", name, "spec.policy", "references missing Policy "+block.Policy))
			}
		}
		for inputName, input := range block.Inputs {
			binding, mapped := block.Bindings[inputName]
			if input.Required && !mapped {
				out = append(out, diagnostic("Block", name, "spec.bindings."+inputName, "required input has no typed mapping"))
			}
			if mapped {
				if !map[string]bool{"context": true, "user": true, "tenant": true}[binding.Source] {
					out = append(out, diagnostic("Block", name, "spec.bindings."+inputName+".source", "has no typed resolver"))
				}
				if binding.Source != "tenant" && binding.Name == "" {
					out = append(out, diagnostic("Block", name, "spec.bindings."+inputName+".name", "is required"))
				}
			}
		}
		for inputName := range block.Bindings {
			if _, exists := block.Inputs[inputName]; !exists {
				out = append(out, diagnostic("Block", name, "spec.bindings."+inputName, "references an undeclared input"))
			}
		}
		var target map[string]appir.Field
		if block.Type == "view" && block.View != "" {
			target = a.Views[block.View].ExposedFilters
		}
		if block.Type == "webform" && block.Webform != "" {
			target = a.Actions[a.Webforms[block.Webform].Action].Input
		}
		if block.Type == "resource-list" && block.Resource != "" {
			resource := a.AdminResources[block.Resource]
			target = a.Views[resource.View].ExposedFilters
			resourceFilters := nameSet(resource.List.Filters)
			interactive := nameSet(block.Filters)
			for i, filterName := range block.Filters {
				if _, bound := block.Bindings[filterName]; bound {
					out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "cannot expose an immutable bound input"))
				}
				if !resourceFilters[filterName] {
					out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "is not configured by AdminResource "+block.Resource))
				}
				if _, exposed := target[filterName]; !exposed {
					out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "has no matching View exposed filter"))
				}
			}
			for filterName, value := range block.DefaultFilters {
				definition, exposed := target[filterName]
				if !interactive[filterName] {
					out = append(out, diagnostic("Block", name, "spec.defaultFilters."+filterName, "must reference an interactive filter"))
				} else if exposed {
					definition.Name = filterName
					if err := field.Validate(definition, value); err != nil {
						out = append(out, diagnostic("Block", name, "spec.defaultFilters."+filterName, err.Error()))
					}
				}
			}
		}
		if target != nil {
			for inputName, input := range block.Inputs {
				expected, exists := target[inputName]
				if !exists {
					out = append(out, diagnostic("Block", name, "spec.inputs."+inputName, "has no matching target input"))
				} else if input.Type != expected.Type {
					out = append(out, diagnostic("Block", name, "spec.inputs."+inputName+".type", "does not match target input type "+expected.Type))
				}
			}
		}
	}
	if a.LocalRegistration != nil {
		action, ok := a.Actions[a.LocalRegistration.Action]
		if !ok || action.Operation != "register_local_user" {
			out = append(out, diagnostic("LocalRegistration", "local", "spec.action", "must reference a register_local_user Action"))
		}
		if route := a.LocalRegistration.Route; route != "" {
			if !strings.HasPrefix(route, "/") || strings.Contains(route, ":") {
				out = append(out, diagnostic("LocalRegistration", "local", "spec.route", "must be a static absolute Page route"))
			} else if routeErr := validateRegistrationPage(a, route, a.LocalRegistration.Action); routeErr != "" {
				out = append(out, diagnostic("LocalRegistration", "local", "spec.route", routeErr))
			}
		}
	}
	layouts := map[string]map[string]bool{"single-column": {"main": true}, "two-column": {"left": true, "right": true}, "sidebar-main": {"sidebar": true, "main": true}, "main-sidebar": {"main": true, "sidebar": true}, "grid": {"main": true}}
	for name, panel := range a.Panels {
		regions, ok := layouts[panel.Layout]
		if !ok {
			out = append(out, diagnostic("Panel", name, "spec.layout", "invalid layout"))
			continue
		}
		for _, region := range panel.Regions {
			if !regions[region.Name] {
				out = append(out, diagnostic("Panel", name, "spec.regions."+region.Name, "invalid Panel region"))
			}
			for _, block := range region.Blocks {
				if _, ok := a.Blocks[block]; !ok {
					out = append(out, diagnostic("Panel", name, "spec.regions."+region.Name, "references missing Block "+block))
				}
			}
		}
		if panel.Policy != "" {
			if _, ok := a.Policies[panel.Policy]; !ok {
				out = append(out, diagnostic("Panel", name, "spec.policy", "references missing Policy "+panel.Policy))
			}
		}
	}
	for name, page := range a.Pages {
		if !strings.HasPrefix(page.Route, "/") {
			out = append(out, diagnostic("Page", name, "spec.route", "must start with /"))
		}
		if old := routes[page.Route]; old != "" {
			out = append(out, diagnostic("Page", name, "spec.route", "duplicates route used by "+old))
		}
		routes[page.Route] = "Page/" + name
		if _, ok := a.Panels[page.Panel]; !ok {
			out = append(out, diagnostic("Page", name, "spec.panel", "references missing Panel "+page.Panel))
		}
		if page.Policy != "" {
			if _, ok := a.Policies[page.Policy]; !ok {
				out = append(out, diagnostic("Page", name, "spec.policy", "references missing Policy "+page.Policy))
			}
		}
		for key, binding := range page.Context {
			if !map[string]bool{"route": true, "query": true, "user": true, "tenant": true}[binding.Source] {
				out = append(out, diagnostic("Page", name, "spec.context."+key+".source", "has no typed resolver"))
			}
			if binding.Source != "tenant" && binding.Name == "" {
				out = append(out, diagnostic("Page", name, "spec.context."+key+".name", "is required"))
			}
		}
	}
	for name, job := range a.Jobs {
		if _, ok := a.Actions[job.Action]; !ok {
			out = append(out, diagnostic("Job", name, "spec.action", "references missing Action "+job.Action))
		}
	}
	for name, menu := range a.Menus {
		for i, item := range menu.Items {
			if item.Policy != "" {
				if _, ok := a.Policies[item.Policy]; !ok {
					out = append(out, diagnostic("Menu", name, fmt.Sprintf("spec.items.%d.policy", i), "references missing Policy "+item.Policy))
				}
			}
		}
	}
	for name, resource := range a.AdminResources {
		entity, ok := a.Entities[resource.Entity]
		if !ok {
			out = append(out, diagnostic("AdminResource", name, "spec.entity", "references missing Entity "+resource.Entity))
			continue
		}
		viewDefinition, ok := a.Views[resource.View]
		if !ok || viewDefinition.Entity != resource.Entity {
			out = append(out, diagnostic("AdminResource", name, "spec.view", "must reference a View for Entity "+resource.Entity))
			continue
		}
		fields := fieldSet(entity)
		selected := map[string]bool{}
		for _, field := range viewDefinition.Fields {
			selected[field] = true
		}
		checkFields := func(path string, names []string, requireSelected bool) {
			for _, field := range names {
				if !fields[field] {
					out = append(out, diagnostic("AdminResource", name, path, "references missing field "+field))
				} else if requireSelected && !selected[field] {
					out = append(out, diagnostic("AdminResource", name, path, "field "+field+" is not selected by View "+resource.View))
				}
			}
		}
		checkFields("spec.list.columns", resource.List.Columns, true)
		checkFields("spec.list.search", resource.List.Search, true)
		checkFields("spec.list.filters", resource.List.Filters, true)
		checkFields("spec.form.fields", resource.Form.Fields, false)
		checkFields("spec.form.readonly", resource.Form.Readonly, true)
		checkFields("spec.labelField", []string{resource.LabelField}, true)
		for _, order := range resource.List.Sort {
			checkFields("spec.list.sort", []string{order.Field}, true)
		}
		if resource.List.PageSize < 1 || resource.List.PageSize > 200 {
			out = append(out, diagnostic("AdminResource", name, "spec.list.pageSize", "must be between 1 and 200"))
		}
		for path, actionName := range map[string]string{"spec.createAction": resource.CreateAction, "spec.updateAction": resource.UpdateAction, "spec.deleteAction": resource.DeleteAction} {
			action, exists := a.Actions[actionName]
			if !exists || action.Entity != resource.Entity {
				out = append(out, diagnostic("AdminResource", name, path, "must reference an Action for Entity "+resource.Entity))
			}
		}
		for _, actionName := range resource.Actions {
			action, exists := a.Actions[actionName]
			if !exists || action.Entity != resource.Entity {
				out = append(out, diagnostic("AdminResource", name, "spec.actions", "must reference Actions for Entity "+resource.Entity))
			}
		}
	}
	return out
}

func recordFields(expression *expr.Expr) []string {
	if expression == nil {
		return nil
	}
	out := []string{}
	if expression.Left != nil && expression.Left.Source == "record" {
		out = append(out, expression.Left.Name)
	}
	if expression.Right != nil && expression.Right.Source == "record" {
		out = append(out, expression.Right.Name)
	}
	for i := range expression.Args {
		out = append(out, recordFields(&expression.Args[i])...)
	}
	return out
}

func compatibleFormType(formType, fieldType string) bool {
	allowed := map[string][]string{
		"text":             {"string", "text", "richtext", "uuid", "url", "slug"},
		"password":         {"password"},
		"textarea":         {"string", "text", "richtext"},
		"email":            {"email"},
		"number":           {"integer", "money", "decimal"},
		"integer":          {"integer", "money"},
		"checkbox":         {"boolean"},
		"select":           {"enum", "string"},
		"date":             {"date"},
		"datetime":         {"datetime"},
		"entity reference": {"relation", "uuid"},
		"group":            {"json"},
	}
	for _, candidate := range allowed[formType] {
		if candidate == fieldType {
			return true
		}
	}
	return false
}

func hasAssignment(step appir.Step, name string) bool {
	_, ok := assignment(step, name)
	return ok
}

func validViewField(name string, base map[string]bool, relationships map[string]appir.ViewRelationship, a *appir.App) bool {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return base[name]
	}
	if len(parts) != 2 {
		return false
	}
	relationship, ok := relationships[parts[0]]
	if !ok {
		return false
	}
	return fieldSet(a.Entities[relationship.Entity])[parts[1]]
}

func viewFieldType(name string, base appir.Entity, relationships map[string]appir.ViewRelationship, a *appir.App) (string, bool) {
	parts := strings.Split(name, ".")
	entity := base
	fieldName := name
	if len(parts) == 2 {
		relationship, ok := relationships[parts[0]]
		if !ok {
			return "", false
		}
		entity, ok = a.Entities[relationship.Entity]
		if !ok {
			return "", false
		}
		fieldName = parts[1]
	} else if len(parts) != 1 {
		return "", false
	}
	for _, fieldDefinition := range entity.Fields {
		if fieldDefinition.Name == fieldName {
			return fieldDefinition.Type, true
		}
	}
	for _, fieldDefinition := range []appir.Field{{Name: "id", Type: "uuid"}, {Name: "created_at", Type: "datetime"}, {Name: "updated_at", Type: "datetime"}, {Name: "version", Type: "integer"}} {
		if fieldDefinition.Name == fieldName {
			return fieldDefinition.Type, true
		}
	}
	return "", false
}

func usesEntity(op string) bool {
	return map[string]bool{"load": true, "query": true, "assert_no_overlap": true, "create": true, "update": true, "conditional_update": true, "decrement": true, "delete": true, "transition": true}[op]
}

func stepValueFields(op string, entity appir.Entity, action appir.Action) map[string]bool {
	fields := map[string]bool{}
	switch op {
	case "create", "update", "conditional_update", "transition":
		fields["entity"] = true
		if op != "create" {
			fields["id"] = true
			fields["message"] = true
		}
		for _, field := range entity.Fields {
			fields[field.Name] = true
		}
	case "load", "delete":
		fields["entity"] = true
		fields["id"] = true
	case "assert":
		fields["message"] = true
	case "assert_no_overlap":
		for _, field := range []string{"match", "start", "end", "message"} {
			fields[field] = true
		}
	case "decrement":
		for _, field := range []string{"entity", "field", "id_input", "amount_input", "message"} {
			fields[field] = true
		}
	case "emit", "schedule":
		for _, assignment := range action.Output {
			fields[assignment.Name] = true
		}
		// Event and job payloads are explicit JSON objects and may use arbitrary keys.
		for _, step := range action.Steps {
			if step.Op == op {
				for _, assignment := range step.Values {
					fields[assignment.Field] = true
				}
			}
		}
	case "return":
		for name := range action.Output {
			fields[name] = true
		}
	}
	return fields
}

func validateExpr(expression expr.Expr, database bool) error {
	logical := map[string]int{"and": -1, "or": -1, "not": 1}
	if arity, ok := logical[expression.Op]; ok {
		if arity == -1 && len(expression.Args) == 0 {
			return fmt.Errorf("%s requires arguments", expression.Op)
		}
		if arity >= 0 && len(expression.Args) != arity {
			return fmt.Errorf("%s requires %d argument", expression.Op, arity)
		}
		for _, child := range expression.Args {
			if e := validateExpr(child, database); e != nil {
				return e
			}
		}
		return nil
	}
	if !map[string]bool{"eq": true, "ne": true, "gt": true, "gte": true, "lt": true, "lte": true, "in": true, "not_in": true, "contains": true, "starts_with": true, "ends_with": true, "is_null": true, "is_not_null": true}[expression.Op] {
		return fmt.Errorf("unsupported expression operator %q", expression.Op)
	}
	if expression.Left == nil {
		return fmt.Errorf("left value is required")
	}
	if expression.Op != "is_null" && expression.Op != "is_not_null" && expression.Right == nil {
		return fmt.Errorf("right value is required")
	}
	allowedSources := map[string]bool{"literal": true, "input": true, "record": true, "user": true, "tenant": true, "route": true, "context": true}
	if !allowedSources[expression.Left.Source] || expression.Right != nil && !allowedSources[expression.Right.Source] {
		return fmt.Errorf("expression has an unsupported value source")
	}
	if database && expression.Left.Source != "record" {
		return fmt.Errorf("database expression left side must be a record field")
	}
	if database && expression.Right != nil && expression.Right.Source == "record" {
		return fmt.Errorf("database expression right side cannot be a record field")
	}
	return nil
}

func validateRegistrationPage(a *appir.App, route, actionName string) string {
	var registrationPage *appir.Page
	for _, pageDefinition := range a.Pages {
		if pageDefinition.Route == route {
			copy := pageDefinition
			registrationPage = &copy
			break
		}
	}
	if registrationPage == nil {
		return "must reference a Page containing a Webform for the registration Action"
	}
	panelDefinition := a.Panels[registrationPage.Panel]
	anonymous := beanctx.Request{}
	if _, err := page.ResolveContext(*registrationPage, map[string]string{}, map[string]string{}, anonymous); err != nil {
		return "must resolve Page context for an anonymous request to the advertised static route"
	}
	if registrationPage.Policy != "" && !policy.Can(a.Policies[registrationPage.Policy], false, anonymous, nil) || panelDefinition.Policy != "" && !policy.Can(a.Policies[panelDefinition.Policy], false, anonymous, nil) {
		return "must reference a Page and Panel accessible to anonymous users"
	}
	actionDefinition := a.Actions[actionName]
	var missing []string
	found := false
	for _, region := range panelDefinition.Regions {
		for _, blockName := range region.Blocks {
			blockDefinition := a.Blocks[blockName]
			formDefinition := a.Webforms[blockDefinition.Webform]
			if blockDefinition.Type != "webform" || formDefinition.Action != actionName {
				continue
			}
			found = true
			if blockDefinition.Policy != "" && !policy.Can(a.Policies[blockDefinition.Policy], false, anonymous, nil) {
				continue
			}
			fields := map[string]bool{}
			for _, element := range formDefinition.Elements {
				fields[element.Name] = element.Required && element.Visible == nil
			}
			missing = missing[:0]
			for inputName, inputDefinition := range actionDefinition.Input {
				if inputDefinition.Required && !fields[inputName] {
					missing = append(missing, inputName)
				}
			}
			if len(missing) == 0 {
				return ""
			}
			sort.Strings(missing)
		}
	}
	if !found {
		return "must reference a Page containing a Webform for the registration Action"
	}
	if len(missing) > 0 {
		return "Webform must unconditionally collect required registration inputs: " + strings.Join(missing, ", ")
	}
	return "must reference a registration Webform Block accessible to anonymous users"
}

func validateForm(name string, form appir.Webform) []definition.Diagnostic {
	allowed := map[string]bool{"text": true, "textarea": true, "email": true, "password": true, "number": true, "integer": true, "checkbox": true, "select": true, "date": true, "datetime": true, "entity reference": true, "group": true}
	out := []definition.Diagnostic{}
	seen := map[string]bool{}
	var walk func([]appir.FormElement, string)
	walk = func(elements []appir.FormElement, path string) {
		for i, element := range elements {
			p := fmt.Sprintf("%s.%d", path, i)
			if element.Name == "" {
				out = append(out, diagnostic("Webform", name, p+".name", "is required"))
			} else if seen[element.Name] {
				out = append(out, diagnostic("Webform", name, p+".name", "duplicates another element"))
			}
			seen[element.Name] = true
			if !allowed[element.Type] {
				out = append(out, diagnostic("Webform", name, p+".type", "has no server and UI implementation"))
			}
			if element.Type == "group" {
				if len(element.Children) == 0 {
					out = append(out, diagnostic("Webform", name, p+".children", "repeating group requires children"))
				}
				walk(element.Children, p+".children")
			} else if len(element.Children) > 0 {
				out = append(out, diagnostic("Webform", name, p+".children", "is only valid for group elements"))
			}
			for conditionPath, condition := range map[string]*expr.Expr{"visible": element.Visible, "requiredWhen": element.RequiredWhen} {
				if condition != nil {
					if er := validateExpr(*condition, false); er != nil {
						out = append(out, diagnostic("Webform", name, p+"."+conditionPath, er.Error()))
					}
				}
			}
		}
	}
	walk(form.Elements, "spec.elements")
	stepUse := map[string]int{}
	for i, step := range form.Steps {
		for _, element := range step {
			stepUse[element]++
			if !seen[element] {
				out = append(out, diagnostic("Webform", name, fmt.Sprintf("spec.steps.%d", i), "references missing element "+element))
			}
		}
	}
	if len(form.Steps) > 0 {
		for _, element := range form.Elements {
			if stepUse[element.Name] != 1 {
				out = append(out, diagnostic("Webform", name, "spec.steps", "must include element "+element.Name+" exactly once"))
			}
		}
	}
	return out
}
func fieldSet(e appir.Entity) map[string]bool {
	m := map[string]bool{"id": true, "created_at": true, "updated_at": true, "version": true}
	if e.Owner {
		m["owner_id"] = true
	}
	if e.Tenant {
		m["tenant_id"] = true
	}
	if e.SoftDelete {
		m["deleted_at"] = true
	}
	for _, f := range e.Fields {
		m[f.Name] = true
	}
	return m
}

func nameSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
func generate(a *appir.App, name string, e appir.Entity) {
	fields := []string{"id"}
	for _, f := range e.Fields {
		fields = append(fields, f.Name)
	}
	fields = append(fields, "created_at", "updated_at", "version")
	if _, ok := a.Views[name+"_list"]; !ok {
		a.Views[name+"_list"] = appir.View{Name: name + "_list", Entity: name, Fields: fields, Policy: e.Policy, DefaultLimit: 50, MaxLimit: 200}
	}
	for _, op := range []string{"create", "update", "delete"} {
		n := name + "_" + op
		if _, ok := a.Actions[n]; !ok {
			a.Actions[n] = appir.Action{Name: n, Entity: name, Operation: op, Policy: e.Policy}
		}
	}
	for _, resource := range a.AdminResources {
		if resource.Entity == name {
			return
		}
	}
	a.AdminResources[name] = appir.AdminResource{Name: name, Entity: name}
}

func normalizeAdminResources(a *appir.App) {
	for name, resource := range a.AdminResources {
		entity, ok := a.Entities[resource.Entity]
		if !ok {
			continue
		}
		if resource.Label == "" {
			resource.Label = entity.Label
		}
		if resource.View == "" {
			resource.View = resource.Entity + "_list"
		}
		if resource.CreateAction == "" {
			resource.CreateAction = resource.Entity + "_create"
		}
		if resource.UpdateAction == "" {
			resource.UpdateAction = resource.Entity + "_update"
		}
		if resource.DeleteAction == "" {
			resource.DeleteAction = resource.Entity + "_delete"
		}
		if resource.LabelField == "" {
			resource.LabelField = "id"
			for _, candidate := range []string{"title", "name", "email"} {
				for _, field := range entity.Fields {
					if field.Name == candidate {
						resource.LabelField = candidate
						break
					}
				}
				if resource.LabelField != "id" {
					break
				}
			}
		}
		if len(resource.List.Columns) == 0 {
			resource.List.Columns = []string{"id"}
			for i, field := range entity.Fields {
				if i == 4 {
					break
				}
				resource.List.Columns = append(resource.List.Columns, field.Name)
			}
			resource.List.Columns = append(resource.List.Columns, "updated_at")
		}
		if len(resource.List.Search) == 0 {
			for _, field := range entity.Fields {
				if field.Type == "string" || field.Type == "text" || field.Type == "richtext" || field.Type == "email" || field.Type == "url" {
					resource.List.Search = append(resource.List.Search, field.Name)
				}
			}
		}
		if len(resource.List.Filters) == 0 {
			for _, field := range entity.Fields {
				if field.Type == "enum" || field.Type == "boolean" || field.Type == "relation" {
					resource.List.Filters = append(resource.List.Filters, field.Name)
				}
			}
		}
		if resource.List.PageSize == 0 {
			resource.List.PageSize = 25
		}
		if len(resource.Form.Fields) == 0 {
			for _, field := range entity.Fields {
				resource.Form.Fields = append(resource.Form.Fields, field.Name)
			}
		}
		if len(resource.Form.Readonly) == 0 {
			resource.Form.Readonly = []string{"created_at", "updated_at", "version"}
		}
		if resource.List.Columns == nil {
			resource.List.Columns = []string{}
		}
		if resource.List.Search == nil {
			resource.List.Search = []string{}
		}
		if resource.List.Filters == nil {
			resource.List.Filters = []string{}
		}
		if resource.List.Sort == nil {
			resource.List.Sort = []appir.Sort{}
		}
		if resource.Form.Fields == nil {
			resource.Form.Fields = []string{}
		}
		if resource.Form.Readonly == nil {
			resource.Form.Readonly = []string{}
		}
		if resource.Actions == nil {
			resource.Actions = []string{}
		}
		a.AdminResources[name] = resource
	}
}

func normalizeResourceListBlocks(a *appir.App) {
	for name, block := range a.Blocks {
		if block.Type != "resource-list" {
			continue
		}
		if resource, ok := a.AdminResources[block.Resource]; ok {
			block.View = resource.View
		}
		if block.Filters == nil {
			block.Filters = []string{}
		}
		if block.DefaultFilters == nil {
			block.DefaultFilters = map[string]any{}
		}
		a.Blocks[name] = block
	}
}
