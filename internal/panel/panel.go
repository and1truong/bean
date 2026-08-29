package panel

import (
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	"github.com/beanruntime/bean/internal/render"
)

func Node(a *appir.App, p appir.Panel, ctx map[string]any) render.Node {
	children := []render.Node{}
	for _, r := range p.Regions {
		blocks := []render.Node{}
		for _, name := range r.Blocks {
			blocks = append(blocks, block.Node(a.Blocks[name], ctx))
		}
		children = append(children, render.Node{Component: "Region", Props: map[string]any{"name": r.Name}, Children: blocks})
	}
	return render.Node{Component: "Panel", Props: map[string]any{"layout": p.Layout}, Children: children}
}
