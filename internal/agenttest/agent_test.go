package agenttest

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type envelope struct {
	APIVersion  string         `json:"apiVersion"`
	Command     string         `json:"command"`
	OK          bool           `json:"ok"`
	Result      map[string]any `json:"result"`
	Diagnostics []struct {
		Code       string   `json:"code"`
		Path       string   `json:"path"`
		Candidates []string `json:"candidates"`
	} `json:"diagnostics"`
}

func TestJSONOnlyClientRepairsAndPublishesApplication(t *testing.T) {
	binary := requireBinary(t)
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	database := filepath.Join(directory, "bean.db")
	initOutput, stderr, exit := command(binary, "app", "init", "--dir", directory, "--name", "Applicant Tracking", "--json")
	if exit != 0 || stderr != "" {
		t.Fatalf("app init exit=%d stderr=%q stdout=%s", exit, stderr, initOutput)
	}
	initialized := decodeEnvelope(t, initOutput)
	if !initialized.OK || initialized.Command != "app.init" || initialized.APIVersion != "bean.cli/v1alpha1" {
		t.Fatalf("app init envelope=%#v", initialized)
	}
	source := "apiVersion: bean/v1alpha1\nname: Applicant Tracking\n---\nkind: Entity\nname: candidate\nfields:\n  - {name: name, label: Name, type: string, required: true}\n  - {name: status, label: Status, type: enum, options: [applied, interview, hired], required: true}\nlabel: Candidate\n---\nkind: View\nname: candidates\nentity: canddate\nfields: [id, name, stage]\n---\nkind: Action\nname: advance_candidate\nentity: candidate\noperation: transition\ntransitions:\n  screening: [hired]\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	invalidOutput, stderr, exit := command(binary, "app", "validate", "--file", manifest, "--json")
	if exit != 1 || stderr != "" {
		t.Fatalf("invalid validate exit=%d stderr=%q stdout=%s", exit, stderr, invalidOutput)
	}
	invalid := decodeEnvelope(t, invalidOutput)
	unknownEntity := findDiagnostic(invalid, "BEAN-E2001", "spec.entity")
	invalidTransition := findDiagnostic(invalid, "BEAN-E2201", "spec.transitions.screening")
	if invalid.OK || unknownEntity == nil || invalidTransition == nil || len(unknownEntity.Candidates) == 0 || len(invalidTransition.Candidates) == 0 {
		t.Fatalf("diagnostic = %#v", invalid)
	}

	// Every repair uses only stable codes, paths, and candidates. The client
	// never reads Bean source code or parses a human diagnostic message.
	repaired := strings.Replace(source, "entity: canddate", "entity: "+unknownEntity.Candidates[0], 1)
	if err := os.WriteFile(manifest, []byte(repaired), 0o600); err != nil {
		t.Fatal(err)
	}
	fieldOutput, _, fieldExit := command(binary, "app", "validate", "--file", manifest, "--json")
	if fieldExit != 1 {
		t.Fatalf("field repair baseline exit=%d output=%s", fieldExit, fieldOutput)
	}
	fieldEnvelope := decodeEnvelope(t, fieldOutput)
	invalidField := findDiagnostic(fieldEnvelope, "BEAN-E2101", "spec.fields")
	if invalidField == nil || !contains(invalidField.Candidates, "status") {
		t.Fatalf("field diagnostic = %#v", fieldEnvelope)
	}
	repaired = strings.Replace(repaired, "fields: [id, name, stage]", "fields: [id, name, status]", 1)
	if err := os.WriteFile(manifest, []byte(repaired), 0o600); err != nil {
		t.Fatal(err)
	}
	transitionOutput, _, transitionExit := command(binary, "app", "validate", "--file", manifest, "--json")
	if transitionExit != 1 {
		t.Fatalf("transition repair baseline exit=%d output=%s", transitionExit, transitionOutput)
	}
	transitionEnvelope := decodeEnvelope(t, transitionOutput)
	invalidTransition = findDiagnostic(transitionEnvelope, "BEAN-E2201", "spec.transitions.screening")
	if invalidTransition == nil || len(invalidTransition.Candidates) == 0 {
		t.Fatalf("transition diagnostic = %#v", transitionEnvelope)
	}
	repaired = strings.Replace(repaired, "screening: [hired]", invalidTransition.Candidates[0]+": [hired]", 1)
	if err := os.WriteFile(manifest, []byte(repaired), 0o600); err != nil {
		t.Fatal(err)
	}

	commands := [][]string{
		{"capabilities", "--json"},
		{"schema", "View", "--json"},
		{"app", "validate", "--file", manifest, "--json"},
		{"app", "inspect", "--file", manifest, "View", "candidates", "--json"},
		{"app", "plan", "--file", manifest, "--json"},
		{"app", "publish", "--file", manifest, "--db", database, "--json"},
		{"app", "diff", "--file", manifest, "--db", database, "--json"},
		{"app", "test", "--file", manifest, "--json"},
	}
	for _, args := range commands {
		stdout, stderr, exit := command(binary, args...)
		if exit != 0 || stderr != "" {
			t.Fatalf("%s exit=%d stderr=%q stdout=%s", strings.Join(args, " "), exit, stderr, stdout)
		}
		decoded := decodeEnvelope(t, stdout)
		if !decoded.OK || decoded.APIVersion != "bean.cli/v1alpha1" || len(decoded.Diagnostics) != 0 {
			t.Fatalf("%s envelope=%#v", strings.Join(args, " "), decoded)
		}
		if decoded.Command == "app.diff" && len(decoded.Result["changes"].([]any)) != 0 {
			t.Fatalf("post-publication diff = %#v", decoded.Result)
		}
	}
}

