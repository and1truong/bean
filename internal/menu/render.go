package menu

import (
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/policy"
)

type RenderItem struct {
	ID, Label, Route string
	Weight, Level    int
	Current, Active  bool
	Children         []RenderItem `json:",omitempty"`
}

func StaticTree(app *appir.App, definition appir.Menu, request beanctx.Request) []RenderItem {
	if definition.Profile == "" {
		out := []RenderItem{}
		for _, item := range definition.Items {
			if item.Policy != "" && !policy.Can(app.Policies[item.Policy], false, request, nil) {
				continue
			}
			resolved, allowed := resolveStaticTarget(app, item, request)
			if allowed {
				out = append(out, renderItem(resolved, request.Route, 1, nil))
			}
		}
		return out
	}
	visible := map[string]appir.MenuItem{}
	for _, item := range definition.Items {
		if item.Policy != "" && !policy.Can(app.Policies[item.Policy], false, request, nil) {
			continue
		}
		resolved, allowed := resolveStaticTarget(app, item, request)
		if !allowed {
			continue
		}
		if resolved.Label == "" {
			resolved.Label = resolved.ID
		}
		visible[resolved.ID] = resolved
	}
	children := map[string][]appir.MenuItem{}
	for _, item := range visible {
		if item.Parent != "" {
			if _, parentVisible := visible[item.Parent]; !parentVisible {
				continue
			}
		}
		children[item.Parent] = append(children[item.Parent], item)
	}
	for parent := range children {
		sort.Slice(children[parent], func(i, j int) bool {
			if children[parent][i].Weight != children[parent][j].Weight {
				return children[parent][i].Weight < children[parent][j].Weight
			}
			return children[parent][i].ID < children[parent][j].ID
		})
	}
	return renderChildren(children, "", request.Route, 1)
}

func resolveStaticTarget(app *appir.App, item appir.MenuItem, request beanctx.Request) (appir.MenuItem, bool) {
	if item.Target.Page != "" {
		page, exists := app.Pages[item.Target.Page]
		if !exists || page.Policy != "" && !policy.Can(app.Policies[page.Policy], false, request, nil) {
			return item, false
		}
		item.Route = page.Route
		if item.Label == "" {
			item.Label = page.Title
			if item.Label == "" {
				item.Label = page.Name
			}
		}
		return item, true
	}
	if item.Target.View != "" {
		viewDefinition, exists := app.Views[item.Target.View]
		display, displayExists := viewDefinition.Displays[item.Target.Display]
		if !exists || !displayExists {
			return item, false
		}
		policyName := policy.EffectiveViewPolicyName(viewDefinition, app.Entities[viewDefinition.Entity])
		if policyName != "" && !policy.Can(app.Policies[policyName], false, request, nil) {
			return item, false
		}
		item.Route = display.Route
		if item.Label == "" {
			item.Label = display.Title.Text
			if item.Label == "" {
				item.Label = display.Title.Fallback
			}
			if item.Label == "" {
				item.Label = item.Target.Display
			}
		}
		return item, true
	}
	return item, item.Route != ""
}

func renderChildren(children map[string][]appir.MenuItem, parent, route string, level int) []RenderItem {
	out := make([]RenderItem, 0, len(children[parent]))
	for _, item := range children[parent] {
		descendants := renderChildren(children, item.ID, route, level+1)
		out = append(out, renderItem(item, route, level, descendants))
	}
	return out
}

func renderItem(item appir.MenuItem, route string, level int, children []RenderItem) RenderItem {
	itemPath, _, _ := strings.Cut(item.Route, "?")
	current := itemPath == route
	active := current
	for _, child := range children {
		active = active || child.Active
	}
	return RenderItem{ID: item.ID, Label: item.Label, Route: item.Route, Weight: item.Weight, Level: level, Current: current, Active: active, Children: children}
}
