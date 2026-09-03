package panel

import (
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/render"
)

func Node(a *appir.App, p appir.Panel, ctx map[string]any, c beanctx.Request) (render.Node, bool, error) {
	if p.Policy != "" && !policy.Can(a.Policies[p.Policy], false, c, nil) {
		return render.Node{}, false, nil
	}
	children := []render.Node{}
	collapsed := false
	for _, r := range p.Regions {
		blocks := []render.Node{}
		for _, item := range r.OrderedItems() {
			definition, exists := item.ResolveBlock(a)
			if !exists {
				return render.Node{}, false, fmt.Errorf("Panel %s region %s contains an unresolved Block", p.Name, r.Name)
			}
			node, allowed, e := block.Node(a, definition, ctx, c)
			if e != nil {
				return render.Node{}, false, e
			}
			if allowed {
				blocks = append(blocks, node)
			}
		}
		if len(blocks) == 0 && r.CollapseWhenEmpty {
			collapsed = true
			continue
		}
		children = append(children, render.Node{Component: "Region", Props: map[string]any{"name": r.Name}, Children: blocks})
	}
	if collapsed && len(children) == 0 {
		return render.Node{}, false, nil
	}
	if collapsed && len(children) == 1 && len(p.Regions) > 1 {
		children[0].Props["expanded"] = true
	}
	return render.Node{Component: "Panel", Props: map[string]any{"layout": p.Layout}, Children: children}, true, nil
}
