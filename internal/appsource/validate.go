// Package appsource owns validation of reviewable Bean application sources.
package appsource

import (
	"fmt"
	"sort"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

// Validate loads a manifest and reports loader and compiler diagnostics in
// deterministic source order.
func Validate(filename string) (definition.Bundle, []definition.Diagnostic) {
	bundle, diagnostics, complete := definition.LoadFileForValidation(filename)
	if complete {
		diagnostics = mergeDiagnostics(diagnostics, compiler.Compile("default", 1, bundle.Definitions).Diagnostics)
	} else {
		diagnostics = mergeDiagnostics(diagnostics, compiler.CompileRecovered("default", 1, bundle.Definitions).Diagnostics)
	}
	definition.ClassifyDiagnostics(diagnostics)
	sort.SliceStable(diagnostics, func(left, right int) bool {
		a, b := diagnostics[left], diagnostics[right]
		if a.Source.Path != b.Source.Path {
			return a.Source.Path < b.Source.Path
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		if a.Source.Column != b.Source.Column {
			return a.Source.Column < b.Source.Column
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return bundle, diagnostics
}

func mergeDiagnostics(loader, compiled []definition.Diagnostic) []definition.Diagnostic {
	diagnostics := append([]definition.Diagnostic{}, loader...)
	invalidAPIVersion := false
	duplicates := map[string]bool{}
	missingRequiredField := map[string]bool{}
	for _, diagnostic := range loader {
		if diagnostic.Path == "apiVersion" {
			invalidAPIVersion = true
		}
		if diagnostic.Message == "is required" && (diagnostic.Path == "kind" || diagnostic.Path == "name") {
			missingRequiredField[sourceKey(diagnostic)+"/"+diagnostic.Path] = true
		}
		if diagnostic.Message == "duplicate definition" {
			duplicates[diagnostic.Kind+"/"+diagnostic.Name] = true
		}
	}
	for _, diagnostic := range compiled {
		if invalidAPIVersion && diagnostic.Path == "apiVersion" ||
			diagnostic.Path == "kind" && missingRequiredField[sourceKey(diagnostic)+"/kind"] ||
			diagnostic.Path == "metadata.name" && missingRequiredField[sourceKey(diagnostic)+"/name"] ||
			diagnostic.Message == "duplicate machine name" && duplicates[diagnostic.Kind+"/"+diagnostic.Name] {
			continue
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

func sourceKey(diagnostic definition.Diagnostic) string {
	return fmt.Sprintf("%s:%d:%d", diagnostic.Source.Path, diagnostic.Source.Line, diagnostic.Source.Column)
}
