package compiler_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestExtendedSemanticContentCompilesAndNormalizesAcrossSeams(t *testing.T) {
	content := validExtendedContent()
	definitions := []definition.Definition{
		semanticDefinition("Block", "semantic", map[string]any{"type": "content", "content": content}),
		semanticDefinition("Block", "boundary_tabs", map[string]any{"type": "tabs", "label": "Boundaries", "tabs": []any{
			map[string]any{"id": "reads", "label": "Reads", "content": []any{map[string]any{"type": "paragraph", "text": "Views read."}}},
			map[string]any{"id": "writes", "label": "Writes", "content": []any{map[string]any{"type": "choices", "question": "Write boundary?", "choices": []any{map[string]any{"id": "view", "text": "View"}, map[string]any{"id": "action", "text": "Action"}}, "answer": "action", "explanation": "   "}}},
		}}),
		semanticDefinition("Panel", "semantic_panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "items": []any{map[string]any{"content": content}, map[string]any{"block": "boundary_tabs"}}}}}),
	}
	first := compiler.Compile("semantic", 1, definitions)
	second := compiler.Compile("semantic", 1, definitions)
	if len(first.Diagnostics) != 0 || !reflect.DeepEqual(first.App, second.App) {
		t.Fatalf("diagnostics=%v deterministic=%v", first.Diagnostics, reflect.DeepEqual(first.App, second.App))
	}
	named := first.App.Blocks["semantic"].Content
	if named[0].Level != 2 || named[2].OpenIn != "same_tab" || named[4].RowHeader != "none" {
		t.Fatalf("content defaults=%+v", named)
	}
	tabs := first.App.Blocks["boundary_tabs"]
	if tabs.Orientation != "horizontal" || tabs.Variant != "underline" || tabs.Tabs[1].Content[0].Explanation != "" {
		t.Fatalf("tabs defaults=%+v", tabs)
	}
	inline := first.App.Panels["semantic_panel"].Regions[0].Items[0].Content
	if inline[0].Level != 2 || len(inline) != len(named) {
		t.Fatalf("inline content=%+v", inline)
	}
}

func TestExtendedSemanticContentRejectsInvalidContracts(t *testing.T) {
	tests := []struct {
		name, path string
		element    map[string]any
	}{
		{"heading level null", "spec.content.0.level", map[string]any{"type": "heading", "text": "Heading", "level": nil}},
		{"heading level", "spec.content.0.level", map[string]any{"type": "heading", "text": "Heading", "level": 1}},
		{"foreign field", "spec.content.0.target", map[string]any{"type": "paragraph", "text": "Text", "target": ""}},
		{"ordered empty", "spec.content.0.items", map[string]any{"type": "ordered_list", "items": []any{}}},
		{"ordered blank", "spec.content.0.items.0", map[string]any{"type": "ordered_list", "items": []any{" "}}},
		{"ordered long", "spec.content.0.items.0", map[string]any{"type": "ordered_list", "items": []any{strings.Repeat("界", 241)}}},
		{"link missing", "spec.content.0.target", map[string]any{"type": "link", "label": "Open"}},
		{"link unsafe", "spec.content.0.target", map[string]any{"type": "link", "label": "Open", "target": "javascript:alert(1)"}},
		{"link enum", "spec.content.0.openIn", map[string]any{"type": "link", "label": "Open", "target": "/", "openIn": "popup"}},
		{"divider field", "spec.content.0.label", map[string]any{"type": "divider", "label": "No"}},
		{"table duplicate", "spec.content.0.columns.1.id", tableElement([]any{map[string]any{"id": "name", "label": "Name"}, map[string]any{"id": "name", "label": "Again"}}, []any{[]any{"a", "b"}}, "none")},
		{"table width", "spec.content.0.rows.0", tableElement([]any{map[string]any{"id": "name", "label": "Name"}}, []any{[]any{"a", "b"}}, "none")},
		{"table cell type", "spec.content.0.rows.0.0", tableElement([]any{map[string]any{"id": "name", "label": "Name"}}, []any{[]any{1}}, "none")},
		{"table blank header", "spec.content.0.rows.0.0", tableElement([]any{map[string]any{"id": "name", "label": "Name"}}, []any{[]any{" "}}, "first")},
		{"audio query", "spec.content.0.source", map[string]any{"type": "audio", "source": "/intro.mp3?", "title": "Intro", "transcript": "Words"}},
		{"youtube id", "spec.content.0.videoId", map[string]any{"type": "youtube", "videoId": "https://youtu.be/x", "title": "Video", "transcript": "Words"}},
		{"youtube source", "spec.content.0.source", map[string]any{"type": "youtube", "videoId": "M7lc1UVf-VE", "title": "Video", "transcript": "Words", "source": "https://youtube.test"}},
		{"playlist id", "spec.content.0.playlistId", map[string]any{"type": "youtube_playlist", "playlistId": "short", "title": "Playlist", "transcript": "Words"}},
		{"choice duplicate", "spec.content.0.choices.1.id", choicesElement("one", []any{map[string]any{"id": "one", "text": "One"}, map[string]any{"id": "one", "text": "Again"}})},
		{"choice answer", "spec.content.0.answer", choicesElement("missing", []any{map[string]any{"id": "one", "text": "One"}, map[string]any{"id": "two", "text": "Two"}})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", map[string]any{"type": "content", "content": []any{test.element}})})
			if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", test.path, "BEAN-E2881") {
				t.Fatalf("missing %s in %v", test.path, result.Diagnostics)
			}
		})
	}
}

