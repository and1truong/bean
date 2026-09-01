package compiler

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
)

func TestRecoveryAndCandidatesIgnoreDiagnosticWording(t *testing.T) {
	app := appir.Empty()
	app.Entities["candidate"] = appir.Entity{Name: "candidate"}
	diagnostic := definition.MissingReferenceDiagnostic("View", "broken", "spec.entity", "Entity", "canddate")
	diagnostic.Message = "wording deliberately changed"
	diagnostics := []definition.Diagnostic{diagnostic}
	enrichDiagnosticCandidates(app, diagnostics)
	if len(diagnostics[0].Candidates) == 0 || diagnostics[0].Candidates[0] != "candidate" {
		t.Fatalf("candidates=%v", diagnostics[0].Candidates)
	}
	if recovered := suppressMissingDependencies(diagnostics); len(recovered) != 0 {
		t.Fatalf("structured missing dependency was not suppressed: %v", recovered)
	}
}

func TestDefinitionKindRegistryOwnsEveryAppIRDefinitionCollection(t *testing.T) {
	registeredTypes := map[reflect.Type]string{}
	for _, name := range definitionKindRegistry().Names() {
		registered, exists := definitionKindRegistry().Lookup(name)
		if !exists || registered.Specification == nil || registered.Storage == nil || registered.Compile == nil || registered.Normalize == nil || registered.Validate == nil || registered.Lookup == nil || registered.Names == nil {
			t.Fatalf("Definition kind %s is incomplete: %+v", name, registered)
		}
		if previous := registeredTypes[registered.Storage]; previous != "" {
			t.Fatalf("Definition kinds %s and %s share AppIR storage type %v", previous, name, registered.Storage)
		}
		registeredTypes[registered.Storage] = name
	}

	appType := reflect.TypeOf(appir.App{})
	storageTypes := map[reflect.Type]string{}
	for index := 0; index < appType.NumField(); index++ {
		field := appType.Field(index)
		var stored reflect.Type
		switch field.Type.Kind() {
		case reflect.Map:
			stored = field.Type.Elem()
		case reflect.Pointer:
			if field.Type.Elem().Kind() == reflect.Struct {
				stored = field.Type.Elem()
			}
		}
		if stored != nil {
			storageTypes[stored] = field.Name
		}
	}
	for specification, name := range registeredTypes {
		if storageTypes[specification] == "" {
			t.Errorf("Definition kind %s has no AppIR collection for %v", name, specification)
		}
	}
	for stored, fieldName := range storageTypes {
		if registeredTypes[stored] == "" {
			t.Errorf("AppIR definition collection %s (%v) has no registered kind", fieldName, stored)
		}
	}
}
