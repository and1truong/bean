package agentprotocol_test

import (
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
