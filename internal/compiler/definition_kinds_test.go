package compiler

import (
	"reflect"
	"sort"
	"testing"

	"github.com/beanruntime/bean/internal/definition"
)

func TestDefinitionKindRegistryIsComplete(t *testing.T) {
	want := make([]string, 0, len(definition.Kinds))
	for kind := range definition.Kinds {
		want = append(want, kind)
	}
	sort.Strings(want)
	if got := definitionKinds.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("registered kinds=%v want=%v", got, want)
	}
	for _, name := range want {
		registered, exists := definitionKinds.Lookup(name)
		if !exists || registered.Specification == nil || registered.Compile == nil || registered.Lookup == nil || registered.Names == nil {
			t.Fatalf("Definition kind %s is incomplete: %+v", name, registered)
		}
	}
}
