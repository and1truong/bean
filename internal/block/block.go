package block

import (
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/render"
)

func Node(b appir.Block, ctx map[string]any) render.Node {
	props := map[string]any{"name": b.Name, "type": b.Type, "context": ctx}
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
		props["menu"] = b.Menu
	}
	return render.Node{Component: component(b.Type), Props: props}
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
