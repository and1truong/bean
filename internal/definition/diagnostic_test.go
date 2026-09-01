package definition

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDiagnosticCodesAndFactsAreStructural(t *testing.T) {
	diagnostics := []Diagnostic{
		UnknownFieldDiagnostic("Entity", "candidate", "spec", "labell"),
		MissingReferenceDiagnostic("View", "broken", "spec.entity", "Entity", "missing"),
		NewDiagnostic(RuleAction, "Action", "move", "spec.transitions", "invalid Action operation"),
		NewDiagnostic(RuleDuplicate, "", "", "name", "duplicate definition"),
		NewDiagnostic(RuleRoute, "Page", "home", "spec.route", "must start with /"),
	}
	want := []string{"BEAN-E1002", "BEAN-E2001", "BEAN-E2201", "BEAN-E1004", "BEAN-E2601"}
	for index := range diagnostics {
		if diagnostics[index].Code != want[index] {
			t.Errorf("diagnostic %d code=%q want=%q", index, diagnostics[index].Code, want[index])
		}
	}

	beforeCode := diagnostics[1].Code
	beforeTarget := *diagnostics[1].Facts.MissingReference
	diagnostics[1].Message = "wording can change without changing recovery"
	ClassifyDiagnostics(diagnostics)
	if diagnostics[1].Code != beforeCode || *diagnostics[1].Facts.MissingReference != beforeTarget {
		t.Fatalf("wording changed machine facts: %#v", diagnostics[1])
	}
}

func TestDiagnosticFactsAreNotSerialized(t *testing.T) {
	diagnostic := MissingReferenceDiagnostic("View", "broken", "spec.entity", "Entity", "missing")
	data, err := json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if encoded == "" || strings.Contains(encoded, "Facts") || strings.Contains(encoded, "missing-reference") {
		t.Fatalf("serialized diagnostic leaked internal facts: %s", encoded)
	}
}
