package compiler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/beanruntime/bean/internal/actionop"
	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	blockcap "github.com/beanruntime/bean/internal/block"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	beanextension "github.com/beanruntime/bean/internal/extension"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/rule"
	"github.com/beanruntime/bean/internal/valuesource"
)

type Result struct {
	App         *appir.App
	Schema      migration.Schema
	Diagnostics []definition.Diagnostic
}
type actionSource struct {
	Entity, Operation, Policy, StateField string
	Lifecycle                             string
	DefaultRole, Confirm                  string
	Input                                 map[string]appir.Field
	Output                                map[string]appir.Field
	Steps                                 []stepSource
	Transitions                           map[string][]string
	When                                  string
	Derive                                map[string]string
}
type stepSource struct {
	Op, Result, Entity, View, Extension, StateField, Event, Job string
	Values                                                      map[string]any
	Where, Condition                                            *expr.Expr
}

func Compile(appID string, version int, defs []definition.Definition) Result {
	return compile(appID, version, defs, true)
}

// CompileRecovered validates independently decodable definitions without
// validating dependencies that may be absent from an unrecovered source file.
func CompileRecovered(appID string, version int, defs []definition.Definition) Result {
	return compile(appID, version, defs, false)
}

func compile(appID string, version int, defs []definition.Definition, validateGraph bool) (r Result) {
	defer func() {
		enrichDiagnosticCandidates(r.App, r.Diagnostics)
		definition.ClassifyDiagnostics(r.Diagnostics)
		definition.LocateDiagnostics(defs, r.Diagnostics)
	}()
	a := appir.Empty()
	a.AppID = appID
	a.Version = version
	r = Result{App: a}
	seen := map[string]bool{}
	for _, d := range defs {
		r.Diagnostics = append(r.Diagnostics, definition.ValidateEnvelope(d)...)
		key := d.Kind + "/" + d.Metadata.Name
		if seen[key] {
			r.Diagnostics = append(r.Diagnostics, duplicateDiagnostic(d.Kind, d.Metadata.Name, "metadata.name", "duplicate machine name"))
			continue
		}
		seen[key] = true
		registered, exists := definitionKindRegistry().Lookup(d.Kind)
		if !exists {
			r.Diagnostics = append(r.Diagnostics, definition.NewDiagnostic(definition.RuleUnsupportedKind, d.Kind, d.Metadata.Name, "kind", "unsupported definition kind"))
			continue
		}
		r.Diagnostics = append(r.Diagnostics, registered.Compile(a, d)...)
	}
	unavailable := unavailableDefinitions(r.Diagnostics)
	for name, entity := range a.Entities {
		generate(a, name, entity)
	}
	for _, kind := range []string{"View", "Action", "AdminResource", "Block", "TestSuite"} {
		registered, _ := definitionKindRegistry().Lookup(kind)
		registered.Normalize(a)
	}
	validationDiagnostics := suppressUnavailableDependencies(validate(a), unavailable)
	canonicalizeTestSuiteCases(a)
	if !validateGraph {
		validationDiagnostics = suppressMissingDependencies(validationDiagnostics)
	}
	r.Diagnostics = append(r.Diagnostics, validationDiagnostics...)
	if !validateGraph || len(r.Diagnostics) > 0 {
		return r
	}
	for _, e := range a.Entities {
		me := migration.Entity{Name: e.Name, Indexes: e.Indexes, Unique: e.Unique}
		for _, f := range e.Fields {
			mf := migration.Field{Name: f.Name, Type: f.Type, Required: f.Required, Unique: f.Unique}
			if f.Relation != nil {
				mf.RelationEntity, mf.RelationKind, mf.TargetField = f.Relation.Entity, f.Relation.Kind, f.Relation.TargetField
			}
			me.Fields = append(me.Fields, mf)
		}
		if e.Owner {
			me.Fields = append(me.Fields, migration.Field{Name: "owner_id", Type: "uuid"})
		}
		if e.Tenant {
			me.Fields = append(me.Fields, migration.Field{Name: "tenant_id", Type: "uuid"})
		}
		if e.SoftDelete {
			me.Fields = append(me.Fields, migration.Field{Name: "deleted_at", Type: "datetime"})
		}
		r.Schema.Entities = append(r.Schema.Entities, me)
	}
	sort.Slice(r.Schema.Entities, func(i, j int) bool { return r.Schema.Entities[i].Name < r.Schema.Entities[j].Name })
	return r
}

func enrichDiagnosticCandidates(app *appir.App, diagnostics []definition.Diagnostic) {
	if app == nil {
		return
	}
	for index := range diagnostics {
		if len(diagnostics[index].Candidates) > 0 {
			continue
		}
		facts := diagnostics[index].Facts
		if facts == nil {
			continue
		}
		if facts.UnknownField != "" {
			if properties, ok := SchemaProperties(DefinitionSchemas()[diagnostics[index].Kind]); ok {
				names := make([]string, 0, len(properties))
				for name := range properties {
					if name != "apiVersion" && name != "kind" && name != "name" && name != "namespace" {
						names = append(names, name)
					}
				}
				diagnostics[index].Candidates = closest(facts.UnknownField, names)
			}
		}
		if facts.MissingReference != nil {
			registered, exists := definitionKindRegistry().Lookup(facts.MissingReference.Kind)
			if exists && registered.ReferenceCandidates {
				diagnostics[index].Candidates = closest(facts.MissingReference.Name, registered.Names(app))
			}
		}
		if len(diagnostics[index].Candidates) == 0 && facts.MissingField != "" {
			diagnostics[index].Candidates = closest(facts.MissingField, fieldsForDiagnostic(app, diagnostics[index]))
		}
	}
}

func keys[T any](values map[string]T) []string {
	out := make([]string, 0, len(values))
	for name := range values {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func fieldsForDiagnostic(app *appir.App, diagnostic definition.Diagnostic) []string {
	entityName := ""
	registered, exists := definitionKindRegistry().Lookup(diagnostic.Kind)
	if exists && registered.FieldEntity != nil {
		entityName = registered.FieldEntity(app, diagnostic.Name)
	}
	return fieldsForEntity(app, entityName)
}

func fieldsForEntity(app *appir.App, entityName string) []string {
	names := []string{"created_at", "id", "updated_at", "version"}
	if entity, ok := app.Entities[entityName]; ok {
		for _, field := range entity.Fields {
			names = append(names, field.Name)
		}
	}
	sort.Strings(names)
	return names
}

func closest(wanted string, available []string) []string {
	type candidate struct {
		name     string
		distance int
	}
	ranked := make([]candidate, 0, len(available))
	for _, name := range available {
		ranked = append(ranked, candidate{name: name, distance: editDistance(wanted, name)})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].distance != ranked[j].distance {
			return ranked[i].distance < ranked[j].distance
		}
		return ranked[i].name < ranked[j].name
	})
	if len(ranked) > 3 {
		ranked = ranked[:3]
	}
	out := make([]string, len(ranked))
	for index := range ranked {
		out[index] = ranked[index].name
	}
	return out
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for leftIndex, leftRune := range []rune(left) {
		current := []int{leftIndex + 1}
		for rightIndex, rightRune := range []rune(right) {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current = append(current, min(current[rightIndex]+1, previous[rightIndex+1]+1, previous[rightIndex]+cost))
		}
		previous = current
	}
	return previous[len(previous)-1]
}
func diag(d definition.Definition, path, msg string) definition.Diagnostic {
	return definition.NewDiagnostic(diagnosticRule(d.Kind, path), d.Kind, d.Metadata.Name, path, msg)
}

func diagWithRule(d definition.Definition, rule definition.DiagnosticRule, path, msg string) definition.Diagnostic {
	return definition.NewDiagnostic(rule, d.Kind, d.Metadata.Name, path, msg)
}

func diagError(d definition.Definition, path string, err error) definition.Diagnostic {
	var unknown *definition.UnknownFieldError
	if errors.As(err, &unknown) {
		return definition.UnknownFieldDiagnostic(d.Kind, d.Metadata.Name, path, unknown.Field)
	}
	return diag(d, path, err.Error())
}

func unavailableDefinitions(diagnostics []definition.Diagnostic) map[string]bool {
	unavailable := map[string]bool{}
	for _, diagnostic := range diagnostics {
		if diagnostic.Path == "spec" {
			unavailable[diagnostic.Kind+"/"+diagnostic.Name] = true
		}
	}
	return unavailable
}

func suppressUnavailableDependencies(diagnostics []definition.Diagnostic, unavailable map[string]bool) []definition.Diagnostic {
	out := make([]definition.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		target, missing := missingDependency(diagnostic)
		if !missing || !unavailable[target] {
			out = append(out, diagnostic)
		}
	}
	return out
}

func suppressMissingDependencies(diagnostics []definition.Diagnostic) []definition.Diagnostic {
	out := make([]definition.Diagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		if _, missing := missingDependency(diagnostic); !missing {
			out = append(out, diagnostic)
		}
	}
	return out
}

func missingDependency(diagnostic definition.Diagnostic) (string, bool) {
	if diagnostic.Facts == nil || diagnostic.Facts.MissingReference == nil {
		return "", false
	}
	reference := diagnostic.Facts.MissingReference
	return reference.Kind + "/" + reference.Name, true
}

func compileBinding(value any) appir.ValueBinding {
	if text, ok := value.(string); ok && strings.HasPrefix(text, "$") {
		if text == "$now" {
			return appir.ValueBinding{Source: "now"}
		}
		if text == "$context.tenant" {
			return appir.ValueBinding{Source: "tenant"}
		}
		for _, source := range []string{"input", "result", "record", "context", "user"} {
			prefix := "$" + source + "."
			if strings.HasPrefix(text, prefix) {
				return appir.ValueBinding{Source: source, Path: strings.TrimPrefix(text, prefix)}
			}
		}
		return appir.ValueBinding{Source: "invalid", Path: text}
	}
	b, _ := json.Marshal(value)
	return appir.ValueBinding{Source: "literal", Literal: b}
}
func normalizeActions(a *appir.App) {
	for name, action := range a.Actions {
		if action.Input == nil {
			action.Input = map[string]appir.Field{}
		}
		if action.Output == nil {
			action.Output = map[string]appir.Field{}
		}
		if action.Operation == "register_local_user" {
			action.Entity = ""
			action.Input = map[string]appir.Field{
				"display_name":          {Name: "display_name", Type: "string", Required: true},
				"email":                 {Name: "email", Type: "email", Required: true},
				"password":              {Name: "password", Type: "password", Required: true, Sensitive: true},
				"password_confirmation": {Name: "password_confirmation", Type: "password", Required: true, Sensitive: true},
			}
			action.Output = map[string]appir.Field{
				"id":           {Name: "id", Type: "uuid"},
				"display_name": {Name: "display_name", Type: "string"},
				"email":        {Name: "email", Type: "email"},
			}
			a.Actions[name] = action
			continue
		}
		entity, exists := a.Entities[action.Entity]
		if !exists || action.Operation == "transaction" {
			if exists {
				normalizeOutput(&action, entity)
			}
			a.Actions[name] = action
			continue
		}
		if action.Operation == "create" {
			for _, field := range entity.Fields {
				if lifecycle, ok := lifecycleForEntity(a, entity.Name); ok && lifecycle.Initial != "" && field.Name == lifecycle.StateField {
					field.Required = false
				}
				action.Input[field.Name] = field
			}
		} else {
			action.Input["id"] = appir.Field{Name: "id", Type: "uuid", Required: true}
			if action.Operation == "update" {
				for _, field := range entity.Fields {
					if lifecycle, ok := lifecycleForEntity(a, entity.Name); ok && field.Name == lifecycle.StateField {
						continue
					}
					field.Required = false
					action.Input[field.Name] = field
				}
			}
			if action.Operation == "transition" {
				stateField := actionStateField(a, action)
				for _, field := range entity.Fields {
					if field.Name == stateField {
						field.Required = true
						action.Input[field.Name] = field
					}
				}
			}
		}
		normalizeOutput(&action, entity)
		a.Actions[name] = action
	}
}
func normalizeOutput(action *appir.Action, entity appir.Entity) {
	action.Output["id"] = appir.Field{Name: "id", Type: "uuid"}
	if action.Operation == "delete" {
		return
	}
	for _, field := range entity.Fields {
		field.Required = false
		action.Output[field.Name] = field
	}
	if entity.Owner {
		action.Output["owner_id"] = appir.Field{Name: "owner_id", Type: "uuid"}
	}
	if entity.Tenant {
		action.Output["tenant_id"] = appir.Field{Name: "tenant_id", Type: "uuid"}
	}
	if entity.SoftDelete {
		action.Output["deleted_at"] = appir.Field{Name: "deleted_at", Type: "datetime"}
	}
	action.Output["created_at"] = appir.Field{Name: "created_at", Type: "datetime"}
	action.Output["updated_at"] = appir.Field{Name: "updated_at", Type: "datetime"}
	action.Output["version"] = appir.Field{Name: "version", Type: "integer"}
}
func lifecycleForEntity(app *appir.App, entity string) (appir.Lifecycle, bool) {
	for _, name := range keys(app.Lifecycles) {
		lifecycle := app.Lifecycles[name]
		if lifecycle.Entity == entity {
			return lifecycle, true
		}
	}
	return appir.Lifecycle{}, false
}
func actionStateField(app *appir.App, action appir.Action) string {
	if lifecycle, ok := app.Lifecycles[action.Lifecycle]; ok {
		return lifecycle.StateField
	}
	if action.StateField != "" {
		return action.StateField
	}
	return "status"
}
func validateActionTransitionSubset(name string, subset, canonical map[string][]string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for _, from := range keys(subset) {
		for index, target := range subset[from] {
			allowed := false
			for _, candidate := range canonical[from] {
				allowed = allowed || candidate == target
			}
			if !allowed {
				diagnostic := diagnostic("Action", name, fmt.Sprintf("spec.transitions.%s.%d", from, index), "transition edge is not declared by the selected Lifecycle")
				diagnostic.Code = "BEAN-E2201"
				out = append(out, diagnostic)
			}
		}
	}
	return out
}
func assignment(step appir.Step, name string) (appir.ValueBinding, bool) {
	for _, value := range step.Values {
		if value.Field == name {
			return value.Value, true
		}
	}
	return appir.ValueBinding{}, false
}
func diagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(diagnosticRule(kind, path), kind, name, path, message)
}

