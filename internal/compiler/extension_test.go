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
		name string
		step map[string]any
		path string
		code string
	}{
		{"missing extension", map[string]any{"op": "extension", "extension": "missing", "values": map[string]any{"order_id": "$input.order_id"}}, "spec.steps.0.extension", "BEAN-E2001"},
		{"missing input", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{}}, "spec.steps.0.values.order_id", "BEAN-E2871"},
		{"extra input", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.order_id", "extra": "value"}}, "spec.steps.0.values.extra", "BEAN-E2871"},
		{"wrong input type", map[string]any{"op": "extension", "extension": "order_notification", "values": map[string]any{"order_id": "$input.message"}}, "spec.steps.0.values.order_id", "BEAN-E2871"},
		{"result unavailable", map[string]any{"op": "extension", "extension": "order_notification", "result": "provider", "values": map[string]any{"order_id": "$input.order_id"}}, "spec.steps.0.result", "BEAN-E2871"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definitions := []definition.Definition{
				{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{}},
				validExtensionDefinition(),
				{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "notify_order"}, Spec: map[string]any{
					"entity": "order", "operation": "transaction",
					"input": map[string]any{
						"order_id": map[string]any{"type": "uuid", "required": true},
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
