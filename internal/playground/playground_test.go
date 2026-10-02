package playground

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/render"
	"github.com/beanruntime/bean/internal/sequence"
)

func exampleFiles(t *testing.T, name string) (map[string]string, string) {
	t.Helper()
	directory := filepath.Join("..", "..", "examples", name)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read example %s: %v", name, err)
	}
	files := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", entry.Name(), err)
		}
		files[name+"/"+entry.Name()] = string(contents)
	}
	return files, name + "/app.yaml"
}

func compile(t *testing.T, session *Session, files map[string]string, manifest string) Response {
	t.Helper()
	response := session.Handle(Request{V: ProtocolVersion, Op: "compile", Files: files, Manifest: manifest})
	if !response.OK {
		t.Fatalf("compile failed: %+v", response)
	}
	return response
}

func renderRoute(t *testing.T, session *Session, path string) render.Node {
	t.Helper()
	response := session.Handle(Request{V: ProtocolVersion, Op: "render", Path: path})
	if !response.OK {
		t.Fatalf("render %s failed: %+v", path, response.Error)
	}
	encoded, err := json.Marshal(response.Tree)
	if err != nil {
		t.Fatalf("encode tree: %v", err)
	}
	var tree render.Node
	if err := json.Unmarshal(encoded, &tree); err != nil {
		t.Fatalf("decode tree: %v", err)
	}
	return tree
}

func findComponents(node render.Node, out map[string]int) map[string]int {
	out[node.Component]++
	for _, child := range node.Children {
		findComponents(child, out)
	}
	return out
}

func TestCompileAndRenderPresentation(t *testing.T) {
	files, manifest := exampleFiles(t, "presentation")
	session := NewSession()
	response := compile(t, session, files, manifest)
	if response.App == nil || response.App.Name != "Bean Introduction" {
		t.Fatalf("expected presentation summary, got %+v", response.App)
	}
	var sequenceRoute *Route
	for index, route := range response.App.Routes {
		if route.Kind == "sequence" && route.Path == "/presentations/bean" {
			sequenceRoute = &response.App.Routes[index]
		}
	}
	if sequenceRoute == nil {
		t.Fatalf("expected /presentations/bean route, got %+v", response.App.Routes)
	}
	var hasViewBlock bool
	for _, item := range response.App.Unsupported {
		if item.Component == "ViewBlock" {
			hasViewBlock = true
		}
	}
	if !hasViewBlock {
		t.Fatalf("expected ViewBlock to be reported unsupported, got %+v", response.App.Unsupported)
	}
	tree := renderRoute(t, session, "/presentations/bean")
	if tree.Component != "Sequence" {
		t.Fatalf("expected Sequence root, got %s", tree.Component)
	}
	components := findComponents(tree, map[string]int{})
	for _, supported := range []string{"SequenceFrame", "ContentBlock", "TabsBlock", "LessonBlock", "TimelineBlock", "MindmapBlock", "FlashcardBlock"} {
		if components[supported] == 0 {
			t.Errorf("expected %s nodes in the render tree, got %+v", supported, components)
		}
	}
	if components["UnsupportedBlock"] == 0 {
		t.Errorf("expected the view block to be marked UnsupportedBlock, got %+v", components)
	}
	if components["ViewBlock"] != 0 {
		t.Errorf("expected no ViewBlock nodes in the browser tree, got %+v", components)
	}
}

// TestBridgeParity verifies the bridge produces the same AppIR composition as
// the native page/sequence pipeline for the same compiled application.
func TestBridgeParity(t *testing.T) {
	files, manifest := exampleFiles(t, "presentation")
	session := NewSession()
	compile(t, session, files, manifest)
	bundle, diagnostics := definition.LoadFS(fileMap(files), manifest)
	if len(diagnostics) > 0 {
		t.Fatal(diagnostics)
	}
	native := compiler.Compile("playground", 1, bundle.Definitions)
	item, found := sequence.Match(native.App, "/presentations/bean")
	if !found {
		t.Fatal("native sequence match failed")
	}
	want, allowed, err := sequence.Node(native.App, item, beanctx.Request{Route: "/presentations/bean"})
	if err != nil || !allowed {
		t.Fatalf("native render failed: allowed=%v err=%v", allowed, err)
	}
	response := session.Handle(Request{V: ProtocolVersion, Op: "render", Path: "/presentations/bean"})
	if !response.OK {
		t.Fatalf("bridge render failed: %+v", response.Error)
	}
	// The bridge wire payload must equal the native tree after UnsupportedBlock
	// marking. Compare the raw Tree value rather than a render.Node round-trip,
	// which reorders nested struct fields and produces false diffs.
	wantMarked, _ := markUnsupported(want)
	wantJSON, _ := json.Marshal(wantMarked)
	gotJSON, _ := json.Marshal(response.Tree)
	if string(wantJSON) != string(gotJSON) {
		t.Fatalf("bridge and native render trees diverge:\nwant: %s\ngot:  %s", wantJSON, gotJSON)
	}
}