func TestExtendedSemanticContentAcceptsUnicodeBoundaries(t *testing.T) {
	id := "a" + strings.Repeat("x", 63)
	columns := make([]any, 6)
	for index := range columns {
		columns[index] = map[string]any{"id": string(rune('a' + index)), "label": strings.Repeat("界", 80)}
	}
	row := make([]any, 6)
	for index := range row {
		row[index] = strings.Repeat("界", 240)
	}
	rows := make([]any, 12)
	for index := range rows {
		rows[index] = row
	}
	result := compiler.Compile("bounds", 1, []definition.Definition{semanticDefinition("Block", "bounds", map[string]any{"type": "content", "content": []any{
		map[string]any{"type": "heading", "level": 4, "text": "Heading"},
		map[string]any{"type": "ordered_list", "items": []any{strings.Repeat("界", 240), "two", "three", "four", "five", "six"}},
		map[string]any{"type": "link", "label": strings.Repeat("界", 120), "target": "/" + strings.Repeat("a", 2047), "openIn": "new_tab"},
		tableElement(columns, rows, "none"),
		map[string]any{"type": "audio", "source": "/audio.wav", "title": strings.Repeat("界", 120), "transcript": strings.Repeat("界", 4000)},
		map[string]any{"type": "youtube", "videoId": "M7lc1UVf-VE", "title": "Video", "transcript": "Transcript"},
		map[string]any{"type": "youtube_playlist", "playlistId": "ABCDEFGHIJ", "title": "Playlist", "transcript": "Transcript"},
		map[string]any{"type": "choices", "question": strings.Repeat("界", 240), "choices": []any{map[string]any{"id": id, "text": strings.Repeat("界", 120)}, map[string]any{"id": "answer", "text": "Answer"}}, "answer": "answer", "explanation": strings.Repeat("界", 400)},
	}})})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("Unicode boundary values rejected: %v", result.Diagnostics)
	}
}

