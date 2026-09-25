package scenariopropose_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/scenariopropose"
)

func testApp() *appir.App {
	app := appir.Empty()
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Title: "Home"}
	app.Pages["members"] = appir.Page{Name: "members", Route: "/members", Title: "Members", Policy: "members"}
	app.Pages["detail"] = appir.Page{Name: "detail", Route: "/items/:id", Title: "Item"}
	app.Policies["members"] = appir.Policy{Name: "members", Authenticated: true}
	return app
}

func TestProposalsCoversHappyPathAndAuthCheck(t *testing.T) {
	proposals := scenariopropose.Proposals(testApp())
	if len(proposals) != 2 {
		t.Fatalf("expected 2 proposals (param route skipped), got %d: %+v", len(proposals), proposals)
	}
	byKind := map[string]scenariopropose.Proposal{}
	for _, p := range proposals {
		byKind[p.Kind] = p
		if got := compiler.CompileScenarioCandidate(testApp(), p.Name, p.Spec); len(got.Diagnostics) > 0 {
			t.Fatalf("proposal %s does not compile: %v", p.Name, got.Diagnostics)
		}
	}
	happy, ok := byKind[scenariopropose.KindHappyPath]
	if !ok || happy.Name != "happy_path_home" {
		t.Fatalf("missing happy-path proposal for /: %+v", happy)
	}
	nodes, _ := happy.Spec["nodes"].([]any)
	if len(nodes) != 3 || nodes[0].(map[string]any)["type"] != "navigate" || nodes[0].(map[string]any)["url"] != "/" || nodes[2].(map[string]any)["text"] != "Home" {
		t.Fatalf("happy-path spec shape wrong: %+v", nodes)
	}
	auth, ok := byKind[scenariopropose.KindAuthCheck]
	if !ok || auth.Name != "auth_check_members" {
		t.Fatalf("missing auth-check proposal for /members: %+v", auth)
	}
	authNodes, _ := auth.Spec["nodes"].([]any)
	if len(authNodes) != 2 || authNodes[1].(map[string]any)["assertion"] != "url_contains" || authNodes[1].(map[string]any)["text"] != "/login" {
		t.Fatalf("auth-check spec shape wrong: %+v", authNodes)
	}
}

func TestProposalsSkipsTakenNamesAndEmptyApps(t *testing.T) {
	if proposals := scenariopropose.Proposals(nil); proposals != nil {
		t.Fatalf("nil app should yield nil, got %+v", proposals)
	}
	app := testApp()
	app.Scenarios["happy_path_home"] = appir.Scenario{Name: "happy_path_home"}
	proposals := scenariopropose.Proposals(app)
	if len(proposals) != 1 || proposals[0].Kind != scenariopropose.KindAuthCheck {
		t.Fatalf("saved scenario name should be skipped, got %+v", proposals)
	}
}
