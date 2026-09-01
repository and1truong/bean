package block

import (
	"fmt"
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/render"
)

func Node(a *appir.App, b appir.Block, ctx map[string]any, c beanctx.Request) (render.Node, bool, error) {
	if b.Policy != "" && !policy.Can(a.Policies[b.Policy], false, c, nil) {
		return render.Node{}, false, nil
	}
	props := map[string]any{"name": b.Name, "type": b.Type, "context": ctx}
	inputs := map[string]any{}
	for name, definition := range b.Inputs {
		binding, exists := b.Bindings[name]
		var value any
		if exists {
			switch binding.Source {
			case "context":
				value = ctx[binding.Name]
			case "tenant":
				value = c.TenantID
			case "user":
				if c.User != nil && binding.Name == "id" {
					value = c.User.ID
				}
			}
		}
		if definition.Required && (value == nil || value == "") {
			return render.Node{}, false, fmt.Errorf("required Block input %s is missing", name)
		}
		if value != nil {
			definition.Name = name
			if e := field.Validate(definition, value); e != nil {
				return render.Node{}, false, fmt.Errorf("invalid Block input %s", name)
			}
		}
		inputs[name] = value
	}
	props["inputs"] = inputs
	capability, exists := capabilities.Lookup(b.Type)
	if !exists {
		return render.Node{Component: "UnknownBlock", Props: props}, true, nil
	}
	if err := capability.BuildProperties(a, b, c, props); err != nil {
		return render.Node{}, false, err
	}
	return render.Node{Component: capability.Component, Props: props}, true, nil
}
func bindFormElements(elements []appir.FormElement, c beanctx.Request) ([]appir.FormElement, error) {
	bound := make([]appir.FormElement, len(elements))
	for i, element := range elements {
		bound[i] = element
		var err error
		if element.Visible != nil {
			condition, bindErr := expr.BindContext(*element.Visible, c)
			if bindErr != nil {
				return nil, bindErr
			}
			bound[i].Visible = &condition
		}
		if element.RequiredWhen != nil {
			condition, bindErr := expr.BindContext(*element.RequiredWhen, c)
			if bindErr != nil {
				return nil, bindErr
			}
			bound[i].RequiredWhen = &condition
		}
		bound[i].Children, err = bindFormElements(element.Children, c)
		if err != nil {
			return nil, err
		}
	}
	return bound, nil
}

func fileFields(a *appir.App, b appir.Block) []string {
	viewDefinition, exists := a.Views[b.View]
	if !exists {
		return []string{}
	}
	selected := map[string]bool{}
	for _, name := range viewDefinition.Fields {
		selected[name] = true
	}
	out := []string{}
	for _, definition := range a.Entities[viewDefinition.Entity].Fields {
		if definition.Type == "file" && selected[definition.Name] {
			out = append(out, definition.Name)
		}
	}
	return out
}

func formattedFields(a *appir.App, b appir.Block) []string {
	viewDefinition, exists := a.Views[b.View]
	if !exists {
		return []string{}
	}
	trusted := map[string]bool{}
	for fieldName := range viewDefinition.FieldFilters {
		trusted[fieldName] = true
	}
	entity := a.Entities[viewDefinition.Entity]
	for _, legacy := range b.Presentation.RichTextFields {
		for _, fieldDefinition := range entity.Fields {
			if fieldDefinition.Name == legacy && fieldDefinition.Type == "richtext" {
				trusted[legacy] = true
			}
		}
	}
	out := make([]string, 0, len(trusted))
	for fieldName := range trusted {
		out = append(out, fieldName)
	}
	sort.Strings(out)
	return out
}
