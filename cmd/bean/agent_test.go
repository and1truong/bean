package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
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

func TestAgentCapabilitiesAndSchemaAreSelfDescribing(t *testing.T) {
	for _, test := range []struct {
		args    []string
		command string
		assert  func(*testing.T, map[string]any)
	}{
		{args: []string{"capabilities", "--json"}, command: "capabilities", assert: func(t *testing.T, result map[string]any) {
			if result["definitionAPIVersion"] != "bean/v1alpha1" || result["appIRFormat"] != "bean/appir/v1" {
				t.Fatalf("capabilities = %#v", result)
			}
			if len(result["definitionKinds"].([]any)) < 10 || len(result["fieldTypes"].([]any)) < 10 {
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
				ID     string `json:"id"`
				Status string `json:"status"`
			} `json:"checks"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &smoke); err != nil || len(smoke.Result.Checks) != 5 {
		t.Fatalf("smoke err=%v value=%#v output=%s", err, smoke, stdout.String())
	}
	for _, check := range smoke.Result.Checks {
		if check.ID == "" || check.Status != "passed" {
			t.Fatalf("check = %#v", check)
		}
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
	for _, name := range []string{"bean.schema.json", "entity.schema.json", "view.schema.json", "action.schema.json", "page.schema.json"} {
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
	application.Actions["call_service"] = appir.Action{
		Name: "call_service",
		Steps: []appir.Step{{
			Op: "return",
			Values: []appir.Assignment{{
				Field: "api_key",
				Value: appir.ValueBinding{Source: "literal", Literal: json.RawMessage(`"super-secret"`)},
			}},
		}},
	}
	inspectable := redactedApp(application)
	definition, _, exists := inspectedDefinition(inspectable, "Action", "call_service")
	if !exists {
		t.Fatal("redacted Action is not inspectable")
	}
	inspection, err := json.Marshal(definition)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := json.Marshal(semanticDiff(appir.Empty(), application))
	if err != nil {
		t.Fatal(err)
	}
	for name, output := range map[string][]byte{"inspection": inspection, "diff": diff} {
		if bytes.Contains(output, []byte("super-secret")) || !bytes.Contains(output, []byte("[REDACTED]")) {
			t.Fatalf("%s leaked or omitted redaction: %s", name, output)
		}
	}
}

func TestInspectionResolvesEveryMaintainedDefinitionReference(t *testing.T) {
	for _, application := range []string{"asana", "blog", "booking", "cms", "commerce", "community", "crm", "saas", "tracker"} {
		bundle, err := examples.Load(application)
		if err != nil {
			t.Fatal(err)
		}
		compiled := compiler.Compile("default", 1, bundle.Definitions)
		if len(compiled.Diagnostics) != 0 {
			t.Fatalf("%s diagnostics=%v", application, compiled.Diagnostics)
		}
		for _, source := range bundle.Definitions {
			definition, references, exists := inspectedDefinition(compiled.App, source.Kind, source.Metadata.Name)
			if !exists || definition == nil {
				t.Errorf("%s: cannot inspect %s/%s", application, source.Kind, source.Metadata.Name)
				continue
			}
			for _, reference := range references {
				if _, _, targetExists := inspectedDefinition(compiled.App, reference.Kind, reference.Name); !targetExists {
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
		_, references, exists := inspectedDefinition(compiled.App, expected.kind, expected.name)
		if !exists || !hasInspectedReference(references, expected.path, expected.targetKind, expected.targetName) {
			t.Errorf("%s/%s missing %s -> %s/%s: %#v", expected.kind, expected.name, expected.path, expected.targetKind, expected.targetName, references)
		}
	}
}

func hasInspectedReference(references []inspectedReference, path, kind, name string) bool {
	for _, reference := range references {
		if reference.Path == path && reference.Kind == kind && reference.Name == name {
			return true
		}
	}
	return false
}
