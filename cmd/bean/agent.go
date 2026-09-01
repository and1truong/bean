package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/demoseed"
	"github.com/beanruntime/bean/internal/mcpstdio"
	"github.com/beanruntime/bean/internal/patterns"
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
	if len(args) >= 2 && args[0] == "mcp" && args[1] == "serve" {
		return agentMCPServe(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "agent" && args[1] == "call" {
		return agentCall(args[2:], stdout, stderr), true
	}
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
	if len(args) >= 2 && args[0] == "demo" && args[1] == "seed" {
		return agentDemoSeed(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "pattern" && args[1] == "inspect" {
		return agentPatternInspect(args[2:], stdout, stderr), true
	}
	if len(args) >= 2 && args[0] == "package" && args[1] == "verify" {
		return agentPackageVerify(args[2:], stdout, stderr), true
	}
	if len(args) >= 1 && args[0] == "package" {
		return agentPackageBuild(args[1:], stdout, stderr), true
	}
	return 0, false
}

func agentMCPServe(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mcp serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	allowed := flags.String("allow-plane", "definition", "comma-separated protocol planes")
	userID := flags.String("user-id", "", "runtime user ID")
	userEmail := flags.String("user-email", "", "runtime user email")
	roles := flags.String("roles", "", "comma-separated runtime roles")
	tenantID := flags.String("tenant-id", "", "runtime tenant ID")
	requestID := flags.String("request-id", "bean-mcp", "runtime request ID")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		message := "usage: bean mcp serve [--allow-plane PLANES]"
		if err != nil {
			message = err.Error()
		}
		fmt.Fprintln(stderr, message)
		return exitUsage
	}
	planes, err := agentprotocol.ParsePlanes(*allowed)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return exitUsage
	}
	principal := agentprotocol.Principal{Planes: planes, Request: beanctx.Request{TenantID: *tenantID, RequestID: *requestID}}
	roleList := splitNonEmpty(*roles)
	if *userID != "" || *userEmail != "" || len(roleList) > 0 {
		principal.Request.User = &beanctx.User{ID: *userID, Email: *userEmail, Roles: roleList}
	}
	server := mcpstdio.New(mcpstdio.Config{Service: agentprotocol.New(), Principal: principal, Version: version})
	if err = server.Serve(context.Background(), os.Stdin, stdout); err != nil {
		fmt.Fprintln(stderr, redactRuntimeMessage(err.Error()))
		return exitRuntime
	}
	return exitOK
}

func agentCall(args []string, stdout, stderr io.Writer) int {
	return agentCallService(agentprotocol.New(), args, stdout, stderr)
}

func agentCallService(service *agentprotocol.Service, args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return writeCommandFailure("agent.call", "usage: bean agent call OPERATION [--input request.json] [--allow-plane PLANES] [--json]", exitUsage, jsonOutput, stdout, stderr)
	}
	operation := args[0]
	flags := flag.NewFlagSet("agent call", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	inputFile := flags.String("input", "", "JSON operation input file, or - for stdin")
	allowed := flags.String("allow-plane", "definition,release,application", "comma-separated protocol planes")
	userID := flags.String("user-id", "", "runtime user ID")
	userEmail := flags.String("user-email", "", "runtime user email")
	roles := flags.String("roles", "", "comma-separated runtime roles")
	tenantID := flags.String("tenant-id", "", "runtime tenant ID")
	requestID := flags.String("request-id", "bean-agent-cli", "runtime request ID")
	if err := flags.Parse(args[1:]); err != nil || flags.NArg() != 0 {
		message := "usage: bean agent call OPERATION [--input request.json] [--allow-plane PLANES] [--json]"
		if err != nil {
			message = err.Error()
		}
		return writeCommandFailure("agent.call", message, exitUsage, jsonOutput, stdout, stderr)
	}
	planes, err := agentprotocol.ParsePlanes(*allowed)
	if err != nil {
		return writeCommandFailure("agent.call", err.Error(), exitUsage, jsonOutput, stdout, stderr)
	}
	raw := json.RawMessage(`{}`)
	if *inputFile != "" {
		var encoded []byte
		if *inputFile == "-" {
			encoded, err = io.ReadAll(os.Stdin)
		} else {
			encoded, err = os.ReadFile(*inputFile)
		}
		if err != nil {
			return writeRuntimeFailure("agent.call", err, jsonOutput, stdout, stderr)
		}
		raw = encoded
	}
	principal := agentprotocol.Principal{Planes: planes, Request: beanctx.Request{TenantID: *tenantID, RequestID: *requestID}}
	roleList := splitNonEmpty(*roles)
	if *userID != "" || *userEmail != "" || len(roleList) > 0 {
		principal.Request.User = &beanctx.User{ID: *userID, Email: *userEmail, Roles: roleList}
	}
	outcome := service.Call(context.Background(), operation, raw, principal)
	return writeProtocolOutcome("agent.call", outcome, jsonOutput, stdout, stderr)
}

func splitNonEmpty(value string) []string {
	values := []string{}
	for _, item := range strings.Split(value, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func protocolInput(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func callProtocol(operation string, input any) agentprotocol.Outcome {
	return agentprotocol.New().Call(context.Background(), operation, protocolInput(input), agentprotocol.Principal{Planes: agentprotocol.AllPlanes()})
}

func writeProtocolOutcome(command string, outcome agentprotocol.Outcome, jsonOutput bool, stdout, stderr io.Writer) int {
	if outcome.OK {
		if jsonOutput {
			writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: true, Result: outcome.Result, Diagnostics: []machineDiagnostic{}})
		} else {
			writePrettyJSON(stdout, outcome.Result)
		}
		return exitOK
	}
	diagnostics := machineDiagnostics(outcome.Diagnostics, ".")
	exit := exitDefinition
	if outcome.Error != nil {
		diagnostics = append(diagnostics, machineDiagnostic{Code: outcome.Error.Code, Path: "protocol", Message: outcome.Error.Message})
		exit = exitRuntime
		if outcome.Error.Code == "BEAN-P1001" || outcome.Error.Code == "BEAN-P1002" || outcome.Error.Code == "BEAN-P1003" {
			exit = exitUsage
		}
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: command, OK: false, Diagnostics: diagnostics})
	} else {
		for _, diagnostic := range diagnostics {
			fmt.Fprintln(stderr, diagnostic.Message)
		}
	}
	return exit
}