func TestExtendedSemanticContentRejectsSourceShapesAndMaxPlusOne(t *testing.T) {
	sevenColumns := make([]any, 7)
	sevenCells := make([]any, 7)
	for index := range sevenColumns {
		sevenColumns[index] = map[string]any{"id": string(rune('a' + index)), "label": "Column"}
		sevenCells[index] = "cell"
	}
	thirteenRows := make([]any, 13)
	for index := range thirteenRows {
		thirteenRows[index] = []any{"cell"}
	}
	sevenChoices := make([]any, 7)
	for index := range sevenChoices {
		sevenChoices[index] = map[string]any{"id": string(rune('a' + index)), "text": "Choice"}
	}
	tests := []struct {
		name, path string
		element    map[string]any
	}{
		{"missing heading text", "spec.content.0.text", map[string]any{"type": "heading"}},
		{"null heading text", "spec.content.0.text", map[string]any{"type": "heading", "text": nil}},
		{"wrong heading text", "spec.content.0.text", map[string]any{"type": "heading", "text": 2}},
		{"wrong heading level", "spec.content.0.level", map[string]any{"type": "heading", "text": "Heading", "level": "2"}},
		{"ordered wrong list", "spec.content.0.items", map[string]any{"type": "ordered_list", "items": "one"}},
		{"ordered too many", "spec.content.0.items", map[string]any{"type": "ordered_list", "items": []any{"1", "2", "3", "4", "5", "6", "7"}}},
		{"link null required", "spec.content.0.label", map[string]any{"type": "link", "label": nil, "target": "/"}},
		{"link long label", "spec.content.0.label", map[string]any{"type": "link", "label": strings.Repeat("界", 121), "target": "/"}},
		{"link long target", "spec.content.0.target", map[string]any{"type": "link", "label": "Open", "target": "/" + strings.Repeat("a", 2048)}},
		{"link empty enum", "spec.content.0.openIn", map[string]any{"type": "link", "label": "Open", "target": "/", "openIn": ""}},
		{"unknown nested column", "spec.content.0.columns.0.extra", map[string]any{"type": "table", "caption": "Table", "columns": []any{map[string]any{"id": "column", "label": "Column", "extra": "no"}}, "rows": []any{[]any{"cell"}}}},
		{"too many columns", "spec.content.0.columns", tableElement(sevenColumns, []any{sevenCells}, "none")},
		{"too many rows", "spec.content.0.rows", tableElement([]any{map[string]any{"id": "column", "label": "Column"}}, thirteenRows, "none")},
		{"long caption", "spec.content.0.caption", map[string]any{"type": "table", "caption": strings.Repeat("界", 121), "columns": []any{map[string]any{"id": "column", "label": "Column"}}, "rows": []any{[]any{"cell"}}}},
		{"long column label", "spec.content.0.columns.0.label", map[string]any{"type": "table", "caption": "Table", "columns": []any{map[string]any{"id": "column", "label": strings.Repeat("界", 81)}}, "rows": []any{[]any{"cell"}}}},
		{"long cell", "spec.content.0.rows.0.0", map[string]any{"type": "table", "caption": "Table", "columns": []any{map[string]any{"id": "column", "label": "Column"}}, "rows": []any{[]any{strings.Repeat("界", 241)}}}},
		{"table empty enum", "spec.content.0.rowHeader", tableElement([]any{map[string]any{"id": "column", "label": "Column"}}, []any{[]any{"cell"}}, "")},
		{"media missing title", "spec.content.0.title", map[string]any{"type": "audio", "source": "/audio.wav", "transcript": "Transcript"}},
		{"media long title", "spec.content.0.title", map[string]any{"type": "audio", "source": "/audio.wav", "title": strings.Repeat("界", 121), "transcript": "Transcript"}},
		{"media long transcript", "spec.content.0.transcript", map[string]any{"type": "audio", "source": "/audio.wav", "title": "Audio", "transcript": strings.Repeat("界", 4001)}},
		{"youtube html", "spec.content.0.html", map[string]any{"type": "youtube", "videoId": "M7lc1UVf-VE", "title": "Video", "transcript": "Transcript", "html": "<iframe>"}},
		{"choice too few", "spec.content.0.choices", choicesElement("one", []any{map[string]any{"id": "one", "text": "One"}})},
		{"choice too many", "spec.content.0.choices", choicesElement("a", sevenChoices)},
		{"choice invalid id", "spec.content.0.choices.0.id", choicesElement("two", []any{map[string]any{"id": "Bad", "text": "Bad"}, map[string]any{"id": "two", "text": "Two"}})},
		{"choice long id", "spec.content.0.choices.0.id", choicesElement("two", []any{map[string]any{"id": "a" + strings.Repeat("x", 64), "text": "Long"}, map[string]any{"id": "two", "text": "Two"}})},
		{"choice long question", "spec.content.0.question", map[string]any{"type": "choices", "question": strings.Repeat("界", 241), "choices": []any{map[string]any{"id": "one", "text": "One"}, map[string]any{"id": "two", "text": "Two"}}, "answer": "two"}},
		{"choice long text", "spec.content.0.choices.0.text", choicesElement("two", []any{map[string]any{"id": "one", "text": strings.Repeat("界", 121)}, map[string]any{"id": "two", "text": "Two"}})},
		{"choice long explanation", "spec.content.0.explanation", map[string]any{"type": "choices", "question": "Choose", "choices": []any{map[string]any{"id": "one", "text": "One"}, map[string]any{"id": "two", "text": "Two"}}, "answer": "two", "explanation": strings.Repeat("界", 401)}},
		{"choice nested unknown", "spec.content.0.choices.0.extra", map[string]any{"type": "choices", "question": "Choose", "choices": []any{map[string]any{"id": "one", "text": "One", "extra": "no"}, map[string]any{"id": "two", "text": "Two"}}, "answer": "two"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", map[string]any{"type": "content", "content": []any{test.element}})})
			if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", test.path, "BEAN-E2881") {
				t.Fatalf("missing %s in %v", test.path, result.Diagnostics)
			}
		})
	}

	tooMany := make([]any, 13)
	for index := range tooMany {
		tooMany[index] = map[string]any{"type": "divider"}
	}
	result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", map[string]any{"type": "content", "content": tooMany})})
	if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", "spec.content", "BEAN-E2881") {
		t.Fatalf("13 content elements accepted: %v", result.Diagnostics)
	}
}

