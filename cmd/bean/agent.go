package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/release"
)

const cliAPIVersion = "bean.cli/v1alpha1"

const (
	exitOK = iota
	exitDefinition
	exitUsage
	exitRuntime
)

type cliEnvelope struct {
	APIVersion  string              `json:"apiVersion"`
	Command     string              `json:"command"`
	OK          bool                `json:"ok"`
	Result      any                 `json:"result,omitempty"`
	Diagnostics []machineDiagnostic `json:"diagnostics"`
}

type machineDiagnostic struct {
	Code       string               `json:"code"`
	Kind       string               `json:"kind,omitempty"`
	Name       string               `json:"name,omitempty"`
	Path       string               `json:"path,omitempty"`
	Value      any                  `json:"value,omitempty"`
	Message    string               `json:"message"`
	Candidates []string             `json:"candidates,omitempty"`
	Source     *definition.Position `json:"source,omitempty"`
	Related    *definition.Position `json:"related,omitempty"`
}

type validateResult struct {
	Name        string `json:"name"`
	Definitions int    `json:"definitions"`
	Checksum    string `json:"checksum"`
}

func runAgentCommand(args []string, stdout, stderr io.Writer) (int, bool) {
	if len(args) > 0 && args[0] == "capabilities" {
		return agentCapabilities(args[1:], stdout, stderr), true
	}
	if len(args) > 0 && args[0] == "schema" {
		return agentSchema(args[1:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "validate" {
		return agentValidate(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "inspect" {
		return agentInspect(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "plan" {
		return agentPlan(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "diff" {
		return agentDiff(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "init" {
		return agentAppInit(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "publish" {
		return agentAppPublish(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "app" && args[1] == "test" {
		return agentAppTest(args[2:], stdout, stderr), true
	}
	return 0, false
}

func agentCapabilities(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	if len(args) != 0 {
		return writeCommandFailure("capabilities", "capabilities accepts no arguments", exitUsage, jsonOutput, stdout, stderr)
	}
	result := compiler.AgentCapabilities(cliAPIVersion)
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "capabilities", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintf(stdout, "definition API: %s\nAppIR: %s\ndefinition kinds: %s\n", result.DefinitionAPIVersion, result.AppIRFormat, strings.Join(result.DefinitionKinds, ", "))
	}
	return exitOK
}

func agentSchema(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	var outputDirectory string
	var optionError error
	args, outputDirectory, optionError = removeOption(args, "--output")
	if optionError != nil {
		return writeCommandFailure("schema", optionError.Error(), exitUsage, jsonOutput, stdout, stderr)
	}
	if len(args) > 1 {
		return writeCommandFailure("schema", "usage: bean schema [Kind] [--json]", exitUsage, jsonOutput, stdout, stderr)
	}
	schemas := compiler.DefinitionSchemas()
	all := map[string]map[string]any{"Bean": compiler.ManifestSchema()}
	for kind, schema := range schemas {
		all[kind] = schema
	}
	if len(args) == 1 {
		schema, exists := all[args[0]]
		if !exists {
			return writeDefinitionFailure("schema", definition.Diagnostic{Code: "BEAN-E1101", Path: "kind", Message: "unsupported definition kind", Candidates: sortedSchemaNames(all)}, jsonOutput, stdout, stderr)
		}
		result := map[string]any{"kind": args[0], "schema": schema}
		if outputDirectory != "" {
			files, err := writeSchemaFiles(outputDirectory, map[string]map[string]any{args[0]: schema})
			if err != nil {
				return writeRuntimeFailure("schema", err, jsonOutput, stdout, stderr)
			}
			result["files"] = files
		}
		if jsonOutput {
			writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "schema", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
		} else {
			writePrettyJSON(stdout, schema)
		}
		return exitOK
	}
	result := map[string]any{"schemas": all}
	if outputDirectory != "" {
		files, err := writeSchemaFiles(outputDirectory, all)
		if err != nil {
			return writeRuntimeFailure("schema", err, jsonOutput, stdout, stderr)
		}
		result["files"] = files
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "schema", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintln(stdout, strings.Join(sortedSchemaNames(all), "\n"))
	}
	return exitOK
}

func agentValidate(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("app validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" {
		message := "--file is required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("app.validate", message, exitUsage, jsonOutput, stdout, stderr)
	}

	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		if jsonOutput {
			writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.validate", OK: false, Diagnostics: machineDiagnostics(diagnostics, filepath.Dir(*filename))})
		} else {
			for _, diagnostic := range diagnostics {
				fmt.Fprintln(stderr, diagnostic.Error())
			}
		}
		return exitDefinition
	}
	result := validateResult{Name: bundle.Name, Definitions: len(bundle.Definitions), Checksum: bundleChecksum(bundle)}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.validate", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintf(stdout, "valid: %s — %d definitions\n", bundle.Name, len(bundle.Definitions))
	}
	return exitOK
}

func agentInspect(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("app inspect", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	if err := flags.Parse(args); err != nil || *filename == "" || flags.NArg() != 0 && flags.NArg() != 2 {
		message := "usage: bean app inspect --file app.yaml [Kind name] [--json]"
		if err != nil {
			message = err.Error()
		}
		return writeCommandFailure("app.inspect", message, exitUsage, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.inspect", *filename, diagnostics, jsonOutput, stdout, stderr)
	}
	compiled := compiler.Compile("default", 1, bundle.Definitions)
	inspectable := redactedApp(compiled.App)
	result := map[string]any{"checksum": bundleChecksum(bundle), "appIRFormat": compiled.App.FormatVersion}
	if flags.NArg() == 0 {
		result["application"] = normalizedJSON(inspectable)
	} else {
		kind, name := flags.Arg(0), flags.Arg(1)
		value, references, exists := inspectedDefinition(inspectable, kind, name)
		if !exists {
			diagnostic := definition.Diagnostic{Code: "BEAN-E2001", Kind: kind, Name: name, Path: "definition", Message: "definition does not exist", Candidates: definitionNames(inspectable, kind)}
			return writeDefinitionFailure("app.inspect", diagnostic, jsonOutput, stdout, stderr)
		}
		result["kind"], result["name"] = kind, name
		result["definition"] = normalizedJSON(value)
		result["references"] = references
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.inspect", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		writePrettyJSON(stdout, result)
	}
	return exitOK
}

type migrationOutput struct {
	Descriptions []string `json:"descriptions"`
	Statements   []string `json:"statements"`
}

func agentPlan(args []string, stdout, stderr io.Writer) int {
	filename, target, jsonOutput, parseExit := parseSourceTarget("app.plan", args, stdout, stderr)
	if parseExit != exitOK {
		return parseExit
	}
	bundle, diagnostics := validateSource(filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.plan", filename, diagnostics, jsonOutput, stdout, stderr)
	}
	compiled, plan, _, err := previewCandidate(bundle, target)
	if err != nil {
		if isMigrationPlanError(err) {
			return writeMigrationFailure("app.plan", err, jsonOutput, stdout, stderr)
		}
		return writeRuntimeFailure("app.plan", err, jsonOutput, stdout, stderr)
	}
	if len(compiled.Diagnostics) > 0 {
		return writeSourceFailure("app.plan", filename, compiled.Diagnostics, jsonOutput, stdout, stderr)
	}
	result := map[string]any{
		"checksum":    bundleChecksum(bundle),
		"appIRFormat": compiled.App.FormatVersion,
		"migration":   migrationOutput{Descriptions: nonNilStrings(plan.Descriptions), Statements: nonNilStrings(plan.Statements)},
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.plan", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		writePrettyJSON(stdout, result)
	}
	return exitOK
}

func agentDiff(args []string, stdout, stderr io.Writer) int {
	filename, target, jsonOutput, parseExit := parseSourceTarget("app.diff", args, stdout, stderr)
	if parseExit != exitOK {
		return parseExit
	}
	bundle, diagnostics := validateSource(filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.diff", filename, diagnostics, jsonOutput, stdout, stderr)
	}
	compiled, _, current, err := previewCandidate(bundle, target)
	if err != nil {
		if isMigrationPlanError(err) {
			return writeMigrationFailure("app.diff", err, jsonOutput, stdout, stderr)
		}
		return writeRuntimeFailure("app.diff", err, jsonOutput, stdout, stderr)
	}
	if len(compiled.Diagnostics) > 0 {
		return writeSourceFailure("app.diff", filename, compiled.Diagnostics, jsonOutput, stdout, stderr)
	}
	changes := semanticDiff(current, compiled.App)
	result := map[string]any{"checksum": bundleChecksum(bundle), "changes": changes}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.diff", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		writePrettyJSON(stdout, result)
	}
	return exitOK
}

func agentAppInit(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("app init", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	directory := flags.String("dir", ".", "application source directory")
	name := flags.String("name", "Bean Application", "application name")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || strings.TrimSpace(*name) == "" {
		message := "application name must not be empty"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("app.init", message, exitUsage, jsonOutput, stdout, stderr)
	}
	if err := os.MkdirAll(*directory, 0o755); err != nil {
		return writeRuntimeFailure("app.init", err, jsonOutput, stdout, stderr)
	}
	manifest := filepath.Join(*directory, "app.yaml")
	if _, err := os.Stat(manifest); err == nil {
		return writeRuntimeFailure("app.init", fmt.Errorf("refusing to overwrite %s", manifest), jsonOutput, stdout, stderr)
	} else if !os.IsNotExist(err) {
		return writeRuntimeFailure("app.init", err, jsonOutput, stdout, stderr)
	}
	source := "apiVersion: " + definition.APIVersion + "\nname: " + yamlScalar(*name) + "\n"
	if err := os.WriteFile(manifest, []byte(source), 0o644); err != nil {
		return writeRuntimeFailure("app.init", err, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(manifest)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.init", manifest, diagnostics, jsonOutput, stdout, stderr)
	}
	result := map[string]any{"path": filepath.ToSlash(manifest), "name": bundle.Name, "definitions": len(bundle.Definitions), "checksum": bundleChecksum(bundle)}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.init", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintln(stdout, manifest)
	}
	return exitOK
}

func agentAppPublish(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("app publish", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	database := flags.String("db", "bean.db", "SQLite database")
	databaseURL := flags.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" {
		message := "--file is required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("app.publish", message, exitUsage, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.publish", *filename, diagnostics, jsonOutput, stdout, stderr)
	}
	target := databaseTarget(*database, *databaseURL)
	runtime, err := bootstrap.OpenURL(context.Background(), target, false)
	if err != nil {
		return writeRuntimeFailure("app.publish", err, jsonOutput, stdout, stderr)
	}
	defer runtime.DB.Close()
	published, plan, publishDiagnostics, err := runtime.Store.PublishBundle(context.Background(), "default", bundle)
	if err != nil {
		if isMigrationPlanError(err) {
			return writeMigrationFailure("app.publish", err, jsonOutput, stdout, stderr)
		}
		return writeRuntimeFailure("app.publish", err, jsonOutput, stdout, stderr)
	}
	if len(publishDiagnostics) > 0 {
		return writeSourceFailure("app.publish", *filename, publishDiagnostics, jsonOutput, stdout, stderr)
	}
	result := map[string]any{
		"checksum":  bundleChecksum(bundle),
		"release":   map[string]any{"id": published.ID, "version": published.Version, "status": published.Status},
		"migration": migrationOutput{Descriptions: nonNilStrings(plan.Descriptions), Statements: nonNilStrings(plan.Statements)},
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.publish", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintf(stdout, "published release %s version %d\n", published.ID, published.Version)
	}
	return exitOK
}

type smokeCheck struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func agentAppTest(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("app test", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" {
		message := "--file is required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("app.test", message, exitUsage, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("app.test", *filename, diagnostics, jsonOutput, stdout, stderr)
	}
	checks := []smokeCheck{{ID: "definition.load", Status: "passed"}, {ID: "compiler.validate", Status: "passed"}}
	directory, err := os.MkdirTemp("", "bean-app-test-")
	if err != nil {
		return writeRuntimeFailure("app.test", err, jsonOutput, stdout, stderr)
	}
	defer os.RemoveAll(directory)
	database := filepath.Join(directory, "test.db")
	runtime, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		return writeRuntimeFailure("app.test", err, jsonOutput, stdout, stderr)
	}
	compiled, _, err := runtime.Store.PreviewBundle(context.Background(), "default", bundle)
	if err == nil && len(compiled.Diagnostics) == 0 {
		checks = append(checks, smokeCheck{ID: "migration.plan", Status: "passed"})
		err = runtime.Store.SaveBundleExact(context.Background(), "default", bundle)
	}
	if err == nil {
		_, diagnostics, err = runtime.Store.Publish(context.Background(), "default")
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
		runtime, err = bootstrap.Open(context.Background(), database, false)
		if err == nil {
			if _, active := runtime.Kernel.Active(); !active {
				err = fmt.Errorf("published application was not active after restart")
			}
			runtime.DB.Close()
		}
	}
	if err != nil {
		return writeRuntimeFailure("app.test", err, jsonOutput, stdout, stderr)
	}
	checks = append(checks, smokeCheck{ID: "runtime.restart", Status: "passed"})
	result := map[string]any{"checksum": bundleChecksum(bundle), "checks": checks}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.test", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		for _, check := range checks {
			fmt.Fprintf(stdout, "%s: %s\n", check.ID, check.Status)
		}
	}
	return exitOK
}

func parseSourceTarget(command string, args []string, stdout, stderr io.Writer) (string, string, bool, int) {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	database := flags.String("db", "", "existing SQLite database")
	databaseURL := flags.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "existing database URL")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" {
		message := "--file is required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return "", "", jsonOutput, writeCommandFailure(command, message, exitUsage, jsonOutput, stdout, stderr)
	}
	target := databaseTarget(*database, *databaseURL)
	if target != "" && !strings.Contains(target, "://") {
		if _, err := os.Stat(target); err != nil {
			return "", "", jsonOutput, writeRuntimeFailure(command, fmt.Errorf("target database must already exist: %w", err), jsonOutput, stdout, stderr)
		}
	}
	return *filename, target, jsonOutput, exitOK
}

func previewCandidate(bundle definition.Bundle, target string) (compiler.Result, migration.Plan, *appir.App, error) {
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
	runtime, err := bootstrap.OpenInspection(context.Background(), target)
	if err != nil {
		return compiler.Result{}, migration.Plan{}, nil, err
	}
	defer runtime.DB.Close()
	compiled, plan, err := runtime.Store.PreviewBundle(context.Background(), "default", bundle)
	if err != nil {
		return compiled, plan, nil, err
	}
	current, err := runtime.Store.ActiveApp(context.Background(), "default")
	return compiled, plan, current, err
}

func writeCommandFailure(command, message string, exit int, jsonOutput bool, stdout, stderr io.Writer) int {
	diagnostic := machineDiagnostic{Code: "BEAN-E0002", Path: "command", Message: message}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: false, Diagnostics: []machineDiagnostic{diagnostic}})
	} else {
		fmt.Fprintln(stderr, message)
	}
	return exit
}

func writeDefinitionFailure(command string, diagnostic definition.Diagnostic, jsonOutput bool, stdout, stderr io.Writer) int {
	definition.ClassifyDiagnostics([]definition.Diagnostic{diagnostic})
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: false, Diagnostics: machineDiagnostics([]definition.Diagnostic{diagnostic}, ".")})
	} else {
		fmt.Fprintln(stderr, diagnostic.Error())
	}
	return exitDefinition
}

func writeSourceFailure(command, filename string, diagnostics []definition.Diagnostic, jsonOutput bool, stdout, stderr io.Writer) int {
	definition.ClassifyDiagnostics(diagnostics)
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: false, Diagnostics: machineDiagnostics(diagnostics, filepath.Dir(filename))})
	} else {
		for _, diagnostic := range diagnostics {
			fmt.Fprintln(stderr, diagnostic.Error())
		}
	}
	return exitDefinition
}