func agentPatternInspect(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	if len(args) != 1 {
		return writeCommandFailure("pattern.inspect", "usage: bean pattern inspect NAME [--json]", exitUsage, jsonOutput, stdout, stderr)
	}
	result, err := patterns.Inspect(args[0])
	if err != nil {
		return writeDefinitionFailure("pattern.inspect", definition.Diagnostic{Code: "BEAN-E2001", Kind: "Pattern", Name: args[0], Path: "name", Message: err.Error(), Candidates: patterns.Names()}, jsonOutput, stdout, stderr)
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "pattern.inspect", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		writePrettyJSON(stdout, result)
	}
	return exitOK
}

func agentDemoSeed(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	flags := flag.NewFlagSet("demo seed", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	filename := flags.String("file", "", "application manifest")
	database := flags.String("db", "bean.db", "SQLite database")
	databaseURL := flags.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL")
	seed := flags.Int64("seed", 1, "deterministic seed")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *filename == "" {
		message := "--file is required"
		if err != nil {
			message = err.Error()
		} else if flags.NArg() != 0 {
			message = "unexpected arguments: " + strings.Join(flags.Args(), " ")
		}
		return writeCommandFailure("demo.seed", message, exitUsage, jsonOutput, stdout, stderr)
	}
	bundle, diagnostics := validateSource(*filename)
	if len(diagnostics) > 0 {
		return writeSourceFailure("demo.seed", *filename, diagnostics, jsonOutput, stdout, stderr)
	}
	compiled := compiler.Compile("default", 1, bundle.Definitions)
	target := databaseTarget(*database, *databaseURL)
	runtime, err := bootstrap.OpenURL(context.Background(), target, false)
	if err != nil {
		return writeRuntimeFailure("demo.seed", err, jsonOutput, stdout, stderr)
	}
	defer runtime.DB.Close()
	active, exists := runtime.Kernel.Active()
	if !exists {
		return writeRuntimeFailure("demo.seed", fmt.Errorf("target database has no active application; run bean app publish first"), jsonOutput, stdout, stderr)
	}
	if !sameApplicationDefinition(active, compiled.App) {
		return writeRuntimeFailure("demo.seed", fmt.Errorf("active application does not match --file; publish it before seeding"), jsonOutput, stdout, stderr)
	}
	result, err := demoseed.Run(context.Background(), runtime.DB, active, *seed)
	if err != nil {
		return writeRuntimeFailure("demo.seed", err, jsonOutput, stdout, stderr)
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "demo.seed", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintf(stdout, "seeded %d records (checksum %s)\n", result.Records, result.Checksum)
	}
	return exitOK
}

func sameApplicationDefinition(active, candidate *appir.App) bool {
	activeCopy, activeErr := active.Clone()
	candidateCopy, candidateErr := candidate.Clone()
	if activeErr != nil || candidateErr != nil {
		return false
	}
	for _, app := range []*appir.App{activeCopy, candidateCopy} {
		app.ReleaseID = ""
		app.Version = 0
		app.OpenAPI = nil
	}
	return reflect.DeepEqual(activeCopy, candidateCopy)
}

func agentCapabilities(args []string, stdout, stderr io.Writer) int {
	args, jsonOutput := removeFlag(args, "--json")
	if len(args) != 0 {
		return writeCommandFailure("capabilities", "capabilities accepts no arguments", exitUsage, jsonOutput, stdout, stderr)
	}
	outcome := callProtocol("bean.definition.capabilities", struct{}{})
	if !outcome.OK {
		return writeProtocolOutcome("capabilities", outcome, jsonOutput, stdout, stderr)
	}
	result, ok := outcome.Result.(compiler.Capabilities)
	if !ok {
		return writeRuntimeFailure("capabilities", fmt.Errorf("invalid capabilities result"), jsonOutput, stdout, stderr)
	}
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
	kind := ""
	if len(args) == 1 {
		kind = args[0]
	}
	outcome := callProtocol("bean.definition.schema", map[string]any{"kind": kind})
	if !outcome.OK {
		return writeProtocolOutcome("schema", outcome, jsonOutput, stdout, stderr)
	}
	result, ok := outcome.Result.(map[string]any)
	if !ok {
		return writeRuntimeFailure("schema", fmt.Errorf("invalid schema result"), jsonOutput, stdout, stderr)
	}
	if kind != "" {
		schema, ok := result["schema"].(map[string]any)
		if !ok {
			return writeRuntimeFailure("schema", fmt.Errorf("invalid schema result"), jsonOutput, stdout, stderr)
		}
		if outputDirectory != "" {
			files, err := writeSchemaFiles(outputDirectory, map[string]map[string]any{kind: schema})
			if err != nil {
				return writeRuntimeFailure("schema", err, jsonOutput, stdout, stderr)
			}
			result["files"] = files
		}
		if jsonOutput {
			writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "schema", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
		} else {
			writePrettyJSON(stdout, result["schema"])
		}
		return exitOK
	}
	all, ok := result["schemas"].(map[string]map[string]any)
	if !ok {
		return writeRuntimeFailure("schema", fmt.Errorf("invalid schemas result"), jsonOutput, stdout, stderr)
	}
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

	outcome := callProtocol("bean.definition.validate", map[string]any{"file": *filename})
	if !outcome.OK {
		return writeProtocolOutcome("app.validate", outcome, jsonOutput, stdout, stderr)
	}
	var result validateResult
	if err := decodeProtocolResult(outcome.Result, &result); err != nil {
		return writeRuntimeFailure("app.validate", err, jsonOutput, stdout, stderr)
	}
	if jsonOutput {
		writeEnvelope(stdout, cliEnvelope{APIVersion: cliAPIVersion, Command: "app.validate", OK: true, Result: result, Diagnostics: []machineDiagnostic{}})
	} else {
		fmt.Fprintf(stdout, "valid: %s — %d definitions\n", result.Name, result.Definitions)
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
	input := map[string]any{"file": *filename}
	if flags.NArg() == 0 {
		// Inspect the whole immutable AppIR.
	} else {
		input["kind"], input["name"] = flags.Arg(0), flags.Arg(1)
	}
	return writeProtocolOutcome("app.inspect", callProtocol("bean.definition.inspect", input), jsonOutput, stdout, stderr)
}

func agentPlan(args []string, stdout, stderr io.Writer) int {
	filename, target, jsonOutput, parseExit := parseSourceTarget("app.plan", args, stdout, stderr)
	if parseExit != exitOK {
		return parseExit
	}
	return writeProtocolOutcome("app.plan", callProtocol("bean.release.plan", map[string]any{"file": filename, "target": target}), jsonOutput, stdout, stderr)
}

func agentDiff(args []string, stdout, stderr io.Writer) int {
	filename, target, jsonOutput, parseExit := parseSourceTarget("app.diff", args, stdout, stderr)
	if parseExit != exitOK {
		return parseExit
	}
	return writeProtocolOutcome("app.diff", callProtocol("bean.release.diff", map[string]any{"file": filename, "target": target}), jsonOutput, stdout, stderr)
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
	target := databaseTarget(*database, *databaseURL)
	outcome := callProtocol("bean.release.publish", map[string]any{"file": *filename, "target": target})
	if !outcome.OK || jsonOutput {
		return writeProtocolOutcome("app.publish", outcome, jsonOutput, stdout, stderr)
	}
	var result struct {
		Release struct {
			ID      string `json:"id"`
			Version int    `json:"version"`
		} `json:"release"`
	}
	if err := decodeProtocolResult(outcome.Result, &result); err != nil {
		return writeRuntimeFailure("app.publish", err, jsonOutput, stdout, stderr)
	}
	fmt.Fprintf(stdout, "published release %s version %d\n", result.Release.ID, result.Release.Version)
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
	outcome := callProtocol("bean.release.test", map[string]any{"file": *filename})
	if !outcome.OK || jsonOutput {
		return writeProtocolOutcome("app.test", outcome, jsonOutput, stdout, stderr)
	}
	var result struct {
		Checks []smokeCheck `json:"checks"`
	}
	if err := decodeProtocolResult(outcome.Result, &result); err != nil {
		return writeRuntimeFailure("app.test", err, jsonOutput, stdout, stderr)
	}
	for _, check := range result.Checks {
		fmt.Fprintf(stdout, "%s: %s\n", check.ID, check.Status)
	}
	return exitOK
}

func decodeProtocolResult(result any, target any) error {
	encoded, err := json.Marshal(result)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, target)
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