func TestDiagnosticsParity(t *testing.T) {
	files := map[string]string{
		"app.yaml":  "apiVersion: bean/v1alpha1\nname: broken\nresources: [defs.yaml]\n",
		"defs.yaml": "kind: NoSuchKind\nname: x\nspec: {}\n",
	}
	session := NewSession()
	response := session.Handle(Request{V: ProtocolVersion, Op: "compile", Files: files, Manifest: "app.yaml"})
	if response.OK {
		t.Fatal("expected compile failure for unknown kind")
	}
	bundle, loadDiagnostics := definition.LoadFS(fileMap(files), "app.yaml")
	if len(loadDiagnostics) > 0 {
		t.Fatalf("fixture did not load: %v", loadDiagnostics)
	}
	wantCodes := diagnosticCodes(compiler.Compile("playground", 1, bundle.Definitions).Diagnostics)
	gotCodes := diagnosticCodes(response.Diagnostics)
	if len(gotCodes) == 0 {
		t.Fatal("expected diagnostics")
	}
	for _, code := range wantCodes {
		found := false
		for _, got := range gotCodes {
			if got == code {
				found = true
			}
		}
		if !found {
			t.Errorf("missing diagnostic code %s in %+v", code, gotCodes)
		}
	}
	// Source positions survive the bridge.
	for _, diagnostic := range response.Diagnostics {
		if diagnostic.Source.Path != "" && diagnostic.Source.Line < 1 {
			t.Errorf("diagnostic %s lost its source position", diagnostic.Code)
		}
	}
}

func diagnosticCodes(diagnostics []definition.Diagnostic) []string {
	codes := []string{}
	for _, diagnostic := range diagnostics {
		codes = append(codes, diagnostic.Code)
	}
	sort.Strings(codes)
	return codes
}

func TestFileMapValidation(t *testing.T) {
	session := NewSession()
	reject := func(files map[string]string, manifest, code string) {
		t.Helper()
		response := session.Handle(Request{V: ProtocolVersion, Op: "compile", Files: files, Manifest: manifest})
		if response.OK || response.Error == nil || response.Error.Code != code {
			t.Fatalf("expected %s rejection, got %+v", code, response)
		}
	}
	reject(map[string]string{}, "app.yaml", codeFileLimit)
	reject(map[string]string{"app.yaml": "x", "../escape.yaml": "x"}, "app.yaml", codeFilePath)
	reject(map[string]string{"app.yaml": "x", "a//b.yaml": "x"}, "app.yaml", codeFilePath)
	reject(map[string]string{"/abs.yaml": "x"}, "app.yaml", codeFilePath)
	reject(map[string]string{"app.txt": "x"}, "app.txt", codeFilePath)
	reject(map[string]string{"app.yaml": strings.Repeat("x", MaxFileBytes+1)}, "app.yaml", codeFileLimit)
	oversized := map[string]string{"app.yaml": "apiVersion: bean/v1alpha1\nname: x\n"}
	reject(oversized, "missing.yaml", codeManifest)
}

func TestRenderFailures(t *testing.T) {
	session := NewSession()
	response := session.Handle(Request{V: ProtocolVersion, Op: "render", Path: "/"})
	if response.OK || response.Error.Code != CodeNoCompiledSource {
		t.Fatalf("expected %s before compile, got %+v", CodeNoCompiledSource, response.Error)
	}
	files, manifest := exampleFiles(t, "presentation")
	compile(t, session, files, manifest)
	missing := session.Handle(Request{V: ProtocolVersion, Op: "render", Path: "/missing"})
	if missing.OK || missing.Error.Code != CodeNotFound {
		t.Fatalf("expected %s for unknown route, got %+v", CodeNotFound, missing.Error)
	}
	files, manifest = exampleFiles(t, "community")
	community := NewSession()
	compile(t, community, files, manifest)
	var protectedRoute, displayRoute string
	for _, route := range community.Handle(Request{V: ProtocolVersion, Op: "compile", Files: files, Manifest: manifest}).App.Routes {
		if route.Protected && protectedRoute == "" {
			protectedRoute = route.Path
		}
		if route.Kind == "display" && displayRoute == "" {
			displayRoute = route.Path
		}
	}
	if protectedRoute != "" {
		response = community.Handle(Request{V: ProtocolVersion, Op: "render", Path: protectedRoute})
		if response.OK || response.Error.Code != CodeProtected {
			t.Fatalf("expected %s for %s, got %+v", CodeProtected, protectedRoute, response.Error)
		}
	}
	if displayRoute != "" {
		response = community.Handle(Request{V: ProtocolVersion, Op: "render", Path: displayRoute})
		if response.OK || response.Error.Code != CodeBackendRequired {
			t.Fatalf("expected %s for %s, got %+v", CodeBackendRequired, displayRoute, response.Error)
		}
	}
}

func TestFailedCompileKeepsLastPreview(t *testing.T) {
	files, manifest := exampleFiles(t, "presentation")
	session := NewSession()
	compile(t, session, files, manifest)
	broken := map[string]string{"app.yaml": "not: valid\n"}
	response := session.Handle(Request{V: ProtocolVersion, Op: "compile", Files: broken, Manifest: "app.yaml"})
	if response.OK {
		t.Fatal("expected broken source to fail")
	}
	tree := renderRoute(t, session, "/presentations/bean")
	if tree.Component != "Sequence" {
		t.Fatalf("expected the last valid preview to survive, got %+v", tree.Component)
	}
}
