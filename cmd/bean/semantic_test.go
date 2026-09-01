package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAppTestReportsSemanticSuiteEvidenceAndFailures(t *testing.T) {
	manifest := filepath.Join(t.TempDir(), "app.yaml")
	writeSemanticTestSource(t, manifest, "4")
	var stdout, stderr bytes.Buffer
	if exit := execute([]string{"app", "test", "--file", manifest, "--json"}, &stdout, &stderr); exit != exitOK {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var success struct {
		OK     bool `json:"ok"`
		Result struct {
			Suites []struct {
				ID       string `json:"id"`
				Status   string `json:"status"`
				Evidence *struct {
					Family string `json:"family"`
					Source struct {
						Kind string `json:"kind"`
						Name string `json:"name"`
					} `json:"source"`
					Suite string `json:"suite"`
				} `json:"evidence"`
				Cases []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"cases"`
			} `json:"suites"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &success); err != nil || !success.OK || len(success.Result.Suites) != 2 || success.Result.Suites[0].ID != "TestSuite/division_contract" || success.Result.Suites[0].Cases[0].ID != "TestSuite/division_contract/quotient" || success.Result.Suites[0].Cases[0].Status != "passed" {
		t.Fatalf("result=%+v err=%v output=%s", success, err, stdout.String())
	}
	generated := success.Result.Suites[1]
	if generated.ID != "TestSuite/generated_replay_division_contract" || generated.Evidence == nil || generated.Evidence.Family != "replay" || generated.Evidence.Source.Kind != "Rule" || generated.Evidence.Source.Name != "divide" || generated.Evidence.Suite != "division_contract" || generated.Cases[0].ID != "TestSuite/generated_replay_division_contract/replay_quotient" || generated.Cases[0].Status != "passed" {
		t.Fatalf("generated=%+v output=%s", generated, stdout.String())
	}
	successOutput := stdout.String()
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "test", "--file", manifest, "--json"}, &stdout, &stderr); exit != exitOK || stdout.String() != successOutput {
		t.Fatalf("replay exit=%d stderr=%s\nfirst=%s\nsecond=%s", exit, stderr.String(), successOutput, stdout.String())
	}

	writeSemanticTestSource(t, manifest, "99")
	stdout.Reset()
	stderr.Reset()
	if exit := execute([]string{"app", "test", "--file", manifest, "--json"}, &stdout, &stderr); exit != exitDefinition {
		t.Fatalf("exit=%d stderr=%s stdout=%s", exit, stderr.String(), stdout.String())
	}
	var failure struct {
		OK     bool `json:"ok"`
		Result struct {
			Suites []struct {
				Status string `json:"status"`
			} `json:"suites"`
		} `json:"result"`
		Diagnostics []struct {
			Code string `json:"code"`
			Path string `json:"path"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil || failure.OK || len(failure.Result.Suites) != 2 || failure.Result.Suites[0].Status != "failed" || failure.Result.Suites[1].Status != "failed" || len(failure.Diagnostics) != 2 || failure.Diagnostics[0].Code != "BEAN-T1001" || failure.Diagnostics[0].Path != "tests.quotient.expect.result" || failure.Diagnostics[1].Path != "tests.replay_quotient.expect.result" {
		t.Fatalf("result=%+v err=%v output=%s", failure, err, stdout.String())
	}
}

func writeSemanticTestSource(t *testing.T, path, expected string) {
	t.Helper()
	source := `apiVersion: bean/v1alpha1
name: Semantic test
---
kind: Rule
name: divide
result: number
input:
  left: {type: money, required: true}
  right: {type: money, required: true}
expression:
  op: divide
  args:
    - {source: input, path: left}
    - {source: input, path: right}
---
kind: TestSuite
name: division_contract
target: {kind: Rule, name: divide}
tests:
  - name: quotient
    input: {left: 12, right: 3}
    expect: {result: EXPECTED}
`
	if err := os.WriteFile(path, []byte(strings.Replace(source, "EXPECTED", expected, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
}
