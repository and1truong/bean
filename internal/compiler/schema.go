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
	beancontent "github.com/beanruntime/bean/internal/content"
	"github.com/beanruntime/bean/internal/definition"
	beanextension "github.com/beanruntime/bean/internal/extension"
	"github.com/beanruntime/bean/internal/field"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	beanpage "github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/rule"
	beansequence "github.com/beanruntime/bean/internal/sequence"
	"github.com/beanruntime/bean/internal/testsuite"
)

const JSONSchemaVersion = "https://json-schema.org/draft/2020-12/schema"

type Capabilities struct {
	DefinitionAPIVersion       string   `json:"definitionAPIVersion"`
	CLIAPIVersion              string   `json:"cliAPIVersion"`
	AgentProtocolAPIVersion    string   `json:"agentProtocolAPIVersion,omitempty"`
	AppIRFormat                string   `json:"appIRFormat"`
	DefinitionKinds            []string `json:"definitionKinds"`
	SemanticPrimitives         []string `json:"semanticPrimitives"`
	FieldTypes                 []string `json:"fieldTypes"`
	ActionOperations           []string `json:"actionOperations"`
	ActionSteps                []string `json:"actionSteps"`
	BlockTypes                 []string `json:"blockTypes"`
	Presentations              []string `json:"presentations"`
	ViewDisplayTypes           []string `json:"viewDisplayTypes"`
	ViewRenderers              []string `json:"viewRenderers"`
	FieldLayoutColumns         []int    `json:"fieldLayoutColumns"`
	FieldLayoutSpans           []string `json:"fieldLayoutSpans"`
	MaxFieldLayoutGroups       int      `json:"maxFieldLayoutGroups"`
	MaxFieldGroupFields        int      `json:"maxFieldGroupFields"`
	MaxFieldLayoutFields       int      `json:"maxFieldLayoutFields"`
	ViewFilterOperators        []string `json:"viewFilterOperators"`
	ViewControlWidgets         []string `json:"viewControlWidgets"`
	ViewPagers                 []string `json:"viewPagers"`
	ViewResultShapes           []string `json:"viewResultShapes"`
	ViewGroupBuckets           []string `json:"viewGroupBuckets"`
	ViewAggregateFunctions     []string `json:"viewAggregateFunctions"`
	ViewDrillSources           []string `json:"viewDrillSources"`
	ViewSelections             []string `json:"viewSelections"`
	DisplaySerializers         []string `json:"displaySerializers"`
	PanelLayouts               []string `json:"panelLayouts"`
	MaxPageSections            int      `json:"maxPageSections"`
	PageSectionWidths          []string `json:"pageSectionWidths"`
	MenuProfiles               []string `json:"menuProfiles"`
	MenuVariants               []string `json:"menuVariants"`
	MaxMenuDefinitions         int      `json:"maxMenuDefinitions"`
	MaxMenuDepth               int      `json:"maxMenuDepth"`
	MaxMenuPlacements          int      `json:"maxMenuPlacements"`
	SequenceProfiles           []string `json:"sequenceProfiles"`
	SequenceAspectRatios       []string `json:"sequenceAspectRatios"`
	SequenceFrameLayouts       []string `json:"sequenceFrameLayouts"`
	SequenceFrameDirections    []string `json:"sequenceFrameDirections"`
	ContentElementTypes        []string `json:"contentElementTypes"`
	ContentTones               []string `json:"contentTones"`
	DiagramDirections          []string `json:"diagramDirections"`
	HeadingLevels              []int    `json:"headingLevels"`
	LinkTargetKinds            []string `json:"linkTargetKinds"`
	LinkOpenModes              []string `json:"linkOpenModes"`
	TableRowHeaderModes        []string `json:"tableRowHeaderModes"`
	MediaKinds                 []string `json:"mediaKinds"`
	MediaSourcePolicy          string   `json:"mediaSourcePolicy"`
	TabOrientations            []string `json:"tabOrientations"`
	TabVariants                []string `json:"tabVariants"`
	MaxSequenceFrames          int      `json:"maxSequenceFrames"`
	MaxSequenceTitleRunes      int      `json:"maxSequenceTitleRunes"`
	MaxSequenceNotesBytes      int      `json:"maxSequenceNotesBytes"`
	MaxSequenceFrameBlocks     int      `json:"maxSequenceFrameBlocks"`
	MaxSequenceContentUnits    int      `json:"maxSequenceContentUnits"`
	MaxContentElements         int      `json:"maxContentElements"`
	MaxContentBulletItems      int      `json:"maxContentBulletItems"`
	MaxContentDiagramItems     int      `json:"maxContentDiagramItems"`
	MaxContentCodeLines        int      `json:"maxContentCodeLines"`
	MaxContentOrderedItems     int      `json:"maxContentOrderedItems"`
	MaxContentItemRunes        int      `json:"maxContentItemRunes"`
	MaxContentLabelRunes       int      `json:"maxContentLabelRunes"`
	MaxContentTargetRunes      int      `json:"maxContentTargetRunes"`
	MaxContentTableColumns     int      `json:"maxContentTableColumns"`
	MaxContentTableRows        int      `json:"maxContentTableRows"`
	MaxContentColumnLabelRunes int      `json:"maxContentColumnLabelRunes"`
	MaxContentMediaTitleRunes  int      `json:"maxContentMediaTitleRunes"`
	MaxContentTranscriptRunes  int      `json:"maxContentTranscriptRunes"`
	MinContentChoices          int      `json:"minContentChoices"`
	MaxContentChoices          int      `json:"maxContentChoices"`
	MaxContentQuestionRunes    int      `json:"maxContentQuestionRunes"`
	MaxContentChoiceTextRunes  int      `json:"maxContentChoiceTextRunes"`
	MaxContentExplanationRunes int      `json:"maxContentExplanationRunes"`
	MaxContentMachineIDRunes   int      `json:"maxContentMachineIDRunes"`
	MinTabs                    int      `json:"minTabs"`
	MaxTabs                    int      `json:"maxTabs"`
	MaxTabContentElements      int      `json:"maxTabContentElements"`
	DatabaseBackends           []string `json:"databaseBackends"`
	MaxViewLimit               int      `json:"maxViewLimit"`
	MaxFileBytes               int      `json:"maxFileBytes"`
	ThemePresets               []string `json:"themePresets"`
	ThemeAccents               []string `json:"themeAccents"`
	DemoSeedProfiles           []string `json:"demoSeedProfiles"`
	RuleOperators              []string `json:"ruleOperators"`
	RuleSources                []string `json:"ruleSources"`
	MaxRuleNodes               int      `json:"maxRuleNodes"`
	MaxRuleDepth               int      `json:"maxRuleDepth"`
	MaxRuleLiteralBytes        int      `json:"maxRuleLiteralBytes"`
	MaxRuleValueBytes          int      `json:"maxRuleValueBytes"`
	TestSuiteTargets           []string `json:"testSuiteTargets"`
	MaxTestSuites              int      `json:"maxTestSuites"`
	MaxTestCases               int      `json:"maxTestCases"`
	MaxTestFixtures            int      `json:"maxTestFixtures"`
	MaxTestSuiteBytes          int      `json:"maxTestSuiteBytes"`
	ExtensionTransports        []string `json:"extensionTransports"`
	ExtensionPermissions       []string `json:"extensionPermissions"`
	ExtensionSideEffects       []string `json:"extensionSideEffects"`
	ExtensionAuthentication    []string `json:"extensionAuthentication"`
	ExtensionIdempotency       []string `json:"extensionIdempotency"`
	ExtensionTransactions      []string `json:"extensionTransactions"`
	ExtensionFailures          []string `json:"extensionFailures"`
	MaxExtensionTimeout        int      `json:"maxExtensionTimeoutSeconds"`
	MaxExtensionAttempts       int      `json:"maxExtensionAttempts"`
	MaxExtensionDelay          int      `json:"maxExtensionDelaySeconds"`
	MaxExtensionResponse       int      `json:"maxExtensionResponseBytes"`
}

