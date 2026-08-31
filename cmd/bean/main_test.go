package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidSourceReportsActionableCompilerDiagnostic(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	resource := filepath.Join(directory, "posts.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Broken\nresources: [posts.yaml]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("kind: Entity\nname: post\nfields: []\nlabell: Post\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (1 error)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	want := resource + ":4:1: Entity post: json: unknown field \"labell\""
	if !strings.Contains(output.String(), want) {
		t.Fatalf("diagnostic missing %q:\n%s", want, output.String())
	}
}
