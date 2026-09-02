package semantictest

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/event"
	beanextension "github.com/beanruntime/bean/internal/extension"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/generatedtest"
	"github.com/beanruntime/bean/internal/rule"
	"github.com/beanruntime/bean/internal/testsuite"
	"github.com/beanruntime/bean/internal/view"
)

type CaseResult struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type SuiteResult struct {
	ID       string             `json:"id"`
	Status   string             `json:"status"`
	Evidence *GeneratedEvidence `json:"evidence,omitempty"`
	Cases    []CaseResult       `json:"cases"`
}

type GeneratedEvidence struct {
	Family string           `json:"family"`
	Source appir.TestTarget `json:"source"`
	Suite  string           `json:"suite"`
}

func Run(ctx context.Context, bundle definition.Bundle, directory string) ([]SuiteResult, []definition.Diagnostic, error) {
	return run(ctx, bundle, directory, nil)
}

func RunGenerated(ctx context.Context, bundle definition.Bundle, directory string) ([]SuiteResult, []definition.Diagnostic, error) {
	materialized, origins, diagnostics := generatedtest.Materialize(bundle)
	if len(diagnostics) > 0 {
		return nil, diagnostics, nil
	}
	return run(ctx, materialized, directory, origins)
}

func run(ctx context.Context, bundle definition.Bundle, directory string, origins map[string]generatedtest.Origin) ([]SuiteResult, []definition.Diagnostic, error) {
	compiled := compileBundle(bundle)
	results := make([]SuiteResult, 0, len(compiled.TestSuites))
	diagnostics := []definition.Diagnostic{}
	for _, suiteName := range sortedKeys(compiled.TestSuites) {
		suite := compiled.TestSuites[suiteName]
		suiteResult := SuiteResult{ID: "TestSuite/" + suiteName, Status: "passed", Cases: []CaseResult{}}
		if origin, generated := origins[suiteName]; generated {
			suiteResult.Evidence = &GeneratedEvidence{Family: origin.Family, Source: origin.Source, Suite: origin.Suite}
		}
		for caseIndex, test := range suite.Tests {
			caseID := suiteResult.ID + "/" + test.Name
			caseResult := CaseResult{ID: caseID, Status: "passed"}
			caseDirectory := filepath.Join(directory, fmt.Sprintf("%s-%03d", suiteName, caseIndex))
			if err := os.Mkdir(caseDirectory, 0o700); err != nil {
				return nil, nil, err
			}
			caseDiagnostics, err := runIsolatedCase(ctx, bundle, suiteName, test, filepath.Join(caseDirectory, "test.db"))
			removeErr := os.RemoveAll(caseDirectory)
			if err != nil {
				return nil, nil, err
			}
			if removeErr != nil {
				return nil, nil, removeErr
			}
			if len(caseDiagnostics) > 0 {
				caseResult.Status = "failed"
				suiteResult.Status = "failed"
				diagnostics = append(diagnostics, caseDiagnostics...)
			}
			suiteResult.Cases = append(suiteResult.Cases, caseResult)
		}
		results = append(results, suiteResult)
	}
	return results, diagnostics, nil
}

func compileBundle(bundle definition.Bundle) *appir.App {
	return compiler.Compile("default", 1, bundle.Definitions).App
}

