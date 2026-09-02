package agentprotocol_test

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
)

func TestViewDisplayInspectionReferencesAndSemanticDiff(t *testing.T) {
	current := appir.Empty()
	current.Entities["article"] = appir.Entity{Name: "article"}
	current.Actions["move_article"] = appir.Action{Name: "move_article", Entity: "article"}
	current.Views["articles"] = appir.View{Name: "articles", Entity: "article", Displays: map[string]appir.Display{
		"index": {Type: "page", Renderer: appir.ViewRenderer{Type: "table", Fields: []appir.ViewColumn{{Field: "title", Label: "Article"}}, MoveAction: "move_article"}},
	}}
	value, references, exists := compiler.InspectDefinition(current, "View", "articles")
	foundReference := false
	for _, reference := range references {
		foundReference = foundReference || reference == (compiler.DefinitionReference{Path: "displays.index.renderer.moveAction", Kind: "Action", Name: "move_article"})
	}
	if !exists || value == nil || len(references) != 2 || !foundReference {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	view := candidate.Views["articles"]
	display := view.Displays["index"]
	display.Renderer.Fields[0].Label = "Headline"
	view.Displays["index"] = display
	candidate.Views["articles"] = view
	changes := agentprotocol.SemanticDiff(current, candidate)
	found := false
	for _, change := range changes {
		found = found || change.Path == "views.articles.displays.index.renderer.fields"
	}
	if !found {
		t.Fatalf("changes=%+v", changes)
	}
}

func TestExploreVocabularyAndReferencesAreAgentDiscoverable(t *testing.T) {
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", agentprotocol.APIVersion)
	if !reflect.DeepEqual(capabilities.ViewResultShapes, []string{"groups", "metric", "records"}) ||
		!reflect.DeepEqual(capabilities.ViewGroupBuckets, []string{"day", "month", "week"}) ||
		!reflect.DeepEqual(capabilities.ViewAggregateFunctions, []string{"average", "count", "max", "min", "sum"}) ||
		!reflect.DeepEqual(capabilities.ViewDrillSources, []string{"filter", "group"}) ||
		!reflect.DeepEqual(capabilities.ViewSelections, []string{"multiple", "none", "single"}) {
		t.Fatalf("Explore capabilities=%+v", capabilities)
	}
	app := appir.Empty()
	app.Entities["candidate"] = appir.Entity{Name: "candidate"}
	app.Actions["move_candidate"] = appir.Action{Name: "move_candidate", Entity: "candidate"}
	app.Views["candidate_records"] = appir.View{Name: "candidate_records", Entity: "candidate"}
	app.Views["candidates_by_stage"] = appir.View{Name: "candidates_by_stage", Entity: "candidate", Displays: map[string]appir.Display{
		"chart": {Actions: []string{"move_candidate"}, Drill: &appir.ViewDrill{View: "candidate_records"}},
	}}
	app.Blocks["stage_chart"] = appir.Block{Name: "stage_chart", Type: "view", View: "candidates_by_stage"}
	app.Pages["overview"] = appir.Page{Name: "overview", Filters: map[string]appir.PageFilter{
		"stage": {Targets: []appir.PageFilterTarget{{Block: "stage_chart", Filter: "stage"}}},
	}}
	_, viewReferences, _ := compiler.InspectDefinition(app, "View", "candidates_by_stage")
	_, pageReferences, _ := compiler.InspectDefinition(app, "Page", "overview")
	if !containsReference(viewReferences, "displays.chart.actions.0", "Action", "move_candidate") || !containsReference(viewReferences, "displays.chart.drill.view", "View", "candidate_records") {
		t.Fatalf("View references=%+v", viewReferences)
	}
	if !containsReference(pageReferences, "filters.stage.targets.0.block", "Block", "stage_chart") {
		t.Fatalf("Page references=%+v", pageReferences)
	}
}

func containsReference(values []compiler.DefinitionReference, path, kind, name string) bool {
	for _, value := range values {
		if value.Path == path && value.Kind == kind && value.Name == name {
			return true
		}
	}
	return false
}
