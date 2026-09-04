package appir_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
)

func TestCompatibilityV1Fixture(t *testing.T) {
	b, e := os.ReadFile("testdata/v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var app appir.App
	if e = json.Unmarshal(b, &app); e != nil {
		t.Fatal(e)
	}
	if e = app.ValidateFormat(); e != nil {
		t.Fatal(e)
	}
	encoded, e := json.Marshal(&app)
	if e != nil || !json.Valid(encoded) {
		t.Fatalf("round trip failed: %v", e)
	}
}

func TestRejectsUnsupportedFormat(t *testing.T) {
	app := appir.Empty()
	app.FormatVersion = "bean/appir/v99"
	if e := app.ValidateFormat(); e == nil {
		t.Fatal("unsupported format accepted")
	}
}

func TestCloneAndDecodePreserveProviderMockIntegerPrecision(t *testing.T) {
	app := appir.Empty()
	app.TestSuites["notify"] = appir.TestSuite{Name: "notify", Tests: []appir.TestCase{{Providers: map[string][]appir.TestProviderResult{
		"provider": {{Output: map[string]any{"sequence": json.Number("9007199254740993")}}},
	}}}}
	clone, err := app.Clone()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := appir.Decode(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for name, candidate := range map[string]*appir.App{"clone": clone, "decode": decoded} {
		sequence := candidate.TestSuites["notify"].Tests[0].Providers["provider"][0].Output["sequence"]
		number, ok := sequence.(json.Number)
		if !ok || number.String() != "9007199254740993" {
			t.Fatalf("%s sequence=%v (%T)", name, sequence, sequence)
		}
	}
}

func TestLifecycleRequiresV2Format(t *testing.T) {
	app := appir.Empty()
	app.Lifecycles["order_fulfillment"] = appir.Lifecycle{
		Name: "order_fulfillment", Entity: "order", StateField: "status", Initial: "pending",
		Transitions: map[string][]string{"pending": {"paid"}},
	}
	encoded, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	var decoded appir.App
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err = decoded.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	if decoded.Lifecycles["order_fulfillment"].Initial != "pending" {
		t.Fatalf("lifecycle=%+v", decoded.Lifecycles["order_fulfillment"])
	}
	decoded.FormatVersion = appir.LifecycleFormat
	if err = decoded.ValidateFormat(); err != nil {
		t.Fatalf("v2 AppIR rejected Lifecycle semantics: %v", err)
	}
	decoded.FormatVersion = appir.LegacyFormat
	if err = decoded.ValidateFormat(); err == nil {
		t.Fatal("v1 AppIR accepted Lifecycle semantics that a v0.8 runtime would discard")
	}
}

func TestRuleRequiresV3Format(t *testing.T) {
	app := appir.Empty()
	app.Rules["always"] = appir.Rule{Name: "always", Result: "boolean"}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.LifecycleFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v2 AppIR accepted Rule semantics that a v0.9 runtime would discard")
	}
	app.FormatVersion = appir.RuleFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v3 AppIR rejected Rule semantics: %v", err)
	}
}

func TestTestSuiteRequiresV4Format(t *testing.T) {
	app := appir.Empty()
	app.TestSuites["always"] = appir.TestSuite{Name: "always", Target: appir.TestTarget{Kind: "Rule", Name: "always"}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.RuleFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v3 AppIR accepted TestSuite semantics that a v0.10 runtime would discard")
	}
	app.FormatVersion = appir.TestSuiteFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v4 AppIR rejected TestSuite semantics: %v", err)
	}
}

func TestExtensionRequiresV5Format(t *testing.T) {
	app := appir.Empty()
	app.Extensions["notify"] = appir.Extension{Name: "notify", Transport: "http"}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.TestSuiteFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v4 AppIR accepted Extension semantics that a v0.12 runtime would discard")
	}
	app.FormatVersion = appir.ExtensionFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v5 AppIR rejected Extension semantics: %v", err)
	}

	app = appir.Empty()
	app.FormatVersion = appir.TestSuiteFormat
	app.Actions["notify"] = appir.Action{Name: "notify", Steps: []appir.Step{{Op: "extension", Extension: "notify"}}}
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v4 AppIR accepted an Extension-bound Action")
	}

	app.Actions = map[string]appir.Action{}
	app.TestSuites["notify"] = appir.TestSuite{Name: "notify", Tests: []appir.TestCase{{Providers: map[string][]appir.TestProviderResult{"notify": {{Output: map[string]any{}}}}}}}
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v4 AppIR accepted an Extension-bound TestSuite")
	}
}

func TestFirstClassViewDisplaysRequireV6Format(t *testing.T) {
	app := appir.Empty()
	app.Views["articles"] = appir.View{
		Name: "articles", Entity: "article",
		ExposedFilters: map[string]appir.ViewFilter{"status": {Field: "status", Operator: "eq"}},
		Displays: map[string]appir.Display{"index": {
			Type: "page", Route: "/articles", Title: appir.DisplayTitle{Text: "Articles"},
			Renderer: appir.ViewRenderer{Type: "table", Fields: []appir.ViewColumn{{Field: "title", Label: "Article"}}},
			Pager:    appir.ViewPager{Type: "cursor", PageSize: 25},
		}},
	}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.ExtensionFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v5 AppIR accepted first-class View display semantics")
	}

	legacy := appir.Empty()
	legacy.FormatVersion = appir.ExtensionFormat
	legacy.Views["feed"] = appir.View{Name: "feed", Displays: map[string]appir.Display{"rss": {Type: "rss", Route: "/feed.xml"}}}
	if err := legacy.ValidateFormat(); err != nil {
		t.Fatalf("v5 AppIR rejected legacy serializer displays: %v", err)
	}
}

