package compiler_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	beanscenario "github.com/beanruntime/bean/internal/scenario"
)

func scenarioDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "email", "type": "email", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "create_account"}, Spec: map[string]any{"entity": "account", "operation": "create", "input": map[string]any{"email": map[string]any{"type": "email", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Scenario", Metadata: definition.Metadata{Name: "login_invalid_password"}, Spec: map[string]any{
			"title": "Login with invalid password",
			"start": "open_login",
			"nodes": []any{
				map[string]any{"id": "open_login", "type": "navigate", "url": "/login", "next": "fill_email"},
				map[string]any{"id": "fill_email", "type": "fill", "ref": "e1", "text": "user@example.test", "next": "fill_password"},
				map[string]any{"id": "fill_password", "type": "fill", "ref": "e2", "secret": "test_password", "next": "submit"},
				map[string]any{"id": "submit", "type": "click", "ref": "e3", "next": "check_error"},
				map[string]any{"id": "check_error", "type": "assert", "assertion": "text_present", "text": "Invalid credentials", "next": "record"},
				map[string]any{"id": "record", "type": "api_call", "action": "create_account", "input": map[string]any{"email": "audit@example.test"}},
			},
		}},
	}
}

func scenarioNode(defs []definition.Definition, index int) map[string]any {
	return defs[2].Spec["nodes"].([]any)[index].(map[string]any)
}

func assertScenarioDiagnosticCode(t *testing.T, diagnostics []definition.Diagnostic, path, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == "Scenario" && diagnostic.Code == code && diagnostic.Path == path {
			return
		}
	}
	t.Fatalf("missing %s Scenario diagnostic at %s: %v", code, path, diagnostics)
}

func assertScenarioDiagnostic(t *testing.T, diagnostics []definition.Diagnostic, path string) {
	assertScenarioDiagnosticCode(t, diagnostics, path, "BEAN-E2891")
}

func TestScenarioCompilesCanonicalAppIRAndCapabilities(t *testing.T) {
	result := compiler.Compile("test", 1, scenarioDefinitions())
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	item := result.App.Scenarios["login_invalid_password"]
	if item.Start != "open_login" || len(item.Nodes) != 6 || item.Nodes[1].Ref != "e1" || item.Nodes[2].Secret != "test_password" || item.Nodes[5].Action != "create_account" {
		t.Fatalf("scenario=%+v", item)
	}
	if result.App.FormatVersion != appir.ScenarioFormat {
		t.Fatalf("format=%q", result.App.FormatVersion)
	}
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", "bean.agent/v1alpha1")
	if !reflect.DeepEqual(capabilities.ScenarioNodeTypes, beanscenario.NodeTypes()) || capabilities.MaxScenarios != beanscenario.MaxScenarios || capabilities.MaxScenarioNodes != beanscenario.MaxNodes || capabilities.MaxScenarioIterations != beanscenario.MaxIterations {
		t.Fatalf("capabilities=%+v", capabilities)
	}
	schema := compiler.DefinitionSchemas()["Scenario"]
	properties, _ := compiler.SchemaProperties(schema)
	if properties["nodes"] == nil || properties["start"] == nil {
		t.Fatalf("schema properties=%v", properties)
	}
}

func TestScenarioDefaultsStartAndStepBounds(t *testing.T) {
	definitions := scenarioDefinitions()
	delete(definitions[2].Spec, "start")
	nodes := append([]any{}, definitions[2].Spec["nodes"].([]any)[:4]...)
	nodes = append(nodes, map[string]any{"id": "wait_idle", "type": "wait", "condition": "network_idle", "next": "loop"}, map[string]any{"id": "loop", "type": "loop", "body": "fill_email", "until": "url_contains", "text": "/dashboard", "next": "check_error"})
	nodes = append(nodes, definitions[2].Spec["nodes"].([]any)[4:]...)
	nodes[3].(map[string]any)["next"] = "wait_idle"
	definitions[2].Spec["nodes"] = nodes
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	item := result.App.Scenarios["login_invalid_password"]
	if item.Start != "open_login" || item.Nodes[4].TimeoutSeconds != beanscenario.DefaultTimeoutSeconds || item.Nodes[5].MaxIterations != beanscenario.DefaultIterations {
		t.Fatalf("scenario=%+v", item)
	}
}

