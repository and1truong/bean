// Package sequence owns ordered composition semantics shared by sequence profiles.
package sequence

import (
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/panel"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/render"
)

const (
	MinFrames         = 1
	MaxFrames         = 50
	MaxTitleRunes     = 80
	MaxNotesBytes     = 4000
	MinBlocksPerFrame = 1
	MaxBlocksPerFrame = 12
	BaseContentBudget = 700
)

func Profiles() []string { return []string{"presentation"} }

func AspectRatios() []string { return []string{"standard", "wide"} }

func Layouts() []string {
	return []string{"architecture", "bullets", "chart-focus", "closing", "comparison", "image-focus", "process", "quote", "section", "statement", "table", "timeline", "title", "two-column"}
}

func PanelLayoutAllowed(frameLayout, panelLayout string) bool {
	switch frameLayout {
	case "title", "section", "statement", "bullets", "quote", "closing":
		return panelLayout == "single-column"
	case "two-column", "comparison":
		return panelLayout == "two-column"
	default:
		return panelLayout == "single-column" || panelLayout == "two-column"
	}
}

func ContentBudget(layout string) int {
	switch layout {
	case "title":
		return 260
	case "section", "closing":
		return 320
	case "statement", "quote":
		return 420
	case "two-column", "comparison":
		return 875
	default:
		return BaseContentBudget
	}
}

func Match(app *appir.App, path string) (appir.Sequence, bool) {
	items := make([]appir.Sequence, 0, len(app.Sequences))
	for _, item := range app.Sequences {
		items = append(items, item)
	}
	sort.Slice(items, func(left, right int) bool {
		if items[left].Route != items[right].Route {
			return items[left].Route < items[right].Route
		}
		return items[left].Name < items[right].Name
	})
	for _, item := range items {
		if item.Route == path {
			return item, true
		}
	}
	return appir.Sequence{}, false
}

func Protected(app *appir.App, item appir.Sequence) bool {
	if item.Policy != "" {
		return true
	}
	for _, frame := range item.Frames {
		panelDefinition := app.Panels[frame.Panel]
		if panelDefinition.Policy != "" {
			return true
		}
		for _, region := range panelDefinition.Regions {
			for _, item := range region.OrderedItems() {
				blockDefinition, exists := item.ResolveBlock(app)
				if exists && blockDefinition.Policy != "" {
					return true
				}
			}
		}
	}
	return false
}

func Node(app *appir.App, item appir.Sequence, request beanctx.Request) (render.Node, bool, error) {
	if item.Policy != "" && !policy.Can(app.Policies[item.Policy], false, request, nil) {
		return render.Node{}, false, nil
	}
	children := []render.Node{}
	for _, frame := range item.Frames {
		child, allowed, err := panel.Node(app, app.Panels[frame.Panel], map[string]any{}, request)
		if err != nil {
			return render.Node{}, false, err
		}
		if !allowed {
			continue
		}
		visible := false
		for _, region := range child.Children {
			visible = visible || len(region.Children) > 0
		}
		if !visible {
			continue
		}
		children = append(children, render.Node{Component: "SequenceFrame", Props: map[string]any{"name": frame.Name, "title": frame.Title, "layout": frame.Layout, "notes": frame.Notes}, Children: []render.Node{child}})
	}
	if len(children) == 0 {
		return render.Node{}, false, nil
	}
	return render.Node{Component: "Sequence", Props: map[string]any{"title": item.Title, "description": item.Description, "profile": item.Profile, "aspectRatio": item.AspectRatio, "protected": Protected(app, item)}, Children: children}, true, nil
}