func AgentCapabilities(cliAPIVersion string) Capabilities {
	return ProtocolCapabilities(cliAPIVersion, "")
}

func ProtocolCapabilities(cliAPIVersion, agentProtocolAPIVersion string) Capabilities {
	return Capabilities{
		DefinitionAPIVersion:       definition.APIVersion,
		CLIAPIVersion:              cliAPIVersion,
		AgentProtocolAPIVersion:    agentProtocolAPIVersion,
		AppIRFormat:                appir.CurrentFormat,
		DefinitionKinds:            definitionKindRegistry().Names(),
		SemanticPrimitives:         []string{"Lifecycle", "Rule"},
		FieldTypes:                 field.Types(),
		ActionOperations:           actionop.Names(),
		ActionSteps:                actionstep.Names(),
		BlockTypes:                 block.Names(),
		Presentations:              presentationNames(),
		ViewDisplayTypes:           viewDisplayTypes(),
		ViewRenderers:              viewRendererNames(),
		FieldLayoutColumns:         []int{1, 2},
		FieldLayoutSpans:           []string{"single", "full"},
		MaxFieldLayoutGroups:       maxFieldLayoutGroups,
		MaxFieldGroupFields:        maxFieldGroupFields,
		MaxFieldLayoutFields:       maxFieldLayoutFields,
		ViewFilterOperators:        viewFilterOperators(),
		ViewControlWidgets:         viewControlWidgets(),
		ViewPagers:                 viewPagerTypes(),
		ViewResultShapes:           []string{"groups", "metric", "records"},
		ViewGroupBuckets:           []string{"day", "month", "week"},
		ViewAggregateFunctions:     []string{"average", "count", "max", "min", "sum"},
		ViewDrillSources:           []string{"filter", "group"},
		ViewSelections:             []string{"multiple", "none", "single"},
		DisplaySerializers:         displaySerializerNames(),
		PanelLayouts:               panelLayoutNames(),
		MaxPageSections:            beanpage.MaxSections,
		PageSectionWidths:          beanpage.Widths(),
		MenuProfiles:               beanmenu.Profiles(),
		MenuVariants:               beanmenu.Variants(),
		MaxMenuDefinitions:         beanmenu.MaxDefinitions,
		MaxMenuDepth:               beanmenu.MaxDepth,
		MaxMenuPlacements:          beanmenu.MaxPlacements,
		SequenceProfiles:           beansequence.Profiles(),
		SequenceAspectRatios:       beansequence.AspectRatios(),
		SequenceFrameLayouts:       beansequence.Layouts(),
		SequenceFrameDirections:    beansequence.Directions(),
		ContentElementTypes:        beancontent.Types(),
		ContentTones:               beancontent.Tones(),
		DiagramDirections:          beancontent.Directions(),
		HeadingLevels:              beancontent.HeadingLevels(),
		LinkTargetKinds:            []string{"application_path", "https"},
		LinkOpenModes:              beancontent.LinkOpenModes(),
		TableRowHeaderModes:        beancontent.RowHeaderModes(),
		MediaKinds:                 []string{"audio", "image", "youtube", "youtube_playlist"},
		MediaSourcePolicy:          "absolute application path or HTTPS; audio forbids query and fragment; YouTube accepts validated IDs only",
		TabOrientations:            beancontent.TabOrientations(),
		TabVariants:                beancontent.TabVariants(),
		MaxSequenceFrames:          beansequence.MaxFrames,
		MaxSequenceTitleRunes:      beansequence.MaxTitleRunes,
		MaxSequenceNotesBytes:      beansequence.MaxNotesBytes,
		MaxSequenceFrameBlocks:     beansequence.MaxBlocksPerFrame,
		MaxSequenceContentUnits:    beansequence.BaseContentBudget,
		MaxContentElements:         beancontent.MaxElements,
		MaxContentBulletItems:      beancontent.MaxBulletItems,
		MaxContentDiagramItems:     beancontent.MaxDiagramItems,
		MaxContentCodeLines:        beancontent.MaxCodeLines,
		MaxContentOrderedItems:     beancontent.MaxOrderedItems,
		MaxContentItemRunes:        beancontent.MaxItemRunes,
		MaxContentLabelRunes:       beancontent.MaxLabelRunes,
		MaxContentTargetRunes:      beancontent.MaxTargetRunes,
		MaxContentTableColumns:     beancontent.MaxColumns,
		MaxContentTableRows:        beancontent.MaxRows,
		MaxContentColumnLabelRunes: beancontent.MaxColumnLabelRunes,
		MaxContentMediaTitleRunes:  beancontent.MaxMediaTitleRunes,
		MaxContentTranscriptRunes:  beancontent.MaxTranscriptRunes,
		MinContentChoices:          beancontent.MinChoices,
		MaxContentChoices:          beancontent.MaxChoices,
		MaxContentQuestionRunes:    beancontent.MaxQuestionRunes,
		MaxContentChoiceTextRunes:  beancontent.MaxChoiceTextRunes,
		MaxContentExplanationRunes: beancontent.MaxExplanationRunes,
		MaxContentMachineIDRunes:   beancontent.MaxMachineIDRunes,
		MinTabs:                    beancontent.MinTabs,
		MaxTabs:                    beancontent.MaxTabs,
		MaxTabContentElements:      beancontent.MaxTabElements,
		DatabaseBackends:           []string{"postgresql", "sqlite"},
		MaxViewLimit:               200,
		MaxFileBytes:               field.MaxFileBytes,
		ThemePresets:               themePresetNames(),
		ThemeAccents:               themeAccentNames(),
		DemoSeedProfiles:           demoSeedProfileNames(),
		RuleOperators:              rule.Operators(),
		RuleSources:                rule.Sources(),
		MaxRuleNodes:               rule.MaxNodes,
		MaxRuleDepth:               rule.MaxDepth,
		MaxRuleLiteralBytes:        rule.MaxLiteralBytes,
		MaxRuleValueBytes:          rule.MaxValueBytes,
		TestSuiteTargets:           append([]string{}, testsuite.TargetKinds...),
		MaxTestSuites:              testsuite.MaxSuites,
		MaxTestCases:               testsuite.MaxCases,
		MaxTestFixtures:            testsuite.MaxFixtures,
		MaxTestSuiteBytes:          testsuite.MaxEncodedSize,
		ExtensionTransports:        beanextension.Transports(),
		ExtensionPermissions:       beanextension.Permissions(),
		ExtensionSideEffects:       beanextension.SideEffects(),
		ExtensionAuthentication:    beanextension.Authentications(),
		ExtensionIdempotency:       beanextension.IdempotencyModes(),
		ExtensionTransactions:      beanextension.TransactionModes(),
		ExtensionFailures:          beanextension.FailureModes(),
		MaxExtensionTimeout:        beanextension.MaxTimeoutSeconds,
		MaxExtensionAttempts:       beanextension.MaxAttempts,
		MaxExtensionDelay:          beanextension.MaxDelaySeconds,
		MaxExtensionResponse:       beanextension.MaxResponseBytes,
	}
}