func runIsolatedCase(ctx context.Context, bundle definition.Bundle, suiteName string, test appir.TestCase, database string) ([]definition.Diagnostic, error) {
	caseCtx, cancel := context.WithTimeout(ctx, testsuite.CaseTimeoutSec*time.Second)
	defer cancel()
	runtime, err := bootstrap.Open(caseCtx, database, false)
	if err != nil {
		return nil, err
	}
	defer runtime.DB.Close()
	_, _, diagnostics, err := runtime.Store.PublishBundle(caseCtx, "default", bundle)
	if err != nil {
		return nil, err
	}
	if len(diagnostics) > 0 {
		return nil, diagnostics[0]
	}
	app, active := runtime.Kernel.Active()
	if !active {
		return nil, fmt.Errorf("semantic test runtime has no active release")
	}
	suite := app.TestSuites[suiteName]
	if err = insertFixtures(caseCtx, runtime.DB, app, test); err != nil {
		return nil, err
	}
	before, err := entitySnapshot(caseCtx, runtime.DB, app)
	if err != nil {
		return nil, err
	}
	request := requestContext(test.Context)
	var result any
	var executionErr error
	provider := newTestProvider(test.Providers)
	if suite.Target.Kind == "Rule" {
		result, executionErr = executeRule(app.Rules[suite.Target.Name], test, request)
	} else {
		actions, serviceErr := actionService(runtime.DB, test.Context)
		if serviceErr != nil {
			return nil, serviceErr
		}
		result, executionErr = actions.Execute(caseCtx, app, suite.Target.Name, test.Input, request)
		now, parseErr := time.Parse(time.RFC3339Nano, test.Context.Time)
		if parseErr != nil {
			return nil, parseErr
		}
		runner := event.Runner{DB: runtime.DB, BatchSize: len(app.Actions[suite.Target.Name].Steps), Now: func() time.Time { return now }, Deliver: func(deliveryCtx context.Context, topic string, payload map[string]any) error {
			if !beanextension.IsTopic(topic) {
				return nil
			}
			return beanextension.Deliver(deliveryCtx, app, provider, topic, payload)
		}}
		if err = runner.RunOnce(caseCtx); err != nil {
			return nil, err
		}
		provider.finish()
	}
	return assertCase(ctx, runtime.DB, app, suite, test, before, result, executionErr, provider), nil
}

func executeRule(target appir.Rule, test appir.TestCase, request beanctx.Request) (any, error) {
	user := map[string]any(nil)
	if request.User != nil {
		user = map[string]any{"id": request.User.ID, "email": request.User.Email, "display_name": request.User.DisplayName, "roles": request.User.Roles}
	}
	contextValues := map[string]any{}
	if test.Context.Time != "" {
		contextValues["now"] = test.Context.Time
	}
	if request.RequestID != "" {
		contextValues["request_id"] = request.RequestID
	}
	return rule.EvaluateTyped(target.Expression, rule.Environment{This: test.This, Input: test.Input, User: user, TenantID: request.TenantID, Context: contextValues}, target.Result)
}

func actionService(database dbal.Database, context appir.TestContext) (action.Service, error) {
	now, err := time.Parse(time.RFC3339Nano, context.Time)
	if err != nil {
		return action.Service{}, err
	}
	ids := append([]string{}, context.IDs...)
	index := 0
	nextID := func() string {
		var id string
		if index < len(ids) {
			id = ids[index]
		} else if context.Seed != nil {
			id = seededID(*context.Seed, index-len(ids))
		}
		index++
		return id
	}
	return action.Service{
		DB: database, Auth: auth.Service{DB: database}, Now: func() time.Time { return now },
		CreateID: func(appir.Entity, map[string]any) string {
			return nextID()
		},
		CreateInvocationID: nextID,
	}, nil
}

type testProvider struct {
	results  map[string][]appir.TestProviderResult
	calls    []appir.TestProviderCall
	failures []string
}

func newTestProvider(definitions map[string][]appir.TestProviderResult) *testProvider {
	results := make(map[string][]appir.TestProviderResult, len(definitions))
	for name, values := range definitions {
		results[name] = append([]appir.TestProviderResult{}, values...)
	}
	return &testProvider{results: results, calls: []appir.TestProviderCall{}, failures: []string{}}
}

func (p *testProvider) Call(_ context.Context, _ appir.Extension, invocation beanextension.Invocation) (map[string]any, error) {
	p.calls = append(p.calls, appir.TestProviderCall{
		Extension: invocation.Extension, InvocationID: invocation.InvocationID,
		IdempotencyKey: invocation.IdempotencyKey, Input: invocation.Input,
	})
	results := p.results[invocation.Extension]
	if len(results) == 0 {
		p.failures = append(p.failures, "unexpected call to "+invocation.Extension)
		return nil, &beanextension.DeliveryFailure{Code: beanextension.FailureContract}
	}
	result := results[0]
	p.results[invocation.Extension] = results[1:]
	if result.Error != "" {
		retryable := result.Error == beanextension.FailureUnavailable || result.Error == beanextension.FailureTimeout
		return nil, &beanextension.DeliveryFailure{Code: result.Error, CanRetry: retryable}
	}
	return result.Output, nil
}

func (p *testProvider) finish() {
	for _, name := range sortedKeys(p.results) {
		if count := len(p.results[name]); count > 0 {
			p.failures = append(p.failures, fmt.Sprintf("%d unused result(s) for %s", count, name))
		}
	}
}

