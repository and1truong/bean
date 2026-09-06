package compiler_test

import (
	"encoding/json"
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
	}
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
	clone.FormatVersion = "bean/appir/v21"
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