func writeRuntimeFailure(command string, err error, jsonOutput bool, stdout, stderr io.Writer) int {
	message := redactRuntimeMessage(err.Error())
	diagnostic := machineDiagnostic{Code: "BEAN-E0003", Path: "runtime", Message: message}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: false, Diagnostics: []machineDiagnostic{diagnostic}})
	} else {
		fmt.Fprintln(stderr, message)
	}
	return exitRuntime
}

func writeMigrationFailure(command string, err error, jsonOutput bool, stdout, stderr io.Writer) int {
	diagnostic := definition.Diagnostic{Code: "BEAN-E2701", Kind: "Release", Name: "default", Path: "migration", Message: err.Error()}
	return writeDefinitionFailure(command, diagnostic, jsonOutput, stdout, stderr)
}

func isMigrationPlanError(err error) bool {
	var target *release.MigrationPlanError
	return errors.As(err, &target)
}

func writeEnvelope(writer io.Writer, envelope cliEnvelope) {
	if envelope.Diagnostics == nil {
		envelope.Diagnostics = []machineDiagnostic{}
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(envelope)
}

func writePrettyJSON(writer io.Writer, value any) {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(value)
}

func machineDiagnostics(diagnostics []definition.Diagnostic, sourceRoot string) []machineDiagnostic {
	out := make([]machineDiagnostic, len(diagnostics))
	for index, diagnostic := range diagnostics {
		path := diagnostic.Path
		const unknownField = `json: unknown field "`
		if path == "spec" && strings.HasPrefix(diagnostic.Message, unknownField) {
			path += "." + strings.TrimSuffix(strings.TrimPrefix(diagnostic.Message, unknownField), `"`)
		}
		item := machineDiagnostic{Code: diagnostic.Code, Kind: diagnostic.Kind, Name: diagnostic.Name, Path: path, Value: diagnostic.Value, Message: diagnostic.Message, Candidates: diagnostic.Candidates}
		if diagnostic.Source.Path != "" {
			source := relativePosition(diagnostic.Source, sourceRoot)
			item.Source = &source
		}
		if diagnostic.Related != nil {
			related := relativePosition(*diagnostic.Related, sourceRoot)
			item.Related = &related
		}
		out[index] = item
	}
	return out
}

func relativePosition(position definition.Position, root string) definition.Position {
	path, err := filepath.Rel(root, position.Path)
	if err == nil && path != "." && !strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		position.Path = filepath.ToSlash(path)
	} else {
		position.Path = filepath.ToSlash(filepath.Base(position.Path))
	}
	return position
}

