package compiler

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/registry"
)

type DefinitionReference struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type definitionKind struct {
	Specification       reflect.Type
	Storage             reflect.Type
	Compile             func(*appir.App, definition.Definition) []definition.Diagnostic
	Normalize           func(*appir.App)
	Validate            func(*appir.App, *validationState) []definition.Diagnostic
	Lookup              func(*appir.App, string) (any, bool)
	Names               func(*appir.App) []string
	References          func(*appir.App, string) []DefinitionReference
	FieldEntity         func(*appir.App, string) string
	ReferenceCandidates bool
}

var definitionKinds = newDefinitionKinds()

func definitionKindRegistry() registry.Registry[definitionKind] { return definitionKinds }

func newDefinitionKinds() registry.Registry[definitionKind] {
	entity := mappedDefinitionKind(appir.Entity{}, func(app *appir.App) map[string]appir.Entity { return app.Entities }, normalizeEntity)
	entity.References = entityReferences
	entity.FieldEntity = func(_ *appir.App, name string) string { return name }
	entity.ReferenceCandidates = true
	view := mappedDefinitionKind(appir.View{}, func(app *appir.App) map[string]appir.View { return app.Views }, normalizeView)
	view.References = viewReferences
	view.FieldEntity = func(app *appir.App, name string) string { return app.Views[name].Entity }
	view.ReferenceCandidates = true
	action := actionDefinitionKind()
	action.References = actionReferences
	action.FieldEntity = func(app *appir.App, name string) string { return app.Actions[name].Entity }
	action.ReferenceCandidates = true
	lifecycle := mappedDefinitionKind(appir.Lifecycle{}, func(app *appir.App) map[string]appir.Lifecycle { return app.Lifecycles }, func(name string, value *appir.Lifecycle) {
		value.Name = name
		if value.StateField == "" {
			value.StateField = "status"
		}
	})
	lifecycle.References = func(app *appir.App, name string) []DefinitionReference {
		return references(reference("entity", "Entity", app.Lifecycles[name].Entity))
	}
	lifecycle.FieldEntity = func(app *appir.App, name string) string { return app.Lifecycles[name].Entity }
	lifecycle.ReferenceCandidates = true
	ruleKind := mappedDefinitionKind(appir.Rule{}, func(app *appir.App) map[string]appir.Rule { return app.Rules }, func(name string, value *appir.Rule) {
		value.Name = name
		if value.Input == nil {
			value.Input = map[string]appir.Field{}
		}
		for inputName, input := range value.Input {
			if input.Name == "" {
				input.Name = inputName
				value.Input[inputName] = input
			}
		}
	})
	ruleKind.References = func(app *appir.App, name string) []DefinitionReference {
		return references(reference("entity", "Entity", app.Rules[name].Entity))
	}
	ruleKind.FieldEntity = func(app *appir.App, name string) string { return app.Rules[name].Entity }
	ruleKind.ReferenceCandidates = true
	testSuite := mappedDefinitionKind(appir.TestSuite{}, func(app *appir.App) map[string]appir.TestSuite { return app.TestSuites }, func(name string, value *appir.TestSuite) {
		value.Name = name
		for index := range value.Tests {
			if value.Tests[index].Fixtures == nil {
				value.Tests[index].Fixtures = map[string][]map[string]any{}
			}
			if value.Tests[index].Input == nil {
				value.Tests[index].Input = map[string]any{}
			}
		}
	})
	testSuite.References = testSuiteReferences
	testSuite.ReferenceCandidates = true
	extensionKind := mappedDefinitionKind(appir.Extension{}, func(app *appir.App) map[string]appir.Extension { return app.Extensions }, func(name string, value *appir.Extension) {
		value.Name = name
		if value.Input == nil {
			value.Input = map[string]appir.Field{}
		}
		if value.Output == nil {
			value.Output = map[string]appir.Field{}
		}
		for fieldName, item := range value.Input {
			if item.Name == "" {
				item.Name = fieldName
				value.Input[fieldName] = item
			}
		}
		for fieldName, item := range value.Output {
			if item.Name == "" {
				item.Name = fieldName
				value.Output[fieldName] = item
			}
		}
		sort.Strings(value.Permissions)
		sort.Strings(value.SideEffects)
	})
	extensionKind.ReferenceCandidates = true
	policy := mappedDefinitionKind(appir.Policy{}, func(app *appir.App) map[string]appir.Policy { return app.Policies }, nameValue[appir.Policy](func(value *appir.Policy, name string) { value.Name = name }))
	policy.ReferenceCandidates = true
	filter := mappedDefinitionKind(appir.Filter{}, func(app *appir.App) map[string]appir.Filter { return app.Filters }, nameValue[appir.Filter](func(value *appir.Filter, name string) { value.Name = name }))
	filter.ReferenceCandidates = true
	webform := mappedDefinitionKind(appir.Webform{}, func(app *appir.App) map[string]appir.Webform { return app.Webforms }, nameValue[appir.Webform](func(value *appir.Webform, name string) { value.Name = name }))
	webform.References = func(app *appir.App, name string) []DefinitionReference {
		return references(reference("action", "Action", app.Webforms[name].Action))
	}
	block := mappedDefinitionKind(appir.Block{}, func(app *appir.App) map[string]appir.Block { return app.Blocks }, nameValue[appir.Block](func(value *appir.Block, name string) { value.Name = name }))
	block.References = blockReferences
	block.FieldEntity = func(app *appir.App, name string) string { return app.Views[app.Blocks[name].View].Entity }
	block.ReferenceCandidates = true
	panel := mappedDefinitionKind(appir.Panel{}, func(app *appir.App) map[string]appir.Panel { return app.Panels }, nameValue[appir.Panel](func(value *appir.Panel, name string) { value.Name = name }))
	panel.References = panelReferences
	panel.ReferenceCandidates = true
	pageKind := mappedDefinitionKind(appir.Page{}, func(app *appir.App) map[string]appir.Page { return app.Pages }, nameValue[appir.Page](func(value *appir.Page, name string) { value.Name = name }))
	pageKind.References = func(app *appir.App, name string) []DefinitionReference {
		item := app.Pages[name]
		return references(reference("panel", "Panel", item.Panel), reference("policy", "Policy", item.Policy))
	}
	role := mappedDefinitionKind(appir.Role{}, func(app *appir.App) map[string]appir.Role { return app.Roles }, nameValue[appir.Role](func(value *appir.Role, name string) { value.Name = name }))
	role.ReferenceCandidates = true
	menu := mappedDefinitionKind(appir.Menu{}, func(app *appir.App) map[string]appir.Menu { return app.Menus }, nameValue[appir.Menu](func(value *appir.Menu, name string) { value.Name = name }))
	menu.References = menuReferences
	menu.ReferenceCandidates = true
	job := mappedDefinitionKind(appir.Job{}, func(app *appir.App) map[string]appir.Job { return app.Jobs }, nameValue[appir.Job](func(value *appir.Job, name string) { value.Name = name }))
	job.References = func(app *appir.App, name string) []DefinitionReference {
		return references(reference("action", "Action", app.Jobs[name].Action))
	}
	job.ReferenceCandidates = true
	admin := mappedDefinitionKind(appir.AdminResource{}, func(app *appir.App) map[string]appir.AdminResource { return app.AdminResources }, nameValue[appir.AdminResource](func(value *appir.AdminResource, name string) { value.Name = name }))
	admin.References = adminResourceReferences
	admin.FieldEntity = func(app *appir.App, name string) string { return app.AdminResources[name].Entity }
	localRegistration := localRegistrationDefinitionKind()
	theme := themeDefinitionKind()
	demoSeed := demoSeedDefinitionKind()

	action.Normalize = normalizeActions
	admin.Normalize = normalizeAdminResources
	block.Normalize = normalizeResourceListBlocks
	localRegistration.Normalize = noDefinitionNormalization
	theme.Normalize = noDefinitionNormalization
	demoSeed.Normalize = noDefinitionNormalization

	entity.Validate = validateEntities
	view.Validate = validateViews
	action.Validate = validateActions
	lifecycle.Validate = validateLifecycles
	ruleKind.Validate = validateRules
	testSuite.Validate = validateTestSuites
	extensionKind.Validate = validateExtensions
	policy.Validate = validatePolicies
	filter.Validate = validateFilters
	webform.Validate = validateWebforms
	block.Validate = validateBlocks
	panel.Validate = validatePanels
	pageKind.Validate = validatePages
	role.Validate = noDefinitionValidation
	menu.Validate = validateMenus
	job.Validate = validateJobs
	admin.Validate = validateAdminResources
	localRegistration.Validate = validateLocalRegistration
	theme.Validate = validateTheme
	demoSeed.Validate = validateDemoSeed

	return registry.Must(
		registry.Identity[definitionKind],
		registry.Entry[definitionKind]{Name: "Action", Value: action},
		registry.Entry[definitionKind]{Name: "AdminResource", Value: admin},
		registry.Entry[definitionKind]{Name: "Block", Value: block},
		registry.Entry[definitionKind]{Name: "DemoSeed", Value: demoSeed},
		registry.Entry[definitionKind]{Name: "Entity", Value: entity},
		registry.Entry[definitionKind]{Name: "Extension", Value: extensionKind},
		registry.Entry[definitionKind]{Name: "Filter", Value: filter},
		registry.Entry[definitionKind]{Name: "Job", Value: job},
		registry.Entry[definitionKind]{Name: "Lifecycle", Value: lifecycle},
		registry.Entry[definitionKind]{Name: "LocalRegistration", Value: localRegistration},
		registry.Entry[definitionKind]{Name: "Menu", Value: menu},
		registry.Entry[definitionKind]{Name: "Page", Value: pageKind},
		registry.Entry[definitionKind]{Name: "Panel", Value: panel},
		registry.Entry[definitionKind]{Name: "Policy", Value: policy},
		registry.Entry[definitionKind]{Name: "Role", Value: role},
		registry.Entry[definitionKind]{Name: "Rule", Value: ruleKind},
		registry.Entry[definitionKind]{Name: "TestSuite", Value: testSuite},
		registry.Entry[definitionKind]{Name: "Theme", Value: theme},
		registry.Entry[definitionKind]{Name: "View", Value: view},
		registry.Entry[definitionKind]{Name: "Webform", Value: webform},
	)
}

