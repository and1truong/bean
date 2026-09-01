package compiler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/rule"
	"github.com/beanruntime/bean/internal/testsuite"
)

var testCaseName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

func canonicalizeTestSuiteCases(app *appir.App) {
	for name, suite := range app.TestSuites {
		sort.Slice(suite.Tests, func(i, j int) bool { return suite.Tests[i].Name < suite.Tests[j].Name })
		for caseIndex := range suite.Tests {
			test := &suite.Tests[caseIndex]
			if test.Context.Actor != nil {
				sort.Strings(test.Context.Actor.Roles)
			}
			for entityName, rows := range test.Fixtures {
				sort.Slice(rows, func(i, j int) bool { return canonicalTestValue(rows[i]) < canonicalTestValue(rows[j]) })
				test.Fixtures[entityName] = rows
			}
			sort.Slice(test.Expect.Changes, func(i, j int) bool {
				return canonicalTestValue(test.Expect.Changes[i]) < canonicalTestValue(test.Expect.Changes[j])
			})
			sort.Slice(test.Expect.Events, func(i, j int) bool {
				return canonicalTestValue(test.Expect.Events[i]) < canonicalTestValue(test.Expect.Events[j])
			})
			for auditIndex := range test.Expect.Audit {
				sort.Strings(test.Expect.Audit[auditIndex].Changed)
			}
			sort.Slice(test.Expect.Audit, func(i, j int) bool {
				return canonicalTestValue(test.Expect.Audit[i]) < canonicalTestValue(test.Expect.Audit[j])
			})
		}
		app.TestSuites[name] = suite
	}
}

func canonicalTestValue(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func validateTestSuites(app *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if len(app.TestSuites) > testsuite.MaxSuites {
		out = append(out, testSuiteDiagnostic("", "spec", fmt.Sprintf("application exceeds %d TestSuite definitions", testsuite.MaxSuites)))
	}
	encodedSize := 0
	for _, name := range keys(app.TestSuites) {
		suite := app.TestSuites[name]
		encoded, _ := json.Marshal(suite)
		encodedSize += len(encoded)
		out = append(out, validateTestSuite(app, suite)...)
	}
	if encodedSize > testsuite.MaxEncodedSize {
		out = append(out, testSuiteDiagnostic("", "spec", fmt.Sprintf("encoded TestSuite data exceeds %d bytes", testsuite.MaxEncodedSize)))
	}
	return out
}

func validateTestSuite(app *appir.App, suite appir.TestSuite) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	switch suite.Target.Kind {
	case "Rule":
		if _, exists := app.Rules[suite.Target.Name]; !exists {
			out = append(out, missingReferenceDiagnostic("TestSuite", suite.Name, "spec.target.name", "Rule", suite.Target.Name))
		}
	case "Action":
		if _, exists := app.Actions[suite.Target.Name]; !exists {
			out = append(out, missingReferenceDiagnostic("TestSuite", suite.Name, "spec.target.name", "Action", suite.Target.Name))
		}
	default:
		out = append(out, testSuiteDiagnostic(suite.Name, "spec.target.kind", "target kind must be Action or Rule"))
	}
	if suite.Target.Name == "" {
		out = append(out, requiredDiagnostic("TestSuite", suite.Name, "spec.target.name", "is required"))
	}
	if len(suite.Tests) == 0 {
		out = append(out, requiredDiagnostic("TestSuite", suite.Name, "spec.tests", "at least one case is required"))
	} else if len(suite.Tests) > testsuite.MaxCases {
		out = append(out, testSuiteDiagnostic(suite.Name, "spec.tests", fmt.Sprintf("exceeds %d cases", testsuite.MaxCases)))
	}
	seen := map[string]bool{}
	for index, test := range suite.Tests {
		path := fmt.Sprintf("spec.tests.%d", index)
		if !testCaseName.MatchString(test.Name) {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".name", "must match ^[a-z][a-z0-9_]*$"))
		} else if seen[test.Name] {
			out = append(out, duplicateDiagnostic("TestSuite", suite.Name, path+".name", "duplicate test case name"))
		}
		seen[test.Name] = true
		out = append(out, validateTestCase(app, suite, test, path)...)
	}
	return out
}

