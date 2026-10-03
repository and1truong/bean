package playground

import (
	"github.com/beanruntime/bean/internal/appir"
	blockpkg "github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/render"
	"github.com/beanruntime/bean/internal/sequence"
	"github.com/beanruntime/bean/internal/view"
)

const (
	maxRouteBytes  = 512
	maxQueryParams = 32
	maxQueryBytes  = 4 << 10
)

// backendComponentReasons maps render components that need a live Bean server
// (data reads or writes) to the reason shown by the browser preview.
var backendComponentReasons = map[string]string{
	"ViewBlock":         "requires a Bean backend for data",
	"EntityBlock":       "requires a Bean backend for data",
	"ResourceListBlock": "requires a Bean backend for data",
	"WebformBlock":      "requires a Bean backend for writes",
	"ActionBlock":       "requires a Bean backend for writes",
}

func (s *Session) render(path string, query map[string]string) Response {
	if s.app == nil {
		return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeNoCompiledSource, Message: "no compiled application; compile source first"}}
	}
	if len(path) == 0 || len(path) > maxRouteBytes {
		return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeRequest, Message: "path must be a route of at most 512 bytes"}}
	}
	if len(query) > maxQueryParams {
		return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeRequest, Message: "too many query parameters"}}
	}
	for key, value := range query {
		if len(key) > maxQueryBytes || len(value) > maxQueryBytes {
			return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeRequest, Message: "query parameter exceeds the size limit"}}
		}
	}
	a := s.app
	request := beanctx.Request{Route: path}
	p, params, ok := page.Match(a, path)
	if ok {
		if p.Policy != "" {
			return renderFailure(CodeProtected, "this page requires authentication, which the browser preview does not provide")
		}
		ctx, err := page.ResolveContext(p, params, query, request)
		if err != nil {
			return renderFailure(CodeContextMissing, err.Error())
		}
		request.RouteParams = params
		request.Values = ctx
		tree, allowed, err := page.Node(a, p, ctx, request)
		if err != nil {
			return renderFailure(CodeContextMissing, err.Error())
		}
		if !allowed {
			if page.Protected(a, p) {
				return renderFailure(CodeProtected, "this page requires authentication, which the browser preview does not provide")
			}
			return renderFailure(CodeNotFound, "no renderable content at this route")
		}
		return treeResponse(markUnsupported(tree))
	}
	item, found := sequence.Match(a, path)
	if found {
		if item.Policy != "" {
			return renderFailure(CodeProtected, "this sequence requires authentication, which the browser preview does not provide")
		}
		tree, allowed, err := sequence.Node(a, item, request)
		if err != nil {
			return renderFailure(CodeContextMissing, err.Error())
		}
		if !allowed {
			if sequence.Protected(a, item) {
				return renderFailure(CodeProtected, "this sequence requires authentication, which the browser preview does not provide")
			}
			return renderFailure(CodeNotFound, "no renderable content at this route")
		}
		return treeResponse(markUnsupported(tree))
	}
	if _, matched := view.MatchPageDisplay(a, path); matched {
		return renderFailure(CodeBackendRequired, "this route renders a data-backed View display and requires a Bean backend")
	}
	return renderFailure(CodeNotFound, "no page, sequence, or display matches this route")
}

func renderFailure(code, message string) Response {
	return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: code, Message: message}}
}

func treeResponse(tree render.Node, unsupported []UnsupportedComponent) Response {
	return Response{V: ProtocolVersion, OK: true, Tree: tree, Unsupported: unsupported}
}

// markUnsupported rewrites backend-dependent render nodes into explicit
// UnsupportedBlock placeholders so the preview never silently simulates data
// or writes. It returns the per-component replacement counts.
func markUnsupported(tree render.Node) (render.Node, []UnsupportedComponent) {
	counts := map[string]*UnsupportedComponent{}
	var walk func(node render.Node) render.Node
	walk = func(node render.Node) render.Node {
		if reason := unsupportedReason(node); reason != "" {
			component := node.Component
			name, _ := node.Props["name"].(string)
			entry := counts[component]
			if entry == nil {
				entry = &UnsupportedComponent{Component: component, Reason: reason}
				counts[component] = entry
			}
			entry.Count++
			return render.Node{Component: "UnsupportedBlock", Props: map[string]any{"name": name, "component": component, "reason": reason}}
		}
		for index := range node.Children {
			node.Children[index] = walk(node.Children[index])
		}
		return node
	}
	tree = walk(tree)
	components := make([]UnsupportedComponent, 0, len(counts))
	for _, entry := range counts {
		components = append(components, *entry)
	}
	return tree, components
}

func unsupportedReason(node render.Node) string {
	if reason, exists := backendComponentReasons[node.Component]; exists {
		return reason
	}
	if node.Component == "MenuBlock" {
		if owner, _ := node.Props["ownerEntity"].(string); owner != "" {
			return "owner-scoped Menus require a Bean backend"
		}
	}
	return ""
}

// scanUnsupported counts backend-dependent blocks across the compiled app for
// the compile response summary.
func scanUnsupported(a *appir.App) []UnsupportedComponent {
	counts := map[string]*UnsupportedComponent{}
	for _, definition := range a.Blocks {
		specification, exists := blockpkg.Lookup(definition.Type)
		if !exists {
			continue
		}
		reason := backendComponentReasons[specification.Component]
		if reason == "" && specification.Component == "MenuBlock" && a.Menus[definition.Menu].Owner != nil {
			reason = "owner-scoped Menus require a Bean backend"
		}
		if reason == "" {
			continue
		}
		entry := counts[specification.Component]
		if entry == nil {
			entry = &UnsupportedComponent{Component: specification.Component, Reason: reason}
			counts[specification.Component] = entry
		}
		entry.Count++
	}
	components := make([]UnsupportedComponent, 0, len(counts))
	for _, entry := range counts {
		components = append(components, *entry)
	}
	return components
}