func diagnosticRule(kind, path string) definition.DiagnosticRule {
	switch {
	case kind == "Action":
		return definition.RuleAction
	case kind == "Lifecycle":
		return definition.RuleLifecycle
	case kind == "Policy" || strings.Contains(path, "policy"):
		return definition.RulePolicy
	case kind == "Block" && strings.Contains(path, "presentation"), kind == "View" && strings.Contains(path, "displays"):
		return definition.RulePresentation
	case strings.Contains(path, "binding"):
		return definition.RuleBinding
	case kind == "Page" && strings.Contains(path, "route"):
		return definition.RuleRoute
	case kind == "Release" || strings.Contains(path, "migration"):
		return definition.RuleMigration
	case kind == "Theme" || kind == "DemoSeed":
		return definition.RuleFixture
	case kind == "TestSuite":
		return definition.RuleTestSuite
	case kind == "Extension":
		return definition.RuleExtension
	case strings.Contains(strings.ToLower(path), "field"):
		return definition.RuleMissingField
	default:
		return definition.RuleGeneral
	}
}

func requiredDiagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleRequired, kind, name, path, message)
}

func duplicateDiagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleDuplicate, kind, name, path, message)
}

func invalidReferenceDiagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleInvalidReference, kind, name, path, message)
}

type validationError struct {
	rule    definition.DiagnosticRule
	message string
}

func (e *validationError) Error() string { return e.message }

func requiredValidationError(message string) error {
	return &validationError{rule: definition.RuleRequired, message: message}
}

func validationDiagnostic(kind, name, path string, err error) definition.Diagnostic {
	var classified *validationError
	if errors.As(err, &classified) {
		return definition.NewDiagnostic(classified.rule, kind, name, path, err.Error())
	}
	return diagnostic(kind, name, path, err.Error())
}

func missingReferenceDiagnostic(kind, name, path, targetKind, targetName string) definition.Diagnostic {
	return definition.MissingReferenceDiagnostic(kind, name, path, targetKind, targetName)
}

func missingFieldDiagnostic(kind, name, path, fieldName string, target bool) definition.Diagnostic {
	return definition.MissingFieldDiagnostic(kind, name, path, fieldName, target)
}

func withMissingReference(diagnostic definition.Diagnostic, targetKind, targetName string) definition.Diagnostic {
	if diagnostic.Facts == nil {
		diagnostic.Facts = &definition.DiagnosticFacts{Rule: definition.RuleMissingReference}
	}
	diagnostic.Facts.MissingReference = &definition.DiagnosticReference{Kind: targetKind, Name: targetName}
	return diagnostic
}

func withMissingField(diagnostic definition.Diagnostic, fieldName string) definition.Diagnostic {
	if diagnostic.Facts == nil {
		diagnostic.Facts = &definition.DiagnosticFacts{Rule: definition.RuleMissingField}
	}
	diagnostic.Facts.MissingField = fieldName
	return diagnostic
}

func lifecycleDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleLifecycle, "Lifecycle", name, path, message)
}

type validationState struct {
	routes map[string]string
}

func conflictingRoute(routes map[string]string, route string) string {
	for _, existing := range keys(routes) {
		if routesOverlap(existing, route) {
			return routes[existing]
		}
	}
	return ""
}

func routesOverlap(left, right string) bool {
	leftParts := strings.Split(strings.Trim(left, "/"), "/")
	rightParts := strings.Split(strings.Trim(right, "/"), "/")
	if len(leftParts) != len(rightParts) {
		return false
	}
	for index := range leftParts {
		if leftParts[index] != rightParts[index] && !strings.HasPrefix(leftParts[index], ":") && !strings.HasPrefix(rightParts[index], ":") {
			return false
		}
	}
	return true
}

func validate(a *appir.App) []definition.Diagnostic {
	state := &validationState{routes: map[string]string{}}
	out := []definition.Diagnostic{}
	for _, kind := range []string{"Theme", "DemoSeed", "Filter", "Page", "View", "Entity", "Lifecycle", "Rule", "Extension", "Action", "TestSuite", "Webform", "Policy", "Block", "LocalRegistration", "Panel", "Job", "Menu", "AdminResource", "Role"} {
		registered, _ := definitionKindRegistry().Lookup(kind)
		out = append(out, registered.Validate(a, state)...)
	}
	return out
}

func validateExtensions(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for _, name := range keys(a.Extensions) {
		item := a.Extensions[name]
		values := []struct {
			path    string
			actual  string
			allowed []string
		}{
			{"spec.transport", item.Transport, beanextension.Transports()},
			{"spec.authentication", item.Authentication, beanextension.Authentications()},
			{"spec.idempotency", item.Idempotency, beanextension.IdempotencyModes()},
			{"spec.transaction", item.Transaction, beanextension.TransactionModes()},
			{"spec.failure", item.Failure, beanextension.FailureModes()},
		}
		for _, value := range values {
			if !nameSet(value.allowed)[value.actual] {
				out = append(out, extensionDiagnostic(name, value.path, "has no supported Extension value"))
			}
		}
		if !validExtensionEndpoint(item.Endpoint) {
			out = append(out, extensionDiagnostic(name, "spec.endpoint", "must be an absolute HTTPS URL or loopback HTTP URL without credentials, query, or fragment"))
		}
		if !sameStrings(item.Permissions, beanextension.Permissions()) {
			out = append(out, extensionDiagnostic(name, "spec.permissions", "must declare exactly the supported network permission"))
		}
		if !sameStrings(item.SideEffects, beanextension.SideEffects()) {
			out = append(out, extensionDiagnostic(name, "spec.sideEffects", "must declare exactly the supported external-write side effect"))
		}
		if item.TimeoutSeconds < beanextension.MinTimeoutSeconds || item.TimeoutSeconds > beanextension.MaxTimeoutSeconds {
			out = append(out, extensionDiagnostic(name, "spec.timeoutSeconds", fmt.Sprintf("must be between %d and %d", beanextension.MinTimeoutSeconds, beanextension.MaxTimeoutSeconds)))
		}
		if item.Retry.MaxAttempts < beanextension.MinAttempts || item.Retry.MaxAttempts > beanextension.MaxAttempts {
			out = append(out, extensionDiagnostic(name, "spec.retry.maxAttempts", fmt.Sprintf("must be between %d and %d", beanextension.MinAttempts, beanextension.MaxAttempts)))
		}
		if item.Retry.DelaySeconds < beanextension.MinDelaySeconds || item.Retry.DelaySeconds > beanextension.MaxDelaySeconds {
			out = append(out, extensionDiagnostic(name, "spec.retry.delaySeconds", fmt.Sprintf("must be between %d and %d", beanextension.MinDelaySeconds, beanextension.MaxDelaySeconds)))
		}
		out = append(out, validateExtensionFields(name, "input", item.Input)...)
		out = append(out, validateExtensionFields(name, "output", item.Output)...)
	}
	return out
}

func validateExtensionFields(name, group string, fields map[string]appir.Field) []definition.Diagnostic {
	if len(fields) == 0 {
		return []definition.Diagnostic{extensionDiagnostic(name, "spec."+group, "requires at least one typed field")}
	}
	out := []definition.Diagnostic{}
	allowedTypes := nameSet([]string{"boolean", "date", "datetime", "decimal", "email", "enum", "integer", "json", "money", "slug", "string", "text", "url", "uuid"})
	for _, fieldName := range keys(fields) {
		item := fields[fieldName]
		path := "spec." + group + "." + fieldName
		if !testCaseName.MatchString(fieldName) || item.Name != fieldName {
			out = append(out, extensionDiagnostic(name, path+".name", "must match its machine-name key"))
		}
		if !allowedTypes[item.Type] {
			out = append(out, extensionDiagnostic(name, path+".type", "has no portable Extension field type"))
		}
		if item.Sensitive || item.Unique || item.Relation != nil {
			out = append(out, extensionDiagnostic(name, path, "cannot declare sensitive, unique, or relation storage semantics"))
		}
		if item.Type == "enum" && len(item.Options) == 0 {
			out = append(out, extensionDiagnostic(name, path+".options", "enum requires at least one option"))
		}
		if item.Type != "enum" && len(item.Options) > 0 {
			out = append(out, extensionDiagnostic(name, path+".options", "options are only valid for enum fields"))
		}
	}
	return out
}

func validExtensionEndpoint(raw string) bool {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil || parsed.Host == "" || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	if port := parsed.Port(); port != "" {
		number, portErr := strconv.Atoi(port)
		if portErr != nil || number < 1 || number > 65535 {
			return false
		}
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func sameStrings(left, right []string) bool {
	return strings.Join(left, "\x00") == strings.Join(right, "\x00")
}

func extensionDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleExtension, "Extension", name, path, message)
}

func actionExtensionDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleExtension, "Action", name, path, message)
}

func validateTheme(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if a.Theme != nil {
		if !nameSet(themePresetNames())[a.Theme.Preset] {
			out = append(out, diagnostic("Theme", a.Theme.Name, "spec.preset", "has no registered theme preset"))
		}
		if !nameSet(themeAccentNames())[a.Theme.Accent] {
			out = append(out, diagnostic("Theme", a.Theme.Name, "spec.accent", "has no registered theme accent"))
		}
	}
	return out
}

func validateDemoSeed(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if a.DemoSeed != nil {
		total := 0
		profiles := nameSet(demoSeedProfileNames())
		for entityName, seed := range a.DemoSeed.Entities {
			path := "spec.entities." + entityName
			entity, exists := a.Entities[entityName]
			if !exists {
				out = append(out, missingReferenceDiagnostic("DemoSeed", a.DemoSeed.Name, path, "Entity", entityName))
				continue
			}
			if seed.Count < 1 || seed.Count > 200 {
				out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path+".count", "must be between 1 and 200"))
			}
			profile := seed.Profile
			if profile == "" {
				profile = "auto"
				seed.Profile = profile
				a.DemoSeed.Entities[entityName] = seed
			}
			if !profiles[profile] {
				out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path+".profile", "has no registered demo seed profile"))
			}
			for _, field := range entity.Fields {
				if field.Required && (field.Type == "file" || field.Type == "password" || field.Sensitive) {
					out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path, "cannot generate required sensitive, password, or file field "+field.Name))
				}
				if field.Relation != nil {
					_, seeded := a.DemoSeed.Entities[field.Relation.Entity]
					if field.Required && !seeded {
						out = append(out, requiredDiagnostic("DemoSeed", a.DemoSeed.Name, path, "requires seeded relation Entity "+field.Relation.Entity))
					} else if seeded && field.Relation.TargetField != "id" {
						targetEntity := a.Entities[field.Relation.Entity]
						target, exists := entityFieldDefinition(targetEntity, field.Relation.TargetField)
						if exists && (target.Sensitive || target.Type == "password" || target.Type == "file") {
							out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path, "cannot generate relation target field "+field.Relation.Entity+"."+field.Relation.TargetField))
						} else if exists && target.Type != "uuid" {
							out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path, "relation target field "+field.Relation.Entity+"."+field.Relation.TargetField+" must be uuid"))
						} else if !exists && fieldSet(targetEntity)[field.Relation.TargetField] {
							out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path, "cannot generate relation target system field "+field.Relation.Entity+"."+field.Relation.TargetField))
						}
					}
				}
				if field.Unique && field.Type == "boolean" && seed.Count > 2 {
					out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path+".count", "exceeds the two unique boolean values available to field "+field.Name))
				}
				if field.Unique && field.Type == "enum" && seed.Count > len(field.Options) {
					out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, path+".count", "exceeds the unique enum options available to field "+field.Name))
				}
			}
			total += seed.Count
		}
		if len(a.DemoSeed.Entities) == 0 {
			out = append(out, requiredDiagnostic("DemoSeed", a.DemoSeed.Name, "spec.entities", "requires at least one seeded Entity"))
		}
		if total > 1000 {
			out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, "spec.entities", "cannot generate more than 1000 records"))
		}
		if demoSeedRequiredRelationCycle(a) {
			out = append(out, diagnostic("DemoSeed", a.DemoSeed.Name, "spec.entities", "required seeded relations contain a cycle"))
		}
	}
	return out
}

func validateFilters(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, filterDefinition := range a.Filters {
		if len(filterDefinition.Steps) == 0 {
			out = append(out, requiredDiagnostic("Filter", name, "spec.steps", "requires at least one filter step"))
		}
		for i, step := range filterDefinition.Steps {
			if step.Type != "markdown" {
				out = append(out, diagnostic("Filter", name, fmt.Sprintf("spec.steps.%d.type", i), "has no registered filter implementation"))
			}
		}
	}
	return out
}

