package compiler

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	beanextension "github.com/beanruntime/bean/internal/extension"
)

func TestExtensionDefinitionCompilesToCanonicalAppIRAndCapabilities(t *testing.T) {
	result := Compile("test", 1, []definition.Definition{validExtensionDefinition()})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	item := result.App.Extensions["order_notification"]
	if item.Name != "order_notification" || item.Transport != beanextension.TransportHTTP || item.Input["order_id"].Name != "order_id" || item.Output["accepted"].Name != "accepted" {
		t.Fatalf("extension=%+v", item)
	}
	if result.App.FormatVersion != appir.CurrentFormat {
		t.Fatalf("format=%s", result.App.FormatVersion)
	}

	capabilities := AgentCapabilities("test")
	if !hasExtensionTestString(capabilities.DefinitionKinds, "Extension") || !reflect.DeepEqual(capabilities.ExtensionTransports, []string{"http"}) || !reflect.DeepEqual(capabilities.ExtensionAuthentication, []string{"bearer", "none"}) || capabilities.MaxExtensionTimeout != beanextension.MaxTimeoutSeconds || capabilities.MaxExtensionAttempts != beanextension.MaxAttempts || capabilities.MaxExtensionDelay != beanextension.MaxDelaySeconds || capabilities.MaxExtensionResponse != beanextension.MaxResponseBytes {
		t.Fatalf("capabilities=%+v", capabilities)
	}

	schema := DefinitionSchemas()["Extension"]
	properties, _ := SchemaProperties(schema)
	transport := properties["transport"].(map[string]any)
	if !reflect.DeepEqual(transport["enum"], []string{"http"}) || !hasExtensionTestString(schema["required"].([]string), "retry") {
		t.Fatalf("schema=%+v", schema)
	}
}

func TestExtensionDefinitionRejectsUnsafeOrUnboundedContracts(t *testing.T) {
	item := validExtensionDefinition()
	item.Spec["transport"] = "script"
	item.Spec["endpoint"] = "http://metadata.internal/send?token=secret"
	item.Spec["permissions"] = []any{"filesystem"}
	item.Spec["sideEffects"] = []any{"database"}
	item.Spec["authentication"] = "custom_header"
	item.Spec["timeoutSeconds"] = 31
	item.Spec["retry"] = map[string]any{"maxAttempts": 11, "delaySeconds": 0}
	item.Spec["idempotency"] = "optional"
	item.Spec["transaction"] = "in_transaction"
	item.Spec["failure"] = "ignore"
	item.Spec["input"] = map[string]any{"token": map[string]any{"type": "password", "sensitive": true}}
	item.Spec["output"] = map[string]any{"items": map[string]any{"type": "relation"}}

	result := Compile("test", 1, []definition.Definition{item})
	paths := map[string]bool{}
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == "Extension" {
			if diagnostic.Code != "BEAN-E2871" {
				t.Fatalf("diagnostic=%+v", diagnostic)
			}
			paths[diagnostic.Path] = true
		}
	}
	for _, path := range []string{"spec.transport", "spec.endpoint", "spec.permissions", "spec.sideEffects", "spec.authentication", "spec.timeoutSeconds", "spec.retry.maxAttempts", "spec.retry.delaySeconds", "spec.idempotency", "spec.transaction", "spec.failure", "spec.input.token.type", "spec.output.items.type"} {
		if !paths[path] {
			t.Errorf("missing diagnostic path %s: %v", path, result.Diagnostics)
		}
	}
}

func TestExtensionEndpointAllowsHTTPSAndLoopbackHTTPOnly(t *testing.T) {
	for _, endpoint := range []string{"https://provider.example/v1/send", "http://localhost:8080/send", "http://127.0.0.1:8080/send", "http://[::1]:8080/send"} {
		item := validExtensionDefinition()
		item.Spec["endpoint"] = endpoint
		if diagnostics := Compile("test", 1, []definition.Definition{item}).Diagnostics; len(diagnostics) != 0 {
			t.Errorf("endpoint %s: %v", endpoint, diagnostics)
		}
	}
}

