package agentprotocol_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
)

func TestExtensionInspectionAndSemanticDiff(t *testing.T) {
	current := appir.Empty()
	current.Extensions["notify"] = appir.Extension{
		Name: "notify", Transport: "http", Endpoint: "https://provider.example/notify",
		Input:  map[string]appir.Field{"message": {Name: "message", Type: "string", Required: true}},
		Output: map[string]appir.Field{"accepted": {Name: "accepted", Type: "boolean", Required: true}},
	}
	value, references, exists := compiler.InspectDefinition(current, "Extension", "notify")
	if !exists || value == nil || len(references) != 0 {
		t.Fatalf("value=%+v references=%+v", value, references)
	}
	candidate, err := current.Clone()
	if err != nil {
		t.Fatal(err)
	}
	item := candidate.Extensions["notify"]
	item.Endpoint = "https://provider.example/v2/notify"
	candidate.Extensions["notify"] = item
	changes := agentprotocol.SemanticDiff(current, candidate)
	if len(changes) != 1 || changes[0].Path != "extensions.notify.endpoint" {
		t.Fatalf("changes=%+v", changes)
	}
}
