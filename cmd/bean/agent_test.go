package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/agentprotocol"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/mcpstdio"
)

type testEnvelope struct {
	APIVersion  string `json:"apiVersion"`
	Command     string `json:"command"`
	OK          bool   `json:"ok"`
	Result      any    `json:"result"`
	Diagnostics []struct {
		Code       string   `json:"code"`
		Path       string   `json:"path"`
		Message    string   `json:"message"`
		Candidates []string `json:"candidates"`
		Source     struct {
			Path   string `json:"path"`
			Line   int    `json:"line"`
			Column int    `json:"column"`
		} `json:"source"`
	} `json:"diagnostics"`
}

func TestGenericAgentCallUsesSharedProtocolResult(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	request := filepath.Join(directory, "request.json")
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Generic Agent Call\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(map[string]any{"file": manifest})
	if err := os.WriteFile(request, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"agent", "call", "bean.definition.validate", "--input", request, "--json"}, &stdout, &stderr)
	if exit != exitOK || stderr.Len() != 0 {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	var envelope testEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope.Result.(map[string]any)
	if !envelope.OK || envelope.Command != "agent.call" || result["name"] != "Generic Agent Call" {
		t.Fatalf("envelope=%+v", envelope)
	}
}

func TestGenericAgentCallChecksPlaneBeforeInputWork(t *testing.T) {
	directory := t.TempDir()
	request := filepath.Join(directory, "request.json")
	missing := filepath.Join(directory, "secret-missing.yaml")
	encoded, _ := json.Marshal(map[string]any{"file": missing})
	if err := os.WriteFile(request, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	exit := execute([]string{"agent", "call", "bean.definition.validate", "--allow-plane", "release", "--input", request, "--json"}, &stdout, &stderr)
	if exit != exitUsage || stderr.Len() != 0 || strings.Contains(stdout.String(), missing) {
		t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	var envelope testEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.OK || len(envelope.Diagnostics) != 1 || envelope.Diagnostics[0].Code != "BEAN-P1002" {
		t.Fatalf("envelope=%+v", envelope)
	}
}

func TestCLIAndMCPExposeIdenticalSharedResultsForEveryOperation(t *testing.T) {
	principal := agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}
	operations := agentprotocol.New().Operations(principal)
	overrides := map[string]agentprotocol.Handler{}
	for _, item := range operations {
		operation := item
		overrides[operation.Name] = func(context.Context, json.RawMessage, agentprotocol.Principal) agentprotocol.Outcome {
			return agentprotocol.Outcome{OK: true, Result: map[string]any{"operation": operation.Name, "plane": operation.Plane}, Diagnostics: nil}
		}
	}
	service, err := agentprotocol.NewWithHandlers(overrides)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	inputFile := filepath.Join(directory, "request.json")
	if err := os.WriteFile(inputFile, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	for index, operation := range operations {
		var cliOut, cliErr bytes.Buffer
		if exit := agentCallService(service, []string{operation.Name, "--input", inputFile, "--json"}, &cliOut, &cliErr); exit != exitOK {
			t.Fatalf("%s CLI exit=%d stdout=%s stderr=%s", operation.Name, exit, cliOut.String(), cliErr.String())
		}
		var cli testEnvelope
		if err := json.Unmarshal(cliOut.Bytes(), &cli); err != nil {
			t.Fatal(err)
		}

		request, _ := json.Marshal(map[string]any{
			"jsonrpc": "2.0", "id": index + 1, "method": "tools/call",
			"params": map[string]any{
				"name": operation.Name, "arguments": map[string]any{},
				"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": mcpstdio.ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}},
			},
		})
		var mcpOut bytes.Buffer
		server := mcpstdio.New(mcpstdio.Config{Service: service, Principal: principal})
		if err := server.Serve(context.Background(), bytes.NewReader(append(request, '\n')), &mcpOut); err != nil {
			t.Fatal(err)
		}
		var mcp map[string]any
		if err := json.Unmarshal(mcpOut.Bytes(), &mcp); err != nil {
			t.Fatal(err)
		}
		structured := mcp["result"].(map[string]any)["structuredContent"]
		if !reflect.DeepEqual(cli.Result, structured) {
			t.Fatalf("%s CLI=%v MCP=%v", operation.Name, cli.Result, structured)
		}
	}
}

