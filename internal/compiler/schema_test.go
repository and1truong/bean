package compiler_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

func TestCanonicalSchemasCoverMaintainedExamples(t *testing.T) {
	schemas := compiler.DefinitionSchemas()
	compiled := map[string]*jsonschema.Schema{}
	for kind, document := range schemas {
		validator := jsonschema.NewCompiler()
		location := document["$id"].(string)
		if err := validator.AddResource(location, schemaJSONValue(t, document)); err != nil {
			t.Fatal(err)
		}
		var err error
		compiled[kind], err = validator.Compile(location)
		if err != nil {
			t.Fatalf("compile %s schema: %v", kind, err)
		}
	}
	for _, application := range []string{"asana", "ats", "blog", "booking", "cms", "commerce", "community", "crm", "presentation", "saas", "tracker"} {
		bundle, err := examples.Load(application)
		if err != nil {
			t.Fatal(err)
		}
		for _, definition := range bundle.Definitions {
			properties, ok := compiler.SchemaProperties(schemas[definition.Kind])
			if !ok {
				t.Fatalf("%s: no schema for %s", application, definition.Kind)
			}
			for property := range definition.Spec {
				if _, exists := properties[property]; !exists {
					t.Errorf("%s: %s/%s property %s is missing from canonical schema", application, definition.Kind, definition.Metadata.Name, property)
				}
			}
			if err = compiled[definition.Kind].Validate(flatDefinition(definition)); err != nil {
				t.Errorf("%s: %s/%s rejected by canonical schema: %v", application, definition.Kind, definition.Metadata.Name, err)
			}
		}
	}
}

func TestPublishedCanonicalSchemasDoNotDrift(t *testing.T) {
	schemas := compiler.DefinitionSchemas()
	schemas["Bean"] = compiler.ManifestSchema()
	for kind, expected := range schemas {
		path := filepath.Join("..", "..", "schemas", strings.ToLower(kind)+".schema.json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]any
		if err = json.Unmarshal(data, &actual); err != nil {
			t.Fatal(err)
		}
		expectedJSON, _ := json.Marshal(expected)
		var normalizedExpected map[string]any
		_ = json.Unmarshal(expectedJSON, &normalizedExpected)
		if !reflect.DeepEqual(actual, normalizedExpected) {
			t.Fatalf("%s drifted; regenerate with bean schema --output schemas", path)
		}
	}
}

func TestCanonicalSchemaRejectsUnknownPropertiesByContract(t *testing.T) {
	schema := compiler.DefinitionSchemas()["Entity"]
	if schema["additionalProperties"] != false {
		t.Fatalf("Entity additionalProperties = %#v", schema["additionalProperties"])
	}
	properties, ok := compiler.SchemaProperties(schema)
	if !ok || properties["fields"] == nil || properties["label"] == nil {
		t.Fatalf("Entity properties = %#v", properties)
	}
	if properties["labell"] != nil {
		t.Fatal("unknown property labell is present")
	}
	validator := jsonschema.NewCompiler()
	location := schema["$id"].(string)
	if err := validator.AddResource(location, schemaJSONValue(t, schema)); err != nil {
		t.Fatal(err)
	}
	compiled, err := validator.Compile(location)
	if err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []map[string]any{
		{"kind": "Entity", "name": "candidate", "fields": []any{}, "labell": "Candidates"},
		{"kind": "Entity", "name": "candidate", "fields": "not-an-array"},
		{"kind": "Entity", "fields": []any{}},
	} {
		if err = compiled.Validate(invalid); err == nil {
			t.Fatalf("schema accepted invalid Entity: %#v", invalid)
		}
	}
}

func TestViewSchemaAcceptsLegacyAndTypedGroups(t *testing.T) {
	document := compiler.DefinitionSchemas()["View"]
	validator := jsonschema.NewCompiler()
	location := document["$id"].(string)
	if err := validator.AddResource(location, schemaJSONValue(t, document)); err != nil {
		t.Fatal(err)
	}
	compiled, err := validator.Compile(location)
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range []any{"status", map[string]any{"field": "status", "as": "pipeline_stage"}} {
		definition := map[string]any{"kind": "View", "name": "candidate_groups", "entity": "candidate", "groupBy": []any{group}}
		if err = compiled.Validate(definition); err != nil {
			t.Fatalf("groupBy entry %#v rejected: %v", group, err)
		}
	}
}

func TestPanelSchemaAcceptsOrderedInlineContentAndRejectsAmbiguousItems(t *testing.T) {
	document := compiler.DefinitionSchemas()["Panel"]
	validator := jsonschema.NewCompiler()
	location := document["$id"].(string)
	if err := validator.AddResource(location, schemaJSONValue(t, document)); err != nil {
		t.Fatal(err)
	}
	compiled, err := validator.Compile(location)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"kind": "Panel", "name": "frame", "layout": "single-column", "regions": []any{map[string]any{"name": "main", "items": []any{
		map[string]any{"id": "intro", "content": []any{map[string]any{"type": "heading", "text": "Inline"}}},
		map[string]any{"block": "chart"},
	}}}}
	if err = compiled.Validate(valid); err != nil {
		t.Fatalf("valid ordered region rejected: %v", err)
	}
	for _, invalid := range []map[string]any{
		{"kind": "Panel", "name": "frame", "regions": []any{map[string]any{"name": "main", "blocks": []any{"chart"}, "items": []any{map[string]any{"block": "chart"}}}}},
		{"kind": "Panel", "name": "frame", "regions": []any{map[string]any{"name": "main", "items": []any{map[string]any{"block": "chart", "content": []any{map[string]any{"type": "heading", "text": "Inline"}}}}}}},
		{"kind": "Panel", "name": "frame", "regions": []any{map[string]any{"name": "main", "items": []any{map[string]any{"content": []any{}}}}}},
	} {
		if err = compiled.Validate(invalid); err == nil {
			t.Fatalf("schema accepted ambiguous Panel: %#v", invalid)
		}
	}
}

func TestCanonicalManifestSchemaValidatesContract(t *testing.T) {
	document := compiler.ManifestSchema()
	validator := jsonschema.NewCompiler()
	location := document["$id"].(string)
	if err := validator.AddResource(location, schemaJSONValue(t, document)); err != nil {
		t.Fatal(err)
	}
	compiled, err := validator.Compile(location)
	if err != nil {
		t.Fatal(err)
	}
	valid := map[string]any{"apiVersion": definition.APIVersion, "name": "Applicant Tracking", "resources": []any{"entities.yaml"}}
	if err = compiled.Validate(valid); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	for _, invalid := range []map[string]any{
		{"name": "Applicant Tracking"},
		{"apiVersion": "bean/v0", "name": "Applicant Tracking"},
		{"apiVersion": definition.APIVersion, "name": "Applicant Tracking", "resource": "entities.yaml"},
	} {
		if err = compiled.Validate(invalid); err == nil {
			t.Fatalf("schema accepted invalid manifest: %#v", invalid)
		}
	}
}

func flatDefinition(source definition.Definition) map[string]any {
	value := map[string]any{
		"apiVersion": source.APIVersion,
		"kind":       source.Kind,
		"name":       source.Metadata.Name,
	}
	if source.Metadata.Namespace != "" && source.Metadata.Namespace != "default" {
		value["namespace"] = source.Metadata.Namespace
	}
	for key, item := range source.Spec {
		value[key] = item
	}
	return value
}

func schemaJSONValue(t *testing.T, schema map[string]any) any {
	t.Helper()
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	var value any
	if err = json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	return value
}