func testSuiteReferences(app *appir.App, name string) []DefinitionReference {
	suite := app.TestSuites[name]
	out := []DefinitionReference{reference("target.name", suite.Target.Kind, suite.Target.Name)}
	for caseIndex, test := range suite.Tests {
		for _, entityName := range keys(test.Fixtures) {
			out = append(out, reference(fmt.Sprintf("tests.%d.fixtures.%s", caseIndex, entityName), "Entity", entityName))
		}
		for changeIndex, change := range test.Expect.Changes {
			out = append(out, reference(fmt.Sprintf("tests.%d.expect.changes.%d.entity", caseIndex, changeIndex), "Entity", change.Entity))
		}
	}
	return references(out...)
}

func noDefinitionNormalization(*appir.App)                                        {}
func noDefinitionValidation(*appir.App, *validationState) []definition.Diagnostic { return nil }

func mappedDefinitionKind[T any](specification T, collection func(*appir.App) map[string]T, normalize func(string, *T)) definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(specification),
		Storage:       reflect.TypeOf(specification),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var value T
			if err := definition.DecodeSpec(source.Spec, &value); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			normalize(source.Metadata.Name, &value)
			collection(app)[source.Metadata.Name] = value
			return nil
		},
		Lookup: func(app *appir.App, name string) (any, bool) {
			value, exists := collection(app)[name]
			return value, exists
		},
		Names: func(app *appir.App) []string { return mapNames(collection(app)) },
	}
}

