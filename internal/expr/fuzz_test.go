package expr

import (
	"testing"

	beanctx "github.com/beanruntime/bean/internal/context"
)

func FuzzEval(f *testing.F) {
	f.Add("eq", "literal", "a", "a")
	f.Add("contains", "literal", "bean", "e")
	f.Fuzz(func(t *testing.T, op, source, left, right string) {
		expression := Expr{Op: op, Left: &Value{Source: source, Literal: left}, Right: &Value{Source: "literal", Literal: right}}
		_, _ = Eval(expression, beanctx.Request{}, map[string]any{})
	})
}