func TestTabsBlockValidationAndSourceLocations(t *testing.T) {
	tab := func(id string, content []any) map[string]any {
		return map[string]any{"id": id, "label": "Tab", "content": content}
	}
	paragraph := []any{map[string]any{"type": "paragraph", "text": "Text"}}
	for _, spec := range []map[string]any{
		{"type": "content", "content": paragraph, "label": "Tabs only"},
		{"type": "text", "text": "Text", "tabs": []any{}},
	} {
		result := compiler.Compile("tabs", 1, []definition.Definition{semanticDefinition("Block", "wrong_type", spec)})
		path := "spec.label"
		if spec["tabs"] != nil {
			path = "spec.tabs"
		}
		if !hasSequenceDiagnostic(result.Diagnostics, "Block", "wrong_type", path, "BEAN-E2881") {
			t.Fatalf("tabs-only Block field %s accepted: %v", path, result.Diagnostics)
		}
	}
	sevenTabs := make([]any, 7)
	for index := range sevenTabs {
		sevenTabs[index] = tab(string(rune('a'+index)), paragraph)
	}
	for _, test := range []struct {
		name, path string
		spec       map[string]any
	}{
		{"missing label", "spec.label", map[string]any{"type": "tabs", "tabs": []any{tab("one", paragraph), tab("two", paragraph)}}},
		{"empty orientation", "spec.orientation", map[string]any{"type": "tabs", "label": "Tabs", "orientation": "", "tabs": []any{tab("one", paragraph), tab("two", paragraph)}}},
		{"invalid variant", "spec.variant", map[string]any{"type": "tabs", "label": "Tabs", "variant": "cards", "tabs": []any{tab("one", paragraph), tab("two", paragraph)}}},
		{"too few tabs", "spec.tabs", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("one", paragraph)}}},
		{"too many tabs", "spec.tabs", map[string]any{"type": "tabs", "label": "Tabs", "tabs": sevenTabs}},
		{"invalid tab id", "spec.tabs.0.id", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("Bad", paragraph), tab("two", paragraph)}}},
		{"duplicate tab id", "spec.tabs.1.id", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("same", paragraph), tab("same", paragraph)}}},
		{"empty tab content", "spec.tabs.0.content", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("one", []any{}), tab("two", paragraph)}}},
		{"block reference in tab", "spec.tabs.0.content.0.type", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("one", []any{map[string]any{"block": "other"}}), tab("two", paragraph)}}},
		{"nested tabs", "spec.tabs.0.content.0.tabs", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{tab("one", []any{map[string]any{"type": "tabs", "tabs": []any{}}}), tab("two", paragraph)}}},
		{"foreign content", "spec.content", map[string]any{"type": "tabs", "label": "Tabs", "content": paragraph, "tabs": []any{tab("one", paragraph), tab("two", paragraph)}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.Compile("tabs", 1, []definition.Definition{semanticDefinition("Block", "tabs", test.spec)})
			if !hasSequenceDiagnostic(result.Diagnostics, "Block", "tabs", test.path, "BEAN-E2881") {
				t.Fatalf("missing %s in %v", test.path, result.Diagnostics)
			}
		})
	}

	dense := make([]any, 5)
	for index := range dense {
		dense[index] = map[string]any{"type": "paragraph", "text": "small"}
	}
	tabs := make([]any, 5)
	for index := range tabs {
		tabs[index] = map[string]any{"id": string(rune('a' + index)), "label": "Tab", "content": dense}
	}
	result := compiler.Compile("tabs", 1, []definition.Definition{semanticDefinition("Block", "tabs", map[string]any{"type": "tabs", "label": "Tabs", "tabs": tabs})})
	if !hasSequenceDiagnostic(result.Diagnostics, "Block", "tabs", "spec.tabs", "BEAN-E2881") {
		t.Fatalf("total tab content accepted: %v", result.Diagnostics)
	}

	filesystem := fstest.MapFS{
		"app.yaml":  {Data: []byte("apiVersion: bean/v1alpha1\nname: Tabs\nresources: [tabs.yaml]\n")},
		"tabs.yaml": {Data: []byte("kind: Block\nname: tabs\ntype: tabs\nlabel: Tabs\ntabs:\n  - id: first\n    label: First\n    content:\n      - {type: link, label: Broken, target: 'javascript:alert(1)'}\n  - id: second\n    label: Second\n    content: []\n")},
	}
	bundle, loadDiagnostics := definition.LoadFS(filesystem, "app.yaml")
	if len(loadDiagnostics) != 0 {
		t.Fatal(loadDiagnostics)
	}
	compiled := compiler.Compile("tabs", 1, bundle.Definitions)
	for _, path := range []string{"spec.tabs.0.content.0.target", "spec.tabs.1.content"} {
		var found bool
		for _, diagnostic := range compiled.Diagnostics {
			if diagnostic.Path == path && diagnostic.Source.Path == "tabs.yaml" && diagnostic.Source.Line > 0 && diagnostic.Source.Column > 0 {
				found = true
			}
		}
		if !found {
			t.Errorf("missing located %s in %v", path, compiled.Diagnostics)
		}
	}
}

