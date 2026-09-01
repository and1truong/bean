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
				Status string `json:"status"`
				Cases  []struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"cases"`
			} `json:"suites"`
		} `json:"result"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &success); err != nil || !success.OK || len(success.Result.Suites) != 1 || success.Result.Suites[0].Cases[0].ID != "TestSuite/division_contract/quotient" || success.Result.Suites[0].Cases[0].Status != "passed" {
		t.Fatalf("result=%+v err=%v output=%s", success, err, stdout.String())
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
	if err := json.Unmarshal(stdout.Bytes(), &failure); err != nil || failure.OK || len(failure.Result.Suites) != 1 || failure.Result.Suites[0].Status != "failed" || len(failure.Diagnostics) != 1 || failure.Diagnostics[0].Code != "BEAN-T1001" || failure.Diagnostics[0].Path != "tests.quotient.expect.result" {
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