func validateViews(a *appir.App, state *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	routes := state.routes
	for _, name := range keys(a.Views) {
		v := a.Views[name]
		e, ok := a.Entities[v.Entity]
		if !ok {
			out = append(out, missingReferenceDiagnostic("View", name, "spec.entity", "Entity", v.Entity))
			continue
		}
		fields := fieldSet(e)
		if len(v.Fields) == 0 && len(v.Aggregates) == 0 {
			out = append(out, diagnostic("View", name, "spec.fields", "must select fields or aggregates"))
		}
		relationships := map[string]appir.ViewRelationship{}
		for i, relationship := range v.Relationships {
			path := fmt.Sprintf("spec.relationships.%d", i)
			if relationship.RelationField != "" {
				resolved, found := resolveViewRelationship(e, relationship)
				if !found {
					out = append(out, invalidReferenceDiagnostic("View", name, path+".relationField", "references a field without relation storage"))
					continue
				}
				relationship = resolved
				v.Relationships[i] = relationship
				a.Views[name] = v
			}
			if relationship.Name == "" {
				out = append(out, requiredDiagnostic("View", name, path+".name", "is required"))
			}
			if relationships[relationship.Name].Name != "" {
				out = append(out, duplicateDiagnostic("View", name, path+".name", "duplicates another relationship"))
			}
			relationships[relationship.Name] = relationship
			target, exists := a.Entities[relationship.Entity]
			if !exists {
				out = append(out, missingReferenceDiagnostic("View", name, path+".entity", "Entity", relationship.Entity))
				continue
			}
			if !fields[relationship.LocalField] {
				out = append(out, missingFieldDiagnostic("View", name, path+".localField", relationship.LocalField, false))
			}
			if !fieldSet(target)[relationship.TargetField] {
				out = append(out, missingFieldDiagnostic("View", name, path+".targetField", relationship.TargetField, true))
			}
			if relationship.Type != "inner" && relationship.Type != "left" {
				out = append(out, diagnostic("View", name, path+".type", "must be inner or left"))
			}
		}
		for _, f := range v.Fields {
			if !validViewField(f, fields, relationships, a) {
				out = append(out, missingFieldDiagnostic("View", name, "spec.fields", f, false))
			}
		}
		selected := map[string]bool{}
		for _, field := range v.Fields {
			selected[field] = true
		}
		for fieldName, filterName := range v.FieldFilters {
			path := "spec.fieldFilters." + fieldName
			if !selected[fieldName] {
				out = append(out, invalidReferenceDiagnostic("View", name, path, "must reference a selected View field"))
				continue
			}
			fieldType, exists := viewFieldType(fieldName, e, relationships, a)
			if !exists || !map[string]bool{"string": true, "text": true, "richtext": true}[fieldType] {
				out = append(out, diagnostic("View", name, path, "can only filter textual fields"))
			}
			if _, exists = a.Filters[filterName]; !exists {
				out = append(out, missingReferenceDiagnostic("View", name, path, "Filter", filterName))
			}
		}
		if len(v.GroupBy) == 0 && len(v.Aggregates) == 0 && !selected["id"] {
			out = append(out, requiredDiagnostic("View", name, "spec.fields", "must include id for deterministic cursor pagination"))
		}
		for _, group := range v.GroupBy {
			if !validViewField(group, fields, relationships, a) {
				out = append(out, missingFieldDiagnostic("View", name, "spec.groupBy", group, false))
			}
		}
		aliases := map[string]bool{}
		for i, aggregate := range v.Aggregates {
			path := fmt.Sprintf("spec.aggregates.%d", i)
			if !map[string]bool{"count": true, "sum": true, "min": true, "max": true, "average": true, "avg": true}[strings.ToLower(aggregate.Function)] {
				out = append(out, diagnostic("View", name, path+".function", "has no query-plan implementation"))
			}
			if !validViewField(aggregate.Field, fields, relationships, a) {
				out = append(out, missingFieldDiagnostic("View", name, path+".field", aggregate.Field, false))
			}
			if aggregate.Alias == "" || aliases[aggregate.Alias] {
				out = append(out, diagnostic("View", name, path+".alias", "must be a unique machine name"))
			}
			aliases[aggregate.Alias] = true
		}
		for i, order := range v.Sort {
			if !validViewField(order.Field, fields, relationships, a) && !aliases[order.Field] {
				out = append(out, missingFieldDiagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), order.Field, false))
			} else if !selected[order.Field] && !aliases[order.Field] {
				out = append(out, diagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), "must be selected so cursor state is stable"))
			}
		}
		if len(v.Aggregates) > 0 && len(v.Sort) == 0 {
			for _, group := range v.GroupBy {
				v.Sort = append(v.Sort, appir.Sort{Field: group})
			}
			a.Views[name] = v
		}
		for key, exposed := range v.ExposedFilters {
			fieldName := exposed.Target(key)
			if !validViewField(fieldName, fields, relationships, a) {
				out = append(out, missingFieldDiagnostic("View", name, "spec.exposedFilters."+key, fieldName, false))
				continue
			}
			fieldType, _ := viewFieldType(fieldName, e, relationships, a)
			allowed := map[string]bool{"eq": true}
			if map[string]bool{"email": true, "richtext": true, "slug": true, "string": true, "text": true, "url": true}[fieldType] {
				allowed["contains"] = true
			}
			if map[string]bool{"date": true, "datetime": true, "decimal": true, "integer": true, "money": true}[fieldType] {
				allowed["gte"], allowed["lte"] = true, true
			}
			if !allowed[exposed.Operator] {
				out = append(out, diagnostic("View", name, "spec.exposedFilters."+key+".operator", "is incompatible with field type "+fieldType))
			}
		}
		for path, expression := range map[string]*expr.Expr{"spec.filter": v.Filter, "spec.contextFilter": v.ContextFilter} {
			if expression != nil {
				if er := validateExpr(*expression, true); er != nil {
					out = append(out, validationDiagnostic("View", name, path, er))
				}
			}
		}
		if v.DefaultLimit < 1 || v.MaxLimit < 1 || v.MaxLimit > 200 || v.DefaultLimit > v.MaxLimit {
			out = append(out, diagnostic("View", name, "spec.maxLimit", "must be between the default and 200"))
		}
		policyName := policy.EffectiveViewPolicyName(v, e)
		policyDefinition, policyExists := a.Policies[policyName]
		if v.Policy != "" && !policyExists {
			out = append(out, missingReferenceDiagnostic("View", name, "spec.policy", "Policy", v.Policy))
		}
		if policyName != "" && policyExists {
			redacted := nameSet(policyDefinition.Redact)
			for i, order := range v.Sort {
				if redacted[order.Field] {
					out = append(out, diagnostic("View", name, fmt.Sprintf("spec.sort.%d.field", i), "redacted fields cannot control ordering"))
				}
			}
			for key, exposed := range v.ExposedFilters {
				if redacted[exposed.Target(key)] {
					out = append(out, diagnostic("View", name, "spec.exposedFilters."+key, "redacted fields cannot be exposed as filters"))
				}
			}
			for _, expression := range []*expr.Expr{v.Filter, v.ContextFilter} {
				for _, field := range recordFields(expression) {
					if redacted[field] {
						out = append(out, diagnostic("View", name, "spec.filter", "redacted fields cannot control filtering"))
					}
				}
			}
		}
		for _, displayName := range keys(v.Displays) {
			display := v.Displays[displayName]
			if strings.HasPrefix(displayName, "_") {
				continue
			}
			if nameSet(displaySerializerNames())[display.Type] {
				if display.Route == "" {
					continue
				}
				if old := conflictingRoute(routes, display.Route); old != "" {
					out = append(out, duplicateDiagnostic("View", name, "spec.displays."+displayName+".route", "overlaps route used by "+old))
					continue
				}
				routes[display.Route] = "View/" + name
				continue
			}
			if display.Type != "page" && display.Type != "block" {
				out = append(out, diagnostic("View", name, "spec.displays."+displayName+".type", "has no registered display type"))
				continue
			}
			out = append(out, validateViewDisplay(name, displayName, v, display, e, relationships, a)...)
			if display.Type == "page" {
				if display.Route == "" {
					out = append(out, requiredDiagnostic("View", name, "spec.displays."+displayName+".route", "page display route is required"))
				} else if old := conflictingRoute(routes, display.Route); old != "" {
					out = append(out, duplicateDiagnostic("View", name, "spec.displays."+displayName+".route", "overlaps route used by "+old))
				} else {
					routes[display.Route] = "View/" + name
				}
			} else if display.Route != "" {
				out = append(out, diagnostic("View", name, "spec.displays."+displayName+".route", "block display cannot declare a route"))
			}
		}
	}
	return out
}

func validateViewDisplay(viewName, displayName string, view appir.View, display appir.Display, entity appir.Entity, relationships map[string]appir.ViewRelationship, app *appir.App) []definition.Diagnostic {
	base := "spec.displays." + displayName
	out := []definition.Diagnostic{}
	selected := nameSet(view.Fields)
	redacted := nameSet(app.Policies[policy.EffectiveViewPolicyName(view, entity)].Redact)
	renderer := display.Renderer
	if !nameSet(viewRendererNames())[renderer.Type] {
		out = append(out, diagnostic("View", viewName, base+".renderer.type", "has no registered renderer"))
	} else if renderer.Type == "table" {
		if len(renderer.Fields) == 0 {
			out = append(out, requiredDiagnostic("View", viewName, base+".renderer.fields", "table requires at least one field"))
		}
		for index, column := range renderer.Fields {
			path := fmt.Sprintf("%s.renderer.fields.%d", base, index)
			if !selected[column.Field] {
				out = append(out, diagnostic("View", viewName, path+".field", "must be selected by View "+viewName))
			} else if redacted[column.Field] {
				out = append(out, diagnostic("View", viewName, path+".field", "must not be redacted by View policy"))
			}
			if column.LinkRoute != "" && (!strings.HasPrefix(column.LinkRoute, "/") || strings.HasPrefix(column.LinkRoute, "//")) {
				out = append(out, diagnostic("View", viewName, path+".linkRoute", "must be an absolute application route"))
			}
			for _, match := range regexp.MustCompile(`:([a-zA-Z0-9_.]+)`).FindAllStringSubmatch(column.LinkRoute, -1) {
				fieldName := match[1]
				if !selected[fieldName] || redacted[fieldName] {
					out = append(out, diagnostic("View", viewName, path+".linkRoute", "route field "+fieldName+" must be selected and visible"))
				}
			}
		}
	} else {
		legacy := validatePresentation(viewName, appir.Block{View: viewName, Presentation: renderer.Presentation()}, app)
		for _, item := range legacy {
			item.Kind = "View"
			item.Path = strings.Replace(item.Path, "spec.presentation", base+".renderer", 1)
			out = append(out, item)
		}
	}
	if display.Title.Text != "" && display.Title.Field != "" {
		out = append(out, diagnostic("View", viewName, base+".title", "must use text or a result field, not both"))
	}
	if display.Title.Field != "" {
		if renderer.Type != "detail" {
			out = append(out, diagnostic("View", viewName, base+".title.field", "result title requires a detail renderer"))
		}
		if !selected[display.Title.Field] || redacted[display.Title.Field] {
			out = append(out, diagnostic("View", viewName, base+".title.field", "must reference a selected, visible field"))
		}
		if display.Title.Fallback == "" {
			out = append(out, requiredDiagnostic("View", viewName, base+".title.fallback", "is required for a result title"))
		}
		if display.Type == "page" {
			singleRecord := false
			for filterName, binding := range display.Bindings {
				filter, exists := view.ExposedFilters[filterName]
				target := filter.Target(filterName)
				definition, fieldExists := entityFieldDefinition(entity, target)
				singleRecord = singleRecord || exists && binding.Required && (target == "id" || fieldExists && definition.Unique)
			}
			if !singleRecord {
				out = append(out, diagnostic("View", viewName, base+".title.field", "result title requires a unique bound filter"))
			}
		}
	}
	if display.Type == "block" && len(display.Bindings) > 0 {
		out = append(out, diagnostic("View", viewName, base+".bindings", "block display bindings must be declared by the mounting Block"))
	}
	bound := map[string]bool{}
	for filterName, binding := range display.Bindings {
		bound[filterName] = true
		if _, exists := view.ExposedFilters[filterName]; !exists {
			out = append(out, invalidReferenceDiagnostic("View", viewName, base+".bindings."+filterName, "has no matching exposed filter"))
		}
		if !valuesource.Allows(valuesource.Page, binding.Source) {
			out = append(out, diagnostic("View", viewName, base+".bindings."+filterName+".source", "has no typed resolver"))
		} else if binding.Source == valuesource.Query {
			out = append(out, diagnostic("View", viewName, base+".bindings."+filterName+".source", "query values are not immutable display bindings"))
		}
		if binding.Source != "tenant" && binding.Name == "" {
			out = append(out, requiredDiagnostic("View", viewName, base+".bindings."+filterName+".name", "is required"))
		}
	}
	seenControls := map[string]bool{}
	widgets := map[string]bool{"auto": true, "text": true, "select": true, "checkbox": true, "number": true, "date": true}
	for index, control := range display.Controls {
		path := fmt.Sprintf("%s.controls.%d", base, index)
		filter, exists := view.ExposedFilters[control.Filter]
		if !exists {
			out = append(out, invalidReferenceDiagnostic("View", viewName, path+".filter", "has no matching exposed filter"))
			continue
		}
		if seenControls[control.Filter] {
			out = append(out, duplicateDiagnostic("View", viewName, path+".filter", "duplicates another display control"))
		}
		seenControls[control.Filter] = true
		if bound[control.Filter] {
			out = append(out, diagnostic("View", viewName, path+".filter", "cannot expose an immutable bound input"))
		}
		if !widgets[control.Widget] {
			out = append(out, diagnostic("View", viewName, path+".widget", "has no registered control widget"))
		}
		definition := filter.Definition(control.Filter)
		compatible := map[string]bool{"auto": true}
		switch definition.Type {
		case "boolean":
			compatible["checkbox"] = true
		case "enum":
			compatible["select"] = true
		case "integer", "decimal", "money":
			compatible["number"] = true
		case "date", "datetime":
			compatible["date"] = true
		case "email", "richtext", "slug", "string", "text", "url", "uuid":
			compatible["text"] = true
		}
		if widgets[control.Widget] && !compatible[control.Widget] {
			out = append(out, diagnostic("View", viewName, path+".widget", "is incompatible with field type "+definition.Type))
		}
		if control.Default != nil {
			if err := field.Validate(definition, control.Default); err != nil {
				out = append(out, diagnostic("View", viewName, path+".default", err.Error()))
			}
		}
	}
	if display.Pager.Type != "none" && display.Pager.Type != "cursor" {
		out = append(out, diagnostic("View", viewName, base+".pager.type", "has no registered pager"))
	}
	if display.Pager.PageSize < 1 || display.Pager.PageSize > view.MaxLimit {
		out = append(out, diagnostic("View", viewName, base+".pager.pageSize", "must be between 1 and the View maximum"))
	}
	return out
}

