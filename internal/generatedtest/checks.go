package generatedtest

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/rule"
)

type Check struct {
	ID       string           `json:"id"`
	Status   string           `json:"status"`
	Source   appir.TestTarget `json:"source"`
	Evidence map[string]any   `json:"evidence"`
}

func StructuralChecks(bundle definition.Bundle, app *appir.App) []Check {
	checks := []Check{}
	definitions := append([]definition.Definition{}, bundle.Definitions...)
	sort.Slice(definitions, func(i, j int) bool {
		if definitions[i].Kind != definitions[j].Kind {
			return definitions[i].Kind < definitions[j].Kind
		}
		return definitions[i].Metadata.Name < definitions[j].Metadata.Name
	})
	for _, item := range definitions {
		checks = append(checks, passedCheck("generated/schema/"+item.Kind+"/"+item.Metadata.Name, item.Kind, item.Metadata.Name, map[string]any{"contract": "canonical-schema"}))
	}
	for _, name := range sortedKeys(app.Rules) {
		checks = append(checks, passedCheck("generated/rule/Rule/"+name, "Rule", name, map[string]any{
			"result": app.Rules[name].Result, "sources": rule.Sources(), "maxNodes": rule.MaxNodes, "maxDepth": rule.MaxDepth,
			"maxLiteralBytes": rule.MaxLiteralBytes, "maxValueBytes": rule.MaxValueBytes,
		}))
	}
	for _, name := range sortedKeys(app.Policies) {
		item := app.Policies[name]
		checks = append(checks, passedCheck("generated/policy/Policy/"+name, "Policy", name, map[string]any{"readRoles": len(item.ReadRoles), "writeRoles": len(item.WriteRoles), "authenticated": item.Authenticated, "owner": item.Owner, "tenant": item.Tenant}))
	}
	for _, name := range sortedKeys(app.Lifecycles) {
		item := app.Lifecycles[name]
		edges := 0
		for _, targets := range item.Transitions {
			edges += len(targets)
		}
		checks = append(checks, passedCheck("generated/transition/Lifecycle/"+name, "Lifecycle", name, map[string]any{"entity": item.Entity, "initial": item.Initial, "edges": edges}))
	}
	for _, name := range sortedKeys(app.Pages) {
		item := app.Pages[name]
		checks = append(checks, passedCheck("generated/route/Page/"+name, "Page", name, map[string]any{"route": item.Route, "panel": item.Panel}))
	}
	for _, name := range sortedKeys(app.Views) {
		for _, displayName := range sortedKeys(app.Views[name].Displays) {
			display := app.Views[name].Displays[displayName]
			checks = append(checks, passedCheck("generated/route/View/"+name+"/"+displayName, "View", name, map[string]any{"route": display.Route, "display": displayName, "type": display.Type}))
		}
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return checks
}

func JourneyChecks(ctx context.Context, app *appir.App, handler http.Handler) ([]Check, []definition.Diagnostic) {
	checks := []Check{}
	diagnostics := []definition.Diagnostic{}
	for _, name := range sortedKeys(app.Pages) {
		item := app.Pages[name]
		if !staticAnonymousPage(app, item) {
			continue
		}
		status := requestStatus(ctx, handler, "/api/system/page?path="+url.QueryEscape(item.Route))
		frontendStatus := requestStatus(ctx, handler, item.Route)
		check := passedCheck("generated/journey/Page/"+name, "Page", name, map[string]any{"route": item.Route, "pageStatus": status, "frontendStatus": frontendStatus})
		if status != http.StatusOK || frontendStatus != http.StatusOK {
			check.Status = "failed"
			diagnostics = append(diagnostics, journeyDiagnostic("Page", name, check.ID, status, frontendStatus))
		}
		checks = append(checks, check)
	}
	for _, name := range sortedKeys(app.Views) {
		view := app.Views[name]
		if !anonymousView(app, view) {
			continue
		}
		for _, displayName := range sortedKeys(view.Displays) {
			display := view.Displays[displayName]
			if display.Route == "" {
				continue
			}
			status := requestStatus(ctx, handler, display.Route)
			check := passedCheck("generated/journey/View/"+name+"/"+displayName, "View", name, map[string]any{"route": display.Route, "display": displayName, "status": status})
			if status != http.StatusOK {
				check.Status = "failed"
				diagnostics = append(diagnostics, journeyDiagnostic("View", name, check.ID, status, 0))
			}
			checks = append(checks, check)
		}
	}
	return checks, diagnostics
}

func passedCheck(id, kind, name string, evidence map[string]any) Check {
	return Check{ID: id, Status: "passed", Source: appir.TestTarget{Kind: kind, Name: name}, Evidence: evidence}
}

func staticAnonymousPage(app *appir.App, item appir.Page) bool {
	if strings.Contains(item.Route, ":") || !anonymousPolicy(app, item.Policy) {
		return false
	}
	for _, binding := range item.Context {
		if binding.Required {
			return false
		}
	}
	contextValues, err := page.ResolveContext(item, nil, map[string]string{"path": item.Route}, beanctx.Request{})
	if err != nil {
		return false
	}
	request := beanctx.Request{Values: contextValues}
	panel, exists := app.Panels[item.Panel]
	if !exists || !anonymousPolicyForRequest(app, panel.Policy, request) {
		return false
	}
	for _, region := range panel.Regions {
		for _, blockName := range region.Blocks {
			block, exists := app.Blocks[blockName]
			if !exists || !anonymousPolicyForRequest(app, block.Policy, request) {
				return false
			}
			if len(block.Inputs) > 0 {
				return false
			}
		}
	}
	return true
}

func anonymousView(app *appir.App, item appir.View) bool {
	entity := app.Entities[item.Entity]
	if entity.Owner || entity.Tenant {
		return false
	}
	return anonymousPolicy(app, policy.EffectiveViewPolicyName(item, entity))
}

func anonymousPolicy(app *appir.App, name string) bool {
	return anonymousPolicyForRequest(app, name, beanctx.Request{})
}

func anonymousPolicyForRequest(app *appir.App, name string, request beanctx.Request) bool {
	if name == "" {
		return true
	}
	item, exists := app.Policies[name]
	return exists && policy.Can(item, false, request, nil)
}

func requestStatus(ctx context.Context, handler http.Handler, path string) int {
	request := httptest.NewRequest(http.MethodGet, path, nil).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response.Code
}

func journeyDiagnostic(kind, name, id string, status, frontendStatus int) definition.Diagnostic {
	value := map[string]any{"status": status}
	if frontendStatus != 0 {
		value["frontendStatus"] = frontendStatus
	}
	return definition.Diagnostic{Code: "BEAN-T1201", Kind: kind, Name: name, Path: "journey", Message: fmt.Sprintf("generated journey %s failed", id), Value: value}
}