func validateTestCase(app *appir.App, suite appir.TestSuite, test appir.TestCase, path string) []definition.Diagnostic {
	out := validateTestContext(app, suite.Name, test.Context, path+".context")
	fixtureCount := 0
	fixtureValues := map[string]bool{}
	fixtureIdentities := map[string]string{}
	for entityName, rows := range test.Fixtures {
		for rowIndex, row := range rows {
			for fieldName, value := range row {
				fixtureValues[entityName+"/"+fieldName+"/"+fmt.Sprint(value)] = true
			}
			identity := entityName + "/" + fmt.Sprint(row["id"])
			path := fmt.Sprintf("%s.fixtures.%s.%d.id", path, entityName, rowIndex)
			if first := fixtureIdentities[identity]; row["id"] != nil && first != "" {
				out = append(out, duplicateDiagnostic("TestSuite", suite.Name, path, "duplicates fixture identity at "+first))
			} else {
				fixtureIdentities[identity] = path
			}
		}
	}
	for _, entityName := range keys(test.Fixtures) {
		rows := test.Fixtures[entityName]
		fixtureCount += len(rows)
		entity, exists := app.Entities[entityName]
		if !exists {
			out = append(out, missingReferenceDiagnostic("TestSuite", suite.Name, path+".fixtures."+entityName, "Entity", entityName))
			continue
		}
		for rowIndex, row := range rows {
			fixturePath := fmt.Sprintf("%s.fixtures.%s.%d", path, entityName, rowIndex)
			out = append(out, validateFixture(app, suite.Name, entity, row, fixturePath)...)
			out = append(out, validateFixtureRelations(suite.Name, entity, row, fixtureValues, fixturePath)...)
		}
	}
	out = append(out, validateFixtureUniqueness(app, suite.Name, test.Fixtures, path+".fixtures")...)
	out = append(out, validateFixtureCycles(app, suite.Name, test.Fixtures, fixtureValues, path+".fixtures")...)
	if fixtureCount > testsuite.MaxFixtures {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".fixtures", fmt.Sprintf("exceeds %d fixture records", testsuite.MaxFixtures)))
	}
	resultPresent := len(bytes.TrimSpace(test.Expect.Result)) > 0
	if suite.Target.Kind == "Rule" && resultPresent == (test.Expect.Error != "") {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".expect", "Rule cases must specify exactly one of result or error"))
	}
	if suite.Target.Kind == "Action" {
		if resultPresent && test.Expect.Error != "" {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".expect", "Action cases cannot combine result and error"))
		}
		if !resultPresent && test.Expect.Error == "" && len(test.Expect.Changes) == 0 && len(test.Expect.Events) == 0 && len(test.Expect.Audit) == 0 && !test.Expect.NoChanges && !test.Expect.NoEvents {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".expect", "Action cases must specify at least one assertion"))
		}
	}
	if test.Expect.Error != "" && !nameSet(testsuite.ErrorCodes)[test.Expect.Error] {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.error", "is not a stable semantic test error code"))
	}
	if test.Expect.NoChanges && len(test.Expect.Changes) > 0 {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.changes", "cannot combine changes with noChanges"))
	}
	if test.Expect.NoEvents && len(test.Expect.Events) > 0 {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.events", "cannot combine events with noEvents"))
	}
	switch suite.Target.Kind {
	case "Rule":
		out = append(out, validateRuleTestCase(app, suite, test, path)...)
	case "Action":
		out = append(out, validateActionTestCase(app, suite, test, path)...)
	}
	return out
}

func validateFixtureUniqueness(app *appir.App, suiteName string, fixtures map[string][]map[string]any, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for _, entityName := range keys(fixtures) {
		entity, exists := app.Entities[entityName]
		if !exists {
			continue
		}
		constraints := append([][]string{}, entity.Unique...)
		for _, item := range entity.Fields {
			if item.Unique {
				constraints = append(constraints, []string{item.Name})
			}
		}
		for _, constraint := range constraints {
			seen := map[string]string{}
			for rowIndex, row := range fixtures[entityName] {
				values := make([]any, 0, len(constraint))
				complete := true
				for _, fieldName := range constraint {
					value, exists := row[fieldName]
					if !exists || value == nil {
						complete = false
						break
					}
					values = append(values, value)
				}
				if !complete {
					continue
				}
				key := canonicalTestValue(values)
				rowPath := fmt.Sprintf("%s.%s.%d", path, entityName, rowIndex)
				if first := seen[key]; first != "" {
					out = append(out, testSuiteDiagnostic(suiteName, rowPath, "duplicates fixture unique constraint at "+first))
				} else {
					seen[key] = rowPath
				}
			}
		}
	}
	return out
}