func validateEntities(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	allowedRelations := map[string]bool{"one-to-one": true, "one-to-many": true, "many-to-one": true, "many-to-many": true}
	for name, entity := range a.Entities {
		for _, validationName := range keys(entity.Validations) {
			path := "spec.validations." + validationName
			item, exists := a.Rules[entity.Validations[validationName]]
			if !exists {
				out = append(out, missingReferenceDiagnostic("Entity", name, path, "Rule", entity.Validations[validationName]))
				continue
			}
			if item.Entity != name {
				out = append(out, ruleConsumerDiagnostic("Entity", name, path, "validation Rule entity does not match Entity"))
			}
			if item.Result != rule.Boolean {
				out = append(out, ruleConsumerDiagnostic("Entity", name, path, "validation Rule must return boolean"))
			}
			if len(item.Input) > 0 {
				out = append(out, ruleConsumerDiagnostic("Entity", name, path, "validation Rule cannot declare Action inputs"))
			}
		}
		if entity.Policy != "" {
			if _, ok := a.Policies[entity.Policy]; !ok {
				out = append(out, missingReferenceDiagnostic("Entity", name, "spec.policy", "Policy", entity.Policy))
			}
		}
		for i, field := range entity.Fields {
			if field.Type != "relation" {
				continue
			}
			if field.Relation == nil {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation", i), "relation storage is required"))
				continue
			}
			if !allowedRelations[field.Relation.Kind] {
				out = append(out, diagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation", i), "relation kind is invalid"))
				continue
			}
			if _, ok := a.Entities[field.Relation.Entity]; !ok {
				out = append(out, missingReferenceDiagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.entity", i), "Entity", field.Relation.Entity))
			}
			if field.Relation.TargetField == "" {
				out = append(out, requiredDiagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.targetField", i), "is required"))
			} else if target, ok := a.Entities[field.Relation.Entity]; ok && !fieldSet(target)[field.Relation.TargetField] {
				out = append(out, missingFieldDiagnostic("Entity", name, fmt.Sprintf("spec.fields.%d.relation.targetField", i), field.Relation.TargetField, true))
			}
		}
	}
	return out
}

func validateLifecycles(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	lifecycleEntities := map[string]string{}
	for _, name := range keys(a.Lifecycles) {
		lifecycle := a.Lifecycles[name]
		entity, entityExists := a.Entities[lifecycle.Entity]
		if !entityExists {
			out = append(out, withMissingReference(lifecycleDiagnostic(name, "spec.entity", "references missing Entity "+lifecycle.Entity), "Entity", lifecycle.Entity))
			continue
		}
		if existing := lifecycleEntities[lifecycle.Entity]; existing != "" {
			out = append(out, lifecycleDiagnostic(name, "spec.entity", "duplicates Lifecycle "+existing+" for Entity "+lifecycle.Entity))
		} else {
			lifecycleEntities[lifecycle.Entity] = name
		}
		state, stateExists := entityFieldDefinition(entity, lifecycle.StateField)
		if !stateExists {
			diagnostic := withMissingField(lifecycleDiagnostic(name, "spec.stateField", "references missing field "+lifecycle.StateField), lifecycle.StateField)
			diagnostic.Candidates = closest(lifecycle.StateField, fieldsForEntity(a, lifecycle.Entity))
			out = append(out, diagnostic)
			continue
		}
		if state.Type != "enum" {
			out = append(out, lifecycleDiagnostic(name, "spec.stateField", "Lifecycle state field must be an enum"))
			continue
		}
		options := append([]string{}, state.Options...)
		sort.Strings(options)
		allowed := nameSet(options)
		graphValid := allowed[lifecycle.Initial]
		if lifecycle.Initial == "" {
			out = append(out, lifecycleDiagnostic(name, "spec.initial", "is required"))
		} else if !allowed[lifecycle.Initial] {
			diagnostic := lifecycleDiagnostic(name, "spec.initial", "initial state is not an option of "+lifecycle.StateField)
			diagnostic.Candidates = options
			out = append(out, diagnostic)
		}
		for _, from := range keys(lifecycle.Transitions) {
			if !allowed[from] {
				graphValid = false
				diagnostic := lifecycleDiagnostic(name, "spec.transitions."+from, "transition source is not an option of "+lifecycle.StateField)
				diagnostic.Candidates = options
				out = append(out, diagnostic)
			}
			seenTargets := map[string]bool{}
			for index, target := range lifecycle.Transitions[from] {
				path := fmt.Sprintf("spec.transitions.%s.%d", from, index)
				if !allowed[target] {
					graphValid = false
					diagnostic := lifecycleDiagnostic(name, path, "transition target is not an option of "+lifecycle.StateField)
					diagnostic.Candidates = options
					out = append(out, diagnostic)
				} else if seenTargets[target] {
					out = append(out, lifecycleDiagnostic(name, path, "duplicates transition edge "+from+" -> "+target))
				}
				seenTargets[target] = true
			}
		}
		if graphValid {
			reachable := map[string]bool{lifecycle.Initial: true}
			queue := []string{lifecycle.Initial}
			for len(queue) > 0 {
				from := queue[0]
				queue = queue[1:]
				for _, target := range lifecycle.Transitions[from] {
					if !reachable[target] {
						reachable[target] = true
						queue = append(queue, target)
					}
				}
			}
			for _, state := range options {
				if !reachable[state] {
					out = append(out, lifecycleDiagnostic(name, "spec.transitions", "state "+state+" is unreachable from initial state "+lifecycle.Initial))
				}
			}
		}
	}
	return out
}

func validateRules(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for _, name := range keys(a.Rules) {
		item := a.Rules[name]
		if item.Entity != "" {
			if _, exists := a.Entities[item.Entity]; !exists {
				out = append(out, missingReferenceDiagnostic("Rule", name, "spec.entity", "Entity", item.Entity))
				continue
			}
		}
		if !validRuleType(item.Result) {
			out = append(out, ruleDefinitionDiagnostic(name, "spec.result", "result must be a supported Rule type"))
			continue
		}
		inputTypes := map[string]rule.Type{}
		for _, inputName := range keys(item.Input) {
			input := item.Input[inputName]
			path := "spec.input." + inputName
			if input.Name != inputName {
				out = append(out, ruleDefinitionDiagnostic(name, path+".name", "must match its input key"))
			}
			inputType, supported := rule.TypeForField(input.Type)
			if !supported || input.Sensitive {
				out = append(out, ruleDefinitionDiagnostic(name, path+".type", "Rule inputs must use a non-sensitive scalar field type"))
				continue
			}
			inputTypes[inputName] = inputType
		}
		thisTypes := map[string]rule.Type{}
		if entity, exists := a.Entities[item.Entity]; exists {
			thisTypes = ruleTypesForEntity(entity)
		}
		inferred, err := rule.Check(item.Expression, rule.TypeEnvironment{This: thisTypes, Input: inputTypes})
		if err != nil {
			var expressionError *rule.Error
			path := "spec.expression"
			if errors.As(err, &expressionError) {
				path = "spec." + expressionError.Path
			}
			out = append(out, ruleDefinitionDiagnostic(name, path, err.Error()))
			continue
		}
		if item.Result == rule.Boolean && rule.StaticallyNullable(item.Expression) {
			out = append(out, ruleDefinitionDiagnostic(name, "spec.result", "boolean Rule expression may return null"))
			continue
		}
		if !rule.ResultCompatible(item.Result, inferred) {
			out = append(out, ruleDefinitionDiagnostic(name, "spec.result", fmt.Sprintf("declares %s but expression returns %s", item.Result, inferred)))
		}
	}
	return out
}

func validRuleType(value rule.Type) bool {
	for _, candidate := range []rule.Type{rule.Boolean, rule.Integer, rule.Number, rule.String, rule.Date, rule.DateTime, rule.Strings} {
		if value == candidate {
			return true
		}
	}
	return false
}

func ruleTypesForEntity(entity appir.Entity) map[string]rule.Type {
	out := map[string]rule.Type{
		"id": rule.String, "created_at": rule.DateTime, "updated_at": rule.DateTime, "version": rule.Integer,
	}
	for _, field := range entity.Fields {
		if field.Sensitive {
			continue
		}
		if fieldType, supported := rule.TypeForField(field.Type); supported {
			out[field.Name] = fieldType
		}
	}
	if entity.Owner {
		out["owner_id"] = rule.String
	}
	if entity.Tenant {
		out["tenant_id"] = rule.String
	}
	if entity.SoftDelete {
		out["deleted_at"] = rule.DateTime
	}
	return out
}

func ruleDefinitionDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleExpression, "Rule", name, path, message)
}

func ruleConsumerDiagnostic(kind, name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleExpression, kind, name, path, message)
}

