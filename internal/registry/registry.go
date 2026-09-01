package registry

import (
	"fmt"
	"sort"
)

type Entry[T any] struct {
	Name  string
	Value T
}

// Registry is immutable after construction. Names returns a copy so callers
// cannot alter its deterministic iteration order.
type Registry[T any] struct {
	entries map[string]T
	names   []string
}

func New[T any](entries ...Entry[T]) (Registry[T], error) {
	values := make(map[string]T, len(entries))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "" {
			return Registry[T]{}, fmt.Errorf("registry entry name is required")
		}
		if _, exists := values[entry.Name]; exists {
			return Registry[T]{}, fmt.Errorf("duplicate registry entry %q", entry.Name)
		}
		values[entry.Name] = entry.Value
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return Registry[T]{entries: values, names: names}, nil
}

func Must[T any](entries ...Entry[T]) Registry[T] {
	result, err := New(entries...)
	if err != nil {
		panic(err)
	}
	return result
}

func (r Registry[T]) Lookup(name string) (T, bool) {
	value, exists := r.entries[name]
	return value, exists
}

func (r Registry[T]) Names() []string {
	return append([]string{}, r.names...)
}

func (r Registry[T]) Len() int {
	return len(r.names)
}
