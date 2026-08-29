package block

import (
	"fmt"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
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
	switch b.Type {
	case "text":
		props["text"] = b.Text
	case "view":
		props["view"] = b.View
	case "entity":
		props["entity"] = b.Entity
	case "webform":
		props["webform"] = b.Webform
	case "action":
		props["action"] = b.Action
	case "menu":
		items := []appir.MenuItem{}
		for _, item := range a.Menus[b.Menu].Items {
			if item.Policy == "" || policy.Can(a.Policies[item.Policy], false, c, nil) {
				items = append(items, item)
			}
		}
		props["items"] = items
	}
	return render.Node{Component: component(b.Type), Props: props}, true, nil
}
func component(t string) string {
	switch t {
	case "text":
		return "TextBlock"
	case "view":
		return "ViewBlock"
	case "entity":
		return "EntityBlock"
	case "webform":
		return "WebformBlock"
	case "action":
		return "ActionBlock"
	case "menu":
		return "MenuBlock"
	}
	return "UnknownBlock"
}