func removeFlag(args []string, name string) ([]string, bool) {
	out := make([]string, 0, len(args))
	found := false
	for _, argument := range args {
		if argument == name {
			found = true
			continue
		}
		out = append(out, argument)
	}
	return out, found
}

func removeOption(args []string, name string) ([]string, string, error) {
	out := make([]string, 0, len(args))
	value := ""
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if strings.HasPrefix(argument, name+"=") {
			if value != "" {
				return nil, "", fmt.Errorf("%s may be supplied only once", name)
			}
			value = strings.TrimPrefix(argument, name+"=")
			continue
		}
		if argument != name {
			out = append(out, argument)
			continue
		}
		if value != "" || index+1 >= len(args) {
			return nil, "", fmt.Errorf("%s requires one directory", name)
		}
		index++
		value = args[index]
	}
	return out, value, nil
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

func writeSchemaFiles(directory string, schemas map[string]map[string]any) ([]string, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	files := []string{}
	for _, kind := range sortedSchemaNames(schemas) {
		name := strings.ToLower(kind) + ".schema.json"
		path := filepath.Join(directory, name)
		encoded, err := json.MarshalIndent(schemas[kind], "", "  ")
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, '\n')
		if err = os.WriteFile(path, encoded, 0o644); err != nil {
			return nil, err
		}
		files = append(files, filepath.ToSlash(path))
	}
	return files, nil
}