type testFixtureRecord struct {
	entity appir.Entity
	row    map[string]any
}

func validateFixtureCycles(app *appir.App, suiteName string, fixtures map[string][]map[string]any, all map[string]bool, path string) []definition.Diagnostic {
	pending := []testFixtureRecord{}
	for _, entityName := range keys(fixtures) {
		entity, exists := app.Entities[entityName]
		if !exists {
			continue
		}
		for _, row := range fixtures[entityName] {
			pending = append(pending, testFixtureRecord{entity: entity, row: row})
		}
	}
	inserted := map[string]bool{}
	for len(pending) > 0 {
		progress := false
		remaining := pending[:0]
		for _, record := range pending {
			if !testFixtureDependenciesReady(record, all, inserted) {
				remaining = append(remaining, record)
				continue
			}
			for fieldName, value := range record.row {
				inserted[record.entity.Name+"/"+fieldName+"/"+fmt.Sprint(value)] = true
			}
			progress = true
		}
		if !progress {
			return []definition.Diagnostic{testSuiteDiagnostic(suiteName, path, "fixture relations contain a cycle")}
		}
		pending = remaining
	}
	return nil
}

func testFixtureDependenciesReady(record testFixtureRecord, all, inserted map[string]bool) bool {
	for _, item := range record.entity.Fields {
		if item.Type != "relation" || item.Relation == nil || item.Relation.Kind == "one-to-many" || item.Relation.Kind == "many-to-many" {
			continue
		}
		value := record.row[item.Name]
		if value == nil {
			continue
		}
		targetField := item.Relation.TargetField
		if targetField == "" {
			targetField = "id"
		}
		key := item.Relation.Entity + "/" + targetField + "/" + fmt.Sprint(value)
		if all[key] && !inserted[key] {
			return false
		}
	}
	return true
}

func validateFixtureRelations(suiteName string, entity appir.Entity, row map[string]any, fixtureValues map[string]bool, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for _, item := range entity.Fields {
		if item.Type != "relation" || item.Relation == nil || row[item.Name] == nil {
			continue
		}
		values := []any{row[item.Name]}
		if item.Relation.Kind == "one-to-many" || item.Relation.Kind == "many-to-many" {
			values, _ = row[item.Name].([]any)
		}
		for index, value := range values {
			targetField := item.Relation.TargetField
			if targetField == "" {
				targetField = "id"
			}
			if fixtureValues[item.Relation.Entity+"/"+targetField+"/"+fmt.Sprint(value)] {
				continue
			}
			relationPath := path + "." + item.Name
			if len(values) > 1 {
				relationPath += fmt.Sprintf(".%d", index)
			}
			out = append(out, testSuiteDiagnostic(suiteName, relationPath, "references a record absent from case fixtures"))
		}
	}
	return out
}

func validateTestContext(app *appir.App, suiteName string, context appir.TestContext, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if context.Time != "" {
		if _, err := time.Parse(time.RFC3339Nano, context.Time); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, path+".time", "must be RFC3339"))
		}
	}
	if context.Actor != nil {
		if context.Actor.ID != "" {
			if err := field.Validate(appir.Field{Name: "id", Type: "uuid"}, context.Actor.ID); err != nil {
				out = append(out, testSuiteDiagnostic(suiteName, path+".actor.id", err.Error()))
			}
		}
		if context.Actor.Email != "" {
			if err := field.Validate(appir.Field{Name: "email", Type: "email"}, context.Actor.Email); err != nil {
				out = append(out, testSuiteDiagnostic(suiteName, path+".actor.email", err.Error()))
			}
		}
		seenRoles := map[string]bool{}
		for index, roleName := range context.Actor.Roles {
			rolePath := fmt.Sprintf("%s.actor.roles.%d", path, index)
			if seenRoles[roleName] {
				out = append(out, duplicateDiagnostic("TestSuite", suiteName, rolePath, "duplicate actor role"))
			} else if _, exists := app.Roles[roleName]; !exists && roleName != "administrator" {
				out = append(out, missingReferenceDiagnostic("TestSuite", suiteName, rolePath, "Role", roleName))
			}
			seenRoles[roleName] = true
		}
	}
	if context.Tenant != "" {
		if err := field.Validate(appir.Field{Name: "tenant", Type: "uuid"}, context.Tenant); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, path+".tenant", err.Error()))
		}
	}
	seenIDs := map[string]bool{}
	for index, id := range context.IDs {
		idPath := fmt.Sprintf("%s.ids.%d", path, index)
		if id == "" {
			out = append(out, requiredDiagnostic("TestSuite", suiteName, idPath, "is required"))
		} else if err := field.Validate(appir.Field{Name: "id", Type: "uuid"}, id); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, idPath, err.Error()))
		} else if seenIDs[id] {
			out = append(out, duplicateDiagnostic("TestSuite", suiteName, idPath, "duplicate deterministic ID"))
		}
		seenIDs[id] = true
	}
	return out
}

