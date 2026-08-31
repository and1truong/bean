package definition

import "testing"

func TestClassifyDiagnosticsAssignsPublicCodes(t *testing.T) {
	diagnostics := []Diagnostic{
		{Path: "spec.labell", Message: `json: unknown field "labell"`},
		{Kind: "View", Path: "spec.entity", Message: "references missing Entity missing"},
		{Kind: "Action", Path: "spec.transitions", Message: "invalid Action operation"},
		{Path: "name", Message: "duplicate definition"},
		{Kind: "Page", Path: "spec.route", Message: "must start with /"},
	}
	ClassifyDiagnostics(diagnostics)
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == "" {
			t.Errorf("unclassified diagnostic: %#v", diagnostic)
		}
	}
	if diagnostics[0].Code != "BEAN-E1002" {
		t.Fatalf("unknown field code = %q", diagnostics[0].Code)
	}
	if diagnostics[1].Code != "BEAN-E2001" {
		t.Fatalf("unknown reference code = %q", diagnostics[1].Code)
	}
}
