package valuesource

import (
	"reflect"
	"testing"

	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestContextVocabulariesAreExplicitAndCopied(t *testing.T) {
	want := map[Context][]string{
		Expression: {"context", "input", "literal", "record", "route", "tenant", "user"},
		Action:     {"context", "input", "literal", "now", "record", "result", "tenant", "user"},
		Block:      {"context", "tenant", "user"},
		Page:       {"query", "route", "tenant", "user"},
	}
	for context, expected := range want {
		actual := Names(context)
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("%s sources=%v want=%v", context, actual, expected)
		}
		actual[0] = "changed"
		if reflect.DeepEqual(Names(context), actual) {
			t.Fatalf("%s names exposed mutable state", context)
		}
	}
}

func TestResolveFailsClosedAndTraversesResults(t *testing.T) {
	environment := Environment{
		Request: beanctx.Request{User: &beanctx.User{ID: "user-1", Email: "user@example.com", DisplayName: "User"}, TenantID: "tenant-1"},
		Results: map[string]any{"loaded": []map[string]any{{"id": "record-1"}}},
	}
	value, err := Resolve(Action, Result, "loaded.0.id", environment)
	if err != nil || value != "record-1" {
		t.Fatalf("value=%v err=%v", value, err)
	}
	if _, err = Resolve(Action, Query, "q", environment); err == nil {
		t.Fatal("Action accepted a Page-only query source")
	}
	if _, err = Resolve(Action, User, "roles", environment); err == nil {
		t.Fatal("unknown user value was silently resolved")
	}
}