func TestLessonBlockCompilesAndNormalizesAcrossSeams(t *testing.T) {
	paragraph := []any{map[string]any{"type": "paragraph", "text": "A bounded step."}}
	lesson := lessonSpec([]any{
		lessonSection("idea", paragraph),
		map[string]any{"id": "formula", "heading": "The formula", "content": []any{formulaElement(map[string]any{"kind": "paren", "inner": literalNode("x")}, "parenthesized x")}},
	})
	definitions := []definition.Definition{
		semanticDefinition("Block", "lesson", lesson),
		semanticDefinition("Panel", "panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"lesson"}}}}),
		semanticDefinition("Sequence", "sequence", map[string]any{"route": "/sequence", "title": "Sequence", "profile": "presentation", "aspectRatio": "wide", "frames": []any{map[string]any{"name": "frame", "title": "Frame", "layout": "bullets", "panel": "panel"}}}),
	}
	result := compiler.Compile("lesson", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	block := result.App.Blocks["lesson"]
	if block.Title != "Lesson" || len(block.Sections) != 2 || block.Sections[0].Heading != "" || block.Sections[1].Heading != "The formula" {
		t.Fatalf("lesson=%+v", block)
	}
	formula := block.Sections[1].Content[0].Expr
	if formula == nil || formula.Kind != "paren" || formula.Style != "round" || formula.Inner.Kind != "literal" {
		t.Fatalf("formula defaults=%+v", formula)
	}
}

func TestLessonAndFormulaRejectInvalidContracts(t *testing.T) {
	paragraph := []any{map[string]any{"type": "paragraph", "text": "Step"}}
	section := lessonSection("one", paragraph)
	deep := literalNode("x")
	for index := 0; index < 8; index++ {
		deep = map[string]any{"kind": "paren", "inner": deep}
	}
	dense := make([]any, 7)
	for index := range dense {
		dense[index] = map[string]any{"type": "paragraph", "text": "Step"}
	}
	wide := make([]any, 25)
	for index := range wide {
		wide[index] = literalNode("x")
	}
	manyNodes := make([]any, 24)
	for index := range manyNodes {
		manyNodes[index] = literalNode("x")
	}
	tests := []struct {
		name, path string
		spec       map[string]any
	}{
		{"missing title", "spec.title", map[string]any{"type": "lesson", "sections": []any{section}}},
		{"long title", "spec.title", map[string]any{"type": "lesson", "title": strings.Repeat("界", 121), "sections": []any{section}}},
		{"missing sections", "spec.sections", map[string]any{"type": "lesson", "title": "Lesson"}},
		{"empty sections", "spec.sections", lessonSpec([]any{})},
		{"foreign content", "spec.content", map[string]any{"type": "lesson", "title": "Lesson", "sections": []any{section}, "content": paragraph}},
		{"foreign tabs", "spec.tabs", map[string]any{"type": "lesson", "title": "Lesson", "sections": []any{section}, "tabs": []any{}}},
		{"lesson field on content", "spec.title", map[string]any{"type": "content", "content": paragraph, "title": "Lesson"}},
		{"lesson field on text", "spec.sections", map[string]any{"type": "text", "text": "Text", "sections": []any{section}}},
		{"bad section id", "spec.sections.0.id", lessonSpec([]any{map[string]any{"id": "Bad", "content": paragraph}})},
		{"duplicate section id", "spec.sections.1.id", lessonSpec([]any{section, section})},
		{"long section heading", "spec.sections.0.heading", lessonSpec([]any{map[string]any{"id": "one", "heading": strings.Repeat("界", 121), "content": paragraph}})},
		{"empty section content", "spec.sections.0.content", lessonSpec([]any{lessonSection("one", []any{})})},
		{"missing formula expr", "spec.sections.0.content.0.expr", lessonSpec([]any{map[string]any{"id": "one", "content": []any{map[string]any{"type": "formula", "text": "x"}}}})},
		{"formula alt missing", "spec.sections.0.content.0.text", lessonSpec([]any{map[string]any{"id": "one", "content": []any{map[string]any{"type": "formula", "expr": literalNode("x")}}}})},
		{"formula alt long", "spec.sections.0.content.0.text", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(literalNode("x"), strings.Repeat("界", 401))}}})},
		{"formula expr not object", "spec.sections.0.content.0.expr", lessonSpec([]any{map[string]any{"id": "one", "content": []any{map[string]any{"type": "formula", "expr": "x", "text": "x"}}}})},
		{"formula node kind missing", "spec.sections.0.content.0.expr.kind", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"text": "x"}, "x")}}})},
		{"formula node kind", "spec.sections.0.content.0.expr.kind", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "matrix"}, "x")}}})},
		{"formula foreign field", "spec.sections.0.content.0.expr.inner", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "literal", "text": "x", "inner": literalNode("y")}, "x")}}})},
		{"literal blank", "spec.sections.0.content.0.expr.text", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(literalNode(" "), "x")}}})},
		{"literal long", "spec.sections.0.content.0.expr.text", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(literalNode(strings.Repeat("x", 41)), "x")}}})},
		{"group empty", "spec.sections.0.content.0.expr.parts", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "group", "parts": []any{}}, "x")}}})},
		{"group too wide", "spec.sections.0.content.0.expr.parts", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "group", "parts": wide}, "x")}}})},
		{"too many nodes", "spec.sections.0.content.0.expr", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "group", "parts": []any{map[string]any{"kind": "group", "parts": manyNodes}, map[string]any{"kind": "group", "parts": manyNodes}}}, "x")}}})},
		{"too deep", "spec.sections.0.content.0.expr.inner.inner.inner.inner.inner.inner.inner.inner", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(deep, "x")}}})},
		{"paren style", "spec.sections.0.content.0.expr.style", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "paren", "style": "curly", "inner": literalNode("x")}, "x")}}})},
		{"paren missing inner", "spec.sections.0.content.0.expr.inner", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "paren"}, "x")}}})},
		{"frac missing denominator", "spec.sections.0.content.0.expr.denominator", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "frac", "numerator": literalNode("1")}, "x")}}})},
		{"root missing index", "spec.sections.0.content.0.expr.index", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "root", "inner": literalNode("x")}, "x")}}})},
		{"sup missing exponent", "spec.sections.0.content.0.expr.exponent", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "sup", "base": literalNode("x")}, "x")}}})},
		{"sub missing subscript", "spec.sections.0.content.0.expr.subscript", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "sub", "base": literalNode("x")}, "x")}}})},
		{"func name", "spec.sections.0.content.0.expr.name", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "func", "name": "eval"}, "x")}}})},
		{"sum missing body", "spec.sections.0.content.0.expr.body", lessonSpec([]any{map[string]any{"id": "one", "content": []any{formulaElement(map[string]any{"kind": "sum", "lower": literalNode("i=1"), "upper": literalNode("n")}, "x")}}})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", test.spec)})
			if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", test.path, "BEAN-E2881") {
				t.Fatalf("missing %s in %v", test.path, result.Diagnostics)
			}
		})
	}

	nineSections := make([]any, 9)
	for index := range nineSections {
		nineSections[index] = lessonSection(fmt.Sprintf("section_%d", index), paragraph)
	}
	result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", lessonSpec(nineSections))})
	if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", "spec.sections", "BEAN-E2881") {
		t.Fatalf("nine sections accepted: %v", result.Diagnostics)
	}
	eightSections := make([]any, 8)
	for index := range eightSections {
		eightSections[index] = lessonSection(fmt.Sprintf("section_%d", index), dense)
	}
	result = compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", lessonSpec(eightSections))})
	if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", "spec.sections", "BEAN-E2881") {
		t.Fatalf("56 elements across sections accepted: %v", result.Diagnostics)
	}
}

