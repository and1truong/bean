package page

import (
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/panel"
	"github.com/beanruntime/bean/internal/render"
	"strings"
)

func Match(a *appir.App, path string) (appir.Page, map[string]string, bool) {
	for _, p := range a.Pages {
		pp := strings.Split(strings.Trim(p.Route, "/"), "/")
		actual := strings.Split(strings.Trim(path, "/"), "/")
		if len(pp) != len(actual) {
			continue
		}
		params := map[string]string{}
		ok := true
		for i, v := range pp {
			if strings.HasPrefix(v, ":") {
				params[strings.TrimPrefix(v, ":")] = actual[i]
			} else if v != actual[i] {
				ok = false
			}
		}
		if ok {
			return p, params, true
		}
	}
	return appir.Page{}, nil, false
}
func Node(a *appir.App, p appir.Page, ctx map[string]any) render.Node {
	return render.Node{Component: "Page", Props: map[string]any{"title": p.Title, "description": p.Description}, Children: []render.Node{panel.Node(a, a.Panels[p.Panel], ctx)}}
}
