// Package scenariopropose derives scenario-graph proposals from the compiled
// application itself: the route surface becomes happy-path checks, and routes
// gated by policy become authorization checks. Proposals are drafts — they are
// returned to the caller for review in the scenario editor and persist only
// through the normal definition flow, never written by this package.
package scenariopropose

import (
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/scenariogen"
)

// Proposal kinds classify the check a draft performs.
const (
	KindHappyPath = "happy_path"
	KindAuthCheck = "auth_check"
)

// loginRoute is the unauthenticated redirect target of the hosted app.
const loginRoute = "/login"

// Proposal is one suggested Scenario draft.
type Proposal struct {
	Name    string
	Kind    string
	Title   string
	Summary string
	Spec    map[string]any
}

// Proposals walks the application's pages and emits a draft per navigable
// route: unprotected routes get a happy-path check (land on the route, read
// its title), policy-gated routes get an authorization check (land redirected
// to login). Parameterized routes are skipped — a proposal cannot invent the
// ids a route needs — and names already taken by saved scenarios are not
// re-proposed.
func Proposals(app *appir.App) []Proposal {
	if app == nil {
		return nil
	}
	names := make([]string, 0, len(app.Pages))
	for name := range app.Pages {
		names = append(names, name)
	}
	sort.Strings(names)
	proposals := []Proposal{}
	for _, name := range names {
		p := app.Pages[name]
		if p.Route == "" || strings.Contains(p.Route, ":") {
			continue
		}
		var proposal Proposal
		if page.Protected(app, p) {
			proposal = authCheck(p)
		} else {
			proposal = happyPath(p)
		}
		if _, taken := app.Scenarios[proposal.Name]; taken {
			continue
		}
		proposals = append(proposals, proposal)
	}
	return proposals
}

// happyPath drafts navigate → assert the app stayed on the route → assert the
// page title renders when the page declares one.
func happyPath(p appir.Page) Proposal {
	title := p.Title
	if title == "" {
		title = p.Name
	}
	nodes := []any{
		map[string]any{"id": "step_1", "type": "navigate", "url": p.Route, "next": "step_2"},
		map[string]any{"id": "step_2", "type": "assert", "assertion": "url_contains", "text": p.Route},
	}
	if p.Title != "" {
		nodes[1].(map[string]any)["next"] = "step_3"
		nodes = append(nodes, map[string]any{"id": "step_3", "type": "assert", "assertion": "text_present", "text": p.Title})
	}
	return Proposal{
		Name:    scenariogen.Slug(KindHappyPath + " " + p.Name),
		Kind:    KindHappyPath,
		Title:   "Happy path: " + p.Route,
		Summary: "Navigates to " + p.Route + " and checks it renders (" + title + ").",
		Spec: map[string]any{
			"title":       "Happy path: " + title,
			"description": "App-generated happy-path proposal for " + p.Route + " — review before saving.",
			"start":       "step_1",
			"nodes":       nodes,
		},
	}
}

// authCheck drafts navigate → assert the app redirected to login.
func authCheck(p appir.Page) Proposal {
	return Proposal{
		Name:    scenariogen.Slug(KindAuthCheck + " " + p.Name),
		Kind:    KindAuthCheck,
		Title:   "Authorization check: " + p.Route,
		Summary: "Navigates to " + p.Route + " without a session and checks the redirect to " + loginRoute + ".",
		Spec: map[string]any{
			"title":       "Authorization check: " + p.Route,
			"description": "App-generated authorization proposal for " + p.Route + " — review before saving.",
			"start":       "step_1",
			"nodes": []any{
				map[string]any{"id": "step_1", "type": "navigate", "url": p.Route, "next": "step_2"},
				map[string]any{"id": "step_2", "type": "assert", "assertion": "url_contains", "text": loginRoute},
			},
		},
	}
}