func TestCLIAndMCPExposeIdenticalLifecycleInspection(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := `apiVersion: bean/v1alpha1
name: Lifecycle Transport
---
kind: Entity
name: order
fields:
  - {name: status, type: enum, required: true, options: [pending, paid]}
---
kind: Lifecycle
name: order_payment
entity: order
initial: pending
transitions:
  pending: [paid]
---
kind: Action
name: pay_order
entity: order
operation: transition
lifecycle: order_payment
`
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"file": manifest, "kind": "Lifecycle", "name": "order_payment"}
	encoded, _ := json.Marshal(input)
	inputFile := filepath.Join(directory, "request.json")
	if err := os.WriteFile(inputFile, encoded, 0o600); err != nil {
		t.Fatal(err)
	}
	service := agentprotocol.New()
	var cliOut, cliErr bytes.Buffer
	if exit := agentCallService(service, []string{"bean.definition.inspect", "--allow-plane", "definition", "--input", inputFile, "--json"}, &cliOut, &cliErr); exit != exitOK {
		t.Fatalf("CLI exit=%d stdout=%s stderr=%s", exit, cliOut.String(), cliErr.String())
	}
	var cli testEnvelope
	if err := json.Unmarshal(cliOut.Bytes(), &cli); err != nil {
		t.Fatal(err)
	}
	request, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{
			"name": "bean.definition.inspect", "arguments": input,
			"_meta": map[string]any{"io.modelcontextprotocol/protocolVersion": mcpstdio.ProtocolVersion, "io.modelcontextprotocol/clientCapabilities": map[string]any{}},
		},
	})
	var mcpOut bytes.Buffer
	server := mcpstdio.New(mcpstdio.Config{Service: service, Principal: agentprotocol.Principal{Planes: map[agentprotocol.Plane]bool{agentprotocol.DefinitionPlane: true}}})
	if err := server.Serve(context.Background(), bytes.NewReader(append(request, '\n')), &mcpOut); err != nil {
		t.Fatal(err)
	}
	var mcp map[string]any
	if err := json.Unmarshal(mcpOut.Bytes(), &mcp); err != nil {
		t.Fatal(err)
	}
	structured := mcp["result"].(map[string]any)["structuredContent"]
	if !reflect.DeepEqual(cli.Result, structured) {
		t.Fatalf("CLI=%v MCP=%v", cli.Result, structured)
	}
	definition := cli.Result.(map[string]any)["definition"].(map[string]any)
	if definition["entity"] != "order" || definition["initial"] != "pending" {
		t.Fatalf("definition=%v", definition)
	}
}

func TestGenericCLIAuthorizesEveryPlaneIndependently(t *testing.T) {
	overrides := map[string]agentprotocol.Handler{}
	for _, item := range agentprotocol.New().Operations(agentprotocol.Principal{Planes: agentprotocol.AllPlanes()}) {
		operation := item
		overrides[operation.Name] = func(context.Context, json.RawMessage, agentprotocol.Principal) agentprotocol.Outcome {
			return agentprotocol.Outcome{OK: true, Result: operation.Name}
		}
	}
	service, err := agentprotocol.NewWithHandlers(overrides)
	if err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	inputFile := filepath.Join(directory, "request.json")
	if err := os.WriteFile(inputFile, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	representatives := map[agentprotocol.Plane]string{
		agentprotocol.DefinitionPlane:  "bean.definition.capabilities",
		agentprotocol.ReleasePlane:     "bean.release.plan",
		agentprotocol.ApplicationPlane: "bean.application.query",
	}
	for plane, allowed := range representatives {
		var stdout, stderr bytes.Buffer
		exit := agentCallService(service, []string{allowed, "--allow-plane", string(plane), "--input", inputFile, "--json"}, &stdout, &stderr)
		if exit != exitOK {
			t.Fatalf("plane=%s allowed=%s exit=%d stdout=%s stderr=%s", plane, allowed, exit, stdout.String(), stderr.String())
		}
		denied := representatives[agentprotocol.DefinitionPlane]
		if plane == agentprotocol.DefinitionPlane {
			denied = representatives[agentprotocol.ReleasePlane]
		}
		stdout.Reset()
		stderr.Reset()
		exit = agentCallService(service, []string{denied, "--allow-plane", string(plane), "--input", inputFile, "--json"}, &stdout, &stderr)
		if exit != exitUsage || !strings.Contains(stdout.String(), "BEAN-P1002") {
			t.Fatalf("plane=%s denied=%s exit=%d stdout=%s stderr=%s", plane, denied, exit, stdout.String(), stderr.String())
		}
	}
}

func TestAgentValidateJSONUsesStableEnvelopeAndDiagnostic(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Broken\n---\nkind: Entity\nname: candidate\nfields: []\nlabell: Candidate\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	exit := execute([]string{"app", "validate", "--file", manifest, "--json"}, &stdout, &stderr)
	if exit != exitDefinition {
		t.Fatalf("exit = %d, want %d; stderr=%s", exit, exitDefinition, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("JSON command wrote stderr: %s", stderr.String())
	}
	var envelope testEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout.String())
	}
	if envelope.APIVersion != cliAPIVersion || envelope.Command != "app.validate" || envelope.OK {
		t.Fatalf("envelope = %#v", envelope)
	}
	if len(envelope.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %#v", envelope.Diagnostics)
	}
	diagnostic := envelope.Diagnostics[0]
	if diagnostic.Code != "BEAN-E1002" || diagnostic.Path != "spec.labell" || diagnostic.Source.Line != 7 {
		t.Fatalf("diagnostic = %#v", diagnostic)
	}
	if len(diagnostic.Candidates) == 0 || diagnostic.Candidates[0] != "label" {
		t.Fatalf("diagnostic candidates = %#v", diagnostic.Candidates)
	}
	if diagnostic.Source.Path != "app.yaml" {
		t.Fatalf("source path = %q, want source-relative app.yaml", diagnostic.Source.Path)
	}
}

