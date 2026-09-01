package compiler

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"unicode"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
)

const JSONSchemaVersion = "https://json-schema.org/draft/2020-12/schema"

type Capabilities struct {
	DefinitionAPIVersion string   `json:"definitionAPIVersion"`
	CLIAPIVersion        string   `json:"cliAPIVersion"`
	AppIRFormat          string   `json:"appIRFormat"`
	DefinitionKinds      []string `json:"definitionKinds"`
	FieldTypes           []string `json:"fieldTypes"`
	ActionOperations     []string `json:"actionOperations"`
	ActionSteps          []string `json:"actionSteps"`
	BlockTypes           []string `json:"blockTypes"`
	Presentations        []string `json:"presentations"`
	DisplaySerializers   []string `json:"displaySerializers"`
	PanelLayouts         []string `json:"panelLayouts"`
	DatabaseBackends     []string `json:"databaseBackends"`
	MaxViewLimit         int      `json:"maxViewLimit"`
	MaxFileBytes         int      `json:"maxFileBytes"`
	ThemePresets         []string `json:"themePresets"`
	ThemeAccents         []string `json:"themeAccents"`
	DemoSeedProfiles     []string `json:"demoSeedProfiles"`
}

func AgentCapabilities(cliAPIVersion string) Capabilities {
	kinds := make([]string, 0, len(definition.Kinds))
	for kind := range definition.Kinds {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	return Capabilities{
		DefinitionAPIVersion: definition.APIVersion,
		CLIAPIVersion:        cliAPIVersion,
		AppIRFormat:          appir.CurrentFormat,
		DefinitionKinds:      kinds,
		FieldTypes:           []string{"boolean", "date", "datetime", "decimal", "email", "enum", "file", "integer", "json", "money", "password", "relation", "richtext", "slug", "string", "text", "url", "uuid"},
		ActionOperations:     []string{"create", "delete", "register_local_user", "transaction", "transition", "update"},
		ActionSteps:          []string{"assert", "assert_no_overlap", "conditional_update", "create", "decrement", "delete", "emit", "load", "query", "return", "schedule", "transition", "update"},
		BlockTypes:           []string{"action", "entity", "menu", "resource-list", "text", "view", "webform"},
		Presentations:        []string{"board", "detail", "list", "metric", "timeline", "tree"},
		DisplaySerializers:   []string{"csv", "json", "rss"},
		PanelLayouts:         []string{"grid", "main-sidebar", "sidebar-main", "single-column", "two-column"},
		DatabaseBackends:     []string{"postgresql", "sqlite"},
		MaxViewLimit:         200,
		MaxFileBytes:         5 << 20,
		ThemePresets:         []string{"minimal", "professional", "warm"},
		ThemeAccents:         []string{"amber", "blue", "emerald", "indigo", "rose", "slate", "violet"},
		DemoSeedProfiles:     []string{"activities", "auto", "companies", "jobs", "notes", "people"},
	}
}

func ManifestSchema() map[string]any {
	return map[string]any{
		"$schema":              JSONSchemaVersion,
		"$id":                  "https://bean.build/schemas/bean.schema.json",
		"title":                "Bean application manifest",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"apiVersion", "name"},
		"properties": map[string]any{
			"apiVersion": map[string]any{"const": definition.APIVersion},
			"name":       map[string]any{"type": "string", "minLength": 1},
			"resources":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "uniqueItems": true},
		},
	}
}