func normalizedJSON(value any) any {
	encoded, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(encoded, &decoded)
	return normalizeJSONKeys(decoded)
}

func normalizeJSONKeys(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			runes := []rune(key)
			if len(runes) > 0 {
				runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
			}
			out[string(runes)] = normalizeJSONKeys(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = normalizeJSONKeys(typed[index])
		}
		return out
	default:
		return value
	}
}

type inspectedReference struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	Name string `json:"name"`
}

func inspectedDefinition(app *appir.App, kind, name string) (any, []inspectedReference, bool) {
	var value any
	var exists bool
	switch kind {
	case "Entity":
		value, exists = app.Entities[name]
	case "View":
		value, exists = app.Views[name]
	case "Action":
		value, exists = app.Actions[name]
	case "Policy":
		value, exists = app.Policies[name]
	case "Filter":
		value, exists = app.Filters[name]
	case "Webform":
		value, exists = app.Webforms[name]
	case "Block":
		value, exists = app.Blocks[name]
	case "Panel":
		value, exists = app.Panels[name]
	case "Page":
		value, exists = app.Pages[name]
	case "Role":
		value, exists = app.Roles[name]
	case "Menu":
		value, exists = app.Menus[name]
	case "Job":
		value, exists = app.Jobs[name]
	case "AdminResource":
		value, exists = app.AdminResources[name]
	case "LocalRegistration":
		if app.LocalRegistration != nil {
			value, exists = *app.LocalRegistration, true
		}
	}
	if !exists {
		return nil, nil, false
	}
	return value, definitionReferences(app, kind, name), true
}