func validateActions(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, action := range a.Actions {
		out = append(out, validateActionRules(a, name, action)...)
		if action.Operation == "register_local_user" {
			if action.DefaultRole == "" {
				out = append(out, requiredDiagnostic("Action", name, "spec.defaultRole", "is required"))
			} else if _, ok := a.Roles[action.DefaultRole]; !ok {
				out = append(out, missingReferenceDiagnostic("Action", name, "spec.defaultRole", "Role", action.DefaultRole))
			} else if action.DefaultRole == "editor" || action.DefaultRole == "administrator" {
				out = append(out, diagnostic("Action", name, "spec.defaultRole", "cannot grant a privileged administration role"))
			}
		} else if _, ok := a.Entities[action.Entity]; !ok {
			out = append(out, missingReferenceDiagnostic("Action", name, "spec.entity", "Entity", action.Entity))
		}
		if !actionop.Valid(action.Operation) {
			out = append(out, diagnostic("Action", name, "spec.operation", "invalid Action operation"))
		}
		lifecycle, lifecycleExists := a.Lifecycles[action.Lifecycle]
		entityLifecycle, entityHasLifecycle := lifecycleForEntity(a, action.Entity)
		if entityHasLifecycle && action.Operation == "transition" && action.Lifecycle == "" {
			diagnostic := diagnostic("Action", name, "spec.lifecycle", "transition Action must reference Lifecycle "+entityLifecycle.Name)
			diagnostic.Code = "BEAN-E2201"
			out = append(out, diagnostic)
		}
		if action.Lifecycle != "" {
			if !lifecycleExists {
				diagnostic := withMissingReference(diagnostic("Action", name, "spec.lifecycle", "references missing Lifecycle "+action.Lifecycle), "Lifecycle", action.Lifecycle)
				diagnostic.Code = "BEAN-E2201"
				out = append(out, diagnostic)
			} else {
				if lifecycle.Entity != action.Entity {
					diagnostic := diagnostic("Action", name, "spec.lifecycle", "Lifecycle entity does not match Action entity")
					diagnostic.Code = "BEAN-E2201"
					out = append(out, diagnostic)
				}
				if action.StateField != "" {
					diagnostic := diagnostic("Action", name, "spec.stateField", "stateField is owned by Lifecycle "+action.Lifecycle)
					diagnostic.Code = "BEAN-E2201"
					out = append(out, diagnostic)
				}
				if action.Operation != "transition" && action.Operation != "transaction" {
					diagnostic := requiredDiagnostic("Action", name, "spec.lifecycle", "Lifecycle requires a transition or transaction Action")
					diagnostic.Code = "BEAN-E2201"
					out = append(out, diagnostic)
				}
				if action.Transitions != nil {
					out = append(out, validateActionTransitionSubset(name, action.Transitions, lifecycle.Transitions)...)
				}
			}
		}
		if action.Operation == "transition" && action.Lifecycle == "" {
			stateField := actionStateField(a, action)
			entity, entityExists := a.Entities[action.Entity]
			state, stateExists := entityFieldDefinition(entity, stateField)
			if entityExists && !stateExists {
				d := withMissingField(diagnostic("Action", name, "spec.stateField", "references missing field "+stateField), stateField)
				d.Code = "BEAN-E2201"
				d.Candidates = closest(stateField, fieldsForEntity(a, action.Entity))
				out = append(out, d)
			} else if stateExists && state.Type != "enum" {
				d := diagnostic("Action", name, "spec.stateField", "transition state field must be an enum")
				d.Code = "BEAN-E2201"
				out = append(out, d)
			} else if stateExists {
				options := append([]string{}, state.Options...)
				sort.Strings(options)
				allowed := nameSet(options)
				fromStates := make([]string, 0, len(action.Transitions))
				for from := range action.Transitions {
					fromStates = append(fromStates, from)
				}
				sort.Strings(fromStates)
				for _, from := range fromStates {
					if !allowed[from] {
						d := diagnostic("Action", name, "spec.transitions."+from, "transition source is not an option of "+stateField)
						d.Code, d.Candidates = "BEAN-E2201", options
						out = append(out, d)
					}
					for index, target := range action.Transitions[from] {
						if !allowed[target] {
							d := diagnostic("Action", name, fmt.Sprintf("spec.transitions.%s.%d", from, index), "transition target is not an option of "+stateField)
							d.Code, d.Candidates = "BEAN-E2201", options
							out = append(out, d)
						}
					}
				}
			}
		}
		if action.Policy != "" {
			if _, ok := a.Policies[action.Policy]; !ok {
				out = append(out, missingReferenceDiagnostic("Action", name, "spec.policy", "Policy", action.Policy))
			}
		}
		for inputName, input := range action.Input {
			if input.Name != inputName {
				out = append(out, diagnostic("Action", name, "spec.input."+inputName+".name", "must match its input key"))
			}
			if input.Type == "" {
				out = append(out, requiredDiagnostic("Action", name, "spec.input."+inputName+".type", "is required"))
			}
		}
		for outputName, output := range action.Output {
			if output.Sensitive {
				out = append(out, diagnostic("Action", name, "spec.output."+outputName, "sensitive fields cannot be Action outputs"))
			}
			if output.Name != outputName || output.Type == "" {
				out = append(out, requiredDiagnostic("Action", name, "spec.output."+outputName, "requires a matching name and type"))
			}
		}
		if action.Operation == "transaction" && len(action.Input) == 0 {
			out = append(out, requiredDiagnostic("Action", name, "spec.input", "transaction Action requires a typed input schema"))
		}
		if action.Operation == "transaction" && len(action.Steps) == 0 {
			out = append(out, requiredDiagnostic("Action", name, "spec.steps", "transaction requires at least one step"))
		}
		if action.Operation != "transaction" && len(action.Steps) > 0 {
			out = append(out, diagnostic("Action", name, "spec.steps", "steps are only valid for transaction Actions"))
		}
		results := map[string]bool{}
		resultEntities := map[string]appir.Entity{}
		for i, step := range action.Steps {
			path := fmt.Sprintf("spec.steps.%d", i)
			stepSpecification, registered := actionstep.Lookup(step.Op)
			if !registered {
				out = append(out, diagnostic("Action", name, path+".op", "has no runtime executor"))
				continue
			}
			if step.Result != "" {
				if results[step.Result] {
					out = append(out, duplicateDiagnostic("Action", name, path+".result", "duplicates a previous step result"))
				}
			}
			for _, assignment := range step.Values {
				switch assignment.Value.Source {
				case valuesource.Literal, valuesource.Request, valuesource.Tenant, valuesource.User, valuesource.Record, valuesource.Now:
				case valuesource.Input:
					if _, ok := action.Input[assignment.Value.Path]; !ok {
						out = append(out, invalidReferenceDiagnostic("Action", name, path+".values."+assignment.Field, "references undeclared input "+assignment.Value.Path))
					}
				case valuesource.Result:
					root := strings.Split(assignment.Value.Path, ".")[0]
					if !results[root] {
						out = append(out, invalidReferenceDiagnostic("Action", name, path+".values."+assignment.Field, "references unavailable step result "+root))
					}
				default:
					out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "has unsupported binding "+assignment.Value.Path))
				}
			}
			if step.Result != "" {
				results[step.Result] = true
			}
			entity := actionstep.EntityName(action, step)
			if step.Result != "" {
				switch step.Op {
				case "create", "load", "update", "conditional_update", "transition", "delete":
					if target, exists := a.Entities[entity]; exists {
						resultEntities[step.Result] = target
					}
				}
			}
			if stepSpecification.UsesEntity {
				if _, ok := a.Entities[entity]; !ok {
					out = append(out, missingReferenceDiagnostic("Action", name, path+".entity", "Entity", entity))
				}
			}
			if target, ok := a.Entities[entity]; ok && !stepSpecification.AnyValues {
				allowedValues := stepValueFields(stepSpecification, target, action)
				for _, assignment := range step.Values {
					if !allowedValues[assignment.Field] {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "is not used by the "+step.Op+" executor"))
					}
				}
			}
			if targetLifecycle, exists := lifecycleForEntity(a, entity); exists {
				if stepSpecification.Transition && action.Lifecycle != targetLifecycle.Name {
					diagnostic := diagnostic("Action", name, path+".op", "transition step must use Lifecycle "+targetLifecycle.Name)
					diagnostic.Code = "BEAN-E2201"
					out = append(out, diagnostic)
				}
				if stepSpecification.ProtectLifecycleState {
					if hasAssignment(step, targetLifecycle.StateField) {
						diagnostic := requiredDiagnostic("Action", name, path+".values."+targetLifecycle.StateField, "Lifecycle state requires a transition step")
						diagnostic.Code = "BEAN-E2201"
						out = append(out, diagnostic)
					}
				}
			}
			if stepSpecification.RequiresView && step.View != "" {
				if _, ok := a.Views[step.View]; !ok {
					out = append(out, missingReferenceDiagnostic("Action", name, path+".view", "View", step.View))
				}
			}
			if stepSpecification.RequiresView && step.View == "" {
				out = append(out, requiredDiagnostic("Action", name, path+".view", "is required so reads use a compiled View"))
			}
			if stepSpecification.RequiresCondition && step.Condition == nil {
				out = append(out, requiredDiagnostic("Action", name, path+".condition", "is required"))
			}
			if step.Condition != nil {
				if er := validateExpr(*step.Condition, false); er != nil {
					out = append(out, validationDiagnostic("Action", name, path+".condition", er))
				}
			}
			if step.Where != nil {
				if er := validateExpr(*step.Where, true); er != nil {
					out = append(out, validationDiagnostic("Action", name, path+".where", er))
				}
			}
			if stepSpecification.RequiresID && !hasAssignment(step, "id") {
				out = append(out, requiredDiagnostic("Action", name, path+".values.id", "is required"))
			}
			if stepSpecification.RequiresJob {
				if _, ok := a.Jobs[step.Job]; !ok {
					out = append(out, missingReferenceDiagnostic("Action", name, path+".job", "Job", step.Job))
				}
			}
			if stepSpecification.RequiresEvent && step.Event == "" {
				out = append(out, requiredDiagnostic("Action", name, path+".event", "is required"))
			}
			if stepSpecification.RequiresEvent && strings.HasPrefix(step.Event, beanextension.TopicPrefix) {
				out = append(out, actionExtensionDiagnostic(name, path+".event", "uses the reserved Extension topic prefix"))
			}
			if stepSpecification.RequiresExtension {
				extensionDefinition, exists := a.Extensions[step.Extension]
				if !exists {
					out = append(out, missingReferenceDiagnostic("Action", name, path+".extension", "Extension", step.Extension))
				} else {
					assigned := map[string]bool{}
					for _, item := range step.Values {
						assigned[item.Field] = true
						fieldDefinition, declared := extensionDefinition.Input[item.Field]
						if !declared {
							out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "Action binds undeclared Extension input"))
							continue
						}
						validateTypedSource := func(source appir.Field) {
							if source.Type != fieldDefinition.Type {
								out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "bound value type does not match Extension input"))
							}
							if source.Type == "enum" && fieldDefinition.Type == "enum" {
								allowed := map[string]bool{}
								for _, option := range fieldDefinition.Options {
									allowed[option] = true
								}
								for _, option := range source.Options {
									if !allowed[option] {
										out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "bound enum options exceed Extension input options"))
										break
									}
								}
							}
							if fieldDefinition.Required && !source.Required {
								out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "optional value cannot bind required Extension input"))
							}
						}
						switch item.Value.Source {
						case valuesource.Input:
							if actionInput, exists := action.Input[item.Value.Path]; exists {
								validateTypedSource(actionInput)
							}
						case valuesource.Result:
							parts := strings.Split(item.Value.Path, ".")
							if len(parts) >= 1 && results[parts[0]] {
								var source appir.Field
								found := false
								if len(parts) == 2 {
									if resultEntity, exists := resultEntities[parts[0]]; exists {
										source, found = entityFieldDefinition(resultEntity, parts[1])
										if !found {
											switch parts[1] {
											case "id":
												source, found = appir.Field{Name: "id", Type: "uuid", Required: true}, true
											case "created_at", "updated_at":
												source, found = appir.Field{Name: parts[1], Type: "datetime", Required: true}, true
											case "version":
												source, found = appir.Field{Name: "version", Type: "integer", Required: true}, true
											}
										}
									}
								}
								if found {
									validateTypedSource(source)
								} else {
									out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "step result has no statically typed Extension value"))
								}
							}
						case valuesource.Literal:
						default:
							out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "Extension input source has no static type contract"))
						}
						if valuesource.IsLiteral(item.Value.Source) {
							var value any
							decoder := json.NewDecoder(strings.NewReader(string(item.Value.Literal)))
							decoder.UseNumber()
							if err := decoder.Decode(&value); err != nil {
								out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "literal does not match Extension input type"))
								continue
							}
							if beanextension.ValidateValues(map[string]appir.Field{item.Field: fieldDefinition}, map[string]any{item.Field: value}) != nil {
								out = append(out, actionExtensionDiagnostic(name, path+".values."+item.Field, "literal does not match Extension input type"))
							}
						}
					}
					for inputName, inputDefinition := range extensionDefinition.Input {
						if inputDefinition.Required && !assigned[inputName] {
							out = append(out, actionExtensionDiagnostic(name, path+".values."+inputName, "required Extension input is not bound"))
						}
					}
				}
				if step.Result != "" {
					out = append(out, actionExtensionDiagnostic(name, path+".result", "after-commit Extension output cannot be used by the Action transaction"))
				}
			}
			if stepSpecification.OutputValues {
				for _, assignment := range step.Values {
					if _, declared := action.Output[assignment.Field]; !declared {
						out = append(out, diagnostic("Action", name, path+".values."+assignment.Field, "is not declared in the Action output schema"))
					}
				}
			}
		}
	}
	return out
}

func validateActionRules(a *appir.App, name string, action appir.Action) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	derivedInputs := nameSet(keys(action.Derive))
	requiresIDBeforeRules := action.Operation == "update" || action.Operation == "delete" || action.Operation == "transition"
	if action.Operation == "transaction" {
		if item, exists := a.Rules[action.When]; exists && rule.UsesSource(item.Expression, "this") {
			requiresIDBeforeRules = true
		}
		for _, ruleName := range action.Derive {
			if item, exists := a.Rules[ruleName]; exists && rule.UsesSource(item.Expression, "this") {
				requiresIDBeforeRules = true
			}
		}
	}
	if action.Operation == "register_local_user" && (action.When != "" || len(action.Derive) > 0) {
		out = append(out, ruleConsumerDiagnostic("Action", name, "spec.when", "local registration does not accept Rule consumers"))
		return out
	}
	if action.When != "" {
		item, exists := a.Rules[action.When]
		if !exists {
			out = append(out, missingReferenceDiagnostic("Action", name, "spec.when", "Rule", action.When))
		} else {
			out = append(out, validateActionRuleBinding(name, "spec.when", action, item)...)
			if action.Operation == "transaction" && rule.UsesSource(item.Expression, "this") {
				if _, hasID := action.Input["id"]; !hasID {
					out = append(out, ruleConsumerDiagnostic("Action", name, "spec.when", "record-aware transaction Rule requires an id input"))
				}
			}
			if item.Result != rule.Boolean {
				out = append(out, ruleConsumerDiagnostic("Action", name, "spec.when", "Action guard Rule must return boolean"))
			}
		}
	}
	for _, fieldName := range keys(action.Derive) {
		path := "spec.derive." + fieldName
		if fieldName == "id" && requiresIDBeforeRules {
			out = append(out, ruleConsumerDiagnostic("Action", name, path, "record identifier is required before Rule evaluation"))
			continue
		}
		if lifecycle, exists := lifecycleForEntity(a, action.Entity); exists && fieldName == lifecycle.StateField {
			diagnostic := diagnostic("Action", name, path, "Lifecycle state is owned by "+lifecycle.Name)
			diagnostic.Code = "BEAN-E2201"
			out = append(out, diagnostic)
			continue
		}
		if action.Operation == "update" {
			owned := false
			for _, candidateName := range keys(a.Actions) {
				candidate := a.Actions[candidateName]
				if candidate.Entity == action.Entity && candidate.Operation == "transition" && candidate.Lifecycle == "" && fieldName == actionStateField(a, candidate) {
					diagnostic := diagnostic("Action", name, path, "State is owned by transition Action "+candidateName)
					diagnostic.Code = "BEAN-E2201"
					out = append(out, diagnostic)
					owned = true
					break
				}
			}
			if owned {
				continue
			}
		}
		item, exists := a.Rules[action.Derive[fieldName]]
		if !exists {
			out = append(out, missingReferenceDiagnostic("Action", name, path, "Rule", action.Derive[fieldName]))
			continue
		}
		out = append(out, validateActionRuleBinding(name, path, action, item)...)
		if action.Operation == "create" && rule.UsesSource(item.Expression, "this") {
			out = append(out, ruleConsumerDiagnostic("Action", name, path, "create derivation cannot reference a candidate before derives are complete"))
		}
		if action.Operation == "transaction" && rule.UsesSource(item.Expression, "this") {
			if _, hasID := action.Input["id"]; !hasID {
				out = append(out, ruleConsumerDiagnostic("Action", name, path, "record-aware transaction Rule requires an id input"))
			}
		}
		for _, inputName := range rule.InputPaths(item.Expression) {
			if derivedInputs[inputName] {
				out = append(out, ruleConsumerDiagnostic("Action", name, path, "derived Rules cannot reference derived input "+inputName))
			}
		}
		field, exists := action.Input[fieldName]
		if !exists {
			out = append(out, missingFieldDiagnostic("Action", name, path, fieldName, false))
			continue
		}
		targetType, supported := rule.TypeForField(field.Type)
		if !supported || !rule.ResultCompatible(targetType, item.Result) {
			out = append(out, ruleConsumerDiagnostic("Action", name, path, "derived Rule result is incompatible with the Action input"))
		}
	}
	return out
}