func TestSequenceDensityCountsLessonContent(t *testing.T) {
	long := strings.Repeat("x", 400)
	definitions := []definition.Definition{
		semanticDefinition("Block", "lesson", lessonSpec([]any{
			lessonSection("one", []any{map[string]any{"type": "paragraph", "text": long}}),
			lessonSection("two", []any{map[string]any{"type": "paragraph", "text": long}}),
		})),
		semanticDefinition("Panel", "panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"lesson"}}}}),
		semanticDefinition("Sequence", "sequence", map[string]any{"route": "/sequence", "title": "Sequence", "profile": "presentation", "aspectRatio": "wide", "frames": []any{map[string]any{"name": "frame", "title": "Frame", "layout": "bullets", "panel": "panel"}}}),
	}
	result := compiler.Compile("density", 1, definitions)
	if !hasSequenceDiagnostic(result.Diagnostics, "Sequence", "sequence", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("lesson content was excluded from density: %v", result.Diagnostics)
	}
}

func TestTimelineBlockCompilesAndNormalizesAcrossSeams(t *testing.T) {
	definitions := []definition.Definition{
		semanticDefinition("Block", "timeline", timelineSpec([]any{
			timelineEntry("second", "Day 1", "Second milestone", ""),
			timelineEntry("first", "5th century BCE", "First milestone", "An era label stays literal."),
		})),
		semanticDefinition("Panel", "panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"timeline"}}}}),
		semanticDefinition("Sequence", "sequence", map[string]any{"route": "/sequence", "title": "Sequence", "profile": "presentation", "aspectRatio": "wide", "frames": []any{map[string]any{"name": "frame", "title": "Frame", "layout": "bullets", "panel": "panel"}}}),
	}
	result := compiler.Compile("timeline", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	block := result.App.Blocks["timeline"]
	if block.Title != "Timeline" || len(block.Entries) != 2 {
		t.Fatalf("timeline=%+v", block)
	}
	if block.Entries[0].ID != "second" || block.Entries[0].Label != "Day 1" || block.Entries[0].Title != "Second milestone" || block.Entries[0].Description != "" {
		t.Fatalf("source order or literal label lost: %+v", block.Entries[0])
	}
	if block.Entries[1].Label != "5th century BCE" || block.Entries[1].Description != "An era label stays literal." {
		t.Fatalf("entry=%+v", block.Entries[1])
	}
}

func TestTimelineRejectsInvalidContracts(t *testing.T) {
	entry := timelineEntry("one", "1440", "Movable type", "")
	tests := []struct {
		name, path string
		spec       map[string]any
	}{
		{"missing title", "spec.title", map[string]any{"type": "timeline", "entries": []any{entry}}},
		{"long title", "spec.title", map[string]any{"type": "timeline", "title": strings.Repeat("界", 121), "entries": []any{entry}}},
		{"missing entries", "spec.entries", map[string]any{"type": "timeline", "title": "Timeline"}},
		{"empty entries", "spec.entries", timelineSpec([]any{})},
		{"entries not a list", "spec.entries", map[string]any{"type": "timeline", "title": "Timeline", "entries": "entries"}},
		{"foreign sections", "spec.sections", map[string]any{"type": "timeline", "title": "Timeline", "entries": []any{entry}, "sections": []any{}}},
		{"foreign content", "spec.content", map[string]any{"type": "timeline", "title": "Timeline", "entries": []any{entry}, "content": []any{}}},
		{"entries on lesson", "spec.entries", map[string]any{"type": "lesson", "title": "Lesson", "sections": []any{lessonSection("one", []any{map[string]any{"type": "divider"}})}, "entries": []any{entry}}},
		{"bad entry id", "spec.entries.0.id", timelineSpec([]any{timelineEntry("Bad id", "1440", "T", "")})},
		{"duplicate entry id", "spec.entries.1.id", timelineSpec([]any{entry, entry})},
		{"missing entry id", "spec.entries.0.id", timelineSpec([]any{map[string]any{"label": "1440", "title": "T"}})},
		{"missing entry label", "spec.entries.0.label", timelineSpec([]any{map[string]any{"id": "one", "title": "T"}})},
		{"missing entry title", "spec.entries.0.title", timelineSpec([]any{map[string]any{"id": "one", "label": "1440"}})},
		{"foreign entry field", "spec.entries.0.date", timelineSpec([]any{map[string]any{"id": "one", "label": "1440", "title": "T", "date": "1440"}})},
		{"entry not object", "spec.entries.0", timelineSpec([]any{"entry"})},
		{"blank label", "spec.entries.0.label", timelineSpec([]any{timelineEntry("one", " ", "T", "")})},
		{"long label", "spec.entries.0.label", timelineSpec([]any{timelineEntry("one", strings.Repeat("界", 81), "T", "")})},
		{"long entry title", "spec.entries.0.title", timelineSpec([]any{timelineEntry("one", "1440", strings.Repeat("界", 121), "")})},
		{"long description", "spec.entries.0.description", timelineSpec([]any{timelineEntry("one", "1440", "T", strings.Repeat("界", 401))})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", test.spec)})
			if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", test.path, "BEAN-E2881") {
				t.Fatalf("missing %s in %v", test.path, result.Diagnostics)
			}
		})
	}

	seventeen := make([]any, 17)
	for index := range seventeen {
		seventeen[index] = timelineEntry(fmt.Sprintf("entry_%d", index), "1440", "T", "")
	}
	result := compiler.Compile("invalid", 1, []definition.Definition{semanticDefinition("Block", "invalid", timelineSpec(seventeen))})
	if !hasSequenceDiagnostic(result.Diagnostics, "Block", "invalid", "spec.entries", "BEAN-E2881") {
		t.Fatalf("17 entries accepted: %v", result.Diagnostics)
	}
}

func TestSequenceDensityCountsTimelineContent(t *testing.T) {
	long := strings.Repeat("x", 400)
	definitions := []definition.Definition{
		semanticDefinition("Block", "timeline", timelineSpec([]any{
			timelineEntry("one", "1440", "First", long),
			timelineEntry("two", "1945", "Second", long),
		})),
		semanticDefinition("Panel", "panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"timeline"}}}}),
		semanticDefinition("Sequence", "sequence", map[string]any{"route": "/sequence", "title": "Sequence", "profile": "presentation", "aspectRatio": "wide", "frames": []any{map[string]any{"name": "frame", "title": "Frame", "layout": "bullets", "panel": "panel"}}}),
	}
	result := compiler.Compile("density", 1, definitions)
	if !hasSequenceDiagnostic(result.Diagnostics, "Sequence", "sequence", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("timeline content was excluded from density: %v", result.Diagnostics)
	}
}

func TestSequenceDensityCountsAllTabContent(t *testing.T) {
	long := strings.Repeat("x", 400)
	definitions := []definition.Definition{
		semanticDefinition("Block", "tabs", map[string]any{"type": "tabs", "label": "Tabs", "tabs": []any{
			map[string]any{"id": "one", "label": "One", "content": []any{map[string]any{"type": "paragraph", "text": long}}},
			map[string]any{"id": "two", "label": "Two", "content": []any{map[string]any{"type": "paragraph", "text": long}}},
		}}),
		semanticDefinition("Panel", "panel", map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"tabs"}}}}),
		semanticDefinition("Sequence", "sequence", map[string]any{"route": "/sequence", "title": "Sequence", "profile": "presentation", "aspectRatio": "wide", "frames": []any{map[string]any{"name": "frame", "title": "Frame", "layout": "bullets", "panel": "panel"}}}),
	}
	result := compiler.Compile("density", 1, definitions)
	if !hasSequenceDiagnostic(result.Diagnostics, "Sequence", "sequence", "spec.frames.0", "BEAN-E2881") {
		t.Fatalf("inactive tab content was excluded from density: %v", result.Diagnostics)
	}
}

