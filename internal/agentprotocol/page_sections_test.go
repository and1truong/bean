package agentprotocol_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
)

func TestPageSectionReferencesAndSemanticDiffAreAgentDiscoverable(t *testing.T) {
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", agentprotocol.APIVersion)
	if capabilities.MaxPageSections != 32 {
		t.Fatalf("max Page sections=%d", capabilities.MaxPageSections)
	}
	current := appir.Empty()
	current.Panels["body"] = appir.Panel{Name: "body"}
	current.Panels["related"] = appir.Panel{Name: "related"}
	current.Pages["article"] = appir.Page{Name: "article", Route: "/article", Panel: "body"}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	candidate.Pages["article"] = appir.Page{Name: "article", Route: "/article", Sections: []appir.PageSection{{ID: "body", Panel: "body", Identity: "@section/article/body"}, {ID: "related", Panel: "related", Identity: "@section/article/related"}}}

	_, references, exists := compiler.InspectDefinition(candidate, "Page", "article")
	if !exists || !containsReference(references, "sections.0.panel", "Panel", "body") || !containsReference(references, "sections.1.panel", "Panel", "related") {
		t.Fatalf("Page references=%+v", references)
	}
	changes := agentprotocol.SemanticDiff(current, candidate)
	foundPanel, foundSections := false, false
	for _, change := range changes {
		foundPanel = foundPanel || change.Path == "pages.article.panel"
		foundSections = foundSections || change.Path == "pages.article.sections"
	}
	if !foundPanel || !foundSections {
		t.Fatalf("changes=%+v", changes)
	}
}
