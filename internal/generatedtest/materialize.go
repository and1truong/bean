// Package generatedtest derives deterministic semantic checks from compiled
// application definitions without inventing application expectations.
package generatedtest

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/demoseed"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/policy"
)

const generatedPrefix = "generated_"

type Origin struct {
	Family string           `json:"family"`
	Source appir.TestTarget `json:"source"`
	Suite  string           `json:"suite"`
}

// Materialize appends generated TestSuite definitions to bundle. Explicit
// expectations remain the oracle: replay cases copy them verbatim and are
// compiled and executed through the ordinary TestSuite contract.
func Materialize(bundle definition.Bundle) (definition.Bundle, map[string]Origin, []definition.Diagnostic) {
	compiled := compiler.Compile("generated-test", 1, bundle.Definitions)
	if len(compiled.Diagnostics) > 0 {
		return bundle, nil, compiled.Diagnostics
	}
	for name := range compiled.App.TestSuites {
		if strings.HasPrefix(name, generatedPrefix) {
			return bundle, nil, []definition.Diagnostic{{
				Code: "BEAN-T1101", Kind: "TestSuite", Name: name, Path: "metadata.name",
				Message: "generated_ is reserved for generated TestSuite identities",
			}}
		}
	}

	out := bundle
	out.Definitions = append([]definition.Definition{}, bundle.Definitions...)
	origins := map[string]Origin{}
	for _, suiteName := range sortedKeys(compiled.App.TestSuites) {
		suite := compiled.App.TestSuites[suiteName]
		replays := make([]appir.TestCase, 0, len(suite.Tests))
		for _, test := range suite.Tests {
			test.Name = "replay_" + test.Name
			replays = append(replays, test)
		}
		appendGeneratedSuite(&out, origins, "replay", suiteName, suite.Target, replays)
		if suite.Target.Kind != "Action" {
			continue
		}
		action := compiled.App.Actions[suite.Target.Name]
		if denied := policyDenialCases(compiled.App, action, suite.Tests); len(denied) > 0 {
			appendGeneratedSuite(&out, origins, "policy", suiteName, suite.Target, denied)
		}
		if invalid := invalidTransitionCases(compiled.App, action, suite.Tests); len(invalid) > 0 {
			appendGeneratedSuite(&out, origins, "transition", suiteName, suite.Target, invalid)
		}
	}
	if compiled.App.DemoSeed != nil {
		records, err := demoseed.Generate(compiled.App, 42)
		if err != nil {
			return bundle, nil, []definition.Diagnostic{{Code: "BEAN-T1102", Kind: "DemoSeed", Name: compiled.App.DemoSeed.Name, Path: "spec", Message: err.Error()}}
		}
		for _, suite := range crudSuites(compiled.App, records) {
			appendGeneratedSuite(&out, origins, "crud", suite.Target.Name, suite.Target, suite.Tests)
		}
	}
	generated := compiler.Compile("generated-test", 1, out.Definitions)
	if len(generated.Diagnostics) > 0 {
		return bundle, nil, generated.Diagnostics
	}
	return out, origins, nil
}

func crudSuites(app *appir.App, records []demoseed.Record) []appir.TestSuite {
	first := map[string]demoseed.Record{}
	for _, record := range records {
		if _, exists := first[record.Entity]; !exists {
			first[record.Entity] = record
		}
	}
	suites := []appir.TestSuite{}
	for _, entityName := range sortedKeys(first) {
		record := first[entityName]
		entity := app.Entities[entityName]
		for _, operation := range []string{"create", "delete", "update"} {
			actionName := entityName + "_" + operation
			action, exists := app.Actions[actionName]
			if !exists || action.Entity != entityName || action.Operation != operation {
				continue
			}
			test := appir.TestCase{
				Name:     "smoke_" + operation,
				Fixtures: crudFixtures(app, records, record, operation == "create"),
				Input:    map[string]any{},
				Context:  generatedContext(app, record.ID, operation == "create"),
			}
			switch operation {
			case "create":
				test.Input = copyMap(record.Values)
				for name := range action.Derive {
					delete(test.Input, name)
				}
				if lifecycle, exists := lifecycleForEntity(app, entityName); exists {
					test.Input[lifecycle.StateField] = lifecycle.Initial
				}
				test.Expect = appir.TestExpectation{Result: raw(map[string]any{"id": record.ID}), Changes: []appir.TestMutation{{Entity: entityName, ID: record.ID, Values: map[string]any{"id": record.ID}}}, NoEvents: true}
			case "update":
				test.Input = map[string]any{"id": record.ID}
				test.Expect = appir.TestExpectation{Result: raw(map[string]any{"id": record.ID}), Changes: []appir.TestMutation{{Entity: entityName, ID: record.ID, Values: map[string]any{"version": 2}}}, NoEvents: true}
			case "delete":
				test.Input = map[string]any{"id": record.ID}
				change := appir.TestMutation{Entity: entityName, ID: record.ID, Absent: true}
				if entity.SoftDelete {
					change = appir.TestMutation{Entity: entityName, ID: record.ID, Values: map[string]any{"deleted_at": test.Context.Time}}
				}
				test.Expect = appir.TestExpectation{Result: raw(map[string]any{"id": record.ID}), Changes: []appir.TestMutation{change}, NoEvents: true}
			}
			validatedMutation := operation != "delete" && len(entity.Validations) > 0
			if action.When != "" || validatedMutation || !crudInputSupported(action, test.Input) || !crudAuthorized(app, action, record, test.Context) {
				continue
			}
			suites = append(suites, appir.TestSuite{Target: appir.TestTarget{Kind: "Action", Name: actionName}, Tests: []appir.TestCase{test}})
		}
	}
	return suites
}