func validExtendedContent() []any {
	return []any{
		map[string]any{"type": "heading", "text": "Boundaries"},
		map[string]any{"type": "ordered_list", "items": []any{"Define", "Validate"}},
		map[string]any{"type": "link", "label": "Open", "target": "/presentations/bean?frame=architecture"},
		map[string]any{"type": "divider"},
		tableElement([]any{map[string]any{"id": "primitive", "label": "Primitive"}}, []any{[]any{"View"}, []any{""}}, "none"),
		map[string]any{"type": "audio", "source": "/media/intro.mp3", "title": "Introduction", "transcript": "Bean introduction."},
		map[string]any{"type": "youtube", "videoId": "M7lc1UVf-VE", "title": "Video", "transcript": "Video transcript."},
		map[string]any{"type": "youtube_playlist", "playlistId": "PL1234567890ABCDEFG", "title": "Playlist", "transcript": "Playlist transcript."},
		choicesElement("action", []any{map[string]any{"id": "view", "text": "View"}, map[string]any{"id": "action", "text": "Action"}}),
		formulaElement(map[string]any{"kind": "sqrt", "inner": literalNode("x")}, "square root of x"),
	}
}

func literalNode(text string) map[string]any {
	return map[string]any{"kind": "literal", "text": text}
}

func formulaElement(expr map[string]any, text string) map[string]any {
	return map[string]any{"type": "formula", "expr": expr, "text": text}
}