func presentationNames() []string {
	return []string{"board", "calendar", "cards", "chart", "detail", "list", "metric", "timeline", "tree"}
}
func viewRendererNames() []string {
	return []string{"board", "calendar", "cards", "chart", "detail", "list", "metric", "table", "timeline", "tree"}
}
func viewDisplayTypes() []string    { return []string{"block", "csv", "json", "page", "rss"} }
func viewFilterOperators() []string { return []string{"contains", "eq", "gte", "lte"} }
func viewControlWidgets() []string {
	return []string{"auto", "checkbox", "date", "number", "select", "text"}
}
func viewPagerTypes() []string         { return []string{"cursor", "none"} }
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
	if kind == "Authentication" {
		document["required"] = []string{"kind", "name", "preset"}
		properties["name"] = map[string]any{"const": "auth"}
		properties["preset"] = map[string]any{"type": "string", "enum": []string{"local", "internal", "public"}}
		properties["registration"] = map[string]any{"type": "boolean", "default": false}
		properties["passwordRecovery"] = map[string]any{"type": "boolean", "default": false}
		properties["emailVerification"] = map[string]any{"type": "boolean", "default": false}
	}
	if kind == "View" {
		groupBy := properties["groupBy"].(map[string]any)
		groupBy["items"] = map[string]any{"anyOf": []any{map[string]any{"type": "string"}, groupBy["items"]}}
	}
	if kind == "Page" {
		document["oneOf"] = []any{
			map[string]any{"required": []string{"panel"}, "not": map[string]any{"required": []string{"sections"}}},
			map[string]any{"required": []string{"sections"}, "not": map[string]any{"required": []string{"panel"}}},
		}
		sections := properties["sections"].(map[string]any)
		sections["minItems"] = 1
		sections["maxItems"] = beanpage.MaxSections
		for name, raw := range builder.definitions {
			if strings.HasSuffix(name, "internal_appir_PageSection") {
				section := raw.(map[string]any)
				section["required"] = []string{"panel"}
				sectionProperties := section["properties"].(map[string]any)
				delete(sectionProperties, "identity")
				delete(sectionProperties, "iD")
				sectionProperties["id"] = map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$"}
				sectionProperties["width"] = map[string]any{"type": "string", "enum": beanpage.Widths()}
			}
		}
	}
	if kind == "Menu" {
		properties["profile"] = map[string]any{"type": "string", "enum": beanmenu.Profiles()}
		properties["variant"] = map[string]any{"type": "string", "enum": beanmenu.Variants()}
		properties["maxDepth"] = map[string]any{"type": "integer", "minimum": beanmenu.MaxDepth, "maximum": beanmenu.MaxDepth}
		items := properties["items"].(map[string]any)
		items["maxItems"] = beanmenu.MaxPlacements
		for name, raw := range builder.definitions {
			schema := raw.(map[string]any)
			schemaProperties := schema["properties"].(map[string]any)
			switch {
			case strings.HasSuffix(name, "internal_appir_MenuItem"):
				delete(schemaProperties, "iD")
				schemaProperties["id"] = map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$"}
				schemaProperties["label"] = map[string]any{"type": "string", "maxLength": beanmenu.MaxLabelOverrideLength}
				schemaProperties["weight"] = map[string]any{"type": "integer", "minimum": beanmenu.MinWeight, "maximum": beanmenu.MaxWeight}
			case strings.HasSuffix(name, "internal_appir_MenuTarget"):
				schema["oneOf"] = []any{
					map[string]any{"required": []string{"page"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"view"}}, map[string]any{"required": []string{"display"}}}}},
					map[string]any{"required": []string{"view", "display"}, "not": map[string]any{"required": []string{"page"}}},
				}
			}
		}
	}
	if kind == "Entity" {
		for name, raw := range builder.definitions {
			schema := raw.(map[string]any)
			switch {
			case strings.HasSuffix(name, "internal_appir_EntityNavigation"):
				schema["required"] = []string{"labelField", "destination", "menus"}
				menus := schema["properties"].(map[string]any)["menus"].(map[string]any)
				menus["minItems"] = 1
				menus["maxItems"] = beanmenu.MaxDefinitions
			case strings.HasSuffix(name, "internal_appir_NavigationDestination"):
				schema["required"] = []string{"view", "display"}
			}
		}
	}
	if kind == "Panel" {
		for name, raw := range builder.definitions {
			schema := raw.(map[string]any)
			schemaProperties := schema["properties"].(map[string]any)
			switch {
			case strings.HasSuffix(name, "internal_compiler_panelRegionSource"):
				schema["not"] = map[string]any{"required": []string{"blocks", "items"}}
			case strings.HasSuffix(name, "internal_compiler_panelRegionItemSource"):
				schema["oneOf"] = []any{
					map[string]any{"required": []string{"block"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"content"}}, map[string]any{"required": []string{"id"}}}}},
					map[string]any{"required": []string{"content"}, "not": map[string]any{"required": []string{"block"}}},
				}
				schemaProperties["id"] = map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$"}
				content := schemaProperties["content"].(map[string]any)
				content["minItems"] = 1
				content["maxItems"] = beancontent.MaxElements
			}
		}
	}
	if kind == "Block" {
		properties["type"] = map[string]any{"type": "string", "enum": block.Names()}
		properties["label"] = boundedString(beancontent.MaxLabelRunes)
		properties["orientation"] = map[string]any{"type": "string", "enum": beancontent.TabOrientations(), "default": "horizontal"}
		properties["variant"] = map[string]any{"type": "string", "enum": beancontent.TabVariants(), "default": "underline"}
		content := properties["content"].(map[string]any)
		content["minItems"] = 1
		content["maxItems"] = beancontent.MaxElements
		tabs := properties["tabs"].(map[string]any)
		tabs["minItems"] = beancontent.MinTabs
		tabs["maxItems"] = beancontent.MaxTabs
		forbidden := []any{}
		for _, field := range []string{"view", "display", "entity", "webform", "action", "menu", "text", "resource", "inputs", "bindings", "filters", "defaultFilters", "presentation", "content"} {
			forbidden = append(forbidden, map[string]any{"required": []string{field}})
		}
		document["oneOf"] = []any{
			map[string]any{"properties": map[string]any{"type": map[string]any{"const": "tabs"}}, "required": []string{"type", "label", "tabs"}, "not": map[string]any{"anyOf": forbidden}},
			map[string]any{"properties": map[string]any{"type": map[string]any{"enum": without(block.Names(), "tabs")}}, "required": []string{"type"}, "not": map[string]any{"anyOf": []any{map[string]any{"required": []string{"label"}}, map[string]any{"required": []string{"orientation"}}, map[string]any{"required": []string{"variant"}}, map[string]any{"required": []string{"tabs"}}}}},
		}
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
	if kind == "Extension" {
		document["required"] = []string{"kind", "name", "transport", "endpoint", "input", "output", "permissions", "sideEffects", "authentication", "timeoutSeconds", "retry", "idempotency", "transaction", "failure"}
		properties["transport"] = map[string]any{"type": "string", "enum": beanextension.Transports()}
		properties["permissions"] = map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": beanextension.Permissions()}, "minItems": 1, "maxItems": 1, "uniqueItems": true}
		properties["sideEffects"] = map[string]any{"type": "array", "items": map[string]any{"type": "string", "enum": beanextension.SideEffects()}, "minItems": 1, "maxItems": 1, "uniqueItems": true}
		properties["authentication"] = map[string]any{"type": "string", "enum": beanextension.Authentications()}
		properties["timeoutSeconds"] = map[string]any{"type": "integer", "minimum": beanextension.MinTimeoutSeconds, "maximum": beanextension.MaxTimeoutSeconds}
		properties["idempotency"] = map[string]any{"type": "string", "enum": beanextension.IdempotencyModes()}
		properties["transaction"] = map[string]any{"type": "string", "enum": beanextension.TransactionModes()}
		properties["failure"] = map[string]any{"type": "string", "enum": beanextension.FailureModes()}
		for name, raw := range builder.definitions {
			schema := raw.(map[string]any)
			schemaProperties := schema["properties"].(map[string]any)
			switch {
			case strings.HasSuffix(name, "internal_appir_ExtensionRetry"):
				schema["required"] = []string{"maxAttempts", "delaySeconds"}
				schemaProperties["maxAttempts"] = map[string]any{"type": "integer", "minimum": beanextension.MinAttempts, "maximum": beanextension.MaxAttempts}
				schemaProperties["delaySeconds"] = map[string]any{"type": "integer", "minimum": beanextension.MinDelaySeconds, "maximum": beanextension.MaxDelaySeconds}
			case strings.HasSuffix(name, "internal_appir_Field"):
				schemaProperties["type"] = map[string]any{"type": "string", "enum": []string{"boolean", "date", "datetime", "decimal", "email", "enum", "integer", "json", "money", "slug", "string", "text", "url", "uuid"}}
			}
		}
	}
	fieldLayoutSchemaDefinitions(builder.definitions)
	semanticContentSchemaDefinitions(builder.definitions)
	return document
}