func crudInputSupported(action appir.Action, input map[string]any) bool {
	for name, value := range input {
		definition, declared := action.Input[name]
		if !declared {
			return false
		}
		if _, derived := action.Derive[name]; derived || field.Validate(definition, value) != nil {
			return false
		}
	}
	for name, definition := range action.Input {
		if _, derived := action.Derive[name]; derived {
			continue
		}
		if _, supplied := input[name]; !supplied && field.Validate(definition, nil) != nil {
			return false
		}
	}
	return true
}

func crudAuthorized(app *appir.App, action appir.Action, record demoseed.Record, context appir.TestContext) bool {
	if action.Policy == "" {
		return true
	}
	definition, exists := app.Policies[action.Policy]
	if !exists {
		return false
	}
	row := copyMap(record.Values)
	row["id"] = record.ID
	entity := app.Entities[record.Entity]
	if entity.Owner {
		row["owner_id"] = generatedActorID
	}
	if entity.Tenant {
		row["tenant_id"] = generatedTenantID
	}
	return policy.Can(definition, true, request(context), row)
}

func crudFixtures(app *appir.App, records []demoseed.Record, target demoseed.Record, omitTarget bool) map[string][]map[string]any {
	fixtures := map[string][]map[string]any{}
	included := dependencyRecords(app, records, target)
	if !omitTarget {
		included = append(included, target)
		sort.Slice(included, func(i, j int) bool {
			if included[i].Entity != included[j].Entity {
				return included[i].Entity < included[j].Entity
			}
			return included[i].ID < included[j].ID
		})
	}
	for _, record := range included {
		values := copyMap(record.Values)
		values["id"] = record.ID
		entity := app.Entities[record.Entity]
		if entity.Owner {
			values["owner_id"] = generatedActorID
		}
		if entity.Tenant {
			values["tenant_id"] = generatedTenantID
		}
		fixtures[record.Entity] = append(fixtures[record.Entity], values)
	}
	return fixtures
}

