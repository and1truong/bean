package agentprotocol

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/appsource"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
)

const cliAPIVersion = "bean.cli/v1alpha1"

type schemaInput struct {
	Kind string `json:"kind"`
}

type fileInput struct {
	File string `json:"file"`
}

type inspectInput struct {
	File string `json:"file"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type targetInput struct {
	File   string `json:"file"`
	Target string `json:"target"`
}

type queryInput struct {
	Target string      `json:"target"`
	View   string      `json:"view"`
	Params view.Params `json:"params"`
}

type executeInput struct {
	Target string         `json:"target"`
	Action string         `json:"action"`
	Input  map[string]any `json:"input"`
}

type migrationOutput struct {
	Descriptions []string `json:"descriptions"`
	Statements   []string `json:"statements"`
}

type smokeCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func (s *Service) registerHandlers() {
	s.Register("bean.definition.capabilities", s.capabilities)
	s.Register("bean.definition.schema", s.schema)
	s.Register("bean.definition.validate", s.validate)
	s.Register("bean.definition.inspect", s.inspect)
	s.Register("bean.release.plan", s.plan)
	s.Register("bean.release.diff", s.diff)
	s.Register("bean.release.publish", s.publish)
	s.Register("bean.release.test", s.test)
	s.Register("bean.application.query", s.query)
	s.Register("bean.application.execute", s.execute)
}

func (s *Service) capabilities(_ context.Context, raw json.RawMessage, _ Principal) Outcome {
	if err := decodeInput(raw, &struct{}{}); err != nil {
		return invalidInput(err)
	}
	result := compiler.ProtocolCapabilities(cliAPIVersion, APIVersion)
	return success(result)
}

func (s *Service) schema(_ context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input schemaInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	schemas := compiler.DefinitionSchemas()
	all := map[string]map[string]any{"Bean": compiler.ManifestSchema()}
	for kind, schema := range schemas {
		all[kind] = schema
	}
	if input.Kind == "" {
		return success(map[string]any{"schemas": all})
	}
	schema, exists := all[input.Kind]
	if !exists {
		return diagnosticFailure(definition.Diagnostic{Code: "BEAN-E1101", Path: "kind", Message: "unsupported definition kind", Candidates: sortedSchemaNames(all)})
	}
	return success(map[string]any{"kind": input.Kind, "schema": schema})
}

func (s *Service) validate(_ context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input fileInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" {
		return invalidInput(fmt.Errorf("file is required"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	return success(map[string]any{"name": bundle.Name, "definitions": len(bundle.Definitions), "checksum": bundleChecksum(bundle)})
}

func (s *Service) inspect(_ context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input inspectInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" {
		return invalidInput(fmt.Errorf("file is required"))
	}
	if (input.Kind == "") != (input.Name == "") {
		return invalidInput(fmt.Errorf("kind and name must be supplied together"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	compiled := compiler.Compile("default", 1, bundle.Definitions)
	inspectable := RedactedApp(compiled.App)
	result := map[string]any{"checksum": bundleChecksum(bundle), "appIRFormat": compiled.App.FormatVersion}
	if input.Kind == "" {
		result["application"] = normalizedJSON(inspectable)
		return success(result)
	}
	value, references, exists := InspectedDefinition(inspectable, input.Kind, input.Name)
	if !exists {
		return diagnosticFailure(definition.Diagnostic{Code: "BEAN-E2001", Kind: input.Kind, Name: input.Name, Path: "definition", Message: "definition does not exist", Candidates: DefinitionNames(inspectable, input.Kind)})
	}
	result["kind"], result["name"] = input.Kind, input.Name
	result["definition"] = normalizedJSON(value)
	result["references"] = references
	return success(result)
}

func (s *Service) plan(ctx context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input targetInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" {
		return invalidInput(fmt.Errorf("file is required"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	compiled, plan, _, err := previewCandidate(ctx, bundle, input.Target)
	if err != nil {
		return operationError(err)
	}
	if len(compiled.Diagnostics) > 0 {
		return sourceFailure(input.File, compiled.Diagnostics)
	}
	return success(map[string]any{
		"checksum": bundleChecksum(bundle), "appIRFormat": compiled.App.FormatVersion,
		"migration": migrationOutput{Descriptions: nonNilStrings(plan.Descriptions), Statements: nonNilStrings(plan.Statements)},
	})
}

func (s *Service) diff(ctx context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input targetInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" {
		return invalidInput(fmt.Errorf("file is required"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	compiled, _, current, err := previewCandidate(ctx, bundle, input.Target)
	if err != nil {
		return operationError(err)
	}
	if len(compiled.Diagnostics) > 0 {
		return sourceFailure(input.File, compiled.Diagnostics)
	}
	return success(map[string]any{"checksum": bundleChecksum(bundle), "changes": SemanticDiff(current, compiled.App)})
}

func (s *Service) publish(ctx context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input targetInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" || input.Target == "" {
		return invalidInput(fmt.Errorf("file and target are required"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	runtime, err := bootstrap.OpenURL(ctx, input.Target, false)
	if err != nil {
		return operationError(err)
	}
	defer runtime.DB.Close()
	published, plan, diagnostics, err := runtime.Store.PublishBundle(ctx, "default", bundle)
	if err != nil {
		return operationError(err)
	}
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	return success(map[string]any{
		"checksum":  bundleChecksum(bundle),
		"release":   map[string]any{"id": published.ID, "version": published.Version, "status": published.Status},
		"migration": migrationOutput{Descriptions: nonNilStrings(plan.Descriptions), Statements: nonNilStrings(plan.Statements)},
	})
}

func (s *Service) test(ctx context.Context, raw json.RawMessage, _ Principal) Outcome {
	var input fileInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.File == "" {
		return invalidInput(fmt.Errorf("file is required"))
	}
	bundle, diagnostics := appsource.Validate(input.File)
	if len(diagnostics) > 0 {
		return sourceFailure(input.File, diagnostics)
	}
	checks := []smokeCheck{{ID: "definition.load", Status: "passed"}, {ID: "compiler.validate", Status: "passed"}}
	directory, err := os.MkdirTemp("", "bean-app-test-")
	if err != nil {
		return operationError(err)
	}
	defer os.RemoveAll(directory)
	database := filepath.Join(directory, "test.db")
	runtime, err := bootstrap.Open(ctx, database, false)
	if err != nil {
		return operationError(err)
	}
	compiled, _, err := runtime.Store.PreviewBundle(ctx, "default", bundle)
	if err == nil && len(compiled.Diagnostics) == 0 {
		checks = append(checks, smokeCheck{ID: "migration.plan", Status: "passed"})
		err = runtime.Store.SaveBundleExact(ctx, "default", bundle)
	}
	if err == nil {
		_, diagnostics, err = runtime.Store.Publish(ctx, "default")
		if len(diagnostics) > 0 {
			err = diagnostics[0]
		}
	}
	if err == nil {
		checks = append(checks, smokeCheck{ID: "release.publish", Status: "passed"})
	}
	closeErr := runtime.DB.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		runtime, err = bootstrap.Open(ctx, database, false)
		if err == nil {
			if _, active := runtime.Kernel.Active(); !active {
				err = fmt.Errorf("published application was not active after restart")
			}
			runtime.DB.Close()
		}
	}
	if err != nil {
		return operationError(err)
	}
	checks = append(checks, smokeCheck{ID: "runtime.restart", Status: "passed"})
	return success(map[string]any{"checksum": bundleChecksum(bundle), "checks": checks})
}

func (s *Service) query(ctx context.Context, raw json.RawMessage, principal Principal) Outcome {
	var input queryInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.Target == "" || input.View == "" {
		return invalidInput(fmt.Errorf("target and view are required"))
	}
	runtime, err := bootstrap.OpenURL(ctx, input.Target, false)
	if err != nil {
		return operationError(err)
	}
	defer runtime.DB.Close()
	app, exists := runtime.Kernel.Active()
	if !exists {
		return operationError(fmt.Errorf("target database has no active application"))
	}
	result, err := runtime.HTTP.Views.RunPage(ctx, app, input.View, input.Params, principal.Request)
	if err != nil {
		return operationError(err)
	}
	if result.Rows == nil {
		result.Rows = []dbal.Row{}
	}
	return success(result)
}

func (s *Service) execute(ctx context.Context, raw json.RawMessage, principal Principal) Outcome {
	var input executeInput
	if err := decodeInput(raw, &input); err != nil {
		return invalidInput(err)
	}
	if input.Target == "" || input.Action == "" || input.Input == nil {
		return invalidInput(fmt.Errorf("target, action, and input are required"))
	}
	runtime, err := bootstrap.OpenURL(ctx, input.Target, false)
	if err != nil {
		return operationError(err)
	}
	defer runtime.DB.Close()
	app, exists := runtime.Kernel.Active()
	if !exists {
		return operationError(fmt.Errorf("target database has no active application"))
	}
	result, err := runtime.HTTP.Actions.Execute(ctx, app, input.Action, input.Input, principal.Request)
	if err != nil {
		return operationError(err)
	}
	return success(result)
}

func decodeInput(raw json.RawMessage, value any) error {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("input must contain one JSON value")
		}
		return err
	}
	return nil
}

func previewCandidate(ctx context.Context, bundle definition.Bundle, target string) (compiler.Result, migration.Plan, *appir.App, error) {
	if target == "" {
		compiled := compiler.Compile("default", 1, bundle.Definitions)
		if len(compiled.Diagnostics) > 0 {
			return compiled, migration.Plan{}, nil, nil
		}
		plan, err := migration.Build(migration.Schema{}, compiled.Schema)
		if err != nil {
			err = &release.MigrationPlanError{Err: err}
		}
		return compiled, plan, nil, err
	}
	runtime, err := bootstrap.OpenInspection(ctx, target)
	if err != nil {
		return compiler.Result{}, migration.Plan{}, nil, err
	}
	defer runtime.DB.Close()
	compiled, plan, err := runtime.Store.PreviewBundle(ctx, "default", bundle)
	if err != nil {
		return compiled, plan, nil, err
	}
	current, err := runtime.Store.ActiveApp(ctx, "default")
	return compiled, plan, current, err
}

func invalidInput(err error) Outcome {
	return failure("", "BEAN-P1003", "invalid operation input: "+err.Error())
}

func diagnosticFailure(diagnostic definition.Diagnostic) Outcome {
	definition.ClassifyDiagnostics([]definition.Diagnostic{diagnostic})
	return Outcome{OK: false, Diagnostics: []definition.Diagnostic{diagnostic}}
}

func sourceFailure(filename string, diagnostics []definition.Diagnostic) Outcome {
	definition.ClassifyDiagnostics(diagnostics)
	root := filepath.Dir(filename)
	out := make([]definition.Diagnostic, len(diagnostics))
	for index, diagnostic := range diagnostics {
		diagnostic.Source = relativePosition(diagnostic.Source, root)
		if diagnostic.Related != nil {
			related := relativePosition(*diagnostic.Related, root)
			diagnostic.Related = &related
		}
		out[index] = diagnostic
	}
	return Outcome{OK: false, Diagnostics: out}
}

func operationError(err error) Outcome {
	var migrationError *release.MigrationPlanError
	if errors.As(err, &migrationError) {
		return diagnosticFailure(definition.Diagnostic{Code: "BEAN-E2701", Kind: "Release", Name: "default", Path: "migration", Message: migrationError.Error()})
	}
	return failure("", "BEAN-P3001", redactRuntimeMessage(err.Error()))
}

func relativePosition(position definition.Position, root string) definition.Position {
	if position.Path == "" {
		return position
	}
	path, err := filepath.Rel(root, position.Path)
	if err == nil && path != "." && path != ".." && !filepath.IsAbs(path) {
		position.Path = filepath.ToSlash(path)
	} else {
		position.Path = filepath.ToSlash(filepath.Base(position.Path))
	}
	return position
}

func bundleChecksum(bundle definition.Bundle) string {
	encoded, _ := json.Marshal(bundle)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func sortedSchemaNames(schemas map[string]map[string]any) []string {
	names := make([]string, 0, len(schemas))
	for name := range schemas {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

var (
	databasePassword = regexp.MustCompile(`(?i)((?:postgres|postgresql)://[^:/@\s]+:)[^@\s]+@`)
	passwordOption   = regexp.MustCompile(`(?i)(password=)[^&\s]+`)
)

func redactRuntimeMessage(message string) string {
	message = databasePassword.ReplaceAllString(message, `${1}REDACTED@`)
	return passwordOption.ReplaceAllString(message, `${1}REDACTED`)
}