func nameValue[T any](set func(*T, string)) func(string, *T) {
	return func(name string, value *T) { set(value, name) }
}

func mapNames[T any](values map[string]T) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeEntity(name string, value *appir.Entity) {
	value.Name = name
	for index := range value.Fields {
		if value.Fields[index].Type != "relation" {
			continue
		}
		if value.Fields[index].Relation == nil {
			target := strings.TrimSuffix(value.Fields[index].Name, "_id")
			value.Fields[index].Relation = &appir.Relation{Entity: target, Kind: "many-to-one", TargetField: "id"}
		}
		if value.Fields[index].Relation.Kind == "" {
			value.Fields[index].Relation.Kind = "many-to-one"
		}
		if value.Fields[index].Relation.TargetField == "" {
			value.Fields[index].Relation.TargetField = "id"
		}
	}
	if value.Label == "" {
		value.Label = value.Name
	}
}

func normalizeView(name string, value *appir.View) {
	value.Name = name
	for index := range value.Relationships {
		if value.Relationships[index].Type == "" {
			value.Relationships[index].Type = "inner"
		}
	}
	if value.DefaultLimit == 0 {
		value.DefaultLimit = 50
	}
	if value.MaxLimit == 0 {
		value.MaxLimit = 200
	}
}

func actionDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(actionSource{}),
		Storage:       reflect.TypeOf(appir.Action{}),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var raw actionSource
			if err := definition.DecodeSpec(source.Spec, &raw); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			value := appir.Action{Name: source.Metadata.Name, Entity: raw.Entity, Operation: raw.Operation, Policy: raw.Policy, Lifecycle: raw.Lifecycle, StateField: raw.StateField, DefaultRole: raw.DefaultRole, Confirm: raw.Confirm, Input: raw.Input, Output: raw.Output, Transitions: raw.Transitions, When: raw.When, Derive: raw.Derive}
			for inputName, input := range value.Input {
				if input.Name == "" {
					input.Name = inputName
					value.Input[inputName] = input
				}
			}
			for outputName, output := range value.Output {
				if output.Name == "" {
					output.Name = outputName
					value.Output[outputName] = output
				}
			}
			for _, step := range raw.Steps {
				compiled := appir.Step{Op: step.Op, Result: step.Result, Entity: step.Entity, View: step.View, StateField: step.StateField, Where: step.Where, Condition: step.Condition, Event: step.Event, Job: step.Job}
				keys := make([]string, 0, len(step.Values))
				for key := range step.Values {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					compiled.Values = append(compiled.Values, appir.Assignment{Field: key, Value: compileBinding(step.Values[key])})
				}
				value.Steps = append(value.Steps, compiled)
			}
			app.Actions[value.Name] = value
			return nil
		},
		Lookup: func(app *appir.App, name string) (any, bool) {
			value, exists := app.Actions[name]
			return value, exists
		},
		Names: func(app *appir.App) []string { return mapNames(app.Actions) },
	}
}

func localRegistrationDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(appir.LocalRegistration{}),
		Storage:       reflect.TypeOf(appir.LocalRegistration{}),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var value appir.LocalRegistration
			if err := definition.DecodeSpec(source.Spec, &value); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			if app.LocalRegistration != nil {
				return []definition.Diagnostic{diagWithRule(source, definition.RuleDuplicate, "metadata.name", "only one local registration definition is allowed")}
			}
			app.LocalRegistration = &value
			return nil
		},
		Lookup: func(app *appir.App, _ string) (any, bool) {
			if app.LocalRegistration == nil {
				return nil, false
			}
			return *app.LocalRegistration, true
		},
		Names: func(*appir.App) []string { return []string{} },
		References: func(app *appir.App, _ string) []DefinitionReference {
			if app.LocalRegistration == nil {
				return nil
			}
			return references(reference("action", "Action", app.LocalRegistration.Action))
		},
	}
}

func themeDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(appir.Theme{}),
		Storage:       reflect.TypeOf(appir.Theme{}),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var value appir.Theme
			if err := definition.DecodeSpec(source.Spec, &value); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			value.Name = source.Metadata.Name
			if app.Theme != nil {
				return []definition.Diagnostic{diagWithRule(source, definition.RuleDuplicate, "metadata.name", "only one Theme definition is allowed")}
			}
			if value.DisplayName == "" {
				value.DisplayName = "Bean"
			}
			if value.Preset == "" {
				value.Preset = "professional"
			}
			if value.Accent == "" {
				value.Accent = "emerald"
			}
			app.Theme = &value
			return nil
		},
		Lookup: func(app *appir.App, name string) (any, bool) {
			if app.Theme == nil || app.Theme.Name != name {
				return nil, false
			}
			return *app.Theme, true
		},
		Names: func(app *appir.App) []string {
			if app.Theme == nil {
				return []string{}
			}
			return []string{app.Theme.Name}
		},
	}
}

func demoSeedDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(appir.DemoSeed{}),
		Storage:       reflect.TypeOf(appir.DemoSeed{}),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var value appir.DemoSeed
			if err := definition.DecodeSpec(source.Spec, &value); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			value.Name = source.Metadata.Name
			if app.DemoSeed != nil {
				return []definition.Diagnostic{diagWithRule(source, definition.RuleDuplicate, "metadata.name", "only one DemoSeed definition is allowed")}
			}
			app.DemoSeed = &value
			return nil
		},
		Lookup: func(app *appir.App, name string) (any, bool) {
			if app.DemoSeed == nil || app.DemoSeed.Name != name {
				return nil, false
			}
			return *app.DemoSeed, true
		},
		Names: func(app *appir.App) []string {
			if app.DemoSeed == nil {
				return []string{}
			}
			return []string{app.DemoSeed.Name}
		},
	}
}

func DefinitionKindNames() []string {
	return definitionKindRegistry().Names()
}

func InspectDefinition(app *appir.App, kind, name string) (any, []DefinitionReference, bool) {
	registered, exists := definitionKindRegistry().Lookup(kind)
	if !exists {
		return nil, nil, false
	}
	value, exists := registered.Lookup(app, name)
	if !exists {
		return nil, nil, false
	}
	found := []DefinitionReference{}
	if registered.References != nil {
		found = registered.References(app, name)
	}
	return value, found, true
}

