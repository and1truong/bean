package agenttest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

type explorePrompt struct {
	Prompt    string `json:"prompt"`
	Example   string `json:"example"`
	Artifacts []struct {
		Kind      string   `json:"kind"`
		Name      string   `json:"name"`
		Semantics []string `json:"semantics"`
	} `json:"artifacts"`
}

func TestExplorePromptRubricUsesOrdinaryDeterministicDefinitions(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "explore_prompts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var prompts []explorePrompt
	if err = json.Unmarshal(data, &prompts); err != nil {
		t.Fatal(err)
	}
	if len(prompts) != 5 {
		t.Fatalf("prompts=%d", len(prompts))
	}
	apps := map[string]*appir.App{}
	for _, fixture := range prompts {
		t.Run(fixture.Prompt, func(t *testing.T) {
			app := apps[fixture.Example]
			if app == nil {
				bundle, diagnostics := definition.LoadFile(filepath.Join("..", "..", "examples", fixture.Example, "app.yaml"))
				if len(diagnostics) != 0 {
					t.Fatalf("load diagnostics=%v", diagnostics)
				}
				first := compiler.Compile("default", 1, bundle.Definitions)
				second := compiler.Compile("default", 1, bundle.Definitions)
				if len(first.Diagnostics) != 0 || len(second.Diagnostics) != 0 {
					t.Fatalf("compile diagnostics=%v / %v", first.Diagnostics, second.Diagnostics)
				}
				if !reflect.DeepEqual(first.App, second.App) {
					t.Fatal("identical definitions did not produce identical AppIR")
				}
				app = first.App
				apps[fixture.Example] = app
			}
			for _, artifact := range fixture.Artifacts {
				if _, _, exists := compiler.InspectDefinition(app, artifact.Kind, artifact.Name); !exists {
					t.Fatalf("missing inspectable artifact %s/%s", artifact.Kind, artifact.Name)
				}
				for _, semantic := range artifact.Semantics {
					if !exploreSemantic(app, artifact.Kind, artifact.Name, semantic) {
						t.Errorf("%s/%s does not prove %q", artifact.Kind, artifact.Name, semantic)
					}
				}
			}
		})
	}
}

func exploreSemantic(app *appir.App, kind, name, semantic string) bool {
	view := app.Views[name]
	switch semantic {
	case "groups":
		return kind == "View" && view.ResultShape == "groups" && len(view.GroupBy) > 0
	case "chart":
		return hasRenderer(view, "chart")
	case "metric":
		return kind == "View" && view.ResultShape == "metric" && hasRenderer(view, "metric")
	case "typed_drill":
		for _, display := range view.Displays {
			if display.Drill != nil && display.Drill.View != "" && display.Drill.Route != "" {
				return true
			}
		}
	case "money_sum":
		return hasAggregate(view, "sum", "amount")
	case "policy":
		return view.Policy != ""
	case "fixed_filter":
		return view.Filter != nil
	case "records":
		return view.ResultShape == "records"
	case "multiple_selection":
		for _, display := range view.Displays {
			if display.Selection == "multiple" {
				return true
			}
		}
	case "record_action":
		for _, display := range view.Displays {
			for _, action := range display.Actions {
				if action == "move_candidate" {
					return true
				}
			}
		}
	case "lifecycle_action":
		return app.Actions["move_candidate"].Lifecycle == "candidate_pipeline"
	case "page_filters":
		return kind == "Page" && len(app.Pages[name].Filters) >= 3
	case "dashboard_composition":
		page := app.Pages[name]
		return page.Panel != "" && len(app.Panels[page.Panel].Regions) > 0
	}
	return false
}

func hasRenderer(view appir.View, renderer string) bool {
	for _, display := range view.Displays {
		if display.Renderer.Type == renderer {
			return true
		}
	}
	return false
}

func hasAggregate(view appir.View, function, field string) bool {
	for _, aggregate := range view.Aggregates {
		if aggregate.Function == function && aggregate.Field == field {
			return true
		}
	}
	return false
}