func seededID(seed int64, index int) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("bean-semantic-test-seed/v1:%d:%d", seed, index)))
	digest[6] = digest[6]&0x0f | 0x40
	digest[8] = digest[8]&0x3f | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", digest[0:4], digest[4:6], digest[6:8], digest[8:10], digest[10:16])
}

func requestContext(context appir.TestContext) beanctx.Request {
	request := beanctx.Request{TenantID: context.Tenant, RequestID: context.RequestID}
	if context.Actor != nil {
		request.User = &beanctx.User{ID: context.Actor.ID, Email: context.Actor.Email, DisplayName: context.Actor.DisplayName, Roles: append([]string{}, context.Actor.Roles...)}
	}
	return request
}

type fixtureRecord struct {
	entity appir.Entity
	row    map[string]any
}

func insertFixtures(ctx context.Context, database dbal.Database, app *appir.App, test appir.TestCase) error {
	pending := []fixtureRecord{}
	all := map[string]bool{}
	for _, entityName := range sortedKeys(test.Fixtures) {
		entity := app.Entities[entityName]
		for _, row := range test.Fixtures[entityName] {
			pending = append(pending, fixtureRecord{entity: entity, row: row})
			for fieldName, value := range row {
				all[entityName+"/"+fieldName+"/"+fmt.Sprint(value)] = true
			}
		}
	}
	inserted := map[string]bool{}
	for len(pending) > 0 {
		progress := false
		remaining := pending[:0]
		for _, record := range pending {
			if !fixtureDependenciesReady(record, all, inserted) {
				remaining = append(remaining, record)
				continue
			}
			if err := insertFixture(ctx, database, record.entity, record.row, test.Context.Time); err != nil {
				return err
			}
			for fieldName, value := range record.row {
				inserted[record.entity.Name+"/"+fieldName+"/"+fmt.Sprint(value)] = true
			}
			progress = true
		}
		if !progress {
			return fmt.Errorf("fixture relation cycle cannot be inserted")
		}
		pending = remaining
	}
	return insertFixtureRelations(ctx, database, app, test)
}