func TestExtensionEndpointRejectsMissingHostOrInvalidPort(t *testing.T) {
	for _, endpoint := range []string{"https://:443/callback", "https://provider.example:0/callback", "https://provider.example:65536/callback"} {
		item := validExtensionDefinition()
		item.Spec["endpoint"] = endpoint
		result := Compile("test", 1, []definition.Definition{item})
		found := false
		for _, diagnostic := range result.Diagnostics {
			found = found || diagnostic.Kind == "Extension" && diagnostic.Code == "BEAN-E2871" && diagnostic.Path == "spec.endpoint"
		}
		if !found {
			t.Errorf("endpoint %s diagnostics=%v", endpoint, result.Diagnostics)
		}
	}
}

func TestTransactionActionBindsTypedExtensionInputs(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		validExtensionDefinition(),
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction",
			"input": map[string]any{"order_id": map[string]any{"type": "uuid", "required": true}},
			"steps": []any{map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.order_id"}}},
		}},
	}
	result := Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	step := result.App.Actions["notify_order"].Steps[0]
	if step.Extension != "order_notification" || len(step.Values) != 1 || step.Values[0].Value.Source != "input" {
		t.Fatalf("step=%+v", step)
	}
	_, references, exists := InspectDefinition(result.App, "Action", "notify_order")
	if !exists || len(references) != 2 || references[1] != (DefinitionReference{Path: "steps.0.extension", Kind: "Extension", Name: "order_notification"}) {
		t.Fatalf("references=%+v", references)
	}
}

func TestTransactionActionRejectsInvalidExtensionBindings(t *testing.T) {
	tests := []struct {
		name            string
		step            map[string]any
		path            string
		code            string
		orderIDRequired bool
	}{
		{"missing extension", map[string]any{"op": "extension", "extension": "missing", "values": map[string]any{"order_id": "$input.order_id"}}, "spec.steps.0.extension", "BEAN-E2001", true},
		{"missing input", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{}}, "spec.steps.0.values.order_id", "BEAN-E2871", true},
		{"extra input", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.order_id", "extra": "value"}}, "spec.steps.0.values.extra", "BEAN-E2871", true},
		{"wrong input type", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.message"}}, "spec.steps.0.values.order_id", "BEAN-E2871", true},
		{"result unavailable", map[string]any{"op": "extension", "extension": "order_notification", "result": "provider", "values": map[string]any{"order_id": "$input.order_id"}}, "spec.steps.0.result", "BEAN-E2871", true},
		{"optional input", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.order_id"}}, "spec.steps.0.values.order_id", "BEAN-E2871", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := []definition.Definition{
				{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
				validExtensionDefinition(),
				{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
					"entity": "order", "operation": "transaction",
					"input": map[string]any{
						"order_id": map[string]any{"type": "uuid", "required": test.orderIDRequired},
						"message":  map[string]any{"type": "string", "required": true},
					},
					"steps": []any{test.step},
				}},
			}
			result := Compile("test", 1, definitions)
			found := false
			for _, diagnostic := range result.Diagnostics {
				found = found || diagnostic.Kind == "Action" && diagnostic.Name == "notify_order" && diagnostic.Path == test.path && diagnostic.Code == test.code
			}
			if !found {
				t.Fatalf("diagnostics=%v", result.Diagnostics)
			}
		})
	}
}

func TestActionRejectsFractionalExtensionIntegerLiteral(t *testing.T) {
	extensionDefinition := validExtensionDefinition()
	extensionDefinition.Spec["input"] = map[string]any{"sequence": map[string]any{"type": "integer", "required": true}}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		extensionDefinition,
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction", "steps": []any{map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"sequence": 1.5}}},
		}},
	}
	result := Compile("test", 1, definitions)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == "Action" && diagnostic.Code == "BEAN-E2871" && diagnostic.Path == "spec.steps.0.values.sequence" {
			return
		}
	}
	t.Fatalf("diagnostics=%v", result.Diagnostics)
}

func TestActionPreservesExtensionIntegerLiteral(t *testing.T) {
	extensionDefinition := validExtensionDefinition()
	extensionDefinition.Spec["input"] = map[string]any{"sequence": map[string]any{"type": "integer", "required": true}}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		extensionDefinition,
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction", "input": map[string]any{"reason": map[string]any{"type": "string"}}, "steps": []any{map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"sequence": int64(9007199254740993)}}},
		}},
	}
	result := Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}
	if literal := result.App.Actions["notify_order"].Steps[0].Values[0].Value.Literal; string(literal) != "9007199254740993" {
		t.Fatalf("literal=%s", literal)
	}
}

