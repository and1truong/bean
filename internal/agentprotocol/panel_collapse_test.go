package agentprotocol_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
)

func TestCollapsiblePanelRegionSemanticDiffIsAgentDiscoverable(t *testing.T) {
	current := appir.Empty()
	current.Panels["article"] = appir.Panel{Name: "article", Layout: "sidebar-main", Regions: []appir.Region{{Name: "sidebar"}, {Name: "main"}}}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	panel := candidate.Panels["article"]
	panel.Regions[0].CollapseWhenEmpty = true
	candidate.Panels["article"] = panel

	changes := agentprotocol.SemanticDiff(current, candidate)
	for _, change := range changes {
		if change.Path == "panels.article.regions" {
			return
		}
	}
	t.Fatalf("changes=%+v", changes)
}