func TestAgentCommandsRetainHumanOutput(t *testing.T) {
	binary := requireBinary(t)
	directory := t.TempDir()
	sourceDirectory := filepath.Join(directory, "source")
	for _, args := range [][]string{
		{"app", "init", "--dir", sourceDirectory, "--name", "Candidates"},
		{"capabilities"},
		{"schema", "Entity"},
	} {
		stdout, stderr, exit := command(binary, args...)
		if exit != 0 || stdout == "" || stderr != "" {
			t.Fatalf("%s exit=%d stdout=%q stderr=%q", strings.Join(args, " "), exit, stdout, stderr)
		}
	}
	manifest := filepath.Join(sourceDirectory, "app.yaml")
	source := "apiVersion: bean/v1alpha1\nname: Candidates\n---\nkind: Entity\nname: candidate\nfields: []\nlabel: Candidate\n"
	if err := os.WriteFile(manifest, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	database := filepath.Join(directory, "human.db")
	for _, args := range [][]string{
		{"app", "validate", "--file", manifest},
		{"app", "inspect", "--file", manifest, "Entity", "candidate"},
		{"app", "plan", "--file", manifest},
		{"app", "diff", "--file", manifest},
		{"app", "publish", "--file", manifest, "--db", database},
		{"app", "test", "--file", manifest},
	} {
		stdout, stderr, exit := command(binary, args...)
		if exit != 0 || stdout == "" || stderr != "" {
			t.Fatalf("%s exit=%d stdout=%q stderr=%q", strings.Join(args, " "), exit, stdout, stderr)
		}
	}
}

func requireBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("BEAN_BINARY")
	if binary == "" {
		t.Skip("BEAN_BINARY is required for black-box agent tests")
	}
	return binary
}

func command(binary string, args ...string) (string, string, int) {
	cmd := exec.Command(binary, args...)
	stdout := &strings.Builder{}
	stderr := &strings.Builder{}
	cmd.Stdout, cmd.Stderr = stdout, stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		var exitError *exec.ExitError
		if errors.As(err, &exitError) {
			exit = exitError.ExitCode()
		} else {
			exit = -1
		}
	}
	return stdout.String(), stderr.String(), exit
}

func decodeEnvelope(t *testing.T, value string) envelope {
	t.Helper()
	var decoded envelope
	if err := json.Unmarshal([]byte(value), &decoded); err != nil {
		t.Fatalf("invalid JSON envelope: %v\n%s", err, value)
	}
	return decoded
}

func findDiagnostic(value envelope, code, path string) *struct {
	Code       string   `json:"code"`
	Path       string   `json:"path"`
	Candidates []string `json:"candidates"`
} {
	for index := range value.Diagnostics {
		if value.Diagnostics[index].Code == code && value.Diagnostics[index].Path == path {
			return &value.Diagnostics[index]
		}
	}
	return nil
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