func validateActionRuleBinding(actionName, path string, action appir.Action, item appir.Rule) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if item.Entity != "" && item.Entity != action.Entity {
		out = append(out, ruleConsumerDiagnostic("Action", actionName, path, "Rule entity does not match Action entity"))
	}
	for _, inputName := range keys(item.Input) {
		actionInput, exists := action.Input[inputName]
		if !exists {
			out = append(out, missingFieldDiagnostic("Action", actionName, path, inputName, false))
			continue
		}
		if actionInput.Sensitive {
			out = append(out, ruleConsumerDiagnostic("Action", actionName, path, "Rule cannot read sensitive Action input "+inputName))
			continue
		}
		expected, expectedOK := rule.TypeForField(item.Input[inputName].Type)
		actual, actualOK := rule.TypeForField(actionInput.Type)
		if !expectedOK || !actualOK || !rule.ResultCompatible(expected, actual) || !rule.ResultCompatible(actual, expected) {
			out = append(out, ruleConsumerDiagnostic("Action", actionName, path, "Rule input "+inputName+" is incompatible with the Action input"))
		}
	}
	return out
}

func validateWebforms(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, form := range a.Webforms {
		action, ok := a.Actions[form.Action]
		if !ok {
			out = append(out, missingReferenceDiagnostic("Webform", name, "spec.action", "Action", form.Action))
		} else {
			for i, element := range form.Elements {
				input, exists := action.Input[element.Name]
				if !exists {
					out = append(out, invalidReferenceDiagnostic("Webform", name, fmt.Sprintf("spec.elements.%d.name", i), "has no matching Action input"))
				} else if _, derived := action.Derive[element.Name]; derived {
					out = append(out, diagnostic("Webform", name, fmt.Sprintf("spec.elements.%d.name", i), "derived Action input is server-owned"))
				} else if !compatibleFormType(element.Type, input.Type) {
					out = append(out, diagnostic("Webform", name, fmt.Sprintf("spec.elements.%d.type", i), "does not match Action input type "+input.Type))
				}
			}
		}
		out = append(out, validateForm(name, form)...)
	}
	return out
}

func validatePolicies(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, policy := range a.Policies {
		if policy.Condition != nil {
			if er := validateExpr(*policy.Condition, true); er != nil {
				out = append(out, validationDiagnostic("Policy", name, "spec.condition", er))
			}
		}
	}
	return out
}

func validateBlocks(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, block := range a.Blocks {
		blockSpecification, registered := blockcap.Lookup(block.Type)
		if !registered {
			out = append(out, diagnostic("Block", name, "spec.type", "has no registered renderer"))
		}
		if blockSpecification.RequiresResource && block.Resource == "" {
			out = append(out, requiredDiagnostic("Block", name, "spec.resource", "is required"))
		}
		if blockSpecification.RequiresEditorReadPolicy && (block.Policy == "" || !editorOnlyReadPolicy(a.Policies[block.Policy])) {
			out = append(out, diagnostic("Block", name, "spec.policy", "resource-list Block must be restricted to editor and administrator roles"))
		}
		refs := []struct{ kind, value string }{{"view", block.View}, {"entity", block.Entity}, {"webform", block.Webform}, {"action", block.Action}, {"resource", block.Resource}}
		for _, ref := range refs {
			if ref.value == "" {
				continue
			}
			ok := false
			switch ref.kind {
			case "view":
				_, ok = a.Views[ref.value]
			case "entity":
				_, ok = a.Entities[ref.value]
			case "webform":
				_, ok = a.Webforms[ref.value]
			case "action":
				_, ok = a.Actions[ref.value]
			case "resource":
				_, ok = a.AdminResources[ref.value]
			}
			if !ok {
				out = append(out, invalidReferenceDiagnostic("Block", name, "spec."+ref.kind, "invalid Block input reference "+ref.value))
			}
		}
		if blockSpecification.SupportsPresentation && block.View != "" {
			out = append(out, validatePresentation(name, block, a)...)
			if block.Display != "" {
				display, exists := a.Views[block.View].Displays[block.Display]
				if !exists {
					out = append(out, invalidReferenceDiagnostic("Block", name, "spec.display", "references missing View display "+block.Display))
				} else if display.Type != "block" {
					out = append(out, diagnostic("Block", name, "spec.display", "must reference a block display"))
				} else {
					if display.Title.Field != "" {
						viewDefinition := a.Views[block.View]
						entity := a.Entities[viewDefinition.Entity]
						singleRecord := false
						for filterName := range block.Bindings {
							filter, exposed := viewDefinition.ExposedFilters[filterName]
							target := filter.Target(filterName)
							definition, fieldExists := entityFieldDefinition(entity, target)
							singleRecord = singleRecord || exposed && (target == "id" || fieldExists && definition.Unique)
						}
						if !singleRecord {
							out = append(out, diagnostic("Block", name, "spec.display", "result title requires a unique bound View filter"))
						}
					}
					for index, control := range display.Controls {
						if _, bound := block.Bindings[control.Filter]; bound {
							out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.bindings.%s", control.Filter), fmt.Sprintf("cannot bind filter exposed by display control %d", index)))
						}
					}
				}
			}
		}
		if block.Menu != "" {
			if _, ok := a.Menus[block.Menu]; !ok {
				out = append(out, missingReferenceDiagnostic("Block", name, "spec.menu", "Menu", block.Menu))
			}
		}
		if block.Policy != "" {
			if _, ok := a.Policies[block.Policy]; !ok {
				out = append(out, missingReferenceDiagnostic("Block", name, "spec.policy", "Policy", block.Policy))
			}
		}
		for inputName, input := range block.Inputs {
			binding, mapped := block.Bindings[inputName]
			if input.Required && !mapped {
				out = append(out, diagnostic("Block", name, "spec.bindings."+inputName, "required input has no typed mapping"))
			}
			if mapped {
				if !valuesource.Allows(valuesource.Block, binding.Source) {
					out = append(out, diagnostic("Block", name, "spec.bindings."+inputName+".source", "has no typed resolver"))
				}
				if binding.Source != "tenant" && binding.Name == "" {
					out = append(out, requiredDiagnostic("Block", name, "spec.bindings."+inputName+".name", "is required"))
				}
			}
		}
		for inputName := range block.Bindings {
			if _, exists := block.Inputs[inputName]; !exists {
				out = append(out, invalidReferenceDiagnostic("Block", name, "spec.bindings."+inputName, "references an undeclared input"))
			}
		}
		var target map[string]appir.Field
		switch blockSpecification.InputTarget {
		case blockcap.ViewInputTarget:
			if block.View != "" {
				target = exposedFilterFields(a.Views[block.View])
			}
		case blockcap.WebformInputTarget:
			if block.Webform != "" {
				formDefinition := a.Webforms[block.Webform]
				target = a.Actions[formDefinition.Action].Input
				for _, element := range formDefinition.Elements {
					if _, bound := block.Bindings[element.Name]; bound {
						out = append(out, diagnostic("Block", name, "spec.bindings."+element.Name, "cannot bind a field also rendered by the Webform"))
					}
				}
			}
		case blockcap.ResourceInputTarget:
			if block.Resource == "" {
				break
			}
			resource := a.AdminResources[block.Resource]
			target = exposedFilterFields(a.Views[resource.View])
			resourceFilters := nameSet(resource.List.Filters)
			interactive := nameSet(block.Filters)
			for i, filterName := range block.Filters {
				if _, bound := block.Bindings[filterName]; bound {
					out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "cannot expose an immutable bound input"))
				}
				if !resourceFilters[filterName] {
					out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "is not configured by AdminResource "+block.Resource))
				}
				if _, exposed := target[filterName]; !exposed {
					out = append(out, invalidReferenceDiagnostic("Block", name, fmt.Sprintf("spec.filters.%d", i), "has no matching View exposed filter"))
				}
			}
			for filterName, value := range block.DefaultFilters {
				definition, exposed := target[filterName]
				if !interactive[filterName] {
					out = append(out, invalidReferenceDiagnostic("Block", name, "spec.defaultFilters."+filterName, "must reference an interactive filter"))
				} else if exposed {
					definition.Name = filterName
					if err := field.Validate(definition, value); err != nil {
						out = append(out, diagnostic("Block", name, "spec.defaultFilters."+filterName, err.Error()))
					}
				}
			}
		}
		if target != nil {
			for inputName, input := range block.Inputs {
				expected, exists := target[inputName]
				if !exists {
					out = append(out, invalidReferenceDiagnostic("Block", name, "spec.inputs."+inputName, "has no matching target input"))
				} else if input.Type != expected.Type {
					out = append(out, diagnostic("Block", name, "spec.inputs."+inputName+".type", "does not match target input type "+expected.Type))
				}
			}
		}
	}
	return out
}

func exposedFilterFields(view appir.View) map[string]appir.Field {
	out := make(map[string]appir.Field, len(view.ExposedFilters))
	for name, filter := range view.ExposedFilters {
		definition := filter.Definition(name)
		definition.Name = name
		out[name] = definition
	}
	return out
}

func validateLocalRegistration(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if a.LocalRegistration != nil {
		action, ok := a.Actions[a.LocalRegistration.Action]
		if !ok || action.Operation != "register_local_user" {
			out = append(out, invalidReferenceDiagnostic("LocalRegistration", "local", "spec.action", "must reference a register_local_user Action"))
		}
		if route := a.LocalRegistration.Route; route != "" {
			if !strings.HasPrefix(route, "/") || strings.Contains(route, ":") {
				out = append(out, diagnostic("LocalRegistration", "local", "spec.route", "must be a static absolute Page route"))
			} else if routeErr := validateRegistrationPage(a, route, a.LocalRegistration.Action); routeErr != "" {
				out = append(out, diagnostic("LocalRegistration", "local", "spec.route", routeErr))
			}
		}
	}
	return out
}

func validatePanels(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	layouts := panelLayouts()
	for name, panel := range a.Panels {
		regions, ok := layouts[panel.Layout]
		if !ok {
			out = append(out, diagnostic("Panel", name, "spec.layout", "invalid layout"))
			continue
		}
		for _, region := range panel.Regions {
			if !regions[region.Name] {
				out = append(out, diagnostic("Panel", name, "spec.regions."+region.Name, "invalid Panel region"))
			}
			for _, block := range region.Blocks {
				if _, ok := a.Blocks[block]; !ok {
					out = append(out, missingReferenceDiagnostic("Panel", name, "spec.regions."+region.Name, "Block", block))
				}
			}
		}
		if panel.Policy != "" {
			if _, ok := a.Policies[panel.Policy]; !ok {
				out = append(out, missingReferenceDiagnostic("Panel", name, "spec.policy", "Policy", panel.Policy))
			}
		}
	}
	return out
}

func validatePages(a *appir.App, state *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	routes := state.routes
	for _, name := range keys(a.Pages) {
		page := a.Pages[name]
		if !strings.HasPrefix(page.Route, "/") {
			out = append(out, diagnostic("Page", name, "spec.route", "must start with /"))
		}
		if old := conflictingRoute(routes, page.Route); old != "" {
			out = append(out, diagnostic("Page", name, "spec.route", "overlaps route used by "+old))
		} else {
			routes[page.Route] = "Page/" + name
		}
		if _, ok := a.Panels[page.Panel]; !ok {
			out = append(out, missingReferenceDiagnostic("Page", name, "spec.panel", "Panel", page.Panel))
		}
		if page.Policy != "" {
			if _, ok := a.Policies[page.Policy]; !ok {
				out = append(out, missingReferenceDiagnostic("Page", name, "spec.policy", "Policy", page.Policy))
			}
		}
		for key, binding := range page.Context {
			if !valuesource.Allows(valuesource.Page, binding.Source) {
				out = append(out, diagnostic("Page", name, "spec.context."+key+".source", "has no typed resolver"))
			}
			if binding.Source != "tenant" && binding.Name == "" {
				out = append(out, requiredDiagnostic("Page", name, "spec.context."+key+".name", "is required"))
			}
		}
	}
	return out
}

func validateJobs(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, job := range a.Jobs {
		if _, ok := a.Actions[job.Action]; !ok {
			out = append(out, missingReferenceDiagnostic("Job", name, "spec.action", "Action", job.Action))
		}
	}
	return out
}

func validateMenus(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, menu := range a.Menus {
		for i, item := range menu.Items {
			if item.Policy != "" {
				if _, ok := a.Policies[item.Policy]; !ok {
					out = append(out, missingReferenceDiagnostic("Menu", name, fmt.Sprintf("spec.items.%d.policy", i), "Policy", item.Policy))
				}
			}
		}
	}
	return out
}

