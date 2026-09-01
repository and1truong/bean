package action

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
)

func TestActionValueResolutionFailsClosed(t *testing.T) {
	if _, err := resolveValue(appir.ValueBinding{Source: "query", Path: "unsafe"}, nil, nil, beanctx.Request{}); err == nil {
		t.Fatal("unsupported Action value source resolved to silent nil")
	}
}

func TestDeclaredEffectsSelectReadOrWriteAuthorization(t *testing.T) {
	app := appir.Empty()
	app.Policies["records"] = appir.Policy{Name: "records", ReadRoles: []string{"reader"}, WriteRoles: []string{"writer"}}
	entity := appir.Entity{Name: "record", Policy: "records"}
	request := beanctx.Request{User: &beanctx.User{ID: "user", Roles: []string{"reader"}}}
	for _, name := range actionstep.Names() {
		specification, _ := actionstep.Lookup(name)
		if !specification.UsesEntity {
			continue
		}
		execution := stepExecution{app: app, step: appir.Step{Op: name}, specification: specification, request: request}
		allowed := authorizeStepEntity(execution, entity, dbal.Row{"id": "record-1"})
		if specification.Effects.MutatesEntity && allowed {
			t.Errorf("mutating step %s used read authorization", name)
		}
		if !specification.Effects.MutatesEntity && specification.Effects.ReadsEntity && !allowed {
			t.Errorf("reading step %s did not use read authorization", name)
		}
	}
}

func TestStepHandlersMatchRegisteredVocabulary(t *testing.T) {
	if got, want := stepHandlers.Names(), actionstep.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime handlers=%v specifications=%v", got, want)
	}
	for _, name := range actionstep.Names() {
		if handler, exists := stepHandlers.Lookup(name); !exists || handler == nil {
			t.Fatalf("step %s has no runtime handler", name)
		}
	}
}