func insertFixtureRelations(ctx context.Context, database dbal.Database, app *appir.App, test appir.TestCase) error {
	for _, entityName := range sortedKeys(test.Fixtures) {
		entity := app.Entities[entityName]
		for _, row := range test.Fixtures[entityName] {
			for _, item := range entity.Fields {
				if item.Type != "relation" || item.Relation == nil || item.Relation.Kind != "one-to-many" && item.Relation.Kind != "many-to-many" || row[item.Name] == nil {
					continue
				}
				values, _ := row[item.Name].([]any)
				for _, value := range values {
					_, err := database.Insert(ctx, dbal.Insert{Table: entity.Name + "_" + item.Name, Values: map[string]dbal.Value{entity.Name + "_id": row["id"], item.Relation.Entity + "_id": value}})
					if err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func fixtureDependenciesReady(record fixtureRecord, all, inserted map[string]bool) bool {
	for _, item := range record.entity.Fields {
		if item.Type != "relation" || item.Relation == nil || item.Relation.Kind == "one-to-many" || item.Relation.Kind == "many-to-many" {
			continue
		}
		raw := record.row[item.Name]
		if raw == nil {
			continue
		}
		value := fmt.Sprint(raw)
		targetField := item.Relation.TargetField
		if targetField == "" {
			targetField = "id"
		}
		key := item.Relation.Entity + "/" + targetField + "/" + value
		if value != "" && all[key] && !inserted[key] {
			return false
		}
	}
	return true
}

func insertFixture(ctx context.Context, database dbal.Database, entity appir.Entity, row map[string]any, fixedTime string) error {
	values := map[string]dbal.Value{"id": row["id"], "created_at": fixedTime, "updated_at": fixedTime, "version": 1}
	for _, name := range []string{"created_at", "updated_at", "version", "owner_id", "tenant_id", "deleted_at"} {
		if value, exists := row[name]; exists {
			values[name] = value
		}
	}
	for _, item := range entity.Fields {
		if item.Type == "relation" && item.Relation != nil && (item.Relation.Kind == "one-to-many" || item.Relation.Kind == "many-to-many") {
			continue
		}
		value, exists := row[item.Name]
		if !exists || value == nil {
			continue
		}
		encoded, err := field.Encode(item, value)
		if err != nil {
			return err
		}
		values[item.Name] = encoded
	}
	_, err := database.Insert(ctx, dbal.Insert{Table: entity.Name, Values: values})
	return err
}

func assertCase(parentCtx context.Context, database dbal.Database, app *appir.App, suite appir.TestSuite, test appir.TestCase, before map[string][]dbal.Row, result any, executionErr error, provider *testProvider) []definition.Diagnostic {
	ctx, cancel := context.WithTimeout(parentCtx, testsuite.CaseTimeoutSec*time.Second)
	defer cancel()
	out := []definition.Diagnostic{}
	base := "tests." + test.Name + ".expect"
	actualCode := errorCode(executionErr)
	resultPresent := len(bytes.TrimSpace(test.Expect.Result)) > 0
	if test.Expect.Error != "" {
		if actualCode != test.Expect.Error {
			out = append(out, assertionDiagnostic(suite.Name, base+".error", test.Expect.Error, actualCode))
		}
	} else if resultPresent {
		if executionErr != nil {
			out = append(out, assertionDiagnostic(suite.Name, base+".result", decodeExpected(test.Expect.Result), actualCode))
		} else if expected := decodeExpected(test.Expect.Result); !matchesExpected(expected, result) {
			out = append(out, assertionDiagnostic(suite.Name, base+".result", expected, result))
		}
	} else if executionErr != nil {
		out = append(out, assertionDiagnostic(suite.Name, base, "successful execution", actualCode))
	}
	if test.Expect.NoChanges {
		after, err := entitySnapshot(ctx, database, app)
		if err != nil || !reflect.DeepEqual(before, after) {
			out = append(out, assertionDiagnostic(suite.Name, base+".noChanges", true, false))
		}
	}
	for index, expected := range test.Expect.Changes {
		beforeRow, existed := snapshotRecord(before[expected.Entity], expected.ID)
		rows, err := semanticRows(ctx, database, app, expected.Entity, expected.ID)
		matched := err == nil
		if expected.Absent {
			matched = matched && existed && len(rows) == 0
		} else if matched && len(rows) == 1 && rowContains(app.Entities[expected.Entity], rows[0], expected.Values) {
			matched = !existed || assertedValuesChanged(beforeRow, rows[0], expected.Values)
		} else {
			matched = false
		}
		if !matched {
			out = append(out, assertionDiagnostic(suite.Name, fmt.Sprintf("%s.changes.%d", base, index), expected, rows))
		}
	}
	out = append(out, assertEvents(ctx, database, suite.Name, base, test.Expect)...)
	out = append(out, assertAudit(ctx, database, suite.Name, base, test.Expect.Audit)...)
	if len(provider.failures) > 0 {
		out = append(out, assertionDiagnostic(suite.Name, "tests."+test.Name+".providers", "all provider results consumed exactly once", provider.failures))
	}
	if (len(test.Expect.ProviderCalls) > 0 || len(provider.calls) > 0) && !equalJSON(test.Expect.ProviderCalls, provider.calls) {
		out = append(out, assertionDiagnostic(suite.Name, base+".providerCalls", test.Expect.ProviderCalls, provider.calls))
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func assertEvents(ctx context.Context, database dbal.Database, suiteName, base string, expectation appir.TestExpectation) []definition.Diagnostic {
	if len(expectation.Events) == 0 && !expectation.NoEvents {
		return nil
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "bean_outbox", Columns: []string{"topic", "payload"}})
	if err != nil {
		return []definition.Diagnostic{assertionDiagnostic(suiteName, base+".events", expectation.Events, "unavailable")}
	}
	actual := make([]appir.TestEvent, 0, len(rows))
	for _, row := range rows {
		if beanextension.IsTopic(fmt.Sprint(row["topic"])) {
			continue
		}
		payload := map[string]any{}
		_ = json.Unmarshal([]byte(fmt.Sprint(row["payload"])), &payload)
		actual = append(actual, appir.TestEvent{Topic: fmt.Sprint(row["topic"]), Payload: payload})
	}
	sort.Slice(actual, func(i, j int) bool { return canonicalString(actual[i]) < canonicalString(actual[j]) })
	expected := append([]appir.TestEvent{}, expectation.Events...)
	sort.Slice(expected, func(i, j int) bool { return canonicalString(expected[i]) < canonicalString(expected[j]) })
	if expectation.NoEvents && len(actual) != 0 {
		return []definition.Diagnostic{assertionDiagnostic(suiteName, base+".noEvents", true, false)}
	}
	if len(expectation.Events) > 0 && !equalJSON(expected, actual) {
		return []definition.Diagnostic{assertionDiagnostic(suiteName, base+".events", expected, actual)}
	}
	return nil
}

func assertAudit(ctx context.Context, database dbal.Database, suiteName, base string, expected []appir.TestAudit) []definition.Diagnostic {
	if len(expected) == 0 {
		return nil
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "bean_audit", Columns: []string{"action", "user_id", "tenant_id", "entity_type", "entity_id", "changed_fields", "success"}})
	if err != nil || len(rows) != len(expected) {
		return []definition.Diagnostic{assertionDiagnostic(suiteName, base+".audit", expected, rows)}
	}
	sort.Slice(rows, func(i, j int) bool { return auditRowKey(rows[i]) < auditRowKey(rows[j]) })
	expected = append([]appir.TestAudit{}, expected...)
	sort.Slice(expected, func(i, j int) bool { return auditExpectationKey(expected[i]) < auditExpectationKey(expected[j]) })
	for index := range expected {
		if !auditMatches(rows[index], expected[index]) {
			return []definition.Diagnostic{assertionDiagnostic(suiteName, base+".audit", expected, rows)}
		}
	}
	return nil
}

func auditRowKey(row dbal.Row) string {
	return fmt.Sprint(row["action"]) + "\x00" + fmt.Sprint(row["user_id"]) + "\x00" + fmt.Sprint(row["tenant_id"]) + "\x00" + fmt.Sprint(row["entity_type"]) + "\x00" + fmt.Sprint(row["entity_id"])
}

func auditExpectationKey(expected appir.TestAudit) string {
	return expected.Action + "\x00" + expected.ActorID + "\x00" + expected.TenantID + "\x00" + expected.Entity + "\x00" + expected.EntityID
}

func auditMatches(row dbal.Row, expected appir.TestAudit) bool {
	changed := []string{}
	_ = json.Unmarshal([]byte(fmt.Sprint(row["changed_fields"])), &changed)
	if expected.Action != "" && fmt.Sprint(row["action"]) != expected.Action ||
		expected.ActorID != "" && fmt.Sprint(row["user_id"]) != expected.ActorID ||
		expected.TenantID != "" && fmt.Sprint(row["tenant_id"]) != expected.TenantID ||
		expected.Entity != "" && fmt.Sprint(row["entity_type"]) != expected.Entity ||
		expected.EntityID != "" && fmt.Sprint(row["entity_id"]) != expected.EntityID ||
		len(expected.Changed) > 0 && !reflect.DeepEqual(changed, expected.Changed) {
		return false
	}
	if expected.Success != nil {
		actual := fmt.Sprint(row["success"]) == "1"
		return actual == *expected.Success
	}
	return true
}

func entitySnapshot(ctx context.Context, database dbal.Database, app *appir.App) (map[string][]dbal.Row, error) {
	out := map[string][]dbal.Row{}
	for _, entityName := range sortedKeys(app.Entities) {
		rows, err := semanticRows(ctx, database, app, entityName, "")
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			hydrateRow(app.Entities[entityName], row)
		}
		out[entityName] = rows
	}
	return out, nil
}

func semanticRows(ctx context.Context, database dbal.Database, app *appir.App, entityName, recordID string) ([]dbal.Row, error) {
	entity := app.Entities[entityName]
	fields := []string{"id"}
	for _, item := range entity.Fields {
		fields = append(fields, item.Name)
	}
	if entity.Owner {
		fields = append(fields, "owner_id")
	}
	if entity.Tenant {
		fields = append(fields, "tenant_id")
	}
	if entity.SoftDelete {
		fields = append(fields, "deleted_at")
	}
	fields = append(fields, "created_at", "updated_at", "version")
	const viewName = "semantic_test_read"
	const policyName = "semantic_test_runner_read"
	viewApp := *app
	viewApp.Views = make(map[string]appir.View, len(app.Views)+1)
	for name, item := range app.Views {
		viewApp.Views[name] = item
	}
	viewApp.Policies = make(map[string]appir.Policy, len(app.Policies)+1)
	for name, item := range app.Policies {
		viewApp.Policies[name] = item
	}
	viewApp.Policies[policyName] = appir.Policy{Name: policyName, ReadRoles: []string{"administrator"}}
	viewApp.Views[viewName] = appir.View{Name: viewName, Entity: entityName, Fields: fields, Policy: policyName, Sort: []appir.Sort{{Field: "id"}}, DefaultLimit: 200, MaxLimit: 200}
	request := beanctx.Request{User: &beanctx.User{ID: "semantic-test-runner", Roles: []string{"administrator"}}}
	rows := []dbal.Row{}
	for offset := 0; ; offset += 200 {
		result, err := view.ReadPage(ctx, database, &viewApp, viewName, view.ReadOptions{
			Params:         view.Params{RecordID: recordID, Limit: 200, Offset: offset},
			IncludeDeleted: true,
		}, request)
		if err != nil {
			return nil, err
		}
		rows = append(rows, result.Rows...)
		if recordID != "" || len(result.Rows) < 200 {
			return rows, nil
		}
	}
}

func rowContains(entity appir.Entity, row dbal.Row, expected map[string]any) bool {
	hydrateRow(entity, row)
	for name, value := range expected {
		if !equalJSON(value, row[name]) {
			return false
		}
	}
	return true
}

func snapshotRecord(rows []dbal.Row, id string) (dbal.Row, bool) {
	for _, row := range rows {
		if fmt.Sprint(row["id"]) == id {
			return row, true
		}
	}
	return nil, false
}

func assertedValuesChanged(before, after dbal.Row, expected map[string]any) bool {
	for name := range expected {
		if !equalJSON(before[name], after[name]) {
			return true
		}
	}
	return false
}

func hydrateRow(entity appir.Entity, row dbal.Row) {
	for _, item := range entity.Fields {
		if value, exists := row[item.Name]; exists {
			row[item.Name] = field.Decode(item, value)
		}
	}
}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	var ruleError *rule.Error
	if errors.As(err, &ruleError) {
		return ruleError.Code
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var databaseError *dbal.Error
	if errors.As(err, &databaseError) {
		return string(databaseError.Code)
	}
	return "runtime_error"
}

func assertionDiagnostic(suiteName, path string, expected, actual any) definition.Diagnostic {
	value := map[string]any{"expectedDigest": valueDigest(expected), "actualDigest": valueDigest(actual)}
	if code, ok := actual.(string); ok && stableErrorCode(code) {
		value["actualCode"] = code
	}
	if code, ok := expected.(string); ok && stableErrorCode(code) {
		value["expectedCode"] = code
	}
	return definition.Diagnostic{
		Code: "BEAN-T1001", Kind: "TestSuite", Name: suiteName, Path: path,
		Message: "semantic test assertion failed",
		Value:   value,
	}
}

func stableErrorCode(value string) bool {
	for _, code := range testsuite.ErrorCodes {
		if value == code {
			return true
		}
	}
	return false
}

func valueDigest(value any) string {
	encoded, _ := json.Marshal(value)
	digest := sha256.Sum256(encoded)
	return fmt.Sprintf("sha256:%x", digest[:8])
}

func decodeExpected(raw json.RawMessage) any {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	_ = decoder.Decode(&value)
	return value
}

func equalJSON(left, right any) bool { return canonicalString(left) == canonicalString(right) }

func matchesExpected(expected, actual any) bool {
	return matchNormalized(normalizedValue(expected), normalizedValue(actual))
}

func matchNormalized(expected, actual any) bool {
	expectedMap, expectedIsMap := expected.(map[string]any)
	actualMap, actualIsMap := actual.(map[string]any)
	if expectedIsMap && actualIsMap {
		for name, value := range expectedMap {
			actualValue, exists := actualMap[name]
			if !exists || !matchNormalized(value, actualValue) {
				return false
			}
		}
		return true
	}
	return equalJSON(expected, actual)
}

func normalizedValue(value any) any {
	encoded, _ := json.Marshal(value)
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.UseNumber()
	var normalized any
	_ = decoder.Decode(&normalized)
	return normalized
}

func canonicalString(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func sortedKeys[T any](values map[string]T) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
