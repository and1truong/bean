package agentprotocol_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
)

func TestMenuNavigationCapabilitiesReferencesAndDiffAreDiscoverable(t *testing.T) {
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", agentprotocol.APIVersion)
	if !reflect.DeepEqual(capabilities.MenuProfiles, []string{"workspace"}) || capabilities.MaxMenuDefinitions != 32 || capabilities.MaxMenuDepth != 3 || capabilities.MaxMenuPlacements != 200 {
		t.Fatalf("Menu capabilities=%+v", capabilities)
	}
	current := appir.Empty()
	current.Entities["book"] = appir.Entity{Name: "book"}
	current.Menus["contents"] = appir.Menu{Name: "contents", Profile: "workspace", MaxDepth: 3, Owner: &appir.MenuOwner{Entity: "book"}}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	candidate.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home"}
	candidate.Menus["main"] = appir.Menu{Name: "main", Profile: "workspace", MaxDepth: 3, Items: []appir.MenuItem{{ID: "home", Target: appir.MenuTarget{Page: "home"}}}}
	_, references, exists := compiler.InspectDefinition(candidate, "Menu", "main")
	if !exists || !containsReference(references, "items.0.target.page", "Page", "home") {
		t.Fatalf("references=%+v", references)
	}
	changes := agentprotocol.SemanticDiff(current, candidate)
	found := false
	for _, change := range changes {
		found = found || change.Path == "menus.main"
	}
	if !found {
		t.Fatalf("changes=%+v", changes)
	}
}
