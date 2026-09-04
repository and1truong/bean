package compiler

import (
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beancontent "github.com/beanruntime/bean/internal/content"
	"github.com/beanruntime/bean/internal/definition"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	beanpage "github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/registry"
)

type DefinitionReference struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type DefinitionChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
}

type panelSource struct {
	Layout, Policy string
	Regions        []panelRegionSource
}

type panelRegionSource struct {
	Name              string
	CollapseWhenEmpty bool
	Blocks            []string
	Items             []panelRegionItemSource
}

type panelRegionItemSource struct {
	ID      string `json:"id"`
	Block   string
	Content []appir.ContentElement
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

var viewDisplayName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var definitionKinds = newDefinitionKinds()

func definitionKindRegistry() registry.Registry[definitionKind] { return definitionKinds }

func newDefinitionKinds() registry.Registry[definitionKind] {
	entity := mappedDefinitionKind(appir.Entity{}, func(app *appir.App) map[string]appir.Entity { return app.Entities }, normalizeEntity)
	entity.References = entityReferences
	entity.FieldEntity = func(_ *appir.App, name string) string { return name }
	entity.ReferenceCandidates = true
	view := mappedDefinitionKind(appir.View{}, func(app *appir.App) map[string]appir.View { return app.Views }, normalizeView)
	compileView := view.Compile
	view.Compile = func(app *appir.App, source definition.Definition) []definition.Diagnostic {
		out := compileView(app, source)
		for _, displayName := range keys(app.Views[source.Metadata.Name].Displays) {
			if !viewDisplayName.MatchString(displayName) {
				path := "spec.displays." + displayName
				if displayName == "" {
					path = "spec.displays"
				}
				out = append(out, diagnostic("View", source.Metadata.Name, path, "display name must be a nonempty machine name"))
			}
		}
		return out
	}
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
			if value.Tests[index].Providers == nil {
				value.Tests[index].Providers = map[string][]appir.TestProviderResult{}
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
	panel := panelDefinitionKind()
	panel.References = panelReferences
	panel.ReferenceCandidates = true
	pageKind := mappedDefinitionKind(appir.Page{}, func(app *appir.App) map[string]appir.Page { return app.Pages }, nameValue[appir.Page](func(value *appir.Page, name string) {
		value.Name = name
		for index := range value.Sections {
			if value.Sections[index].Width == "" {
				value.Sections[index].Width = beanpage.DefaultWidth
			}
			identity := fmt.Sprintf("%d", index)
			if value.Sections[index].ID != "" {
				identity = value.Sections[index].ID
			}
			value.Sections[index].Identity = "@section/" + name + "/" + identity
		}
	}))
	pageKind.References = func(app *appir.App, name string) []DefinitionReference {
		item := app.Pages[name]
		out := []DefinitionReference{reference("panel", "Panel", item.Panel), reference("policy", "Policy", item.Policy)}
		for index, section := range item.Sections {
			out = append(out, reference(fmt.Sprintf("sections.%d.panel", index), "Panel", section.Panel))
		}
		for filterName, filter := range item.Filters {
			for index, target := range filter.Targets {
				out = append(out, reference(fmt.Sprintf("filters.%s.targets.%d.block", filterName, index), "Block", target.Block))
			}
		}
		return references(out...)
	}
	sequenceKind := mappedDefinitionKind(appir.Sequence{}, func(app *appir.App) map[string]appir.Sequence { return app.Sequences }, func(name string, value *appir.Sequence) {
		value.Name = name
		if value.Profile == "" {
			value.Profile = "presentation"
		}
		if value.AspectRatio == "" {
			value.AspectRatio = "wide"
		}
	})
	sequenceKind.References = func(app *appir.App, name string) []DefinitionReference {
		item := app.Sequences[name]
		out := []DefinitionReference{reference("policy", "Policy", item.Policy)}
		for index, frame := range item.Frames {
			out = append(out, reference(fmt.Sprintf("frames.%d.panel", index), "Panel", frame.Panel))
		}
		return references(out...)
	}
	sequenceKind.ReferenceCandidates = true
	role := mappedDefinitionKind(appir.Role{}, func(app *appir.App) map[string]appir.Role { return app.Roles }, nameValue[appir.Role](func(value *appir.Role, name string) { value.Name = name }))
	role.ReferenceCandidates = true
	menu := mappedDefinitionKind(appir.Menu{}, func(app *appir.App) map[string]appir.Menu { return app.Menus }, nameValue[appir.Menu](func(value *appir.Menu, name string) {
		value.Name = name
		typed := value.Profile != "" || value.Variant != "" || value.MaxDepth != 0 || value.Owner != nil
		for _, item := range value.Items {
			typed = typed || item.ID != "" || item.Parent != "" || item.Weight != 0 || beanmenu.IsTypedTarget(item.Target)
		}
		if typed && value.Profile == "" {
			value.Profile = beanmenu.ProfileWorkspace
		}
		if value.Profile == beanmenu.ProfileWorkspace && value.MaxDepth == 0 {
			value.MaxDepth = beanmenu.MaxDepth
		}
		if value.Profile == beanmenu.ProfileWorkspace && value.Variant == "" {
			value.Variant = beanmenu.VariantDefault
		}
	}))
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
	view.Normalize = normalizeViews
	block.Normalize = normalizeBlocks
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
	sequenceKind.Validate = validateSequences
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
		registry.Entry[definitionKind]{Name: "Sequence", Value: sequenceKind},
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
		for extensionName := range test.Providers {
			out = append(out, reference(fmt.Sprintf("tests.%d.providers.%s", caseIndex, extensionName), "Extension", extensionName))
		}
		for callIndex, call := range test.Expect.ProviderCalls {
			out = append(out, reference(fmt.Sprintf("tests.%d.expect.providerCalls.%d.extension", caseIndex, callIndex), "Extension", call.Extension))
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

func panelDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(panelSource{}),
		Storage:       reflect.TypeOf(appir.Panel{}),
		Normalize:     noDefinitionNormalization,
		Validate:      noDefinitionValidation,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var decoded panelSource
			if err := definition.DecodeSpec(source.Spec, &decoded); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			panel := appir.Panel{Name: source.Metadata.Name, Layout: decoded.Layout, Policy: decoded.Policy, Regions: make([]appir.Region, len(decoded.Regions))}
			for regionIndex, sourceRegion := range decoded.Regions {
				region := appir.Region{Name: sourceRegion.Name, CollapseWhenEmpty: sourceRegion.CollapseWhenEmpty, Blocks: sourceRegion.Blocks}
				if sourceRegion.Items != nil {
					region.Items = make([]appir.RegionItem, len(sourceRegion.Items))
					for itemIndex, sourceItem := range sourceRegion.Items {
						beancontent.Normalize(sourceItem.Content)
						item := appir.RegionItem{ID: sourceItem.ID, Block: sourceItem.Block, Content: sourceItem.Content}
						if sourceItem.Content != nil {
							token := fmt.Sprintf("item/%d", itemIndex)
							if sourceItem.ID != "" {
								token = "id/" + sourceItem.ID
							}
							item.Identity = fmt.Sprintf("@inline/%s/%s/%s", source.Metadata.Name, sourceRegion.Name, token)
						}
						region.Items[itemIndex] = item
					}
				}
				panel.Regions[regionIndex] = region
			}
			app.Panels[source.Metadata.Name] = panel
			return nil
		},
		Lookup: func(app *appir.App, name string) (any, bool) {
			value, exists := app.Panels[name]
			return value, exists
		},
		Names: func(app *appir.App) []string { return mapNames(app.Panels) },
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
	if value.Navigation != nil {
		sort.Strings(value.Navigation.Menus)
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

func normalizeViews(app *appir.App) {
	for name, view := range app.Views {
		entity, exists := app.Entities[view.Entity]
		if !exists {
			continue
		}
		relationships := map[string]appir.ViewRelationship{}
		for index, relationship := range view.Relationships {
			if resolved, found := resolveViewRelationship(entity, relationship); found {
				relationship = resolved
				view.Relationships[index] = relationship
			}
			relationships[relationship.Name] = relationship
		}
		switch {
		case len(view.GroupBy) > 0:
			view.ResultShape = "groups"
		case len(view.Aggregates) > 0:
			view.ResultShape = "metric"
		default:
			view.ResultShape = "records"
		}
		for filterName, filter := range view.ExposedFilters {
			target := filter.Target(filterName)
			if filter.Field == "" {
				filter.Field = target
			}
			if filter.Operator == "" {
				filter.Operator = "eq"
			}
			if definition, found := viewFieldDefinition(target, entity, relationships, app); found {
				if filter.Type == "" {
					filter.Type = definition.Type
				}
				if filter.Label == "" {
					filter.Label = definition.Label
				}
				if len(filter.Options) == 0 {
					filter.Options = append([]string{}, definition.Options...)
				}
				if filter.Relation == nil {
					filter.Relation = definition.Relation
				}
			}
			view.ExposedFilters[filterName] = filter
		}
		if view.Displays == nil {
			view.Displays = map[string]appir.Display{}
		}
		for displayName, display := range view.Displays {
			if display.Type != "page" && display.Type != "block" {
				continue
			}
			if display.Renderer.Type == "" {
				display.Renderer.Type = "list"
			}
			if display.Renderer.EmptyState == "" {
				display.Renderer.EmptyState = display.EmptyState
			}
			if display.Pager.Type == "" {
				if display.Renderer.Type == "detail" || display.Renderer.Type == "metric" || display.Renderer.Type == "board" || display.Renderer.Type == "tree" || display.Renderer.Type == "chart" || display.Renderer.Type == "calendar" {
					display.Pager.Type = "none"
				} else {
					display.Pager.Type = "cursor"
				}
			}
			if display.Pager.PageSize == 0 {
				display.Pager.PageSize = view.DefaultLimit
			}
			for index := range display.Controls {
				if display.Controls[index].Widget == "" {
					display.Controls[index].Widget = "auto"
				}
			}
			view.Displays[displayName] = display
		}
		app.Views[name] = view
	}
	for name, view := range app.Views {
		for displayName, display := range view.Displays {
			if display.Drill != nil {
				drill := *display.Drill
				drill.Route = app.Views[drill.View].Displays[drill.Display].Route
				display.Drill = &drill
				view.Displays[displayName] = display
			}
		}
		app.Views[name] = view
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
				compiled := appir.Step{Op: step.Op, Result: step.Result, Entity: step.Entity, View: step.View, Extension: step.Extension, StateField: step.StateField, Where: step.Where, Condition: step.Condition, Event: step.Event, Job: step.Job}
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

// DefinitionDiff gives authoring clients a deterministic, definition-level
// semantic summary. The Agent Protocol retains its field-level diff.
func DefinitionDiff(current, candidate *appir.App) []DefinitionChange {
	if current == nil {
		current = appir.Empty()
	}
	if candidate == nil {
		candidate = appir.Empty()
	}
	out := []DefinitionChange{}
	for _, kind := range definitionKindRegistry().Names() {
		registered, _ := definitionKindRegistry().Lookup(kind)
		names := append(registered.Names(current), registered.Names(candidate)...)
		sort.Strings(names)
		for index, name := range names {
			if index > 0 && names[index-1] == name {
				continue
			}
			before, beforeExists := registered.Lookup(current, name)
			after, afterExists := registered.Lookup(candidate, name)
			operation := ""
			switch {
			case !beforeExists && afterExists:
				operation = "add"
			case beforeExists && !afterExists:
				operation = "remove"
			case beforeExists && afterExists && !reflect.DeepEqual(before, after):
				operation = "replace"
			}
			if operation != "" {
				out = append(out, DefinitionChange{Operation: operation, Path: "definitions." + kind + "." + name})
			}
		}
	}
	return out
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
	if item.Navigation != nil {
		out = append(out, reference("navigation.destination.view", "View", item.Navigation.Destination.View))
		for index, menuName := range item.Navigation.Menus {
			out = append(out, reference(fmt.Sprintf("navigation.menus.%d", index), "Menu", menuName))
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
	for displayName, display := range item.Displays {
		out = append(out, reference("displays."+displayName+".renderer.moveAction", "Action", display.Renderer.MoveAction))
		for index, action := range display.Actions {
			out = append(out, reference(fmt.Sprintf("displays.%s.actions.%d", displayName, index), "Action", action))
		}
		if display.Drill != nil {
			out = append(out, reference("displays."+displayName+".drill.view", "View", display.Drill.View))
		}
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
			reference(fmt.Sprintf("steps.%d.extension", index), "Extension", step.Extension),
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
		for itemIndex, regionItem := range region.Items {
			if regionItem.Block != "" {
				out = append(out, reference(fmt.Sprintf("regions.%d.items.%d.block", regionIndex, itemIndex), "Block", regionItem.Block))
			}
		}
	}
	return references(out...)
}

func menuReferences(app *appir.App, name string) []DefinitionReference {
	menu := app.Menus[name]
	out := []DefinitionReference{}
	if menu.Owner != nil {
		out = append(out, reference("owner.entity", "Entity", menu.Owner.Entity))
	}
	for index, item := range menu.Items {
		out = append(out, reference(fmt.Sprintf("items.%d.policy", index), "Policy", item.Policy))
		out = append(out, reference(fmt.Sprintf("items.%d.target.page", index), "Page", item.Target.Page))
		out = append(out, reference(fmt.Sprintf("items.%d.target.view", index), "View", item.Target.View))
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