func validateFixture(app *appir.App, suiteName string, entity appir.Entity, row map[string]any, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	allowed := fieldSet(entity)
	for name := range row {
		if !allowed[name] {
			out = append(out, missingFieldDiagnostic("TestSuite", suiteName, path+"."+name, name, false))
		}
	}
	if fmt.Sprint(row["id"]) == "" {
		out = append(out, requiredDiagnostic("TestSuite", suiteName, path+".id", "is required"))
	} else if err := field.Validate(appir.Field{Name: "id", Type: "uuid"}, row["id"]); err != nil {
		out = append(out, testSuiteDiagnostic(suiteName, path+".id", err.Error()))
	}
	for _, name := range []string{"created_at", "updated_at", "version", "owner_id", "tenant_id", "deleted_at"} {
		value, supplied := row[name]
		if !supplied {
			continue
		}
		item, exists := testFieldDefinition(entity, name)
		if !exists {
			continue
		}
		if err := field.Validate(item, value); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, path+"."+name, err.Error()))
		}
	}
	for _, item := range entity.Fields {
		value, supplied := row[item.Name]
		if !supplied && !item.Required {
			continue
		}
		if err := field.Validate(item, value); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, path+"."+item.Name, err.Error()))
		}
	}
	return out
}

func validateRuleTestCase(app *appir.App, suite appir.TestSuite, test appir.TestCase, path string) []definition.Diagnostic {
	targetRule, exists := app.Rules[suite.Target.Name]
	if !exists {
		return nil
	}
	out := []definition.Diagnostic{}
	for _, name := range keys(test.Input) {
		if _, declared := targetRule.Input[name]; !declared {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, "is not declared by target Rule"))
		}
	}
	for _, name := range keys(targetRule.Input) {
		input := targetRule.Input[name]
		value := test.Input[name]
		if err := field.Validate(input, value); err != nil {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, err.Error()))
		}
	}
	if targetRule.Entity == "" && len(test.This) > 0 {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".this", "target Rule does not declare an Entity"))
	} else if entity, entityExists := app.Entities[targetRule.Entity]; entityExists {
		out = append(out, validatePartialRecord(suite.Name, entity, test.This, path+".this")...)
	}
	if len(bytes.TrimSpace(test.Expect.Result)) > 0 {
		value, err := decodeTestValue(test.Expect.Result)
		if err != nil || !rule.ValueMatches(targetRule.Result, value) {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.result", "does not match target Rule result type"))
		}
	}
	if len(test.Expect.Changes) > 0 || len(test.Expect.Events) > 0 || len(test.Expect.Audit) > 0 || test.Expect.NoChanges || test.Expect.NoEvents {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".expect", "Rule cases cannot assert Action side effects"))
	}
	return out
}

