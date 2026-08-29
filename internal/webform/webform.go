package webform

import (
	"fmt"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/expr"
	"net/mail"
	"regexp"
)

type Errors map[string]string

func (e Errors) Error() string { return "webform validation failed" }
func Validate(f appir.Webform, input map[string]any, c beanctx.Request) error {
	errs := Errors{}
	for _, el := range f.Elements {
		visible := true
		if el.Visible != nil {
			visible, _ = expr.Eval(*el.Visible, c, input)
		}
		if !visible {
			continue
		}
		required := el.Required
		if el.RequiredWhen != nil {
			required, _ = expr.Eval(*el.RequiredWhen, c, input)
		}
		v, ok := input[el.Name]
		s, _ := v.(string)
		if required && (!ok || v == nil || s == "") {
			errs[el.Name] = "This field is required."
			continue
		}
		if !ok {
			continue
		}
		if el.MinLength > 0 && len(s) < el.MinLength {
			errs[el.Name] = fmt.Sprintf("Must be at least %d characters.", el.MinLength)
		}
		if el.MaxLength > 0 && len(s) > el.MaxLength {
			errs[el.Name] = fmt.Sprintf("Must be at most %d characters.", el.MaxLength)
		}
		if el.Pattern != "" {
			r, e := regexp.Compile(el.Pattern)
			if e != nil || !r.MatchString(s) {
				errs[el.Name] = "Invalid format."
			}
		}
		if el.Type == "email" {
			if _, e := mail.ParseAddress(s); e != nil {
				errs[el.Name] = "Enter a valid email."
			}
		}
	}
	if len(errs) > 0 {
		return errs
	}
	return nil
}