func validateAdminResources(a *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	for name, resource := range a.AdminResources {
		entity, ok := a.Entities[resource.Entity]
		if !ok {
			out = append(out, missingReferenceDiagnostic("AdminResource", name, "spec.entity", "Entity", resource.Entity))
			continue
		}
		viewDefinition, ok := a.Views[resource.View]
		if !ok || viewDefinition.Entity != resource.Entity {
			out = append(out, invalidReferenceDiagnostic("AdminResource", name, "spec.view", "must reference a View for Entity "+resource.Entity))
			continue
		}
		fields := fieldSet(entity)
		selected := map[string]bool{}
		for _, field := range viewDefinition.Fields {
			selected[field] = true
		}
		checkFields := func(path string, names []string, requireSelected bool) {
			for _, field := range names {
				if !fields[field] {
					out = append(out, missingFieldDiagnostic("AdminResource", name, path, field, false))
				} else if requireSelected && !selected[field] {
					out = append(out, diagnostic("AdminResource", name, path, "field "+field+" is not selected by View "+resource.View))
				}
			}
		}
		checkFields("spec.list.columns", resource.List.Columns, true)
		checkFields("spec.list.search", resource.List.Search, true)
		checkFields("spec.list.filters", resource.List.Filters, true)
		checkFields("spec.form.fields", resource.Form.Fields, false)
		checkFields("spec.form.readonly", resource.Form.Readonly, true)
		checkFields("spec.labelField", []string{resource.LabelField}, true)
		for _, order := range resource.List.Sort {
			checkFields("spec.list.sort", []string{order.Field}, true)
		}
		if resource.List.PageSize < 1 || resource.List.PageSize > 200 {
			out = append(out, diagnostic("AdminResource", name, "spec.list.pageSize", "must be between 1 and 200"))
		}
		for path, actionName := range map[string]string{"spec.createAction": resource.CreateAction, "spec.updateAction": resource.UpdateAction, "spec.deleteAction": resource.DeleteAction} {
			action, exists := a.Actions[actionName]
			if !exists || action.Entity != resource.Entity {
				out = append(out, invalidReferenceDiagnostic("AdminResource", name, path, "must reference an Action for Entity "+resource.Entity))
			}
		}
		for _, actionName := range resource.Actions {
			action, exists := a.Actions[actionName]
			if !exists || action.Entity != resource.Entity {
				out = append(out, invalidReferenceDiagnostic("AdminResource", name, "spec.actions", "must reference Actions for Entity "+resource.Entity))
			}
		}
	}
	return out
}

func demoSeedRequiredRelationCycle(a *appir.App) bool {
	visiting, visited := map[string]bool{}, map[string]bool{}
	var visit func(string) bool
	visit = func(name string) bool {
		if visiting[name] {
			return true
		}
		if visited[name] {
			return false
		}
		visiting[name] = true
		for _, field := range a.Entities[name].Fields {
			if !field.Required || field.Relation == nil {
				continue
			}
			if _, seeded := a.DemoSeed.Entities[field.Relation.Entity]; seeded && visit(field.Relation.Entity) {
				return true
			}
		}
		delete(visiting, name)
		visited[name] = true
		return false
	}
	for name := range a.DemoSeed.Entities {
		if _, exists := a.Entities[name]; exists && visit(name) {
			return true
		}
	}
	return false
}

func recordFields(expression *expr.Expr) []string {
	if expression == nil {
		return nil
	}
	out := []string{}
	if expression.Left != nil && expression.Left.Source == "record" {
		out = append(out, expression.Left.Name)
	}
	if expression.Right != nil && expression.Right.Source == "record" {
		out = append(out, expression.Right.Name)
	}
	for i := range expression.Args {
		out = append(out, recordFields(&expression.Args[i])...)
	}
	return out
}

func validatePresentation(name string, block appir.Block, a *appir.App) []definition.Diagnostic {
	presentation := block.Presentation
	if presentation.Mode == "" {
		return nil
	}
	out := []definition.Diagnostic{}
	if !nameSet(presentationNames())[presentation.Mode] {
		return []definition.Diagnostic{diagnostic("Block", name, "spec.presentation.mode", "has no registered presentation renderer")}
	}
	viewDefinition := a.Views[block.View]
	entity := a.Entities[viewDefinition.Entity]
	selected := nameSet(viewDefinition.Fields)
	aggregates := map[string]bool{}
	for _, aggregate := range viewDefinition.Aggregates {
		selected[aggregate.Alias] = true
		aggregates[aggregate.Alias] = true
	}
	fieldDefinition := func(fieldName string) (appir.Field, bool) {
		for _, systemField := range []appir.Field{{Name: "id", Type: "uuid"}, {Name: "created_at", Type: "datetime"}, {Name: "updated_at", Type: "datetime"}, {Name: "version", Type: "integer"}} {
			if fieldName == systemField.Name {
				return systemField, true
			}
		}
		for _, candidate := range entity.Fields {
			if candidate.Name == fieldName {
				return candidate, true
			}
		}
		return appir.Field{}, false
	}
	policyName := policy.EffectiveViewPolicyName(viewDefinition, entity)
	redacted := nameSet(a.Policies[policyName].Redact)
	if presentation.Mode == "board" || presentation.Mode == "tree" {
		for _, sortDefinition := range viewDefinition.Sort {
			if aggregates[sortDefinition.Field] {
				out = append(out, diagnostic("Block", name, "spec.presentation.mode", "board and tree presentations do not support aggregate-sorted Views"))
				break
			}
		}
	}
	if parts := strings.Split(presentation.BodyField, "."); len(parts) == 2 {
		for _, relationship := range viewDefinition.Relationships {
			if relationship.Name != parts[0] {
				continue
			}
			for _, relatedField := range a.Entities[relationship.Entity].Fields {
				if relatedField.Name == parts[1] && relatedField.Type == "file" {
					out = append(out, diagnostic("Block", name, "spec.presentation.bodyField", "related file fields are not supported by presentation downloads"))
				}
			}
		}
	}
	for _, match := range regexp.MustCompile(`:([a-zA-Z0-9_.]+)`).FindAllStringSubmatch(presentation.LinkRoute, -1) {
		fieldName := match[1]
		if !selected[fieldName] {
			out = append(out, diagnostic("Block", name, "spec.presentation.linkRoute", "field "+fieldName+" must be selected by View "+block.View))
		} else if redacted[fieldName] {
			out = append(out, diagnostic("Block", name, "spec.presentation.linkRoute", "field "+fieldName+" must not be redacted by View policy "+policyName))
		}
	}
	for path, fieldName := range map[string]string{"titleField": presentation.TitleField, "bodyField": presentation.BodyField, "groupField": presentation.GroupField, "orderField": presentation.OrderField, "parentField": presentation.ParentField} {
		if fieldName != "" && !selected[fieldName] {
			out = append(out, diagnostic("Block", name, "spec.presentation."+path, "must be selected by View "+block.View))
		}
		if (presentation.Mode == "board" || presentation.Mode == "tree" || presentation.Mode == "timeline") && fieldName != "" && redacted[fieldName] && path != "bodyField" {
			out = append(out, diagnostic("Block", name, "spec.presentation."+path, "must not be redacted by View policy "+policyName))
		}
	}
	searchable := map[string]bool{"email": true, "richtext": true, "slug": true, "string": true, "text": true, "url": true}
	for index, fieldName := range presentation.SearchFields {
		field, exists := fieldDefinition(fieldName)
		path := fmt.Sprintf("spec.presentation.searchFields.%d", index)
		if !selected[fieldName] {
			out = append(out, diagnostic("Block", name, path, "must be selected by View "+block.View))
		} else if !exists || !searchable[field.Type] {
			out = append(out, invalidReferenceDiagnostic("Block", name, path, "must reference a searchable text field"))
		} else if redacted[fieldName] {
			out = append(out, diagnostic("Block", name, path, "must not be redacted by View policy "+policyName))
		}
	}
	if presentation.Mode == "metric" {
		if presentation.MetricField == "" || !aggregates[presentation.MetricField] {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.metricField", "metric requires a selected aggregate alias"))
		}
		if len(viewDefinition.GroupBy) > 0 {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.metricField", "metric requires an ungrouped View"))
		}
		if len(viewDefinition.Fields) > 0 {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.metricField", "metric requires an aggregate-only View"))
		}
		if len(presentation.SearchFields) > 0 {
			out = append(out, diagnostic("Block", name, "spec.presentation.searchFields", "metric does not support search"))
		}
	}
	if presentation.Mode == "timeline" {
		if presentation.TitleField == "" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.titleField", "timeline requires a selected title field"))
		}
		field, exists := fieldDefinition(presentation.TimeField)
		if presentation.TimeField == "" || !selected[presentation.TimeField] || !exists || field.Type != "date" && field.Type != "datetime" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.timeField", "timeline requires a selected date or datetime field"))
		} else if redacted[presentation.TimeField] {
			out = append(out, diagnostic("Block", name, "spec.presentation.timeField", "must not be redacted by View policy "+policyName))
		}
	}
	if presentation.Mode == "board" {
		if presentation.TitleField == "" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.titleField", "board requires a selected title field"))
		}
		group, exists := fieldDefinition(presentation.GroupField)
		if presentation.GroupField == "" || !exists || group.Type != "enum" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.groupField", "board requires a selected enum field"))
		}
		if len(presentation.Columns) == 0 {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.columns", "board requires at least one column"))
		}
		allowedColumns := nameSet(group.Options)
		for i, column := range presentation.Columns {
			if !allowedColumns[column] {
				out = append(out, diagnostic("Block", name, fmt.Sprintf("spec.presentation.columns.%d", i), "is not an option of "+presentation.GroupField))
			}
		}
		action, exists := a.Actions[presentation.MoveAction]
		stateField := actionStateField(a, action)
		if presentation.MoveAction == "" || !exists || action.Entity != viewDefinition.Entity || action.Operation != "transition" || stateField != presentation.GroupField {
			out = append(out, invalidReferenceDiagnostic("Block", name, "spec.presentation.moveAction", "must reference a transition Action for the board entity and group field"))
		} else {
			for inputName, inputDefinition := range action.Input {
				if inputDefinition.Required && inputName != "id" && inputName != presentation.GroupField {
					out = append(out, diagnostic("Block", name, "spec.presentation.moveAction", "transition Action has unsupported required input "+inputName))
				}
			}
		}
		if presentation.OrderField != "" {
			order, orderExists := fieldDefinition(presentation.OrderField)
			if !orderExists || order.Type != "integer" {
				out = append(out, diagnostic("Block", name, "spec.presentation.orderField", "board order field must be an integer"))
			}
		}
	}
	if presentation.Mode == "tree" {
		parent, exists := fieldDefinition(presentation.ParentField)
		if presentation.ParentField == "" || !exists || parent.Type != "relation" || parent.Relation == nil || parent.Relation.Entity != viewDefinition.Entity || parent.Relation.Kind != "many-to-one" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.parentField", "tree requires a selected many-to-one self relation"))
		}
		if presentation.TitleField == "" {
			out = append(out, requiredDiagnostic("Block", name, "spec.presentation.titleField", "tree requires a selected title field"))
		}
		if presentation.OrderField != "" {
			order, exists := fieldDefinition(presentation.OrderField)
			if !exists || order.Type != "integer" {
				out = append(out, diagnostic("Block", name, "spec.presentation.orderField", "tree order field must be an integer"))
			}
		}
	}
	return out
}

func compatibleFormType(formType, fieldType string) bool {
	allowed := map[string][]string{
		"text":             {"string", "text", "richtext", "uuid", "url", "slug"},
		"password":         {"password"},
		"textarea":         {"string", "text", "richtext"},
		"email":            {"email"},
		"number":           {"integer", "money", "decimal"},
		"integer":          {"integer", "money"},
		"checkbox":         {"boolean"},
		"select":           {"enum", "string"},
		"date":             {"date"},
		"datetime":         {"datetime"},
		"entity reference": {"relation", "uuid"},
		"file":             {"file"},
		"group":            {"json"},
	}
	for _, candidate := range allowed[formType] {
		if candidate == fieldType {
			return true
		}
	}
	return false
}

func hasAssignment(step appir.Step, name string) bool {
	_, ok := assignment(step, name)
	return ok
}

func validViewField(name string, base map[string]bool, relationships map[string]appir.ViewRelationship, a *appir.App) bool {
	parts := strings.Split(name, ".")
	if len(parts) == 1 {
		return base[name]
	}
	if len(parts) != 2 {
		return false
	}
	relationship, ok := relationships[parts[0]]
	if !ok {
		return false
	}
	return fieldSet(a.Entities[relationship.Entity])[parts[1]]
}

func viewFieldType(name string, base appir.Entity, relationships map[string]appir.ViewRelationship, a *appir.App) (string, bool) {
	definition, exists := viewFieldDefinition(name, base, relationships, a)
	return definition.Type, exists
}

func resolveViewRelationship(entity appir.Entity, relationship appir.ViewRelationship) (appir.ViewRelationship, bool) {
	if relationship.RelationField == "" {
		return relationship, true
	}
	for _, field := range entity.Fields {
		if field.Name == relationship.RelationField && field.Relation != nil {
			relationship.Entity = field.Relation.Entity
			relationship.LocalField = relationship.RelationField
			relationship.TargetField = field.Relation.TargetField
			return relationship, true
		}
	}
	return relationship, false
}

func viewFieldDefinition(name string, base appir.Entity, relationships map[string]appir.ViewRelationship, a *appir.App) (appir.Field, bool) {
	parts := strings.Split(name, ".")
	entity := base
	fieldName := name
	if len(parts) == 2 {
		relationship, ok := relationships[parts[0]]
		if !ok {
			return appir.Field{}, false
		}
		entity, ok = a.Entities[relationship.Entity]
		if !ok {
			return appir.Field{}, false
		}
		fieldName = parts[1]
	} else if len(parts) != 1 {
		return appir.Field{}, false
	}
	for _, fieldDefinition := range entity.Fields {
		if fieldDefinition.Name == fieldName {
			return fieldDefinition, true
		}
	}
	for _, fieldDefinition := range []appir.Field{{Name: "id", Type: "uuid"}, {Name: "created_at", Type: "datetime"}, {Name: "updated_at", Type: "datetime"}, {Name: "version", Type: "integer"}} {
		if fieldDefinition.Name == fieldName {
			return fieldDefinition, true
		}
	}
	for _, fieldDefinition := range []struct {
		field   appir.Field
		enabled bool
	}{
		{field: appir.Field{Name: "owner_id", Type: "uuid"}, enabled: entity.Owner},
		{field: appir.Field{Name: "tenant_id", Type: "uuid"}, enabled: entity.Tenant},
		{field: appir.Field{Name: "deleted_at", Type: "datetime"}, enabled: entity.SoftDelete},
	} {
		if fieldDefinition.enabled && fieldDefinition.field.Name == fieldName {
			return fieldDefinition.field, true
		}
	}
	return appir.Field{}, false
}

