package appir_test

import (
	"encoding/json"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
)

func TestExtendedSemanticContentFormatBoundaryIsImmutable(t *testing.T) {
	tests := []struct {
		name string
		edit func(*appir.App)
	}{
		{"named", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Content: []appir.ContentElement{{Type: "link", Label: "Open", Target: "/"}}}
		}},
		{"inline", func(app *appir.App) {
			app.Panels["panel"] = appir.Panel{Regions: []appir.Region{{Items: []appir.RegionItem{{Content: []appir.ContentElement{{Type: "divider"}}}}}}}
		}},
		{"heading level", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Content: []appir.ContentElement{{Type: "heading", Text: "Heading", Level: 2}}}
		}},
		{"tabs", func(app *appir.App) {
			app.Blocks["tabs"] = appir.Block{Type: "tabs", Tabs: []appir.ContentTab{{ID: "one", Content: []appir.ContentElement{{Type: "choices"}}}}}
		}},
		{"formula", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Content: []appir.ContentElement{{Type: "formula", Expr: &appir.FormulaNode{Kind: "literal", Text: "x"}}}}
		}},
		{"lesson", func(app *appir.App) {
			app.Blocks["lesson"] = appir.Block{Type: "lesson", Title: "Lesson", Sections: []appir.LessonSection{{ID: "one", Content: []appir.ContentElement{{Type: "divider"}}}}}
		}},
		{"lesson section content", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Sections: []appir.LessonSection{{ID: "one", Content: []appir.ContentElement{{Type: "formula", Expr: &appir.FormulaNode{Kind: "literal", Text: "x"}}}}}}
		}},
		{"timeline", func(app *appir.App) {
			app.Blocks["timeline"] = appir.Block{Type: "timeline", Title: "Timeline", Entries: []appir.TimelineEntry{{ID: "one", Label: "1440", Title: "Movable type"}}}
		}},
		{"timeline entries", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Entries: []appir.TimelineEntry{{ID: "one", Label: "Day 1", Title: "Start"}}}
		}},
		{"mindmap", func(app *appir.App) {
			app.Blocks["mindmap"] = appir.Block{Type: "mindmap", Root: &appir.MindMapNode{ID: "root", Label: "Topic", Children: []appir.MindMapNode{{ID: "branch", Label: "Branch"}}}}
		}},
		{"mindmap root", func(app *appir.App) {
			app.Blocks["content"] = appir.Block{Type: "content", Root: &appir.MindMapNode{ID: "root", Label: "Topic"}}
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := appir.Empty()
			test.edit(app)
			if err := app.ValidateFormat(); err != nil {
				t.Fatal(err)
			}
			app.FormatVersion = appir.EmailVerificationFormat
			before, _ := json.Marshal(app)
			if app.ValidateFormat() == nil {
				t.Fatal("v19 accepted v20 semantic content")
			}
			after, _ := json.Marshal(app)
			if string(before) != string(after) {
				t.Fatal("format validation mutated AppIR")
			}
		})
	}
}

func TestHistoricalOmittedHeadingLevelRemainsReadableAndUnchanged(t *testing.T) {
	app := appir.Empty()
	app.FormatVersion = appir.EmailVerificationFormat
	app.Blocks["content"] = appir.Block{Type: "content", Content: []appir.ContentElement{{Type: "heading", Text: "Historical"}}}
	before, _ := json.Marshal(app)
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(app)
	if string(before) != string(after) || app.Blocks["content"].Content[0].Level != 0 {
		t.Fatal("historical heading level changed")
	}
}

func TestHistoricalFormatRejectsExplicitEmptyExtendedFields(t *testing.T) {
	for _, encoded := range []string{
		`{"FormatVersion":"bean/appir/v19","Blocks":{"content":{"Type":"content","Content":[{"Type":"heading","Text":"Historical","Level":0}]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"content":{"Type":"content","Content":[{"Type":"paragraph","Text":"Historical","Explanation":""}]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"tabs":{"Type":"content","Tabs":[]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"content":{"Type":"content","Content":[{"Type":"formula","Expr":{"Kind":"literal","Text":"x"}}]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"lesson":{"Type":"lesson","Sections":[{"id":"one","Content":[]}]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"timeline":{"Type":"timeline","Entries":[{"id":"one","Label":"1440","Title":"Movable type"}]}}}`,
		`{"FormatVersion":"bean/appir/v19","Blocks":{"mindmap":{"Type":"mindmap","Root":{"id":"root","Label":"Topic"}}}}`,
	} {
		app, err := appir.Decode([]byte(encoded))
		if err != nil {
			t.Fatal(err)
		}
		if err = app.ValidateFormat(); err == nil {
			t.Fatalf("v19 accepted explicit v20 field: %s", encoded)
		}
	}
}