func TestActionEmitRejectsReservedExtensionTopicPrefix(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "place_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction", "steps": []any{map[string]any{"op": "emit", "event": beanextension.TopicPrefix + "order_notification"}},
		}},
	}
	result := Compile("test", 1, definitions)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == "Action" && diagnostic.Code == "BEAN-E2871" && diagnostic.Path == "spec.steps.0.event" {
			return
		}
	}
	t.Fatalf("diagnostics=%v", result.Diagnostics)
}

func TestActionTestSuiteValidatesExtensionMocksAndCalls(t *testing.T) {
	definitions := extensionTestSuiteDefinitions()
	if result := Compile("test", 1, definitions); len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", result.Diagnostics)
	}

	tests := []struct {
		name   string
		mutate func(map[string]any)
		path   string
	}{
		{"unknown provider", func(test map[string]any) {
			test["providers"] = map[string]any{"missing": []any{map[string]any{"output": map[string]any{"accepted": true}}}}
		}, "spec.tests.0.providers.missing"},
		{"invalid output", func(test map[string]any) {
			test["providers"].(map[string]any)["order_notification"] = []any{map[string]any{"output": map[string]any{"accepted": "yes"}}}
		}, "spec.tests.0.providers.order_notification.0.output"},
		{"output and error", func(test map[string]any) {
			test["providers"].(map[string]any)["order_notification"] = []any{map[string]any{"output": map[string]any{"accepted": true}, "error": beanextension.FailureTimeout}}
		}, "spec.tests.0.providers.order_notification.0"},
		{"unstable error", func(test map[string]any) {
			test["providers"].(map[string]any)["order_notification"] = []any{map[string]any{"error": "network broke"}}
		}, "spec.tests.0.providers.order_notification.0.error"},
		{"wrong idempotency key", func(test map[string]any) {
			test["expect"].(map[string]any)["providerCalls"].([]any)[0].(map[string]any)["idempotencyKey"] = "different"
		}, "spec.tests.0.expect.providerCalls.0.idempotencyKey"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := extensionTestSuiteDefinitions()
			caseDefinition := definitions[3].Spec["tests"].([]any)[0].(map[string]any)
			test.mutate(caseDefinition)
			result := Compile("test", 1, definitions)
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Kind == "TestSuite" && diagnostic.Path == test.path {
					return
				}
			}
			t.Fatalf("missing diagnostic at %s: %v", test.path, result.Diagnostics)
		})
	}
}

func extensionTestSuiteDefinitions() []definition.Definition {
	const invocationID = "00000000-0000-4000-8000-000000000020"
	return []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
		validExtensionDefinition(),
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
			"entity": "order", "operation": "transaction", "input": map[string]any{"order_id": map[string]any{"type": "uuid", "required": true}},
			"steps": []any{map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.order_id"}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "notification_contract"}, Spec: map[string]any{
			"target": map[string]any{"kind": "Action", "name": "notify_order"}, "tests": []any{map[string]any{
				"name": "notifies", "context": map[string]any{"time": "2026-09-01T10:00:00Z", "ids": []any{invocationID}}, "input": map[string]any{"order_id": invocationID},
				"providers": map[string]any{"order_notification": []any{map[string]any{"output": map[string]any{"accepted": true}}}},
				"expect":    map[string]any{"providerCalls": []any{map[string]any{"extension": "order_notification", "invocationId": invocationID, "idempotencyKey": invocationID, "input": map[string]any{"order_id": invocationID}}}},
			}},
		}},
	}
}

func validExtensionDefinition() definition.Definition {
	return definition.Definition{APIVersion: definition.APIVersion, Kind: "Extension", Metadata: definition.Metadata{Name: "order_notification"}, Spec: map[string]any{
		"transport": "http", "endpoint": "https://provider.example/v1/orders",
		"input":       map[string]any{"order_id": map[string]any{"type": "uuid", "required": true}},
		"output":      map[string]any{"accepted": map[string]any{"type": "boolean", "required": true}},
		"permissions": []any{"network"}, "sideEffects": []any{"external_write"}, "authentication": "bearer",
		"timeoutSeconds": 5, "retry": map[string]any{"maxAttempts": 3, "delaySeconds": 60},
		"idempotency": "required", "transaction": "after_commit", "failure": "retry_then_fail",
	}}
}

func hasExtensionTestString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