func TestAgentValidateJSONSuccessIsDeterministic(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Candidates\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\nlabel: Candidate\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	outputs := make([]string, 2)
	for index := range outputs {
		var stdout, stderr bytes.Buffer
		if exit := execute([]string{"app", "validate", "--json", "--file", manifest}, &stdout, &stderr); exit != exitOK {
			t.Fatalf("run %d exit = %d; stderr=%s; stdout=%s", index, exit, stderr.String(), stdout.String())
		}
		outputs[index] = stdout.String()
	}
	if outputs[0] != outputs[1] {
		t.Fatalf("non-deterministic output:\n%s\n%s", outputs[0], outputs[1])
	}
	var envelope testEnvelope
	if err := json.Unmarshal([]byte(outputs[0]), &envelope); err != nil {
		t.Fatal(err)
	}
	if !envelope.OK || envelope.Diagnostics == nil {
		t.Fatalf("envelope = %#v", envelope)
	}
}

func TestAgentDemoSeedPublishesDeterministicDatasetThroughActions(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Recruiting\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\n  - {name: stage, label: Stage, type: enum, required: true, options: [applied, interview]}\n---\nkind: View\nname: candidate_list\nentity: candidate\nfields: [id, name, stage, created_at, updated_at, version]\ndefaultLimit: 2\nmaxLimit: 2\n---\nkind: DemoSeed\nname: demo\nentities:\n  candidate: {count: 4, profile: people}\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	checksums := []string{}
	for index := 0; index < 2; index++ {
		database := filepath.Join(directory, fmt.Sprintf("bean-%d.db", index))
		var stdout, stderr bytes.Buffer
		if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
			t.Fatalf("publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
		}
		if index == 0 {
			stale := filepath.Join(directory, "stale.yaml")
			staleSource := source + "---\nkind: Action\nname: import_candidate\nentity: candidate\noperation: create\n"
			if err := os.WriteFile(stale, []byte(staleSource), 0o600); err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			stderr.Reset()
			if exit := execute([]string{"demo", "seed", "--file", stale, "--db", database, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitRuntime || !strings.Contains(stdout.String(), "does not match") {
				t.Fatalf("stale source seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
			}
		}
		stdout.Reset()
		stderr.Reset()
		if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitOK {
			t.Fatalf("seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
		}
		var envelope struct {
			OK     bool `json:"ok"`
			Result struct {
				Records  int    `json:"records"`
				Checksum string `json:"checksum"`
			} `json:"result"`
		}
		if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Result.Records != 4 || envelope.Result.Checksum == "" {
			t.Fatalf("seed result err=%v value=%#v output=%s", err, envelope, stdout.String())
		}
		checksums = append(checksums, envelope.Result.Checksum)
		runtime, err := bootstrap.Open(context.Background(), database, false)
		if err != nil {
			t.Fatal(err)
		}
		rows, err := runtime.DB.Select(context.Background(), dbal.Select{Table: "candidate"})
		if err != nil || len(rows) != 4 {
			t.Fatalf("rows=%d err=%v", len(rows), err)
		}
		runtime.DB.Close()
		stdout.Reset()
		stderr.Reset()
		if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitOK {
			t.Fatalf("idempotent seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
		}
		if index == 0 {
			runtime, err = bootstrap.Open(context.Background(), database, false)
			if err != nil {
				t.Fatal(err)
			}
			_, err = runtime.DB.Update(context.Background(), dbal.Update{Table: "candidate", Values: map[string]dbal.Value{"name": "tampered"}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["id"]}, ExpectedRows: 1})
			runtime.DB.Close()
			if err != nil {
				t.Fatal(err)
			}
			stdout.Reset()
			stderr.Reset()
			if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitRuntime || !strings.Contains(stdout.String(), "does not match") {
				t.Fatalf("unsafe reseed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
			}
		}
	}
	if checksums[0] != checksums[1] {
		t.Fatalf("checksums=%v", checksums)
	}
}

func TestAgentDemoSeedVerifiesCustomCreateActionOutput(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	database := filepath.Join(directory, "bean.db")
	source := "apiVersion: bean/v1alpha1\nname: Broken Seed\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, type: string, required: true}\n---\nkind: Action\nname: candidate_create\nentity: candidate\noperation: transaction\ninput:\n  name: {type: string, required: true}\nsteps:\n  - {op: return}\n---\nkind: DemoSeed\nname: demo\nentities:\n  candidate: {count: 1, profile: people}\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitRuntime || !strings.Contains(stdout.String(), "did not produce") {
		t.Fatalf("seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
}

func TestAgentDemoSeedAllocatesIDsForCustomCreateSteps(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	database := filepath.Join(directory, "bean.db")
	source := "apiVersion: bean/v1alpha1\nname: Custom Seed\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, type: string, required: true}\n---\nkind: Entity\nname: audit_note\nfields:\n  - {name: name, type: string, required: true}\n---\nkind: Action\nname: candidate_create\nentity: candidate\noperation: transaction\ninput:\n  name: {type: string, required: true}\nsteps:\n  - op: create\n    values:\n      name: $input.name\n  - op: create\n    entity: audit_note\n    values:\n      name: $input.name\n---\nkind: DemoSeed\nname: demo\nentities:\n  candidate: {count: 1, profile: people}\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	runtime, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	rows, err := runtime.DB.Select(context.Background(), dbal.Select{Table: "audit_note"})
	if err != nil || len(rows) != 1 {
		t.Fatalf("custom step rows=%v err=%v", rows, err)
	}
}

func TestAgentDemoSeedInspectsAllPhysicalRowsBeforeWriting(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	database := filepath.Join(directory, "bean.db")
	source := "apiVersion: bean/v1alpha1\nname: Partial Target\n---\nkind: Entity\nname: a_empty\nfields:\n  - {name: name, type: string, required: true}\n---\nkind: Entity\nname: z_existing\nfields:\n  - {name: name, type: string, required: true}\n---\nkind: View\nname: z_existing_list\nentity: z_existing\nfields: [id, name, created_at, updated_at, version]\nfilter:\n  left: {name: name, source: record}\n  op: eq\n  right: {literal: generated, source: literal}\ndefaultLimit: 1\nmaxLimit: 1\n---\nkind: DemoSeed\nname: demo\nentities:\n  a_empty: {count: 1}\n  z_existing: {count: 1}\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	runtime, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	stamp := "2026-01-01T00:00:00Z"
	if _, err = runtime.DB.Insert(context.Background(), dbal.Insert{Table: "z_existing", Values: map[string]dbal.Value{"id": "00000000-0000-4000-8000-000000000001", "name": "hidden", "created_at": stamp, "updated_at": stamp, "version": 1}}); err != nil {
		t.Fatal(err)
	}
	runtime.DB.Close()
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"demo", "seed", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitRuntime || !strings.Contains(stdout.String(), "non-empty target") {
		t.Fatalf("seed exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	runtime, err = bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(context.Background(), dbal.Select{Table: "a_empty"})
	runtime.DB.Close()
	if err != nil || len(rows) != 0 {
		t.Fatalf("seed wrote before inspecting all entities: rows=%v err=%v", rows, err)
	}
}

func TestAgentCapabilitiesAndSchemaAreSelfDescribing(t *testing.T) {
	for _, test := range []struct {
		args    []string
		command string
		assert  func(*testing.T, map[string]any)
	}{
		{args: []string{"capabilities", "--json"}, command: "capabilities", assert: func(t *testing.T, result map[string]any) {
			if result["definitionAPIVersion"] != "bean/v1alpha1" || result["appIRFormat"] != "bean/appir/v9" {
				t.Fatalf("capabilities = %#v", result)
			}
			if len(result["definitionKinds"].([]any)) < 10 || len(result["fieldTypes"].([]any)) < 10 || len(result["sequenceFrameLayouts"].([]any)) < 10 || len(result["contentElementTypes"].([]any)) != 8 {
				t.Fatalf("capability vocabulary is incomplete: %#v", result)
			}
		}},
		{args: []string{"schema", "Entity", "--json"}, command: "schema", assert: func(t *testing.T, result map[string]any) {
			schema := result["schema"].(map[string]any)
			if schema["$id"] != "https://bean.build/schemas/entity.schema.json" || schema["additionalProperties"] != false {
				t.Fatalf("schema = %#v", schema)
			}
			properties := schema["properties"].(map[string]any)
			if properties["fields"] == nil || properties["labell"] != nil {
				t.Fatalf("properties = %#v", properties)
			}
		}},
	} {
		t.Run(test.command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exit := execute(test.args, &stdout, &stderr); exit != exitOK {
				t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
			}
			firstOutput := stdout.String()
			var decoded struct {
				APIVersion string         `json:"apiVersion"`
				Command    string         `json:"command"`
				OK         bool           `json:"ok"`
				Result     map[string]any `json:"result"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.APIVersion != cliAPIVersion || decoded.Command != test.command || !decoded.OK {
				t.Fatalf("envelope = %#v", decoded)
			}
			test.assert(t, decoded.Result)
			stdout.Reset()
			stderr.Reset()
			if exit := execute(test.args, &stdout, &stderr); exit != exitOK || stdout.String() != firstOutput {
				t.Fatalf("non-deterministic repeat exit=%d stderr=%s\nfirst=%s\nsecond=%s", exit, stderr.String(), firstOutput, stdout.String())
			}
		})
	}
}

func TestAgentPatternInspectReturnsOrdinaryDefinitions(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"pattern", "inspect", "workflow_resource", "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			Name         string   `json:"name"`
			Definitions  []any    `json:"definitions"`
			Capabilities []string `json:"requiredCapabilities"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil || !envelope.OK || envelope.Result.Name != "workflow_resource" || len(envelope.Result.Definitions) == 0 || len(envelope.Result.Capabilities) == 0 {
		t.Fatalf("err=%v envelope=%#v output=%s", err, envelope, stdout.String())
	}
}

func TestAgentPackageIsVerifiableAndSourceIndependent(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Packaged\n---\nkind: Entity\nname: item\nfields:\n  - {name: enabled, label: Enabled, type: boolean, required: true}\n---\nkind: DemoSeed\nname: demo\nentities:\n  item: {count: 3, profile: auto}\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "dist", "demo")
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"package", "--file", manifest, "--output", output, "--seed", "42", "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("package exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	for _, name := range []string{"bean", "bean.db", "manifest.json"} {
		if _, err := os.Stat(filepath.Join(output, name)); err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
	}
	packageManifestBefore, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	broken := filepath.Join(directory, "broken.yaml")
	if err = os.WriteFile(broken, []byte("apiVersion: bean/v1alpha1\nname: Broken\n---\nkind: Missing\nname: broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"package", "--file", broken, "--output", output, "--json"}, &stdout, &stderr); exit != exitDefinition {
		t.Fatalf("invalid replacement exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	packageManifestAfter, err := os.ReadFile(filepath.Join(output, "manifest.json"))
	if err != nil || !bytes.Equal(packageManifestBefore, packageManifestAfter) {
		t.Fatalf("failed package changed destination: err=%v", err)
	}
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"package", "verify", "--dir", output, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("verify exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	file, err := os.OpenFile(filepath.Join(output, "bean"), os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.Write([]byte("tampered")); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"package", "verify", "--dir", output, "--json"}, &stdout, &stderr); exit != exitRuntime || !strings.Contains(stdout.String(), "checksum mismatch") {
		t.Fatalf("tampered verify exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
}

func TestAgentInspectPlanAndDiffSourceWithoutDatabase(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Candidates\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\nlabel: Candidate\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		command string
		args    []string
		assert  func(*testing.T, map[string]any)
	}{
		{command: "app.inspect", args: []string{"app", "inspect", "--file", manifest, "Entity", "candidate", "--json"}, assert: func(t *testing.T, result map[string]any) {
			definition := result["definition"].(map[string]any)
			if definition["name"] != "candidate" || definition["label"] != "Candidate" {
				t.Fatalf("definition = %#v", definition)
			}
			if refs := result["references"].([]any); len(refs) != 0 {
				t.Fatalf("references = %#v", refs)
			}
		}},
		{command: "app.plan", args: []string{"app", "plan", "--file", manifest, "--json"}, assert: func(t *testing.T, result map[string]any) {
			migration := result["migration"].(map[string]any)
			descriptions := migration["descriptions"].([]any)
			if len(descriptions) == 0 || descriptions[0] != "create entity candidate" {
				t.Fatalf("migration = %#v", migration)
			}
		}},
		{command: "app.diff", args: []string{"app", "diff", "--file", manifest, "--json"}, assert: func(t *testing.T, result map[string]any) {
			if len(result["changes"].([]any)) == 0 {
				t.Fatalf("diff = %#v", result)
			}
		}},
	} {
		t.Run(test.command, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if exit := execute(test.args, &stdout, &stderr); exit != exitOK {
				t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
			}
			firstOutput := stdout.String()
			var decoded struct {
				Command string         `json:"command"`
				OK      bool           `json:"ok"`
				Result  map[string]any `json:"result"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.Command != test.command || !decoded.OK {
				t.Fatalf("envelope = %#v", decoded)
			}
			test.assert(t, decoded.Result)
			stdout.Reset()
			stderr.Reset()
			if exit := execute(test.args, &stdout, &stderr); exit != exitOK || stdout.String() != firstOutput {
				t.Fatalf("non-deterministic repeat exit=%d stderr=%s\nfirst=%s\nsecond=%s", exit, stderr.String(), firstOutput, stdout.String())
			}
		})
	}
	if matches, err := filepath.Glob(filepath.Join(directory, "*.db*")); err != nil || len(matches) != 0 {
		t.Fatalf("read-only source commands created database files: %v %v", matches, err)
	}
}

func TestAgentInitPublishDiffAndLifecycleTest(t *testing.T) {
	directory := t.TempDir()
	sourceDirectory := filepath.Join(directory, "source")
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"app", "init", "--dir", sourceDirectory, "--name", "Candidates", "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("init exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	manifest := filepath.Join(sourceDirectory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Candidates\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\nlabel: Candidate\n---\nkind: Role\nname: recruiter\npermissions: []\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(directory, "bean.db")

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var published struct {
		OK     bool `json:"ok"`
		Result struct {
			Checksum string `json:"checksum"`
			Release  struct {
				Version int `json:"version"`
			} `json:"release"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &published); err != nil || !published.OK || published.Result.Checksum == "" || published.Result.Release.Version != 1 {
		t.Fatalf("publish result err=%v value=%#v output=%s", err, published, stdout.String())
	}

	runtime, err := bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	releasesBefore, err := runtime.Store.Releases(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	draftBefore, err := runtime.Store.Draft(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	runtime.DB.Close()

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "plan", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("plan exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	runtime, err = bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	releasesAfter, _ := runtime.Store.Releases(context.Background(), "default")
	draftAfter, _ := runtime.Store.Draft(context.Background(), "default")
	runtime.DB.Close()
	if len(releasesBefore) != len(releasesAfter) || len(draftBefore) != len(draftAfter) {
		t.Fatalf("plan mutated database: releases %d->%d draft %d->%d", len(releasesBefore), len(releasesAfter), len(draftBefore), len(draftAfter))
	}

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "diff", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("diff exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var diff struct {
		Result struct {
			Changes []any `json:"changes"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &diff); err != nil || len(diff.Result.Changes) != 0 {
		t.Fatalf("diff err=%v value=%#v output=%s", err, diff, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "test", "--file", manifest, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("test exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var smoke struct {
		Result struct {
			Checks []struct {
				ID       string         `json:"id"`
				Status   string         `json:"status"`
				Source   map[string]any `json:"source"`
				Evidence map[string]any `json:"evidence"`
			} `json:"checks"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &smoke); err != nil || len(smoke.Result.Checks) < 7 {
		t.Fatalf("smoke err=%v value=%#v output=%s", err, smoke, stdout.String())
	}
	generated := false
	for _, check := range smoke.Result.Checks {
		if check.ID == "" || check.Status != "passed" {
			t.Fatalf("check = %#v", check)
		}
		if check.ID == "generated/schema/Entity/candidate" {
			generated = check.Source["kind"] == "Entity" && check.Source["name"] == "candidate" && check.Evidence["contract"] == "canonical-schema"
		}
	}
	if !generated {
		t.Fatalf("generated schema evidence missing: %#v", smoke.Result.Checks)
	}

	withoutRole := "apiVersion: bean/v1alpha1\nname: Candidates\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\nlabel: Candidate\n"
	if err := os.WriteFile(manifest, []byte(withoutRole), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "publish", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("exact publish exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	runtime, err = bootstrap.Open(context.Background(), database, false)
	if err != nil {
		t.Fatal(err)
	}
	draft, err := runtime.Store.Draft(context.Background(), "default")
	runtime.DB.Close()
	if err != nil || len(draft) != 1 || draft[0].Kind != "Entity" {
		t.Fatalf("exact draft err=%v definitions=%#v", err, draft)
	}

	incompatible := strings.Replace(withoutRole, "type: string", "type: text", 1)
	if err := os.WriteFile(manifest, []byte(incompatible), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "plan", "--file", manifest, "--db", database, "--json"}, &stdout, &stderr); exit != exitDefinition {
		t.Fatalf("incompatible plan exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var rejected testEnvelope
	if err := json.Unmarshal(stdout.Bytes(), &rejected); err != nil || len(rejected.Diagnostics) != 1 || rejected.Diagnostics[0].Code != "BEAN-E2701" {
		t.Fatalf("migration rejection err=%v envelope=%#v output=%s", err, rejected, stdout.String())
	}
}

func TestSchemaCanPublishCanonicalFiles(t *testing.T) {
	directory := t.TempDir()
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"schema", "--output", directory, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	for _, name := range []string{"bean.schema.json", "entity.schema.json", "view.schema.json", "action.schema.json", "lifecycle.schema.json", "page.schema.json"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		var schema map[string]any
		if err = json.Unmarshal(data, &schema); err != nil || schema["$schema"] == nil || schema["$id"] == nil {
			t.Fatalf("%s is not a canonical schema: err=%v schema=%#v", name, err, schema)
		}
	}
}

func TestRuntimeFailuresRedactDatabaseCredentials(t *testing.T) {
	var stdout, stderr bytes.Buffer
	writeRuntimeFailure("app.plan", errors.New("connect postgres://bean:super-secret@db.example/bean?password=also-secret"), true, &stdout, &stderr)
	if strings.Contains(stdout.String(), "super-secret") || strings.Contains(stdout.String(), "also-secret") {
		t.Fatalf("credential leaked: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "REDACTED") || stderr.Len() != 0 {
		t.Fatalf("redaction output stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestInspectionAndDiffRedactActionLiterals(t *testing.T) {
	application := appir.Empty()
	literalExpression := func(secret string) *expr.Expr {
		return &expr.Expr{Op: "and", Args: []expr.Expr{{Op: "eq", Left: &expr.Value{Source: "literal", Literal: secret}, Right: &expr.Value{Source: "input", Name: "value"}}}}
	}
	application.Actions["call_service"] = appir.Action{
		Name: "call_service",
		Steps: []appir.Step{{
			Op:        "return",
			Where:     literalExpression("action-where-secret"),
			Condition: literalExpression("action-condition-secret"),
			Values: []appir.Assignment{{
				Field: "api_key",
				Value: appir.ValueBinding{Source: "literal", Literal: json.RawMessage(`"super-secret"`)},
			}},
		}},
	}
	application.Views["private"] = appir.View{Name: "private", Filter: literalExpression("view-filter-secret"), ContextFilter: literalExpression("view-context-secret")}
	application.Policies["private"] = appir.Policy{Name: "private", Condition: literalExpression("policy-secret")}
	application.Webforms["private"] = appir.Webform{Name: "private", Elements: []appir.FormElement{{
		Name:         "group",
		Visible:      literalExpression("form-visible-secret"),
		RequiredWhen: literalExpression("form-required-secret"),
		Children:     []appir.FormElement{{Name: "child", Visible: literalExpression("form-child-secret")}},
	}}}
	inspectable := agentprotocol.RedactedApp(application)
	inspections := make([][]byte, 0, 4)
	for _, target := range [][2]string{{"Action", "call_service"}, {"View", "private"}, {"Policy", "private"}, {"Webform", "private"}} {
		definition, _, exists := compiler.InspectDefinition(inspectable, target[0], target[1])
		if !exists {
			t.Fatalf("redacted %s is not inspectable", target[0])
		}
		inspection, err := json.Marshal(definition)
		if err != nil {
			t.Fatal(err)
		}
		inspections = append(inspections, inspection)
	}
	inspection := bytes.Join(inspections, nil)
	diff, err := json.Marshal(agentprotocol.SemanticDiff(appir.Empty(), application))
	if err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string][]byte{"inspection": inspection, "diff": diff} {
		for _, secret := range []string{"super-secret", "action-where-secret", "action-condition-secret", "view-filter-secret", "view-context-secret", "policy-secret", "form-visible-secret", "form-required-secret", "form-child-secret"} {
			if bytes.Contains(output, []byte(secret)) {
				t.Fatalf("%s leaked %q: %s", name, secret, output)
			}
		}
		if !bytes.Contains(output, []byte("[REDACTED]")) {
			t.Fatalf("%s omitted redaction: %s", name, output)
		}
	}
}

func TestInspectionResolvesEveryMaintainedDefinitionReference(t *testing.T) {
	for _, application := range []string{"asana", "ats", "blog", "booking", "cms", "commerce", "community", "crm", "presentation", "saas", "tracker"} {
		bundle, err := examples.Load(application)
		if err != nil {
			t.Fatal(err)
		}
		compiled := compiler.Compile("default", 1, bundle.Definitions)
		if len(compiled.Diagnostics) != 0 {
			t.Fatalf("%s diagnostics=%v", application, compiled.Diagnostics)
		}
		for _, source := range bundle.Definitions {
			definition, references, exists := compiler.InspectDefinition(compiled.App, source.Kind, source.Metadata.Name)
			if !exists || definition == nil {
				t.Errorf("%s: cannot inspect %s/%s", application, source.Kind, source.Metadata.Name)
				continue
			}
			for _, reference := range references {
				if _, _, targetExists := compiler.InspectDefinition(compiled.App, reference.Kind, reference.Name); !targetExists {
					t.Errorf("%s: %s/%s %s does not resolve %s/%s", application, source.Kind, source.Metadata.Name, reference.Path, reference.Kind, reference.Name)
				}
			}
		}
	}
}

func TestInspectionExposesCoreReferenceFamilies(t *testing.T) {
	bundle, err := examples.Load("asana")
	if err != nil {
		t.Fatal(err)
	}
	compiled := compiler.Compile("default", 1, bundle.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatal(compiled.Diagnostics)
	}
	for _, expected := range []struct {
		kind, name, path, targetKind, targetName string
	}{
		{"Entity", "task", "policy", "Policy", "public_access"},
		{"Entity", "task", "fields.0.relation.entity", "Entity", "project"},
		{"View", "project_root_tasks", "entity", "Entity", "task"},
		{"Action", "move_task", "policy", "Policy", "public_access"},
		{"Webform", "create_root_task_form", "action", "Action", "create_root_task"},
		{"Block", "project_board", "view", "View", "project_root_tasks"},
		{"Block", "project_board", "presentation.moveAction", "Action", "move_task"},
		{"Page", "project", "panel", "Panel", "project_panel"},
	} {
		_, references, exists := compiler.InspectDefinition(compiled.App, expected.kind, expected.name)
		if !exists || !hasInspectedReference(references, expected.path, expected.targetKind, expected.targetName) {
			t.Errorf("%s/%s missing %s -> %s/%s: %#v", expected.kind, expected.name, expected.path, expected.targetKind, expected.targetName, references)
		}
	}
}

func hasInspectedReference(references []compiler.DefinitionReference, path, kind, name string) bool {
	for _, reference := range references {
		if reference.Path == path && reference.Kind == kind && reference.Name == name {
			return true
		}
	}
	return false
}
