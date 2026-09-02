package view

import (
	"fmt"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/render"
	"github.com/beanruntime/bean/internal/valuesource"
)

type DisplayMatch struct {
	View, Name string
	Display    appir.Display
	Params     map[string]string
}

func MatchPageDisplay(app *appir.App, path string) (DisplayMatch, bool) {
	candidates := []DisplayMatch{}
	for viewName, definition := range app.Views {
		for displayName, display := range definition.Displays {
			if display.Type == "page" {
				candidates = append(candidates, DisplayMatch{View: viewName, Name: displayName, Display: display})
			}
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i].Display.Route, candidates[j].Display.Route
		leftParts, rightParts := routeParts(left), routeParts(right)
		for index := 0; index < len(leftParts) && index < len(rightParts); index++ {
			leftStatic, rightStatic := !strings.HasPrefix(leftParts[index], ":"), !strings.HasPrefix(rightParts[index], ":")
			if leftStatic != rightStatic {
				return leftStatic
			}
		}
		if left != right {
			return left < right
		}
		if candidates[i].View != candidates[j].View {
			return candidates[i].View < candidates[j].View
		}
		return candidates[i].Name < candidates[j].Name
	})
	for _, candidate := range candidates {
		params, matched := matchRoute(candidate.Display.Route, path)
		if matched {
			candidate.Params = params
			return candidate, true
		}
	}
	return DisplayMatch{}, false
}

func ResolveDisplayBindings(display appir.Display, route, query map[string]string, request beanctx.Request) (map[string]any, error) {
	out := map[string]any{}
	for name, binding := range display.Bindings {
		value, err := valuesource.Resolve(valuesource.Page, binding.Source, binding.Name, valuesource.Environment{Request: request, Route: route, Query: query})
		if err != nil {
			return nil, fmt.Errorf("resolve View display binding %s: %w", name, err)
		}
		if binding.Required && (value == nil || value == "") {
			return nil, fmt.Errorf("required View display binding %s is missing", name)
		}
		out[name] = value
	}
	return out, nil
}

func DisplayPageNode(app *appir.App, match DisplayMatch) render.Node {
	view := app.Views[match.View]
	formatted := []string{}
	for fieldName := range view.FieldFilters {
		formatted = append(formatted, fieldName)
	}
	sort.Strings(formatted)
	props := map[string]any{
		"name": match.Name, "view": match.View, "display": match.Display,
		"filters": view.ExposedFilters, "fieldTypes": FieldTypes(app, view), "formattedFields": formatted, "fileFields": displayFileFields(app, view),
	}
	return render.Node{Component: "Page", Children: []render.Node{{Component: "ViewBlock", Props: props}}}
}

func FieldTypes(app *appir.App, view appir.View) map[string]string {
	out := map[string]string{"id": "uuid", "created_at": "datetime", "updated_at": "datetime", "version": "integer"}
	entities := map[string]appir.Entity{"": app.Entities[view.Entity]}
	for _, relationship := range view.Relationships {
		entities[relationship.Name] = app.Entities[relationship.Entity]
	}
	for _, name := range view.Fields {
		parts := strings.Split(name, ".")
		entity, fieldName := entities[""], name
		if len(parts) == 2 {
			entity, fieldName = entities[parts[0]], parts[1]
		}
		for _, definition := range entity.Fields {
			if definition.Name == fieldName {
				out[name] = definition.Type
			}
		}
	}
	for _, aggregate := range view.Aggregates {
		if strings.EqualFold(aggregate.Function, "count") {
			out[aggregate.Alias] = "integer"
		} else {
			out[aggregate.Alias] = "decimal"
		}
	}
	return out
}

func displayFileFields(app *appir.App, view appir.View) []string {
	selected := map[string]bool{}
	for _, name := range view.Fields {
		selected[name] = true
	}
	out := []string{}
	for _, field := range app.Entities[view.Entity].Fields {
		if field.Type == "file" && selected[field.Name] {
			out = append(out, field.Name)
		}
	}
	return out
}

func routeParts(route string) []string { return strings.Split(strings.Trim(route, "/"), "/") }

func matchRoute(route, path string) (map[string]string, bool) {
	template, actual := routeParts(route), routeParts(path)
	if len(template) != len(actual) {
		return nil, false
	}
	params := map[string]string{}
	for index, part := range template {
		if strings.HasPrefix(part, ":") {
			params[strings.TrimPrefix(part, ":")] = actual[index]
		} else if part != actual[index] {
			return nil, false
		}
	}
	return params, true
}