func without(values []string, omitted string) []string {
	out := []string{}
	for _, value := range values {
		if value != omitted {
			out = append(out, value)
		}
	}
	return out
}

func semanticContentSchemaDefinitions(definitions map[string]any) {
	for name, raw := range definitions {
		schema := raw.(map[string]any)
		switch {
		case strings.HasSuffix(name, "internal_appir_ContentElement"):
			definitions[name] = contentElementSchema()
		case strings.HasSuffix(name, "internal_appir_TableColumn"):
			schema["required"] = []string{"id", "label"}
			properties := schema["properties"].(map[string]any)
			delete(properties, "iD")
			properties["id"] = machineIDSchema()
			properties["label"] = boundedString(beancontent.MaxColumnLabelRunes)
		case strings.HasSuffix(name, "internal_appir_ContentChoice"):
			schema["required"] = []string{"id", "text"}
			properties := schema["properties"].(map[string]any)
			delete(properties, "iD")
			properties["id"] = machineIDSchema()
			properties["text"] = boundedString(beancontent.MaxChoiceTextRunes)
		case strings.HasSuffix(name, "internal_appir_ContentTab"):
			schema["required"] = []string{"id", "label", "content"}
			schema["description"] = "Tab IDs are unique within the Block; all tab content lists contain at most 24 elements in total. These constraints are compiler-enforced."
			properties := schema["properties"].(map[string]any)
			delete(properties, "iD")
			properties["id"] = machineIDSchema()
			properties["label"] = boundedString(beancontent.MaxColumnLabelRunes)
			content := properties["content"].(map[string]any)
			content["minItems"] = 1
			content["maxItems"] = beancontent.MaxElements
		}
	}
}

