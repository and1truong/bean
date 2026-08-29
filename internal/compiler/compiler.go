package compiler

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/migration"
)

type Result struct {
	App         *appir.App
	Schema      migration.Schema
	Diagnostics []definition.Diagnostic
}

func Compile(appID string, version int, defs []definition.Definition) Result {
	a := appir.Empty()
	a.AppID = appID
	a.Version = version
	a.CreatedAt = time.Now().UTC()
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
			if x.DefaultLimit == 0 {
				x.DefaultLimit = 50
			}
			if x.MaxLimit == 0 {
				x.MaxLimit = 200
			}
			a.Views[x.Name] = x
		case "Action":
			var x appir.Action
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Actions[x.Name] = x
		case "Policy":
			var x appir.Policy
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Policies[x.Name] = x
		case "Webform":
			var x appir.Webform
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Webforms[x.Name] = x
		case "Block":
			var x appir.Block
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Blocks[x.Name] = x
		case "Panel":
			var x appir.Panel
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Panels[x.Name] = x
		case "Page":
			var x appir.Page
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Pages[x.Name] = x
		case "Role":
			var x appir.Role
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Roles[x.Name] = x
		case "Menu":
			var x appir.Menu
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Menus[x.Name] = x
		case "Job":
			var x appir.Job
			_ = definition.DecodeSpec(d.Spec, &x)
			x.Name = d.Metadata.Name
			a.Jobs[x.Name] = x
		}
	}
	if len(r.Diagnostics) > 0 {
		return r
	}
	r.Diagnostics = append(r.Diagnostics, validate(a)...)
	if len(r.Diagnostics) > 0 {
		return r
	}
	for name, e := range a.Entities {
		generate(a, name, e)
	}
	for _, e := range a.Entities {
		me := migration.Entity{Name: e.Name, Indexes: e.Indexes, Unique: e.Unique}
		for _, f := range e.Fields {
			mf := migration.Field{Name: f.Name, Type: f.Type, Required: f.Required, Unique: f.Unique}
			if f.Relation != nil {
				mf.RelationEntity, mf.RelationKind = f.Relation.Entity, f.Relation.Kind
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
func diagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.Diagnostic{Kind: kind, Name: name, Path: path, Message: message}
}
func validate(a *appir.App) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	routes := map[string]string{}
	for name, v := range a.Views {
		e, ok := a.Entities[v.Entity]
		if !ok {
			out = append(out, diagnostic("View", name, "spec.entity", "references missing Entity "+v.Entity))
			continue
		}
		fields := fieldSet(e)
		for _, f := range v.Fields {
			if !fields[f] {
				out = append(out, diagnostic("View", name, "spec.fields", "references missing field "+f))
			}
		}
		if v.MaxLimit > 200 || v.DefaultLimit > v.MaxLimit {
			out = append(out, diagnostic("View", name, "spec.maxLimit", "must be between the default and 200"))
		}
		for displayName, display := range v.Displays {
			if display.Route == "" {
				continue
			}
			if old := routes[display.Route]; old != "" {
				out = append(out, diagnostic("View", name, "spec.displays."+displayName+".route", "duplicates route used by "+old))
			}
			routes[display.Route] = "View/" + name
		}
	}
	allowedActions := map[string]bool{"create": true, "update": true, "delete": true, "transition": true, "transaction": true}
	allowedSteps := map[string]bool{"load": true, "query": true, "assert": true, "assert_no_overlap": true, "create": true, "update": true, "conditional_update": true, "decrement": true, "delete": true, "transition": true, "emit": true, "schedule": true, "return": true}
	allowedRelations := map[string]bool{"one-to-one": true, "one-to-many": true, "many-to-one": true, "many-to-many": true}
	for name, entity := range a.Entities {
		for i, field := range entity.Fields {
			if field.Type != "relation" {
				continue
			}
			if field.Relation == nil {
				continue
			}
			if !allowedRelations[field.Relation.Kind] {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation", i), "relation kind is invalid"))
				continue
			}
			if _, ok := a.Entities[field.Relation.Entity]; !ok {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.entity", i), "references missing Entity "+field.Relation.Entity))
			}
		}
	}
	for name, action := range a.Actions {
		if _, ok := a.Entities[action.Entity]; !ok {
			out = append(out, diagnostic("Action", name, "spec.entity", "references missing Entity "+action.Entity))
		}
		if !allowedActions[action.Operation] {
			out = append(out, diagnostic("Action", name, "spec.operation", "invalid Action operation"))
		}
		for i, step := range action.Steps {
			if !allowedSteps[step.Op] {
				out = append(out, diagnostic("Action", name, fmt.Sprintf("spec.steps.%d", i), "invalid Action step"))
			}
		}
	}
	for name, form := range a.Webforms {
		if _, ok := a.Actions[form.Action]; !ok {
			out = append(out, diagnostic("Webform", name, "spec.action", "references missing Action "+form.Action))
		}
	}
	for name, block := range a.Blocks {
		refs := []struct{ kind, value string }{{"view", block.View}, {"entity", block.Entity}, {"webform", block.Webform}, {"action", block.Action}}
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
			}
			if !ok {
				out = append(out, diagnostic("Block", name, "spec."+ref.kind, "invalid Block input reference "+ref.value))
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
func generate(a *appir.App, name string, e appir.Entity) {
	fields := []string{"id"}
	for _, f := range e.Fields {
		fields = append(fields, f.Name)
	}
	if _, ok := a.Views[name+"_list"]; !ok {
		a.Views[name+"_list"] = appir.View{Name: name + "_list", Entity: name, Fields: fields, Policy: e.Policy, DefaultLimit: 50, MaxLimit: 200}
	}
	inputs := map[string]appir.Field{}
	for _, f := range e.Fields {
		inputs[f.Name] = f
	}
	for _, op := range []string{"create", "update", "delete"} {
		n := name + "_" + op
		if _, ok := a.Actions[n]; !ok {
			a.Actions[n] = appir.Action{Name: n, Entity: name, Operation: op, Policy: e.Policy, Input: inputs}
		}
	}
}
