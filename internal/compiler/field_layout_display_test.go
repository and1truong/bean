package compiler_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func detailLayoutDefinitions() []definition.Definition {
	defs := layoutDefinitions()
	layout := defs[1].Spec["form"].(map[string]any)["layout"]
	return append(defs, definition.Definition{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{"entity": "article", "fields": []any{"id", "title", "body"}, "displays": map[string]any{"record": map[string]any{"type": "block", "renderer": map[string]any{"type": "detail", "layout": layout}}}}})
}

func TestDetailFieldLayoutAndDiscovery(t *testing.T) {
	defs := detailLayoutDefinitions()
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) > 0 {
		t.Fatal(result.Diagnostics)
	}
	caps := compiler.AgentCapabilities("test")
	if !reflect.DeepEqual(caps.FieldLayoutColumns, []int{1, 2}) || caps.MaxFieldLayoutFields != 128 {
		t.Fatalf("caps=%+v", caps)
	}
	for _, kind := range []string{"AdminResource", "View"} {
		name := "article"
		if kind == "View" {
			name = "articles"
		}
		value, _, ok := compiler.InspectDefinition(result.App, kind, name)
		if !ok {
			t.Fatal("not inspectable")
		}
		encoded, _ := json.Marshal(value)
		if !strings.Contains(string(encoded), "Content") {
			t.Fatalf("layout absent: %s", encoded)
		}
	}
	next, err := result.App.Clone()
	if err != nil {
		t.Fatal(err)
	}
	next.Views["articles"].Displays["record"].Renderer.Layout.Groups[0].Columns = 2
	next.AdminResources["article"].Form.Layout.Groups[0].Label = "Editorial"
	changes := agentprotocol.SemanticDiff(result.App, next)
	for _, prefix := range []string{"views.articles.displays.record.renderer.layout", "adminResources.article.form.layout"} {
		found := false
		for _, change := range changes {
			found = found || strings.HasPrefix(change.Path, prefix)
		}
		if !found {
			t.Fatalf("missing %s: %+v", prefix, changes)
		}
	}
	for _, version := range []string{appir.LegacyFormat, appir.LifecycleFormat, appir.RuleFormat, appir.TestSuiteFormat, appir.ExtensionFormat, appir.DisplayFormat, appir.ExploreFormat, appir.SequenceFormat, appir.InlinePanelFormat, appir.PageSectionFormat, appir.RegionCollapseFormat, appir.PageWidthFormat, appir.MenuFormat, appir.MenuVariantFormat, appir.DirectionalFormat, appir.AuthenticationFormat, appir.PasswordRecoveryFormat} {
		legacy := appir.Empty()
		legacy.FormatVersion = version
		legacy.Views["articles"] = result.App.Views["articles"]
		if legacy.ValidateFormat() == nil {
			t.Fatalf("%s accepted detail layout", version)
		}
	}
}

func TestDetailFieldLayoutRejectsUnsafeOrAmbiguousPresentation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]definition.Definition, map[string]any)
	}{
		{"non detail", func(_ []definition.Definition, r map[string]any) { r["type"] = "list" }},
		{"serializer", func(d []definition.Definition, _ map[string]any) {
			display := d[2].Spec["displays"].(map[string]any)["record"].(map[string]any)
			display["type"] = "json"
			display["route"] = "/api/articles"
		}},
		{"relationship projection", func(_ []definition.Definition, r map[string]any) {
			r["layout"].(map[string]any)["groups"].([]any)[0].(map[string]any)["fields"].([]any)[0].(map[string]any)["field"] = "author.name"
		}},
		{"role collision", func(_ []definition.Definition, r map[string]any) { r["bodyField"] = "body" }},
		{"link collision", func(_ []definition.Definition, r map[string]any) { r["linkRoute"] = "/articles" }},
		{"missing projection", func(d []definition.Definition, _ map[string]any) { d[2].Spec["fields"] = []any{"id", "title"} }},
		{"redacted", func(d []definition.Definition, _ map[string]any) {
			d[2].Spec["policy"] = "public"
			d[3].Spec["redact"] = []any{"body"}
		}},
		{"sensitive", func(d []definition.Definition, _ map[string]any) {
			d[0].Spec["fields"].([]any)[1].(map[string]any)["sensitive"] = true
		}},
		{"null layout", func(_ []definition.Definition, r map[string]any) { r["layout"] = nil }},
		{"explicit zero", func(_ []definition.Definition, r map[string]any) {
			r["layout"].(map[string]any)["groups"].([]any)[0].(map[string]any)["columns"] = 0
		}},
		{"null span", func(_ []definition.Definition, r map[string]any) {
			r["layout"].(map[string]any)["groups"].([]any)[0].(map[string]any)["fields"].([]any)[0].(map[string]any)["span"] = nil
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defs := detailLayoutDefinitions()
			defs = append(defs, definition.Definition{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "public"}, Spec: map[string]any{}})
			renderer := defs[2].Spec["displays"].(map[string]any)["record"].(map[string]any)["renderer"].(map[string]any)
			tc.change(defs, renderer)
			result := compiler.Compile("test", 1, defs)
			found := false
			for _, diagnostic := range result.Diagnostics {
				found = found || diagnostic.Kind == "View" && strings.HasPrefix(diagnostic.Path, "spec.displays.record.renderer.layout")
			}
			if !found {
				t.Fatalf("missing detail layout diagnostic: %v", result.Diagnostics)
			}
		})
	}
}

func TestFieldLayoutSchemas(t *testing.T) {
	for _, def := range detailLayoutDefinitions()[1:] {
		document := compiler.DefinitionSchemas()[def.Kind]
		validator := jsonschema.NewCompiler()
		location := document["$id"].(string)
		if err := validator.AddResource(location, schemaJSONValue(t, document)); err != nil {
			t.Fatal(err)
		}
		schema, err := validator.Compile(location)
		if err != nil {
			t.Fatal(err)
		}
		source := map[string]any{"kind": def.Kind, "name": def.Metadata.Name}
		for key, value := range def.Spec {
			source[key] = value
		}
		if err = schema.Validate(schemaJSONValue(t, source)); err != nil {
			t.Fatal(err)
		}
		var layout map[string]any
		if def.Kind == "View" {
			layout = source["displays"].(map[string]any)["record"].(map[string]any)["renderer"].(map[string]any)["layout"].(map[string]any)
		} else {
			layout = source["form"].(map[string]any)["layout"].(map[string]any)
		}
		group := layout["groups"].([]any)[0].(map[string]any)
		for _, columns := range []any{0, 3, "2", nil} {
			group["columns"] = columns
			if schema.Validate(schemaJSONValue(t, source)) == nil {
				t.Fatalf("%s schema accepted columns=%v", def.Kind, columns)
			}
		}
		delete(group, "columns")
	}
}
