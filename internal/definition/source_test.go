package definition_test

import (
	"bytes"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestLoadFSComposesFlatInlineAndResourceDefinitions(t *testing.T) {
	files := fstest.MapFS{
		"blog/app.yaml":      {Data: []byte("apiVersion: bean/v1alpha1\nname: Blog\nresources:\n  - comments.yaml\n---\nkind: Entity\nname: post\nfields:\n  - name: title\n    type: string\n")},
		"blog/comments.yaml": {Data: []byte("kind: Entity\nname: comment\nfields:\n  - name: body\n    type: text\n---\nkind: View\nname: comments\nentity: comment\nfields: [id, body]\n")},
	}

	bundle, diagnostics := definition.LoadFS(files, "blog/app.yaml")
	if len(diagnostics) != 0 {
		t.Fatalf("LoadFS() diagnostics = %v", diagnostics)
	}
	if bundle.Name != "Blog" || len(bundle.Definitions) != 3 {
		t.Fatalf("LoadFS() = %#v", bundle)
	}
	if bundle.Definitions[1].Source.Path != "blog/comments.yaml" || bundle.Definitions[1].Source.Line != 1 {
		t.Fatalf("resource source = %#v", bundle.Definitions[1].Source)
	}
	if result := compiler.Compile("test", 1, bundle.Definitions); len(result.Diagnostics) != 0 {
		t.Fatalf("Compile() diagnostics = %v", result.Diagnostics)
	}
}

func TestLoadFSReportsManifestAndResourceErrorsWithLocations(t *testing.T) {
	files := fstest.MapFS{
		"app.yaml": {Data: []byte("apiVersion: bean/v1alpha1\nname: Broken\nresource:\n  - ignored.yaml\nresources:\n  - ../outside.yaml\n  - missing.yaml\n")},
	}

	_, diagnostics := definition.LoadFS(files, "app.yaml")
	got := diagnosticsText(diagnostics)
	for _, want := range []string{
		"app.yaml:3:1: resource: unknown manifest field",
		"app.yaml:6:5: resource \"../outside.yaml\" must be a relative path without '..'",
		"app.yaml:7:5: resource \"missing.yaml\": open missing.yaml: file does not exist",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("diagnostics missing %q:\n%s", want, got)
		}
	}
}

func TestLoadFSReportsDuplicateDefinitionsAtSecondSource(t *testing.T) {
	files := fstest.MapFS{
		"app.yaml": {Data: []byte("apiVersion: bean/v1alpha1\nname: Duplicate\nresources: [one.yaml, two.yaml]\n")},
		"one.yaml": {Data: []byte("kind: Entity\nname: post\nfields: []\n")},
		"two.yaml": {Data: []byte("kind: Entity\nname: post\nfields: []\n")},
	}

	_, diagnostics := definition.LoadFS(files, "app.yaml")
	got := diagnosticsText(diagnostics)
	want := "two.yaml:1:1: Entity post name: duplicate definition; first declared at one.yaml:1:1"
	if !strings.Contains(got, want) {
		t.Fatalf("diagnostics missing %q:\n%s", want, got)
	}
}

func TestCompilerDiagnosticUsesFirstDuplicateDefinitionSource(t *testing.T) {
	files := fstest.MapFS{
		"app.yaml": {Data: []byte("apiVersion: bean/v1alpha1\nname: Duplicate\nresources: [one.yaml, two.yaml]\n")},
		"one.yaml": {Data: []byte("kind: Entity\nname: post\nfields: []\nlabell: Post\n")},
		"two.yaml": {Data: []byte("kind: Entity\nname: post\nfields: []\n")},
	}

	bundle, _ := definition.LoadFS(files, "app.yaml")
	result := compiler.Compile("test", 1, bundle.Definitions)
	for _, diagnostic := range result.Diagnostics {
		if strings.Contains(diagnostic.Message, "unknown field") {
			if !strings.Contains(diagnostic.Error(), "one.yaml:4:1") {
				t.Fatalf("compiler diagnostic source = %q", diagnostic.Error())
			}
			return
		}
	}
	t.Fatalf("unknown-field diagnostic missing: %v", result.Diagnostics)
}

func TestCompilerDiagnosticUsesDefinitionFieldLocation(t *testing.T) {
	files := fstest.MapFS{
		"app.yaml": {Data: []byte("apiVersion: bean/v1alpha1\nname: Typo\n---\nkind: Entity\nname: post\nfields: []\nlabell: Post\n")},
	}

	bundle, diagnostics := definition.LoadFS(files, "app.yaml")
	if len(diagnostics) != 0 {
		t.Fatalf("LoadFS() diagnostics = %v", diagnostics)
	}
	result := compiler.Compile("test", 1, bundle.Definitions)
	if len(result.Diagnostics) != 1 {
		t.Fatalf("Compile() diagnostics = %v", result.Diagnostics)
	}
	got := result.Diagnostics[0].Error()
	if !strings.Contains(got, "app.yaml:7:1: Entity post: json: unknown field \"labell\"") {
		t.Fatalf("diagnostic = %q", got)
	}
}

func TestEncodeProducesLoadableFlatSource(t *testing.T) {
	bundle := definition.Bundle{Name: "Exported", Definitions: []definition.Definition{{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "post"}, Spec: map[string]any{"fields": []any{}}}}}
	var output bytes.Buffer
	if err := definition.Encode(&output, bundle); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "definitions:") || strings.Contains(output.String(), "metadata:") || strings.Contains(output.String(), "spec:") {
		t.Fatalf("Encode() retained canonical wrapper:\n%s", output.String())
	}
	decoded, err := definition.Decode(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != bundle.Name || len(decoded.Definitions) != 1 || decoded.Definitions[0].Metadata.Name != "post" {
		t.Fatalf("Decode(Encode()) = %#v", decoded)
	}
}

func diagnosticsText(diagnostics []definition.Diagnostic) string {
	lines := make([]string, len(diagnostics))
	for i := range diagnostics {
		lines[i] = diagnostics[i].Error()
	}
	return strings.Join(lines, "\n")
}
