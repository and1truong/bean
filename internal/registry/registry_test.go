package registry

import (
	"reflect"
	"strings"
	"testing"
)

func TestRegistryIsDeterministicAndImmutable(t *testing.T) {
	items, err := New(Identity[int],
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

func TestRegistryClonesMutableValuesWhenSealedAndRead(t *testing.T) {
	clone := func(value []string) []string { return append([]string{}, value...) }
	source := []string{"sealed"}
	items, err := New(clone, Entry[[]string]{Name: "item", Value: source})
	if err != nil {
		t.Fatal(err)
	}
	source[0] = "source mutation"
	first, exists := items.Lookup("item")
	if !exists || !reflect.DeepEqual(first, []string{"sealed"}) {
		t.Fatalf("lookup after source mutation=%v exists=%v", first, exists)
	}
	first[0] = "lookup mutation"
	second, _ := items.Lookup("item")
	if !reflect.DeepEqual(second, []string{"sealed"}) {
		t.Fatalf("registry value was mutable through lookup: %v", second)
	}
}

func TestRegistryRejectsInvalidEntries(t *testing.T) {
	tests := []struct {
		name    string
		clone   Clone[int]
		entries []Entry[int]
		message string
	}{
		{name: "clone", message: "clone function is required"},
		{name: "empty", clone: Identity[int], entries: []Entry[int]{{Value: 1}}, message: "name is required"},
		{name: "duplicate", clone: Identity[int], entries: []Entry[int]{{Name: "same"}, {Name: "same"}}, message: `duplicate registry entry "same"`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.clone, test.entries...); err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("error=%v", err)
			}
		})
	}
}
