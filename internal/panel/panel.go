package panel

import (
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
	for _, r := range p.Regions {
		blocks := []render.Node{}
		for _, name := range r.Blocks {
			node, allowed, e := block.Node(a, a.Blocks[name], ctx, c)
			if e != nil {
				return render.Node{}, false, e
			}
			if allowed {
				blocks = append(blocks, node)
			}
		}
		children = append(children, render.Node{Component: "Region", Props: map[string]any{"name": r.Name}, Children: blocks})
	}
	return render.Node{Component: "Panel", Props: map[string]any{"layout": p.Layout}, Children: children}, true, nil
}