func DefinitionSchemas() map[string]map[string]any {
	types := specificationTypes()
	kinds := make([]string, 0, len(types))
	for kind := range types {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	out := make(map[string]map[string]any, len(kinds))
	for _, kind := range kinds {
		out[kind] = definitionSchema(kind, types[kind])
	}
	return out
}

func SchemaProperties(schema map[string]any) (map[string]any, bool) {
	properties, ok := schema["properties"].(map[string]any)
	return properties, ok
}

func specificationTypes() map[string]reflect.Type {
	return map[string]reflect.Type{
		"Action":            reflect.TypeOf(actionSource{}),
		"AdminResource":     reflect.TypeOf(appir.AdminResource{}),
		"Block":             reflect.TypeOf(appir.Block{}),
		"Entity":            reflect.TypeOf(appir.Entity{}),
		"Filter":            reflect.TypeOf(appir.Filter{}),
		"Job":               reflect.TypeOf(appir.Job{}),
		"LocalRegistration": reflect.TypeOf(appir.LocalRegistration{}),
		"Menu":              reflect.TypeOf(appir.Menu{}),
		"Page":              reflect.TypeOf(appir.Page{}),
		"Panel":             reflect.TypeOf(appir.Panel{}),
		"Policy":            reflect.TypeOf(appir.Policy{}),
		"Role":              reflect.TypeOf(appir.Role{}),
		"Theme":             reflect.TypeOf(appir.Theme{}),
		"DemoSeed":          reflect.TypeOf(appir.DemoSeed{}),
		"View":              reflect.TypeOf(appir.View{}),
		"Webform":           reflect.TypeOf(appir.Webform{}),
	}
}

func definitionSchema(kind string, specification reflect.Type) map[string]any {
	builder := schemaBuilder{definitions: map[string]any{}}
	properties := map[string]any{
		"apiVersion": map[string]any{"const": definition.APIVersion},
		"kind":       map[string]any{"const": kind},
		"name":       map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$"},
		"namespace":  map[string]any{"type": "string"},
	}
	for name, schema := range builder.structProperties(specification, true) {
		properties[name] = schema
	}
	document := map[string]any{
		"$schema":              JSONSchemaVersion,
		"$id":                  "https://bean.build/schemas/" + strings.ToLower(kind) + ".schema.json",
		"title":                "Bean " + kind + " definition",
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"kind", "name"},
		"properties":           properties,
	}
	if len(builder.definitions) > 0 {
		document["$defs"] = builder.definitions
	}
	return document
}

type schemaBuilder struct {
	definitions map[string]any
}

func (builder *schemaBuilder) schema(valueType reflect.Type) any {
	if valueType == reflect.TypeOf(json.RawMessage{}) {
		return map[string]any{}
	}
	if valueType.Kind() == reflect.Pointer {
		return builder.schema(valueType.Elem())
	}
	switch valueType.Kind() {
	case reflect.Interface:
		return map[string]any{}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Slice, reflect.Array:
		return map[string]any{"type": "array", "items": builder.schema(valueType.Elem())}
	case reflect.Map:
		return map[string]any{"type": "object", "additionalProperties": builder.schema(valueType.Elem())}
	case reflect.Struct:
		name := schemaDefinitionName(valueType)
		if _, exists := builder.definitions[name]; !exists {
			builder.definitions[name] = map[string]any{}
			builder.definitions[name] = map[string]any{"type": "object", "additionalProperties": false, "properties": builder.structProperties(valueType, false)}
		}
		return map[string]any{"$ref": "#/$defs/" + name}
	default:
		return map[string]any{}
	}
}

func (builder *schemaBuilder) structProperties(valueType reflect.Type, specificationRoot bool) map[string]any {
	properties := map[string]any{}
	for index := 0; index < valueType.NumField(); index++ {
		field := valueType.Field(index)
		if field.PkgPath != "" || specificationRoot && field.Name == "Name" {
			continue
		}
		name := schemaFieldName(field)
		if name == "" || name == "-" {
			continue
		}
		properties[name] = builder.schema(field.Type)
	}
	return properties
}

func schemaFieldName(field reflect.StructField) string {
	for _, tagName := range []string{"json", "yaml"} {
		if tag := field.Tag.Get(tagName); tag != "" {
			name := strings.Split(tag, ",")[0]
			if name != "" {
				return name
			}
		}
	}
	runes := []rune(field.Name)
	runes[0] = unicode.ToLower(runes[0])
	return string(runes)
}

func schemaDefinitionName(valueType reflect.Type) string {
	path := strings.NewReplacer("/", "_", ".", "_", "-", "_").Replace(valueType.PkgPath())
	return path + "_" + valueType.Name()
}
