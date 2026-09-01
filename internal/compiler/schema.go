package compiler

import (
	"encoding/json"
	"reflect"
	"strings"
	"unicode"

	"github.com/beanruntime/bean/internal/actionop"
	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/block"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/rule"
	"github.com/beanruntime/bean/internal/testsuite"
)

const JSONSchemaVersion = "https://json-schema.org/draft/2020-12/schema"

type Capabilities struct {
	DefinitionAPIVersion    string   `json:"definitionAPIVersion"`
	CLIAPIVersion           string   `json:"cliAPIVersion"`
	AgentProtocolAPIVersion string   `json:"agentProtocolAPIVersion,omitempty"`
	AppIRFormat             string   `json:"appIRFormat"`
	DefinitionKinds         []string `json:"definitionKinds"`
	SemanticPrimitives      []string `json:"semanticPrimitives"`
	FieldTypes              []string `json:"fieldTypes"`
	ActionOperations        []string `json:"actionOperations"`
	ActionSteps             []string `json:"actionSteps"`
	BlockTypes              []string `json:"blockTypes"`
	Presentations           []string `json:"presentations"`
	DisplaySerializers      []string `json:"displaySerializers"`
	PanelLayouts            []string `json:"panelLayouts"`
	DatabaseBackends        []string `json:"databaseBackends"`
	MaxViewLimit            int      `json:"maxViewLimit"`
	MaxFileBytes            int      `json:"maxFileBytes"`
	ThemePresets            []string `json:"themePresets"`
	ThemeAccents            []string `json:"themeAccents"`
	DemoSeedProfiles        []string `json:"demoSeedProfiles"`
	RuleOperators           []string `json:"ruleOperators"`
	RuleSources             []string `json:"ruleSources"`
	MaxRuleNodes            int      `json:"maxRuleNodes"`
	MaxRuleDepth            int      `json:"maxRuleDepth"`
	MaxRuleLiteralBytes     int      `json:"maxRuleLiteralBytes"`
	MaxRuleValueBytes       int      `json:"maxRuleValueBytes"`
	TestSuiteTargets        []string `json:"testSuiteTargets"`
	MaxTestSuites           int      `json:"maxTestSuites"`
	MaxTestCases            int      `json:"maxTestCases"`
	MaxTestFixtures         int      `json:"maxTestFixtures"`
	MaxTestSuiteBytes       int      `json:"maxTestSuiteBytes"`
}

func AgentCapabilities(cliAPIVersion string) Capabilities {
	return ProtocolCapabilities(cliAPIVersion, "")
}

func ProtocolCapabilities(cliAPIVersion, agentProtocolAPIVersion string) Capabilities {
	return Capabilities{
		DefinitionAPIVersion:    definition.APIVersion,
		CLIAPIVersion:           cliAPIVersion,
		AgentProtocolAPIVersion: agentProtocolAPIVersion,
		AppIRFormat:             appir.CurrentFormat,
		DefinitionKinds:         definitionKindRegistry().Names(),
		SemanticPrimitives:      []string{"Lifecycle", "Rule"},
		FieldTypes:              field.Types(),
		ActionOperations:        actionop.Names(),
		ActionSteps:             actionstep.Names(),
		BlockTypes:              block.Names(),
		Presentations:           presentationNames(),
		DisplaySerializers:      displaySerializerNames(),
		PanelLayouts:            panelLayoutNames(),
		DatabaseBackends:        []string{"postgresql", "sqlite"},
		MaxViewLimit:            200,
		MaxFileBytes:            field.MaxFileBytes,
		ThemePresets:            themePresetNames(),
		ThemeAccents:            themeAccentNames(),
		DemoSeedProfiles:        demoSeedProfileNames(),
		RuleOperators:           rule.Operators(),
		RuleSources:             rule.Sources(),
		MaxRuleNodes:            rule.MaxNodes,
		MaxRuleDepth:            rule.MaxDepth,
		MaxRuleLiteralBytes:     rule.MaxLiteralBytes,
		MaxRuleValueBytes:       rule.MaxValueBytes,
		TestSuiteTargets:        append([]string{}, testsuite.TargetKinds...),
		MaxTestSuites:           testsuite.MaxSuites,
		MaxTestCases:            testsuite.MaxCases,
		MaxTestFixtures:         testsuite.MaxFixtures,
		MaxTestSuiteBytes:       testsuite.MaxEncodedSize,
	}
}

func presentationNames() []string {
	return []string{"board", "detail", "list", "metric", "timeline", "tree"}
}
func displaySerializerNames() []string { return []string{"csv", "json", "rss"} }
func panelLayoutNames() []string       { return keys(panelLayouts()) }
func panelLayouts() map[string]map[string]bool {
	return map[string]map[string]bool{"single-column": {"main": true}, "two-column": {"left": true, "right": true}, "sidebar-main": {"sidebar": true, "main": true}, "main-sidebar": {"main": true, "sidebar": true}, "grid": {"main": true}}
}
func themePresetNames() []string { return []string{"minimal", "professional", "warm"} }
func themeAccentNames() []string {
	return []string{"amber", "blue", "emerald", "indigo", "rose", "slate", "violet"}
}
func demoSeedProfileNames() []string {
	return []string{"activities", "auto", "companies", "jobs", "notes", "people"}
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
	kinds := definitionKindRegistry().Names()
	out := make(map[string]map[string]any, len(kinds))
	for _, kind := range kinds {
		registered, _ := definitionKindRegistry().Lookup(kind)
		out[kind] = definitionSchema(kind, registered.Specification)
	}
	return out
}

func SchemaProperties(schema map[string]any) (map[string]any, bool) {
	properties, ok := schema["properties"].(map[string]any)
	return properties, ok
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
	if kind == "Rule" {
		properties["result"] = map[string]any{"type": "string", "enum": []string{string(rule.Boolean), string(rule.Date), string(rule.DateTime), string(rule.Integer), string(rule.Number), string(rule.String), string(rule.Strings)}}
		document["required"] = []string{"kind", "name", "result", "expression"}
		for name, raw := range builder.definitions {
			if !strings.HasSuffix(name, "internal_rule_Expression") {
				continue
			}
			expression := raw.(map[string]any)
			expressionProperties := expression["properties"].(map[string]any)
			expressionProperties["op"] = map[string]any{"type": "string", "enum": rule.Operators()}
			expressionProperties["source"] = map[string]any{"type": "string", "enum": rule.Sources()}
		}
	}
	if kind == "TestSuite" {
		document["required"] = []string{"kind", "name", "target", "tests"}
		for name, raw := range builder.definitions {
			if !strings.HasSuffix(name, "internal_appir_TestTarget") {
				continue
			}
			target := raw.(map[string]any)
			target["required"] = []string{"kind", "name"}
			target["properties"].(map[string]any)["kind"] = map[string]any{"type": "string", "enum": testsuite.TargetKinds}
		}
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
