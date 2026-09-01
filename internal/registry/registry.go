package registry

import (
	"fmt"
	"sort"
)

type Entry[T any] struct {
	Name  string
	Value T
}

type Clone[T any] func(T) T

func Identity[T any](value T) T {
	return value
}

// Registry is immutable after construction. Values are copied by the required
// clone function when sealed and returned; Names also returns a copy.
type Registry[T any] struct {
	entries map[string]T
	names   []string
	clone   Clone[T]
}

func New[T any](clone Clone[T], entries ...Entry[T]) (Registry[T], error) {
	if clone == nil {
		return Registry[T]{}, fmt.Errorf("registry clone function is required")
	}
	values := make(map[string]T, len(entries))
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "" {
			return Registry[T]{}, fmt.Errorf("registry entry name is required")
		}
		if _, exists := values[entry.Name]; exists {
			return Registry[T]{}, fmt.Errorf("duplicate registry entry %q", entry.Name)
		}
		values[entry.Name] = clone(entry.Value)
		names = append(names, entry.Name)
	}
	sort.Strings(names)
	return Registry[T]{entries: values, names: names, clone: clone}, nil
}

func Must[T any](clone Clone[T], entries ...Entry[T]) Registry[T] {
	result, err := New(clone, entries...)
	if err != nil {
		panic(err)
	}
	return result
}

func (r Registry[T]) Lookup(name string) (T, bool) {
	value, exists := r.entries[name]
	if !exists {
		var zero T
		return zero, false
	}
	return r.clone(value), true
}

func (r Registry[T]) Names() []string {
	return append([]string{}, r.names...)
}

func (r Registry[T]) Len() int {
	return len(r.names)
}