func dependencyRecords(app *appir.App, records []demoseed.Record, target demoseed.Record) []demoseed.Record {
	included := map[string]demoseed.Record{}
	var add func(demoseed.Record)
	add = func(record demoseed.Record) {
		entity := app.Entities[record.Entity]
		for _, field := range entity.Fields {
			if field.Relation == nil || record.Values[field.Name] == nil {
				continue
			}
			values := []any{record.Values[field.Name]}
			if many, ok := record.Values[field.Name].([]any); ok {
				values = many
			}
			for _, value := range values {
				dependency, exists := findRecord(records, field.Relation.Entity, field.Relation.TargetField, value)
				if !exists {
					continue
				}
				if dependency.Entity == target.Entity && dependency.ID == target.ID {
					continue
				}
				key := dependency.Entity + "/" + dependency.ID
				if _, exists = included[key]; exists {
					continue
				}
				included[key] = dependency
				add(dependency)
			}
		}
	}
	add(target)
	out := make([]demoseed.Record, 0, len(included))
	for _, record := range included {
		out = append(out, record)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Entity != out[j].Entity {
			return out[i].Entity < out[j].Entity
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func findRecord(records []demoseed.Record, entityName, targetField string, value any) (demoseed.Record, bool) {
	if targetField == "" {
		targetField = "id"
	}
	for _, record := range records {
		if record.Entity != entityName {
			continue
		}
		candidate := record.Values[targetField]
		if targetField == "id" {
			candidate = record.ID
		}
		if fmt.Sprint(candidate) == fmt.Sprint(value) {
			return record, true
		}
	}
	return demoseed.Record{}, false
}

const (
	generatedActorID  = "00000000-0000-4000-8000-000000000901"
	generatedTenantID = "00000000-0000-4000-8000-000000000902"
	generatedTime     = "2026-01-02T06:00:00Z"
)

func generatedContext(app *appir.App, id string, withID bool) appir.TestContext {
	roles := []string{"administrator"}
	for name := range app.Roles {
		if name != "administrator" {
			roles = append(roles, name)
		}
	}
	sort.Strings(roles)
	context := appir.TestContext{Actor: &appir.TestActor{ID: generatedActorID, Email: "generated-test@example.test", Roles: roles}, Tenant: generatedTenantID, Time: generatedTime, RequestID: "generated-crud"}
	if withID {
		context.IDs = []string{id}
	}
	return context
}

func lifecycleForEntity(app *appir.App, entityName string) (appir.Lifecycle, bool) {
	for _, name := range sortedKeys(app.Lifecycles) {
		lifecycle := app.Lifecycles[name]
		if lifecycle.Entity == entityName {
			return lifecycle, true
		}
	}
	return appir.Lifecycle{}, false
}

func raw(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func appendGeneratedSuite(bundle *definition.Bundle, origins map[string]Origin, family, suiteName string, target appir.TestTarget, cases []appir.TestCase) {
	generatedName := generatedPrefix + family + "_" + suiteName
	tests := make([]any, len(cases))
	for index, test := range cases {
		tests[index] = testValue(test)
	}
	bundle.Definitions = append(bundle.Definitions, definition.Definition{
		APIVersion: definition.APIVersion,
		Kind:       "TestSuite",
		Metadata:   definition.Metadata{Name: generatedName},
		Spec:       map[string]any{"target": jsonValue(target), "tests": tests},
	})
	origins[generatedName] = Origin{Family: family, Source: target, Suite: suiteName}
}

func policyDenialCases(app *appir.App, action appir.Action, cases []appir.TestCase) []appir.TestCase {
	if action.When != "" {
		return nil
	}
	policyDefinition, exists := app.Policies[action.Policy]
	if !exists {
		return nil
	}
	out := []appir.TestCase{}
	for _, test := range cases {
		if test.Expect.Error != "" || !policy.Can(policyDefinition, true, request(test.Context), nil) {
			continue
		}
		deniedContext := test.Context
		deniedContext.Actor = nil
		if policy.Can(policyDefinition, true, request(deniedContext), nil) {
			continue
		}
		test.Name = "deny_" + test.Name
		test.Context = deniedContext
		test.Expect = deniedExpectation()
		out = append(out, test)
	}
	return out
}

func invalidTransitionCases(app *appir.App, action appir.Action, cases []appir.TestCase) []appir.TestCase {
	lifecycle, exists := app.Lifecycles[action.Lifecycle]
	if !exists || action.Operation != "transition" {
		return nil
	}
	transitions := lifecycle.Transitions
	if action.Transitions != nil {
		transitions = action.Transitions
	}
	out := []appir.TestCase{}
	for _, test := range cases {
		if test.Expect.Error != "" {
			continue
		}
		id, _ := test.Input["id"].(string)
		current := fixtureField(test.Fixtures[lifecycle.Entity], id, lifecycle.StateField)
		if current == "" {
			continue
		}
		invalid := invalidState(app.Entities[lifecycle.Entity], lifecycle.StateField, current, transitions[current])
		if invalid == "" {
			continue
		}
		test.Name = "invalid_" + test.Name
		test.Input = copyMap(test.Input)
		test.Input[lifecycle.StateField] = invalid
		test.Expect = deniedExpectation()
		out = append(out, test)
	}
	return out
}

func request(value appir.TestContext) beanctx.Request {
	request := beanctx.Request{TenantID: value.Tenant, RequestID: value.RequestID}
	if value.Actor != nil {
		request.User = &beanctx.User{ID: value.Actor.ID, Email: value.Actor.Email, DisplayName: value.Actor.DisplayName, Roles: append([]string{}, value.Actor.Roles...)}
	}
	return request
}

func fixtureField(rows []map[string]any, id, field string) string {
	for _, row := range rows {
		if rowID, _ := row["id"].(string); rowID == id {
			value, _ := row[field].(string)
			return value
		}
	}
	return ""
}

func invalidState(entity appir.Entity, stateField, current string, allowed []string) string {
	allowedSet := map[string]bool{}
	for _, value := range allowed {
		allowedSet[value] = true
	}
	for _, field := range entity.Fields {
		if field.Name != stateField {
			continue
		}
		for _, option := range field.Options {
			if !allowedSet[option] {
				return option
			}
		}
	}
	if !allowedSet[current] {
		return current
	}
	return ""
}

func deniedExpectation() appir.TestExpectation {
	return appir.TestExpectation{Error: "conflict", NoChanges: true, NoEvents: true}
}

func copyMap(values map[string]any) map[string]any {
	out := make(map[string]any, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func testValue(test appir.TestCase) any {
	value := jsonValue(test).(map[string]any)
	if len(test.Expect.Result) == 0 {
		delete(value["expect"].(map[string]any), "result")
	}
	return value
}

func jsonValue(value any) any {
	encoded, _ := json.Marshal(value)
	var out any
	_ = json.Unmarshal(encoded, &out)
	return out
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