func CompiledDefinitionNames(app *appir.App, kind string) []string {
	registered, exists := definitionKindRegistry().Lookup(kind)
	if !exists {
		return []string{}
	}
	return registered.Names(app)
}

func reference(path, kind, name string) DefinitionReference {
	return DefinitionReference{Path: path, Kind: kind, Name: name}
}

func references(values ...DefinitionReference) []DefinitionReference {
	out := make([]DefinitionReference, 0, len(values))
	for _, value := range values {
		if value.Name != "" {
			out = append(out, value)
		}
	}
	sort.Slice(out, func(left, right int) bool {
		if out[left].Path != out[right].Path {
			return out[left].Path < out[right].Path
		}
		if out[left].Kind != out[right].Kind {
			return out[left].Kind < out[right].Kind
		}
		return out[left].Name < out[right].Name
	})
	return out
}

func entityReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Entities[name]
	out := []DefinitionReference{reference("policy", "Policy", item.Policy)}
	for validationName, ruleName := range item.Validations {
		out = append(out, reference("validations."+validationName, "Rule", ruleName))
	}
	for index, field := range item.Fields {
		if field.Relation != nil {
			out = append(out, reference(fmt.Sprintf("fields.%d.relation.entity", index), "Entity", field.Relation.Entity))
		}
	}
	return references(out...)
}

func viewReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Views[name]
	out := []DefinitionReference{reference("entity", "Entity", item.Entity), reference("policy", "Policy", item.Policy)}
	for path, filter := range item.FieldFilters {
		out = append(out, reference("fieldFilters."+path, "Filter", filter))
	}
	for index, relationship := range item.Relationships {
		out = append(out, reference(fmt.Sprintf("relationships.%d.entity", index), "Entity", relationship.Entity))
	}
	return references(out...)
}

func actionReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Actions[name]
	out := []DefinitionReference{
		reference("entity", "Entity", item.Entity),
		reference("lifecycle", "Lifecycle", item.Lifecycle),
		reference("policy", "Policy", item.Policy),
		reference("defaultRole", "Role", item.DefaultRole),
		reference("when", "Rule", item.When),
	}
	for fieldName, ruleName := range item.Derive {
		out = append(out, reference("derive."+fieldName, "Rule", ruleName))
	}
	for index, step := range item.Steps {
		out = append(out,
			reference(fmt.Sprintf("steps.%d.entity", index), "Entity", step.Entity),
			reference(fmt.Sprintf("steps.%d.view", index), "View", step.View),
			reference(fmt.Sprintf("steps.%d.job", index), "Job", step.Job),
		)
	}
	return references(out...)
}

func blockReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Blocks[name]
	return references(
		reference("view", "View", item.View),
		reference("entity", "Entity", item.Entity),
		reference("webform", "Webform", item.Webform),
		reference("action", "Action", item.Action),
		reference("menu", "Menu", item.Menu),
		reference("policy", "Policy", item.Policy),
		reference("resource", "AdminResource", item.Resource),
		reference("presentation.moveAction", "Action", item.Presentation.MoveAction),
	)
}

func panelReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Panels[name]
	out := []DefinitionReference{reference("policy", "Policy", item.Policy)}
	for regionIndex, region := range item.Regions {
		for blockIndex, block := range region.Blocks {
			out = append(out, reference(fmt.Sprintf("regions.%d.blocks.%d", regionIndex, blockIndex), "Block", block))
		}
	}
	return references(out...)
}

func menuReferences(app *appir.App, name string) []DefinitionReference {
	out := []DefinitionReference{}
	for index, item := range app.Menus[name].Items {
		out = append(out, reference(fmt.Sprintf("items.%d.policy", index), "Policy", item.Policy))
	}
	return references(out...)
}

func adminResourceReferences(app *appir.App, name string) []DefinitionReference {
	item := app.AdminResources[name]
	out := []DefinitionReference{
		reference("entity", "Entity", item.Entity),
		reference("view", "View", item.View),
		reference("createAction", "Action", item.CreateAction),
		reference("updateAction", "Action", item.UpdateAction),
		reference("deleteAction", "Action", item.DeleteAction),
	}
	for index, action := range item.Actions {
		out = append(out, reference(fmt.Sprintf("actions.%d", index), "Action", action))
	}
	return references(out...)
}