func definitionNames(app *appir.App, kind string) []string {
	names := []string{}
	appendMap := func(values any) {
		value := reflect.ValueOf(values)
		for _, key := range value.MapKeys() {
			names = append(names, key.String())
		}
	}
	switch kind {
	case "Entity":
		appendMap(app.Entities)
	case "View":
		appendMap(app.Views)
	case "Action":
		appendMap(app.Actions)
	case "Policy":
		appendMap(app.Policies)
	case "Filter":
		appendMap(app.Filters)
	case "Webform":
		appendMap(app.Webforms)
	case "Block":
		appendMap(app.Blocks)
	case "Panel":
		appendMap(app.Panels)
	case "Page":
		appendMap(app.Pages)
	case "Role":
		appendMap(app.Roles)
	case "Menu":
		appendMap(app.Menus)
	case "Job":
		appendMap(app.Jobs)
	case "AdminResource":
		appendMap(app.AdminResources)
	}
	sort.Strings(names)
	return names
}

func definitionReferences(app *appir.App, kind, name string) []inspectedReference {
	references := []inspectedReference{}
	add := func(path, targetKind, targetName string) {
		if targetName != "" {
			references = append(references, inspectedReference{Path: path, Kind: targetKind, Name: targetName})
		}
	}
	switch kind {
	case "Entity":
		entity := app.Entities[name]
		add("policy", "Policy", entity.Policy)
		for index, field := range entity.Fields {
			if field.Relation != nil {
				add(fmt.Sprintf("fields.%d.relation.entity", index), "Entity", field.Relation.Entity)
			}
		}
	case "View":
		view := app.Views[name]
		add("entity", "Entity", view.Entity)
		add("policy", "Policy", view.Policy)
		for path, filter := range view.FieldFilters {
			add("fieldFilters."+path, "Filter", filter)
		}
		for index, relationship := range view.Relationships {
			add(fmt.Sprintf("relationships.%d.entity", index), "Entity", relationship.Entity)
		}
	case "Action":
		action := app.Actions[name]
		add("entity", "Entity", action.Entity)
		add("policy", "Policy", action.Policy)
		add("defaultRole", "Role", action.DefaultRole)
		for index, step := range action.Steps {
			add(fmt.Sprintf("steps.%d.entity", index), "Entity", step.Entity)
			add(fmt.Sprintf("steps.%d.view", index), "View", step.View)
			add(fmt.Sprintf("steps.%d.job", index), "Job", step.Job)
		}
	case "Webform":
		add("action", "Action", app.Webforms[name].Action)
	case "Block":
		block := app.Blocks[name]
		add("view", "View", block.View)
		add("entity", "Entity", block.Entity)
		add("webform", "Webform", block.Webform)
		add("action", "Action", block.Action)
		add("menu", "Menu", block.Menu)
		add("policy", "Policy", block.Policy)
		add("resource", "AdminResource", block.Resource)
		add("presentation.moveAction", "Action", block.Presentation.MoveAction)
	case "Panel":
		panel := app.Panels[name]
		add("policy", "Policy", panel.Policy)
		for regionIndex, region := range panel.Regions {
			for blockIndex, block := range region.Blocks {
				add(fmt.Sprintf("regions.%d.blocks.%d", regionIndex, blockIndex), "Block", block)
			}
		}
	case "Page":
		page := app.Pages[name]
		add("panel", "Panel", page.Panel)
		add("policy", "Policy", page.Policy)
	case "Menu":
		for index, item := range app.Menus[name].Items {
			add(fmt.Sprintf("items.%d.policy", index), "Policy", item.Policy)
		}
	case "Job":
		add("action", "Action", app.Jobs[name].Action)
	case "AdminResource":
		resource := app.AdminResources[name]
		add("entity", "Entity", resource.Entity)
		add("view", "View", resource.View)
		add("createAction", "Action", resource.CreateAction)
		add("updateAction", "Action", resource.UpdateAction)
		add("deleteAction", "Action", resource.DeleteAction)
		for index, action := range resource.Actions {
			add(fmt.Sprintf("actions.%d", index), "Action", action)
		}
	case "LocalRegistration":
		if app.LocalRegistration != nil {
			add("action", "Action", app.LocalRegistration.Action)
		}
	}
	sort.Slice(references, func(left, right int) bool {
		if references[left].Path != references[right].Path {
			return references[left].Path < references[right].Path
		}
		if references[left].Kind != references[right].Kind {
			return references[left].Kind < references[right].Kind
		}
		return references[left].Name < references[right].Name
	})
	return references
}

type semanticChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func semanticDiff(current, candidate *appir.App) []semanticChange {
	if current == nil {
		current = appir.Empty()
	}
	left := redactedApp(current)
	right := redactedApp(candidate)
	for _, app := range []*appir.App{left, right} {
		app.ReleaseID = ""
		app.AppID = ""
		app.Version = 0
		app.OpenAPI = nil
	}
	changes := []semanticChange{}
	diffValue("", normalizedJSON(left), normalizedJSON(right), &changes)
	return changes
}

func redactedApp(source *appir.App) *appir.App {
	redacted, _ := source.Clone()
	for name, view := range redacted.Views {
		redactExpression(view.Filter)
		redactExpression(view.ContextFilter)
		redacted.Views[name] = view
	}
	for name, action := range redacted.Actions {
		for stepIndex := range action.Steps {
			redactExpression(action.Steps[stepIndex].Where)
			redactExpression(action.Steps[stepIndex].Condition)
			for valueIndex := range action.Steps[stepIndex].Values {
				value := &action.Steps[stepIndex].Values[valueIndex].Value
				if value.Source == "literal" {
					value.Literal = json.RawMessage(`"[REDACTED]"`)
				}
			}
		}
		redacted.Actions[name] = action
	}
	for name, policy := range redacted.Policies {
		redactExpression(policy.Condition)
		redacted.Policies[name] = policy
	}
	for name, webform := range redacted.Webforms {
		redactFormExpressions(webform.Elements)
		redacted.Webforms[name] = webform
	}
	return redacted
}

