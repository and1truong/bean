package action

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/actionstep"
)

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
