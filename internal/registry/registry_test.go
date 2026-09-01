package registry

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegistryIsDeterministicAndImmutable(t *testing.T) {
	items, err := New(
		Entry[int]{Name: "zeta", Value: 2},
		Entry[int]{Name: "alpha", Value: 1},
	)
	if err != nil {
		t.Fatal(err)
	}
	if items.Len() != 2 || !reflect.DeepEqual(items.Names(), []string{"alpha", "zeta"}) {
		t.Fatalf("registry names=%v len=%d", items.Names(), items.Len())
	}
	names := items.Names()
	names[0] = "changed"
	if !reflect.DeepEqual(items.Names(), []string{"alpha", "zeta"}) {
		t.Fatalf("registry names were mutable: %v", items.Names())
	}
	if value, exists := items.Lookup("alpha"); !exists || value != 1 {
		t.Fatalf("lookup value=%d exists=%v", value, exists)
	}
	if _, exists := items.Lookup("missing"); exists {
		t.Fatal("missing entry existed")
	}
}

func TestRegistryRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name    string
		entries []Entry[int]
		message string
	}{
		{name: "empty", entries: []Entry[int]{{Value: 1}}, message: "name is required"},
		{name: "duplicate", entries: []Entry[int]{{Name: "same"}, {Name: "same"}}, message: `duplicate registry entry "same"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.entries...); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
