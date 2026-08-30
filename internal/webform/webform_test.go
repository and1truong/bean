package webform_test

import (
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/webform"
	"testing"
)

func TestValidation(t *testing.T) {
	f := appir.Webform{Elements: []appir.FormElement{{Name: "email", Type: "email", Required: true}, {Name: "name", Type: "text", MinLength: 3}}}
	if e := webform.Validate(f, map[string]any{"email": "bad", "name": "x"}, beanctx.Request{}); e == nil {
		t.Fatal("invalid form accepted")
	}
	if e := webform.Validate(f, map[string]any{"email": "a@example.test", "name": "Bean"}, beanctx.Request{}); e != nil {
		t.Fatal(e)
	}
}
