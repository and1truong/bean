package compiler

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/policy"
)

const maxFieldLayoutGroups = 16
const maxFieldGroupFields = 64
const maxFieldLayoutFields = 128

var fieldGroupName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// Preserve omission defaults without accepting explicit zero/null/empty values.
func validateSourceFieldLayouts(source definition.Definition) []definition.Diagnostic {
	var out []definition.Diagnostic
	check := func(parent map[string]any, path string) {
		raw, exists := parent["layout"]
		if !exists {
			return
		}
		layout, ok := raw.(map[string]any)
		if !ok {
			out = append(out, diagnostic(source.Kind, source.Metadata.Name, path, "must be a layout object"))
			return
		}
		groups, _ := layout["groups"].([]any)
		for i, rawGroup := range groups {
			group, _ := rawGroup.(map[string]any)
			if columns, present := group["columns"]; present && (columns == nil || columns == 0 || columns == float64(0)) {
				out = append(out, diagnostic(source.Kind, source.Metadata.Name, fmt.Sprintf("%s.groups.%d.columns", path, i), "must be 1 or 2"))
			}
			fields, _ := group["fields"].([]any)
			for j, rawField := range fields {
				field, _ := rawField.(map[string]any)
				if span, present := field["span"]; present && (span == nil || span == "") {
					out = append(out, diagnostic(source.Kind, source.Metadata.Name, fmt.Sprintf("%s.groups.%d.fields.%d.span", path, i, j), "must be single or full"))
				}
			}
		}
	}
	switch source.Kind {
	case "AdminResource":
		form, _ := source.Spec["form"].(map[string]any)
		check(form, "spec.form.layout")
	case "View":
		displays, _ := source.Spec["displays"].(map[string]any)
		for _, name := range keys(displays) {
			display, _ := displays[name].(map[string]any)
			renderer, _ := display["renderer"].(map[string]any)
			check(renderer, "spec.displays."+name+".renderer.layout")
		}
	}
	return out
}

func normalizeFieldLayout(layout *appir.FieldLayout) {
	if layout == nil {
		return
	}
	for i := range layout.Groups {
		group := &layout.Groups[i]
		if group.Columns == 0 {
			group.Columns = 1
		}
		for j := range group.Fields {
			if group.Fields[j].Span == "" {
				group.Fields[j].Span = "single"
			}
		}
	}
}

func validateFieldLayout(kind, name, path string, layout *appir.FieldLayout, allowed map[string]bool, required []string) []definition.Diagnostic {
	if layout == nil {
		return nil
	}
	var out []definition.Diagnostic
	bad := func(p, message string) { out = append(out, diagnostic(kind, name, p, message)) }
	if len(layout.Groups) < 1 || len(layout.Groups) > maxFieldLayoutGroups {
		bad(path+".groups", "must contain between 1 and 16 groups")
	}
	names, seen := map[string]bool{}, map[string]bool{}
	count := 0
	for i, group := range layout.Groups {
		p := fmt.Sprintf("%s.groups.%d", path, i)
		if !fieldGroupName.MatchString(group.Name) || names[group.Name] {
			bad(p+".name", "must be a unique lowercase group name of at most 64 characters")
		}
		names[group.Name] = true
		if strings.TrimSpace(group.Label) == "" || utf8.RuneCountInString(group.Label) > 120 {
			bad(p+".label", "must contain a nonempty label of at most 120 characters")
		}
		if group.Columns != 1 && group.Columns != 2 {
			bad(p+".columns", "must be 1 or 2")
		}
		if len(group.Fields) < 1 || len(group.Fields) > maxFieldGroupFields {
			bad(p+".fields", "must contain between 1 and 64 fields")
		}
		count += len(group.Fields)
		for j, item := range group.Fields {
			fp := fmt.Sprintf("%s.fields.%d", p, j)
			if !allowed[item.Field] {
				bad(fp+".field", "must reference an eligible field of this renderer")
			}
			if seen[item.Field] {
				bad(fp+".field", "field must appear only once in a layout")
			}
			seen[item.Field] = true
			if item.Span != "single" && item.Span != "full" {
				bad(fp+".span", "must be single or full")
			}
		}
	}
	if count > maxFieldLayoutFields {
		bad(path+".groups", "must contain at most 128 fields in total")
	}
	for _, field := range required {
		if !seen[field] {
			bad(path+".groups", "must include configured form field "+field)
		}
	}
	return out
}

func validateDetailLayout(name, path string, view appir.View, renderer appir.ViewRenderer, app *appir.App) []definition.Diagnostic {
	if renderer.Layout == nil {
		return nil
	}
	var out []definition.Diagnostic
	if renderer.Type != "detail" {
		out = append(out, diagnostic("View", name, path, "field layout requires a detail renderer"))
	}
	if renderer.TitleField != "" || renderer.BodyField != "" || len(renderer.MetaFields) > 0 || renderer.LinkRoute != "" || renderer.LinkField != "" {
		out = append(out, diagnostic("View", name, path, "layout cannot be combined with titleField, bodyField, metaFields, linkRoute or linkField; use Display.title for the record heading"))
	}
	entity := app.Entities[view.Entity]
	redacted := nameSet(app.Policies[policy.EffectiveViewPolicyName(view, entity)].Redact)
	for _, field := range entity.Fields {
		if field.Sensitive || field.Type == "password" {
			redacted[field.Name] = true
		}
	}
	relationships := map[string]appir.ViewRelationship{}
	for _, r := range view.Relationships {
		relationships[r.Name] = r
	}
	allowed := map[string]bool{}
	for _, field := range view.Fields {
		// First slice supports stable base-record fields only. To-many projected
		// values need a separate collection contract rather than lossy string joins.
		if !strings.Contains(field, ".") && !redacted[field] && stableViewField(field, entity, relationships, app) {
			allowed[field] = true
		}
	}
	out = append(out, validateFieldLayout("View", name, path, renderer.Layout, allowed, nil)...)
	return out
}

func fieldLayoutSchemaDefinitions(definitions map[string]any) {
	for name, raw := range definitions {
		schema := raw.(map[string]any)
		properties := schema["properties"].(map[string]any)
		switch {
		case strings.HasSuffix(name, "internal_appir_FieldLayout"):
			schema["required"] = []string{"groups"}
			groups := properties["groups"].(map[string]any)
			groups["minItems"] = 1
			groups["maxItems"] = maxFieldLayoutGroups
		case strings.HasSuffix(name, "internal_appir_FieldGroup"):
			schema["required"] = []string{"name", "label", "fields"}
			properties["name"] = map[string]any{"type": "string", "pattern": fieldGroupName.String()}
			properties["label"] = map[string]any{"type": "string", "minLength": 1, "maxLength": 120, "pattern": `\S`}
			properties["columns"] = map[string]any{"type": "integer", "enum": []int{1, 2}}
			fields := properties["fields"].(map[string]any)
			fields["minItems"] = 1
			fields["maxItems"] = maxFieldGroupFields
		case strings.HasSuffix(name, "internal_appir_LayoutField"):
			schema["required"] = []string{"field"}
			properties["field"] = map[string]any{"type": "string", "minLength": 1}
			properties["span"] = map[string]any{"type": "string", "enum": []string{"single", "full"}}
		}
	}
}
