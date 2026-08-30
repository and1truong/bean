package page

import (
	"fmt"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/panel"
	"github.com/beanruntime/bean/internal/render"
)

func ResolveContext(p appir.Page, route, query map[string]string, c beanctx.Request) (map[string]any, error) {
	out := map[string]any{}
	for key, binding := range p.Context {
		var value any
		switch binding.Source {
		case "route":
			value = route[binding.Name]
		case "query":
			value = query[binding.Name]
		case "tenant":
			value = c.TenantID
		case "user":
			if c.User != nil {
				if binding.Name == "id" {
					value = c.User.ID
				} else if binding.Name == "email" {
					value = c.User.Email
				}
			}
		default:
			return nil, fmt.Errorf("unsupported context source")
		}
		if binding.Required && (value == nil || value == "") {
			return nil, fmt.Errorf("required page context %s is missing", key)
		}
		out[key] = value
	}
	for key, value := range route {
		if _, exists := out[key]; !exists {
			out[key] = value
		}
	}
	return out, nil
}

func Match(a *appir.App, path string) (appir.Page, map[string]string, bool) {
	pages := make([]appir.Page, 0, len(a.Pages))
	for _, definition := range a.Pages {
		pages = append(pages, definition)
	}
	sort.Slice(pages, func(i, j int) bool { return routeBefore(pages[i], pages[j]) })
	for _, p := range pages {
		pp := routeParts(p.Route)
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

func routeBefore(left, right appir.Page) bool {
	leftParts, rightParts := routeParts(left.Route), routeParts(right.Route)
	for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
		leftStatic, rightStatic := !strings.HasPrefix(leftParts[index], ":"), !strings.HasPrefix(rightParts[index], ":")
		if leftStatic != rightStatic {
			return leftStatic
		}
	}
	if left.Route != right.Route {
		return left.Route < right.Route
	}
	return left.Name < right.Name
}

func routeParts(route string) []string { return strings.Split(strings.Trim(route, "/"), "/") }

func Protected(a *appir.App, p appir.Page) bool {
	return p.Policy != "" || a.Panels[p.Panel].Policy != ""
}

func Node(a *appir.App, p appir.Page, ctx map[string]any, c beanctx.Request) (render.Node, bool, error) {
	panelDefinition := a.Panels[p.Panel]
	child, allowed, e := panel.Node(a, panelDefinition, ctx, c)
	if e != nil || !allowed {
		return render.Node{}, allowed, e
	}
	return render.Node{Component: "Page", Props: map[string]any{"title": p.Title, "description": p.Description, "protected": Protected(a, p)}, Children: []render.Node{child}}, true, nil
}