func stepValueFields(specification actionstep.Specification, entity appir.Entity, action appir.Action) map[string]bool {
	fields := map[string]bool{}
	for _, name := range specification.AllowedValues {
		fields[name] = true
	}
	if specification.EntityFields {
		for _, field := range entity.Fields {
			fields[field.Name] = true
		}
	}
	if specification.OutputValues {
		for name := range action.Output {
			fields[name] = true
		}
	}
	return fields
}

func validateExpr(expression expr.Expr, database bool) error {
	logical := map[string]int{"and": -1, "or": -1, "not": 1}
	if arity, ok := logical[expression.Op]; ok {
		if arity == -1 && len(expression.Args) == 0 {
			return requiredValidationError(fmt.Sprintf("%s requires arguments", expression.Op))
		}
		if arity >= 0 && len(expression.Args) != arity {
			return requiredValidationError(fmt.Sprintf("%s requires %d argument", expression.Op, arity))
		}
		for _, child := range expression.Args {
			if e := validateExpr(child, database); e != nil {
				return e
			}
		}
		return nil
	}
	if !map[string]bool{"eq": true, "ne": true, "gt": true, "gte": true, "lt": true, "lte": true, "in": true, "not_in": true, "contains": true, "starts_with": true, "ends_with": true, "is_null": true, "is_not_null": true}[expression.Op] {
		return fmt.Errorf("unsupported expression operator %q", expression.Op)
	}
	if expression.Left == nil {
		return requiredValidationError("left value is required")
	}
	if expression.Op != "is_null" && expression.Op != "is_not_null" && expression.Right == nil {
		return requiredValidationError("right value is required")
	}
	if !valuesource.Allows(valuesource.Expression, expression.Left.Source) || expression.Right != nil && !valuesource.Allows(valuesource.Expression, expression.Right.Source) {
		return fmt.Errorf("expression has an unsupported value source")
	}
	if database && expression.Left.Source != "record" {
		return fmt.Errorf("database expression left side must be a record field")
	}
	if database && expression.Right != nil && expression.Right.Source == "record" {
		return fmt.Errorf("database expression right side cannot be a record field")
	}
	return nil
}

func editorOnlyReadPolicy(definition appir.Policy) bool {
	if len(definition.ReadRoles) == 0 {
		return false
	}
	for _, roleName := range definition.ReadRoles {
		if roleName != "editor" && roleName != "administrator" {
			return false
		}
	}
	return true
}

func validateRegistrationPage(a *appir.App, route, actionName string) string {
	var registrationPage *appir.Page
	for _, pageDefinition := range a.Pages {
		if pageDefinition.Route == route {
			copy := pageDefinition
			registrationPage = &copy
			break
		}
	}
	if registrationPage == nil {
		return "must reference a Page containing a Webform for the registration Action"
	}
	panelDefinition := a.Panels[registrationPage.Panel]
	anonymous := beanctx.Request{Route: route, RouteParams: map[string]string{}, Values: map[string]any{}}
	if registrationPage.Policy != "" && !policy.Can(a.Policies[registrationPage.Policy], false, anonymous, nil) {
		return "must reference a Page and Panel accessible to anonymous users"
	}
	resolvedContext, err := page.ResolveContext(*registrationPage, map[string]string{}, map[string]string{}, anonymous)
	if err != nil {
		return "must resolve Page context for an anonymous request to the advertised static route"
	}
	anonymous.Values = resolvedContext
	if _, allowed, renderErr := page.Node(a, *registrationPage, resolvedContext, anonymous); renderErr != nil || !allowed {
		return "must render completely for an anonymous request to the advertised static route"
	}
	if panelDefinition.Policy != "" && !policy.Can(a.Policies[panelDefinition.Policy], false, anonymous, nil) {
		return "must reference a Page and Panel accessible to anonymous users"
	}
	actionDefinition := a.Actions[actionName]
	var missing []string
	found := false
	for _, region := range panelDefinition.Regions {
		for _, blockName := range region.Blocks {
			blockDefinition := a.Blocks[blockName]
			formDefinition := a.Webforms[blockDefinition.Webform]
			specification, registered := blockcap.Lookup(blockDefinition.Type)
			if !registered || specification.InputTarget != blockcap.WebformInputTarget || formDefinition.Action != actionName {
				continue
			}
			found = true
			if blockDefinition.Policy != "" && !policy.Can(a.Policies[blockDefinition.Policy], false, anonymous, nil) {
				continue
			}
			fields := map[string]bool{}
			for _, element := range formDefinition.Elements {
				fields[element.Name] = element.Required && element.Visible == nil
			}
			missing = missing[:0]
			for inputName, inputDefinition := range actionDefinition.Input {
				if inputDefinition.Required && !fields[inputName] {
					missing = append(missing, inputName)
				}
			}
			if len(missing) == 0 {
				return ""
			}
			sort.Strings(missing)
		}
	}
	if !found {
		return "must reference a Page containing a Webform for the registration Action"
	}
	if len(missing) > 0 {
		return "Webform must unconditionally collect required registration inputs: " + strings.Join(missing, ", ")
	}
	return "must reference a registration Webform Block accessible to anonymous users"
}

func validateForm(name string, form appir.Webform) []definition.Diagnostic {
	allowed := map[string]bool{"text": true, "textarea": true, "email": true, "password": true, "number": true, "integer": true, "checkbox": true, "select": true, "date": true, "datetime": true, "entity reference": true, "file": true, "group": true}
	out := []definition.Diagnostic{}
	seen := map[string]bool{}
	var walk func([]appir.FormElement, string, bool)
	walk = func(elements []appir.FormElement, path string, nested bool) {
		for i, element := range elements {
			p := fmt.Sprintf("%s.%d", path, i)
			if element.Name == "" {
				out = append(out, requiredDiagnostic("Webform", name, p+".name", "is required"))
			} else if seen[element.Name] {
				out = append(out, duplicateDiagnostic("Webform", name, p+".name", "duplicates another element"))
			}
			seen[element.Name] = true
			if !allowed[element.Type] {
				out = append(out, diagnostic("Webform", name, p+".type", "has no server and UI implementation"))
			}
			if nested && element.Type == "file" {
				out = append(out, diagnostic("Webform", name, p+".type", "file elements are not supported inside repeating groups"))
			}
			if element.Type == "group" {
				if len(element.Children) == 0 {
					out = append(out, requiredDiagnostic("Webform", name, p+".children", "repeating group requires children"))
				}
				walk(element.Children, p+".children", true)
			} else if len(element.Children) > 0 {
				out = append(out, diagnostic("Webform", name, p+".children", "is only valid for group elements"))
			}
			for conditionPath, condition := range map[string]*expr.Expr{"visible": element.Visible, "requiredWhen": element.RequiredWhen} {
				if condition != nil {
					if er := validateExpr(*condition, false); er != nil {
						out = append(out, validationDiagnostic("Webform", name, p+"."+conditionPath, er))
					}
				}
			}
		}
	}
	walk(form.Elements, "spec.elements", false)
	stepUse := map[string]int{}
	for i, step := range form.Steps {
		for _, element := range step {
			stepUse[element]++
			if !seen[element] {
				out = append(out, missingReferenceDiagnostic("Webform", name, fmt.Sprintf("spec.steps.%d", i), "element", element))
			}
		}
	}
	if len(form.Steps) > 0 {
		for _, element := range form.Elements {
			if stepUse[element.Name] != 1 {
				out = append(out, requiredDiagnostic("Webform", name, "spec.steps", "must include element "+element.Name+" exactly once"))
			}
		}
	}
	return out
}
func fieldSet(e appir.Entity) map[string]bool {
	m := map[string]bool{"id": true, "created_at": true, "updated_at": true, "version": true}
	if e.Owner {
		m["owner_id"] = true
	}
	if e.Tenant {
		m["tenant_id"] = true
	}
	if e.SoftDelete {
		m["deleted_at"] = true
	}
	for _, f := range e.Fields {
		m[f.Name] = true
	}
	return m
}

func entityFieldDefinition(entity appir.Entity, name string) (appir.Field, bool) {
	for _, field := range entity.Fields {
		if field.Name == name {
			return field, true
		}
	}
	return appir.Field{}, false
}

func nameSet(values []string) map[string]bool {
	out := map[string]bool{}
	for _, value := range values {
		out[value] = true
	}
	return out
}
func generate(a *appir.App, name string, e appir.Entity) {
	fields := []string{"id"}
	for _, f := range e.Fields {
		fields = append(fields, f.Name)
	}
	fields = append(fields, "created_at", "updated_at", "version")
	if _, ok := a.Views[name+"_list"]; !ok {
		a.Views[name+"_list"] = appir.View{Name: name + "_list", Entity: name, Fields: fields, Policy: e.Policy, DefaultLimit: 50, MaxLimit: 200}
	}
	for _, op := range []string{"create", "update", "delete"} {
		n := name + "_" + op
		if _, ok := a.Actions[n]; !ok {
			a.Actions[n] = appir.Action{Name: n, Entity: name, Operation: op, Policy: e.Policy}
		}
	}
	for _, resource := range a.AdminResources {
		if resource.Entity == name {
			return
		}
	}
	a.AdminResources[name] = appir.AdminResource{Name: name, Entity: name}
}

func normalizeAdminResources(a *appir.App) {
	for name, resource := range a.AdminResources {
		entity, ok := a.Entities[resource.Entity]
		if !ok {
			continue
		}
		if resource.Label == "" {
			resource.Label = entity.Label
		}
		if resource.View == "" {
			resource.View = resource.Entity + "_list"
		}
		if resource.CreateAction == "" {
			resource.CreateAction = resource.Entity + "_create"
		}
		if resource.UpdateAction == "" {
			resource.UpdateAction = resource.Entity + "_update"
		}
		if resource.DeleteAction == "" {
			resource.DeleteAction = resource.Entity + "_delete"
		}
		if resource.LabelField == "" {
			resource.LabelField = "id"
			for _, candidate := range []string{"title", "name", "email"} {
				for _, field := range entity.Fields {
					if field.Name == candidate {
						resource.LabelField = candidate
						break
					}
				}
				if resource.LabelField != "id" {
					break
				}
			}
		}
		if len(resource.List.Columns) == 0 {
			resource.List.Columns = []string{"id"}
			for i, field := range entity.Fields {
				if i == 4 {
					break
				}
				resource.List.Columns = append(resource.List.Columns, field.Name)
			}
			resource.List.Columns = append(resource.List.Columns, "updated_at")
		}
		if len(resource.List.Search) == 0 {
			for _, field := range entity.Fields {
				if field.Type == "string" || field.Type == "text" || field.Type == "richtext" || field.Type == "email" || field.Type == "url" {
					resource.List.Search = append(resource.List.Search, field.Name)
				}
			}
		}
		if len(resource.List.Filters) == 0 {
			for _, field := range entity.Fields {
				if field.Type == "enum" || field.Type == "boolean" || field.Type == "relation" {
					resource.List.Filters = append(resource.List.Filters, field.Name)
				}
			}
		}
		if resource.List.PageSize == 0 {
			resource.List.PageSize = 25
		}
		if len(resource.Form.Fields) == 0 {
			for _, field := range entity.Fields {
				resource.Form.Fields = append(resource.Form.Fields, field.Name)
			}
		}
		if len(resource.Form.Readonly) == 0 {
			resource.Form.Readonly = []string{"created_at", "updated_at", "version"}
		}
		if resource.List.Columns == nil {
			resource.List.Columns = []string{}
		}
		if resource.List.Search == nil {
			resource.List.Search = []string{}
		}
		if resource.List.Filters == nil {
			resource.List.Filters = []string{}
		}
		if resource.List.Sort == nil {
			resource.List.Sort = []appir.Sort{}
		}
		if resource.Form.Fields == nil {
			resource.Form.Fields = []string{}
		}
		if resource.Form.Readonly == nil {
			resource.Form.Readonly = []string{}
		}
		if resource.Actions == nil {
			resource.Actions = []string{}
		}
		a.AdminResources[name] = resource
	}
}

func normalizeResourceListBlocks(a *appir.App) {
	for name, block := range a.Blocks {
		specification, registered := blockcap.Lookup(block.Type)
		if !registered || !specification.DerivesViewFromResource {
			continue
		}
		if resource, ok := a.AdminResources[block.Resource]; ok {
			block.View = resource.View
		}
		if block.Filters == nil {
			block.Filters = []string{}
		}
		if block.DefaultFilters == nil {
			block.DefaultFilters = map[string]any{}
		}
		a.Blocks[name] = block
	}
}

func normalizeBlocks(a *appir.App) {
	normalizeResourceListBlocks(a)
	for name, block := range a.Blocks {
		if block.Type != "view" || block.View == "" || block.Display != "" {
			continue
		}
		view, exists := a.Views[block.View]
		if !exists {
			continue
		}
		displayName := "_block_" + name
		renderer := appir.RendererFromPresentation(block.Presentation)
		if renderer.Type == "" {
			renderer.Type = "list"
		}
		pagerType := "cursor"
		if renderer.Type == "detail" || renderer.Type == "metric" || renderer.Type == "board" || renderer.Type == "tree" {
			pagerType = "none"
		}
		view.Displays[displayName] = appir.Display{Type: "block", Renderer: renderer, EmptyState: block.Presentation.EmptyState, Pager: appir.ViewPager{Type: pagerType, PageSize: view.DefaultLimit}}
		block.Display = displayName
		a.Views[block.View] = view
		a.Blocks[name] = block
	}
}
