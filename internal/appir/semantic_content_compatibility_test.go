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