func redactExpression(expression *expr.Expr) {
	if expression == nil {
		return
	}
	for _, value := range []*expr.Value{expression.Left, expression.Right} {
		if value != nil && value.Source == "literal" {
			value.Literal = "[REDACTED]"
		}
	}
	for index := range expression.Args {
		redactExpression(&expression.Args[index])
	}
}

func redactFormExpressions(elements []appir.FormElement) {
	for index := range elements {
		redactExpression(elements[index].Visible)
		redactExpression(elements[index].RequiredWhen)
		redactFormExpressions(elements[index].Children)
	}
}

func diffValue(path string, before, after any, changes *[]semanticChange) {
	leftMap, leftIsMap := before.(map[string]any)
	rightMap, rightIsMap := after.(map[string]any)
	if leftIsMap && rightIsMap {
		keys := map[string]bool{}
		for key := range leftMap {
			keys[key] = true
		}
		for key := range rightMap {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			left, leftExists := leftMap[key]
			right, rightExists := rightMap[key]
			switch {
			case !leftExists:
				*changes = append(*changes, semanticChange{Operation: "add", Path: childPath, After: right})
			case !rightExists:
				*changes = append(*changes, semanticChange{Operation: "remove", Path: childPath, Before: left})
			default:
				diffValue(childPath, left, right, changes)
			}
		}
		return
	}
	if !reflect.DeepEqual(before, after) {
		*changes = append(*changes, semanticChange{Operation: "change", Path: path, Before: before, After: after})
	}
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func yamlScalar(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

var (
	databasePassword = regexp.MustCompile(`(?i)((?:postgres|postgresql)://[^:/@\s]+:)[^@\s]+@`)
	passwordOption   = regexp.MustCompile(`(?i)(password=)[^&\s]+`)
)

func redactRuntimeMessage(message string) string {
	message = databasePassword.ReplaceAllString(message, `${1}REDACTED@`)
	return passwordOption.ReplaceAllString(message, `${1}REDACTED`)
}