func validateActionTestCase(app *appir.App, suite appir.TestSuite, test appir.TestCase, path string) []definition.Diagnostic {
	action, exists := app.Actions[suite.Target.Name]
	if !exists {
		return nil
	}
	out := []definition.Diagnostic{}
	if test.Context.Time == "" {
		out = append(out, requiredDiagnostic("TestSuite", suite.Name, path+".context.time", "is required for Action cases"))
	}
	requiredIDs := actionCreatedRecords(action)
	if len(test.Context.IDs) < requiredIDs && test.Context.Seed == nil {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".context.ids", fmt.Sprintf("requires at least %d deterministic IDs", requiredIDs)))
	}
	if action.Operation == "register_local_user" {
		out = append(out, testSuiteDiagnostic(suite.Name, path+".target", "register_local_user is not supported by v0.11 TestSuite cases"))
	}
	for _, name := range keys(test.Input) {
		input, declared := action.Input[name]
		if !declared {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, "is not declared by target Action"))
			continue
		}
		if _, derived := action.Derive[name]; derived {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, "derived Action input is server-owned"))
			continue
		}
		if err := field.Validate(input, test.Input[name]); err != nil {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, err.Error()))
		}
	}
	for _, name := range keys(action.Input) {
		if _, derived := action.Derive[name]; derived {
			continue
		}
		if _, supplied := test.Input[name]; supplied {
			continue
		}
		if err := field.Validate(action.Input[name], nil); err != nil {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".input."+name, err.Error()))
		}
	}
	for index, change := range test.Expect.Changes {
		changePath := fmt.Sprintf("%s.expect.changes.%d", path, index)
		entity, entityExists := app.Entities[change.Entity]
		if !entityExists {
			out = append(out, missingReferenceDiagnostic("TestSuite", suite.Name, changePath+".entity", "Entity", change.Entity))
			continue
		}
		if change.ID == "" {
			out = append(out, requiredDiagnostic("TestSuite", suite.Name, changePath+".id", "is required"))
		} else if err := field.Validate(appir.Field{Name: "id", Type: "uuid"}, change.ID); err != nil {
			out = append(out, testSuiteDiagnostic(suite.Name, changePath+".id", err.Error()))
		}
		out = append(out, validatePartialRecord(suite.Name, entity, change.Values, changePath+".values")...)
	}
	if len(bytes.TrimSpace(test.Expect.Result)) > 0 {
		value, err := decodeTestValue(test.Expect.Result)
		result, isObject := value.(map[string]any)
		if err != nil || value != nil && !isObject {
			out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.result", "Action result assertion must be an object or null"))
		} else if isObject {
			for name, expected := range result {
				output, declared := action.Output[name]
				if !declared {
					out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.result."+name, "is not declared by target Action output"))
					continue
				}
				if err := field.Validate(output, expected); err != nil {
					out = append(out, testSuiteDiagnostic(suite.Name, path+".expect.result."+name, err.Error()))
				}
			}
		}
	}
	for index, event := range test.Expect.Events {
		if event.Topic == "" {
			out = append(out, requiredDiagnostic("TestSuite", suite.Name, fmt.Sprintf("%s.expect.events.%d.topic", path, index), "is required"))
		}
	}
	return out
}

func actionCreatedRecords(action appir.Action) int {
	if action.Operation == "create" {
		return 1
	}
	count := 0
	for _, step := range action.Steps {
		if step.Op == "create" {
			count++
		}
	}
	return count
}

func validatePartialRecord(suiteName string, entity appir.Entity, values map[string]any, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, value := range values {
		item, exists := testFieldDefinition(entity, name)
		if !exists {
			out = append(out, missingFieldDiagnostic("TestSuite", suiteName, path+"."+name, name, false))
			continue
		}
		if err := field.Validate(item, value); err != nil {
			out = append(out, testSuiteDiagnostic(suiteName, path+"."+name, err.Error()))
		}
	}
	return out
}

func testFieldDefinition(entity appir.Entity, name string) (appir.Field, bool) {
	if item, exists := entityFieldDefinition(entity, name); exists {
		return item, true
	}
	switch name {
	case "id":
		return appir.Field{Name: name, Type: "uuid"}, true
	case "owner_id":
		return appir.Field{Name: name, Type: "uuid"}, entity.Owner
	case "tenant_id":
		return appir.Field{Name: name, Type: "uuid"}, entity.Tenant
	case "created_at", "updated_at":
		return appir.Field{Name: name, Type: "datetime", Required: true}, true
	case "deleted_at":
		return appir.Field{Name: name, Type: "datetime"}, entity.SoftDelete
	case "version":
		return appir.Field{Name: name, Type: "integer"}, true
	default:
		return appir.Field{}, false
	}
}

func decodeTestValue(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func testSuiteDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleTestSuite, "TestSuite", name, path, message)
}