func TestScenarioDiagnosticsAreStableAndTyped(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]definition.Definition)
		path   string
		code   string
	}{
		{"unknown node type", func(defs []definition.Definition) { scenarioNode(defs, 0)["type"] = "teleport" }, "spec.nodes.0.type", "BEAN-E2891"},
		{"unsupported field", func(defs []definition.Definition) { scenarioNode(defs, 0)["ref"] = "e9" }, "spec.nodes.0.ref", "BEAN-E2891"},
		{"missing required field", func(defs []definition.Definition) { delete(scenarioNode(defs, 0), "url") }, "spec.nodes.0.url", "BEAN-E1003"},
		{"bad node id", func(defs []definition.Definition) { scenarioNode(defs, 0)["id"] = "Open Login" }, "spec.nodes.0.id", "BEAN-E2891"},
		{"duplicate node id", func(defs []definition.Definition) { scenarioNode(defs, 1)["id"] = "open_login" }, "spec.nodes.1.id", "BEAN-E1004"},
		{"dangling edge", func(defs []definition.Definition) { scenarioNode(defs, 0)["next"] = "missing" }, "spec.nodes.0.next", "BEAN-E2891"},
		{"missing start", func(defs []definition.Definition) { defs[2].Spec["start"] = "missing" }, "spec.start", "BEAN-E2891"},
		{"missing action", func(defs []definition.Definition) { scenarioNode(defs, 5)["action"] = "missing" }, "spec.nodes.5.action", "BEAN-E2001"},
		{"bad wait condition", func(defs []definition.Definition) {
			defs[2].Spec["nodes"].([]any)[0] = map[string]any{"id": "open_login", "type": "wait", "condition": "psychic", "next": "fill_email"}
		}, "spec.nodes.0.condition", "BEAN-E2891"},
		{"wait ref missing", func(defs []definition.Definition) {
			defs[2].Spec["nodes"].([]any)[0] = map[string]any{"id": "open_login", "type": "wait", "condition": "ref_visible", "next": "fill_email"}
		}, "spec.nodes.0.ref", "BEAN-E1003"},
		{"bad assertion", func(defs []definition.Definition) { scenarioNode(defs, 4)["assertion"] = "mostly_works" }, "spec.nodes.4.assertion", "BEAN-E2891"},
		{"assertion ref missing", func(defs []definition.Definition) {
			scenarioNode(defs, 4)["assertion"] = "ref_visible"
			delete(scenarioNode(defs, 4), "text")
		}, "spec.nodes.4.ref", "BEAN-E1003"},
		{"text and secret", func(defs []definition.Definition) { scenarioNode(defs, 2)["text"] = "both" }, "spec.nodes.2", "BEAN-E2891"},
		{"bad loop", func(defs []definition.Definition) {
			scenarioNode(defs, 5)["type"] = "loop"
			scenarioNode(defs, 5)["body"] = "missing"
			delete(scenarioNode(defs, 5), "action")
			delete(scenarioNode(defs, 5), "input")
		}, "spec.nodes.5.body", "BEAN-E2891"},
		{"bad branch condition", func(defs []definition.Definition) {
			defs[2].Spec["nodes"].([]any)[5] = map[string]any{"id": "record", "type": "branch", "branches": []any{map[string]any{"condition": "vibes", "next": "open_login"}}}
		}, "spec.nodes.5.branches.0.condition", "BEAN-E2891"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := scenarioDefinitions()
			test.mutate(definitions)
			assertScenarioDiagnosticCode(t, compiler.Compile("test", 1, definitions).Diagnostics, test.path, test.code)
		})
	}
}

func TestScenarioUnreachableNodesAreDiagnosed(t *testing.T) {
	definitions := scenarioDefinitions()
	scenarioNode(definitions, 0)["next"] = "check_error"
	result := compiler.Compile("test", 1, definitions)
	assertScenarioDiagnostic(t, result.Diagnostics, "spec.nodes.1.id")
	assertScenarioDiagnostic(t, result.Diagnostics, "spec.nodes.2.id")
	assertScenarioDiagnostic(t, result.Diagnostics, "spec.nodes.3.id")
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Path == "spec.nodes.4.id" || diagnostic.Path == "spec.nodes.5.id" {
			t.Fatalf("reachable node diagnosed: %v", result.Diagnostics)
		}
	}
}

func TestScenarioCompilerBounds(t *testing.T) {
	t.Run("nodes", func(t *testing.T) {
		definitions := scenarioDefinitions()
		nodes := make([]any, beanscenario.MaxNodes+1)
		for index := range nodes {
			next := fmt.Sprintf("n%d", index+1)
			if index == len(nodes)-1 {
				next = ""
			}
			nodes[index] = map[string]any{"id": fmt.Sprintf("n%d", index), "type": "pause", "next": next}
		}
		definitions[2].Spec["nodes"] = nodes
		definitions[2].Spec["start"] = "n0"
		assertScenarioDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec.nodes")
	})
	t.Run("scenarios", func(t *testing.T) {
		definitions := scenarioDefinitions()[:2]
		for index := 0; index <= beanscenario.MaxScenarios; index++ {
			definitions = append(definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "Scenario", Metadata: definition.Metadata{Name: fmt.Sprintf("flow_%d", index)}, Spec: map[string]any{"nodes": []any{map[string]any{"id": "start", "type": "pause"}}}})
		}
		assertScenarioDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec")
	})
	t.Run("branches", func(t *testing.T) {
		definitions := scenarioDefinitions()
		branches := make([]any, beanscenario.MaxBranches+1)
		for index := range branches {
			branches[index] = map[string]any{"condition": "last_step_passed", "next": "fill_email"}
		}
		definitions[2].Spec["nodes"].([]any)[5] = map[string]any{"id": "record", "type": "branch", "branches": branches}
		assertScenarioDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec.nodes.5.branches")
	})
}

func TestScenarioFormatGateRejectsLegacyAppIR(t *testing.T) {
	result := compiler.Compile("test", 1, scenarioDefinitions())
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	app, err := result.App.Clone()
	if err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.SemanticContentFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v20 AppIR accepted Scenario definitions")
	}
}