func TestViewOwnedSearchRequiresV7Format(t *testing.T) {
	app := appir.Empty()
	app.Views["articles"] = appir.View{Name: "articles", Entity: "article", Fields: []string{"id", "title"}, Search: appir.ViewSearch{Fields: []string{"title"}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.DisplayFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v6 AppIR accepted View-owned search semantics")
	}
	app.FormatVersion = appir.ExploreFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v7 AppIR rejected Explore semantics: %v", err)
	}
}

func TestSequenceAndSemanticContentRequireV8Format(t *testing.T) {
	app := appir.Empty()
	app.Blocks["title"] = appir.Block{Name: "title", Type: "content", Content: []appir.ContentElement{{Type: "heading", Text: "Bean"}}}
	app.Sequences["bean"] = appir.Sequence{Name: "bean", Route: "/presentations/bean", Title: "Bean", Profile: "presentation", AspectRatio: "wide", Frames: []appir.SequenceFrame{{Name: "opening", Title: "Bean", Layout: "title", Panel: "opening"}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.SequenceFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v8 AppIR rejected Sequence semantics: %v", err)
	}
	app.FormatVersion = appir.ExploreFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v7 AppIR accepted Sequence and semantic content semantics")
	}

	app.Sequences = map[string]appir.Sequence{}
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v7 AppIR accepted semantic content Blocks")
	}
}

func TestInlinePanelContentRequiresV9Format(t *testing.T) {
	app := appir.Empty()
	app.Panels["frame"] = appir.Panel{Name: "frame", Regions: []appir.Region{{Name: "main", Items: []appir.RegionItem{{Identity: "@inline/frame/main/item/0", Content: []appir.ContentElement{{Type: "heading", Text: "Bean"}}}}}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	clone, err := app.Clone()
	if err != nil || clone.Panels["frame"].Regions[0].Items[0].Identity != "@inline/frame/main/item/0" {
		t.Fatalf("clone=%+v err=%v", clone, err)
	}
	app.FormatVersion = appir.InlinePanelFormat
	if err = app.ValidateFormat(); err != nil {
		t.Fatalf("v9 AppIR rejected inline Panel region items: %v", err)
	}
	app.FormatVersion = appir.SequenceFormat
	if err = app.ValidateFormat(); err == nil {
		t.Fatal("v8 AppIR accepted inline Panel region items")
	}
}

func TestOrderedPageSectionsRequireV10Format(t *testing.T) {
	app := appir.Empty()
	app.Pages["home"] = appir.Page{Name: "home", Route: "/", Sections: []appir.PageSection{{Panel: "hero"}, {Panel: "body"}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	clone, err := app.Clone()
	if err != nil || len(clone.Pages["home"].Sections) != 2 || clone.Pages["home"].Sections[1].Panel != "body" {
		t.Fatalf("clone=%+v err=%v", clone, err)
	}
	app.FormatVersion = appir.PageSectionFormat
	if err = app.ValidateFormat(); err != nil {
		t.Fatalf("v10 AppIR rejected ordered Page sections: %v", err)
	}
	app.FormatVersion = appir.InlinePanelFormat
	if err = app.ValidateFormat(); err == nil {
		t.Fatal("v9 AppIR accepted ordered Page sections")
	}

	legacy := appir.Empty()
	legacy.FormatVersion = appir.InlinePanelFormat
	legacy.Pages["home"] = appir.Page{Name: "home", Route: "/", Panel: "home"}
	if err = legacy.ValidateFormat(); err != nil {
		t.Fatalf("v9 AppIR rejected a legacy single-Panel Page: %v", err)
	}
}

func TestCollapsiblePanelRegionsRequireV11Format(t *testing.T) {
	app := appir.Empty()
	app.Panels["article"] = appir.Panel{Name: "article", Regions: []appir.Region{{Name: "sidebar", CollapseWhenEmpty: true}, {Name: "main"}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	clone, err := app.Clone()
	if err != nil || !clone.Panels["article"].Regions[0].CollapseWhenEmpty {
		t.Fatalf("clone=%+v err=%v", clone, err)
	}
	app.FormatVersion = appir.RegionCollapseFormat
	if err = app.ValidateFormat(); err != nil {
		t.Fatalf("v11 AppIR rejected collapsible Panel Regions: %v", err)
	}
	app.FormatVersion = appir.PageSectionFormat
	if err = app.ValidateFormat(); err == nil {
		t.Fatal("v10 AppIR accepted collapsible Panel Regions")
	}
	panel := app.Panels["article"]
	panel.Regions[0].CollapseWhenEmpty = false
	app.Panels["article"] = panel
	if err = app.ValidateFormat(); err != nil {
		t.Fatalf("v10 AppIR rejected legacy Panel Regions: %v", err)
	}
}

func TestSemanticPageSectionWidthsRequireV12Format(t *testing.T) {
	app := appir.Empty()
	app.Pages["article"] = appir.Page{Name: "article", Route: "/article", Sections: []appir.PageSection{{Panel: "hero", Width: "full"}, {Panel: "body", Width: "contained"}}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	clone, err := app.Clone()
	if err != nil || clone.Pages["article"].Sections[1].Width != "contained" {
		t.Fatalf("clone=%+v err=%v", clone, err)
	}
	app.FormatVersion = appir.RegionCollapseFormat
	if err = app.ValidateFormat(); err == nil {
		t.Fatal("v11 AppIR accepted semantic Page section widths")
	}
	page := app.Pages["article"]
	for index := range page.Sections {
		page.Sections[index].Width = ""
	}
	app.Pages["article"] = page
	if err = app.ValidateFormat(); err != nil {
		t.Fatalf("v11 AppIR rejected width-less Page sections: %v", err)
	}
}