func lessonSection(id string, content []any) map[string]any {
	return map[string]any{"id": id, "content": content}
}

func lessonSpec(sections []any) map[string]any {
	return map[string]any{"type": "lesson", "title": "Lesson", "sections": sections}
}

func timelineEntry(id, label, title, description string) map[string]any {
	entry := map[string]any{"id": id, "label": label, "title": title}
	if description != "" {
		entry["description"] = description
	}
	return entry
}

func timelineSpec(entries []any) map[string]any {
	return map[string]any{"type": "timeline", "title": "Timeline", "entries": entries}
}

func tableElement(columns, rows []any, rowHeader string) map[string]any {
	return map[string]any{"type": "table", "caption": "Table", "columns": columns, "rows": rows, "rowHeader": rowHeader}
}

func choicesElement(answer string, choices []any) map[string]any {
	return map[string]any{"type": "choices", "question": "Choose", "choices": choices, "answer": answer}
}

func semanticDefinition(kind, name string, spec map[string]any) definition.Definition {
	return definition.Definition{APIVersion: definition.APIVersion, Kind: kind, Metadata: definition.Metadata{Name: name}, Spec: spec}
}

func TestExtendedContentAppIRRoundTripAndFormatGate(t *testing.T) {
	result := compiler.Compile("semantic", 1, []definition.Definition{semanticDefinition("Block", "semantic", map[string]any{"type": "content", "content": validExtendedContent()})})
	if len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	encoded, err := json.Marshal(result.App)
	clone, err := result.App.Clone()
	cloned, cloneJSONErr := json.Marshal(clone)
	if err != nil || cloneJSONErr != nil || string(encoded) != string(cloned) {
		t.Fatalf("clone err=%v jsonErr=%v equal=%v", err, cloneJSONErr, string(encoded) == string(cloned))
	}
	clone.FormatVersion = appir.EmailVerificationFormat
	if clone.ValidateFormat() == nil {
		t.Fatal("v19 accepted extended semantic content")
	}
	clone.FormatVersion = "bean/appir/v22"
	if clone.ValidateFormat() == nil {
		t.Fatal("future format accepted")
	}
}

func TestExtendedSemanticCapabilitiesUseCompilerBounds(t *testing.T) {
	capabilities := compiler.AgentCapabilities("test")
	if capabilities.AppIRFormat != appir.CurrentFormat || !reflect.DeepEqual(capabilities.HeadingLevels, []int{2, 3, 4}) || !reflect.DeepEqual(capabilities.LinkOpenModes, []string{"new_tab", "same_tab"}) || !reflect.DeepEqual(capabilities.TableRowHeaderModes, []string{"first", "none"}) || !reflect.DeepEqual(capabilities.TabOrientations, []string{"horizontal", "vertical"}) || !reflect.DeepEqual(capabilities.TabVariants, []string{"pills", "underline"}) {
		t.Fatalf("closed semantic capability vocabularies=%+v", capabilities)
	}
	if capabilities.MinContentChoices != 2 || capabilities.MaxContentChoices != 6 || capabilities.MaxContentMachineIDRunes != 64 || capabilities.MinTabs != 2 || capabilities.MaxTabs != 6 || capabilities.MaxTabContentElements != 24 || capabilities.MaxContentTargetRunes != 2048 || capabilities.MaxContentTranscriptRunes != 4000 {
		t.Fatalf("semantic capability bounds=%+v", capabilities)
	}
}
