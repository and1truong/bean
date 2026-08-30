package webform

import (
	"encoding/json"
	"fmt"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/expr"
	"net/mail"
	"regexp"
	"strconv"
)

type Errors map[string]string

func (e Errors) Error() string { return "webform validation failed" }
func Validate(f appir.Webform, input map[string]any, c beanctx.Request) error {
	errs := Errors{}
	validateElements(f.Elements, input, c, "", errs)
	if len(errs) > 0 {
		return errs
	}
	return nil
}

func validateElements(elements []appir.FormElement, input map[string]any, c beanctx.Request, prefix string, errs Errors) {
	for _, el := range elements {
		visible := true
		if el.Visible != nil {
			visible, _ = expr.Eval(*el.Visible, c, input)
		}
		if !visible {
			continue
		}
		required := el.Required
		if el.RequiredWhen != nil {
			conditional, _ := expr.Eval(*el.RequiredWhen, c, input)
			required = required || conditional
		}
		name := prefix + el.Name
		v, ok := input[el.Name]
		s, _ := v.(string)
		if required && empty(v, ok) {
			errs[name] = "This field is required."
			continue
		}
		if !ok {
			continue
		}
		if el.Type == "group" {
			rows, valid := v.([]any)
			if !valid {
				errs[name] = "Must be a list."
				continue
			}
			for i, raw := range rows {
				row, valid := raw.(map[string]any)
				if !valid {
					errs[fmt.Sprintf("%s.%d", name, i)] = "Must be an object."
					continue
				}
				validateElements(el.Children, row, c, fmt.Sprintf("%s.%d.", name, i), errs)
			}
			continue
		}
		if el.MinLength > 0 && len(s) < el.MinLength {
			errs[name] = fmt.Sprintf("Must be at least %d characters.", el.MinLength)
		}
		if el.MaxLength > 0 && len(s) > el.MaxLength {
			errs[name] = fmt.Sprintf("Must be at most %d characters.", el.MaxLength)
		}
		if el.Pattern != "" {
			r, e := regexp.Compile(el.Pattern)
			if e != nil || !r.MatchString(s) {
				errs[name] = "Invalid format."
			}
		}
		if el.Type == "email" {
			if _, e := mail.ParseAddress(s); e != nil {
				errs[name] = "Enter a valid email."
			}
		}
		if el.Type == "number" || el.Type == "integer" {
			n, valid := formNumber(v)
			if !valid || (el.Type == "integer" && n != float64(int64(n))) {
				errs[name] = "Enter a valid number."
				continue
			}
			if el.Min != nil && n < *el.Min {
				errs[name] = fmt.Sprintf("Must be at least %v.", *el.Min)
			}
			if el.Max != nil && n > *el.Max {
				errs[name] = fmt.Sprintf("Must be at most %v.", *el.Max)
			}
		}
	}
}

func empty(value any, exists bool) bool {
	if !exists || value == nil {
		return true
	}
	switch x := value.(type) {
	case string:
		return x == ""
	case []any:
		return len(x) == 0
	}
	return false
}

func formNumber(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case json.Number:
		n, e := x.Float64()
		return n, e == nil
	case string:
		n, e := strconv.ParseFloat(x, 64)
		return n, e == nil
	default:
		return 0, false
	}
}
