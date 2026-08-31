package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadValidSourceAggregatesRecoverableLoaderAndCompilerDiagnostics(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	resource := filepath.Join(directory, "posts.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Broken\nresources: [posts.yaml]\nresourcess: [posts.yaml]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("kind: Entity\nname: post\nfields: []\nlabell: Post\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (2 errors)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	for _, want := range []string{
		manifest + ":4:1: resourcess: unknown manifest field",
		resource + ":4:1: Entity post: json: unknown field \"labell\"",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("diagnostic missing %q:\n%s", want, output.String())
		}
	}
}

func TestLoadValidSourceAggregatesCompilerDecodingAndDependencyDiagnostics(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	resource := filepath.Join(directory, "definitions.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Broken\nresources: [definitions.yaml]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("kind: Entity\nname: post\nfields: []\nlabell: Post\n---\nkind: View\nname: missing_posts\nentity: missing\nfields: [id]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (2 errors)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	for _, want := range []string{"json: unknown field \"labell\"", "references missing Entity missing"} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("diagnostic missing %q:\n%s", want, output.String())
		}
	}
}

func TestLoadValidSourceDefersDependencyValidationForIncompleteSources(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	resource := filepath.Join(directory, "views.yaml")
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Broken\nresources: [missing.yaml, views.yaml]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resource, []byte("kind: View\nname: posts\nentity: post\nfields: [id]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (1 error)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	if strings.Contains(output.String(), "references missing Entity post") {
		t.Fatalf("dependency diagnostic from incomplete source:\n%s", output.String())
	}
}

func TestLoadValidSourceDeduplicatesManifestAPIVersionDiagnostics(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	if err := os.WriteFile(manifest, []byte("name: Missing API version\n---\nkind: Entity\nname: post\nfields: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (1 error)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	if strings.Count(output.String(), "apiVersion") != 1 {
		t.Fatalf("API version diagnostic count:\n%s", output.String())
	}
}

func TestLoadValidSourceDeduplicatesLoaderAndCompilerDuplicateDiagnostics(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "app.yaml")
	for _, file := range []string{"one.yaml", "two.yaml"} {
		if err := os.WriteFile(filepath.Join(directory, file), []byte("kind: Entity\nname: post\nfields: []\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(manifest, []byte("apiVersion: bean/v1alpha1\nname: Duplicate\nresources: [one.yaml, two.yaml]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	_, err := loadValidSource(manifest, &output)
	if err == nil || err.Error() != "application source is invalid (1 error)" {
		t.Fatalf("loadValidSource() error = %v", err)
	}
	if strings.Count(output.String(), "duplicate definition") != 1 || strings.Contains(output.String(), "duplicate machine name") {
		t.Fatalf("duplicate diagnostic count:\n%s", output.String())
	}
}
