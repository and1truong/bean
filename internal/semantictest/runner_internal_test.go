package semantictest

import (
	"context"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
)

func TestExpectedMapRequiresPresentKeys(t *testing.T) {
	if matchesExpected(map[string]any{"optional": nil}, map[string]any{}) {
		t.Fatal("missing result key matched expected null")
	}
	if !matchesExpected(map[string]any{"optional": nil}, map[string]any{"optional": nil}) {
		t.Fatal("present null result key did not match")
	}
}

func TestWrappedDeadlineUsesTimeoutCode(t *testing.T) {
	err := &dbal.Error{Code: dbal.Internal, Message: "database operation failed", Cause: context.DeadlineExceeded}
	if code := errorCode(err); code != "timeout" {
		t.Fatalf("code=%q", code)
	}
}

func TestNoEventExpectationDoesNotReadStorage(t *testing.T) {
	if diagnostics := assertEvents(context.Background(), nil, "suite", "tests.case.expect", appir.TestExpectation{}); len(diagnostics) != 0 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
}
