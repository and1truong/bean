package compiler_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/testsuite"
)

func TestTestSuiteCompilesCanonicalAppIRAndCapabilities(t *testing.T) {
	result := compiler.Compile("test", 1, semanticSuiteDefinitions())
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	suite := result.App.TestSuites["subtotal_contract"]
	if suite.Target.Kind != "Rule" || suite.Target.Name != "subtotal" || len(suite.Tests) != 2 || suite.Tests[0].Name != "first" || suite.Tests[1].Name != "second" {
		t.Fatalf("suite=%+v", suite)
	}
	capabilities := compiler.ProtocolCapabilities("bean.cli/v1alpha1", "bean.agent/v1alpha1")
	if !reflect.DeepEqual(capabilities.TestSuiteTargets, []string{"Action", "Rule"}) || capabilities.MaxTestSuites != testsuite.MaxSuites || capabilities.MaxTestCases != testsuite.MaxCases || capabilities.MaxTestFixtures != testsuite.MaxFixtures || capabilities.MaxTestSuiteBytes != testsuite.MaxEncodedSize {
		t.Fatalf("capabilities=%+v", capabilities)
	}
	schema := compiler.DefinitionSchemas()["TestSuite"]
	properties, _ := compiler.SchemaProperties(schema)
	if properties["target"] == nil || properties["tests"] == nil {
		t.Fatalf("schema properties=%v", properties)
	}
}

func TestTestSuiteDiagnosticsAreStableAndTyped(t *testing.T) {
	tests := []struct {
		name   string
		mutate func([]definition.Definition)
		path   string
		code   string
	}{
		{"missing target", func(defs []definition.Definition) { defs[2].Spec["target"].(map[string]any)["name"] = "missing" }, "spec.target.name", "BEAN-E2001"},
		{"bad case name", func(defs []definition.Definition) {
			defs[2].Spec["tests"].([]any)[0].(map[string]any)["name"] = "Bad case"
		}, "spec.tests.0.name", "BEAN-E2851"},
		{"duplicate case", func(defs []definition.Definition) {
			defs[2].Spec["tests"].([]any)[1].(map[string]any)["name"] = "second"
		}, "spec.tests.1.name", "BEAN-E1004"},
		{"bad input", func(defs []definition.Definition) {
			defs[2].Spec["tests"].([]any)[0].(map[string]any)["input"].(map[string]any)["quantity"] = "three"
		}, "spec.tests.0.input.quantity", "BEAN-E2851"},
		{"bad result", func(defs []definition.Definition) {
			defs[2].Spec["tests"].([]any)[0].(map[string]any)["expect"].(map[string]any)["result"] = true
		}, "spec.tests.0.expect.result", "BEAN-E2851"},
		{"bad time", func(defs []definition.Definition) {
			defs[2].Spec["tests"].([]any)[0].(map[string]any)["context"] = map[string]any{"time": "tomorrow"}
		}, "spec.tests.0.context.time", "BEAN-E2851"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := semanticSuiteDefinitions()
			test.mutate(definitions)
			result := compiler.Compile("test", 1, definitions)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Kind == "TestSuite" && diagnostic.Path == test.path && diagnostic.Code == test.code {
					return
				}
			}
			t.Fatalf("missing %s %s: %v", test.code, test.path, result.Diagnostics)
		})
	}
}

func TestTestSuiteCompilerBounds(t *testing.T) {
	t.Run("cases", func(t *testing.T) {
		definitions := semanticSuiteDefinitions()
		cases := make([]any, testsuite.MaxCases+1)
		for index := range cases {
			cases[index] = map[string]any{"name": fmt.Sprintf("case_%d", index), "input": map[string]any{"quantity": index}, "expect": map[string]any{"result": index}}
		}
		definitions[2].Spec["tests"] = cases
		assertTestSuiteDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec.tests")
	})
	t.Run("fixtures", func(t *testing.T) {
		definitions := semanticSuiteDefinitions()
		rows := make([]any, testsuite.MaxFixtures+1)
		for index := range rows {
			rows[index] = map[string]any{"id": fmt.Sprintf("%08x-0000-4000-8000-%012x", index+1, index+1), "quantity": 1}
		}
		definitions[2].Spec["tests"].([]any)[0].(map[string]any)["fixtures"] = map[string]any{"order": rows}
		assertTestSuiteDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec.tests.0.fixtures")
	})
	t.Run("suites", func(t *testing.T) {
		definitions := semanticSuiteDefinitions()[:2]
		for index := 0; index <= testsuite.MaxSuites; index++ {
			definitions = append(definitions, definition.Definition{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: fmt.Sprintf("suite_%d", index)}, Spec: map[string]any{"target": map[string]any{"kind": "Rule", "name": "subtotal"}, "tests": []any{map[string]any{"name": "case_a", "input": map[string]any{"quantity": 1}, "expect": map[string]any{"result": 1}}}}})
		}
		assertTestSuiteDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec")
	})
	t.Run("encoded size", func(t *testing.T) {
		large := strings.Repeat("a", testsuite.MaxEncodedSize)
		definitions := []definition.Definition{
			{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "identity"}, Spec: map[string]any{"result": "string", "input": map[string]any{"value": map[string]any{"type": "string", "required": true}}, "expression": map[string]any{"source": "input", "path": "value"}}},
			{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "large_suite"}, Spec: map[string]any{"target": map[string]any{"kind": "Rule", "name": "identity"}, "tests": []any{map[string]any{"name": "large_case", "input": map[string]any{"value": large}, "expect": map[string]any{"result": large}}}}},
		}
		assertTestSuiteDiagnostic(t, compiler.Compile("test", 1, definitions).Diagnostics, "spec")
	})
}

func assertTestSuiteDiagnostic(t *testing.T, diagnostics []definition.Diagnostic, path string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "BEAN-E2851" && diagnostic.Path == path {
			return
		}
	}
	t.Fatalf("missing TestSuite diagnostic at %s: %v", path, diagnostics)
}

func semanticSuiteDefinitions() []definition.Definition {
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "quantity", "type": "integer", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "subtotal"}, Spec: map[string]any{"result": "integer", "input": map[string]any{"quantity": map[string]any{"type": "integer", "required": true}}, "expression": map[string]any{"source": "input", "path": "quantity"}}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "subtotal_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Rule", "name": "subtotal"},
			"tests": []any{
				map[string]any{"name": "second", "input": map[string]any{"quantity": 2}, "expect": map[string]any{"result": 2}},
				map[string]any{"name": "first", "input": map[string]any{"quantity": 1}, "expect": map[string]any{"result": 1}},
			},
		}},
	}
}