func contentElementSchema() map[string]any {
	stringValue := map[string]any{"type": "string"}
	text := func(max int) map[string]any {
		schema := map[string]any{"type": "string"}
		if max > 0 {
			schema["maxLength"] = max
		}
		return schema
	}
	nonBlank := func(max int) map[string]any {
		schema := text(max)
		schema["pattern"] = "\\S"
		return schema
	}
	stringList := func(minimum, maximum, maxLength int) map[string]any {
		item := any(stringValue)
		if maxLength > 0 {
			item = text(maxLength)
		}
		return map[string]any{"type": "array", "minItems": minimum, "maxItems": maximum, "items": item}
	}
	variants := []any{
		contentVariant("heading", []string{"text"}, map[string]any{"text": nonBlank(0), "level": map[string]any{"type": "integer", "enum": beancontent.HeadingLevels(), "default": 2}}),
		contentVariant("paragraph", []string{"text"}, map[string]any{"text": nonBlank(0)}),
		contentVariant("bullets", []string{"items"}, map[string]any{"items": stringList(1, beancontent.MaxBulletItems, 0)}),
		contentVariant("quote", []string{"text"}, map[string]any{"text": nonBlank(0), "attribution": stringValue}),
		contentVariant("code", []string{"text"}, map[string]any{"text": nonBlank(0), "language": stringValue}),
		contentVariant("callout", []string{"text"}, map[string]any{"text": nonBlank(0), "tone": map[string]any{"type": "string", "enum": beancontent.Tones(), "default": "info"}}),
		contentVariant("image", []string{"source", "alt"}, map[string]any{"source": stringValue, "alt": nonBlank(0)}),
		contentVariant("diagram", []string{"items"}, map[string]any{"items": stringList(2, beancontent.MaxDiagramItems, 0), "direction": map[string]any{"type": "string", "enum": beancontent.Directions(), "default": "horizontal"}}),
		contentVariant("ordered_list", []string{"items"}, map[string]any{"items": map[string]any{"type": "array", "minItems": 1, "maxItems": beancontent.MaxOrderedItems, "items": nonBlank(beancontent.MaxItemRunes)}}),
		contentVariant("link", []string{"label", "target"}, map[string]any{"label": nonBlank(beancontent.MaxLabelRunes), "target": nonBlank(beancontent.MaxTargetRunes), "openIn": map[string]any{"type": "string", "enum": beancontent.LinkOpenModes(), "default": "same_tab"}}),
		contentVariant("divider", nil, nil),
		contentVariant("table", []string{"caption", "columns", "rows"}, map[string]any{
			"caption": nonBlank(beancontent.MaxLabelRunes), "columns": map[string]any{"type": "array", "minItems": 1, "maxItems": beancontent.MaxColumns, "items": map[string]any{"$ref": "#/$defs/github_com_beanruntime_bean_internal_appir_TableColumn"}},
			"rows": map[string]any{"type": "array", "minItems": 1, "maxItems": beancontent.MaxRows, "items": stringList(1, beancontent.MaxColumns, beancontent.MaxItemRunes)}, "rowHeader": map[string]any{"type": "string", "enum": beancontent.RowHeaderModes(), "default": "none"},
		}),
		contentVariant("audio", []string{"source", "title", "transcript"}, map[string]any{"source": nonBlank(beancontent.MaxTargetRunes), "title": nonBlank(beancontent.MaxMediaTitleRunes), "transcript": nonBlank(beancontent.MaxTranscriptRunes)}),
		contentVariant("youtube", []string{"videoId", "title", "transcript"}, map[string]any{"videoId": map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{11}$"}, "title": nonBlank(beancontent.MaxMediaTitleRunes), "transcript": nonBlank(beancontent.MaxTranscriptRunes)}),
		contentVariant("youtube_playlist", []string{"playlistId", "title", "transcript"}, map[string]any{"playlistId": map[string]any{"type": "string", "pattern": "^[A-Za-z0-9_-]{10,80}$"}, "title": nonBlank(beancontent.MaxMediaTitleRunes), "transcript": nonBlank(beancontent.MaxTranscriptRunes)}),
		contentVariant("choices", []string{"question", "choices", "answer"}, map[string]any{"question": nonBlank(beancontent.MaxQuestionRunes), "choices": map[string]any{"type": "array", "minItems": beancontent.MinChoices, "maxItems": beancontent.MaxChoices, "items": map[string]any{"$ref": "#/$defs/github_com_beanruntime_bean_internal_appir_ContentChoice"}}, "answer": machineIDSchema(), "explanation": text(beancontent.MaxExplanationRunes)}),
	}
	return map[string]any{"oneOf": variants, "description": "Closed semantic content variants. The compiler additionally enforces non-blank text, URL safety, unique IDs, choices answer references, and table row widths."}
}

func contentVariant(typeName string, required []string, properties map[string]any) map[string]any {
	allProperties := map[string]any{"type": map[string]any{"const": typeName}}
	for name, schema := range properties {
		allProperties[name] = schema
	}
	return map[string]any{"type": "object", "additionalProperties": false, "required": append([]string{"type"}, required...), "properties": allProperties}
}

func boundedString(maximum int) map[string]any {
	return map[string]any{"type": "string", "maxLength": maximum, "pattern": "\\S"}
}

func machineIDSchema() map[string]any {
	return map[string]any{"type": "string", "pattern": "^[a-z][a-z0-9_]*$", "maxLength": beancontent.MaxMachineIDRunes}
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
