package expr

import (
	"fmt"
	"reflect"
	"strings"

	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
)

type Value struct {
	Source  string `json:"source" yaml:"source"`
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Literal any    `json:"literal,omitempty" yaml:"literal,omitempty"`
}
type Expr struct {
	Op    string `json:"op" yaml:"op"`
	Left  *Value `json:"left,omitempty" yaml:"left,omitempty"`
	Right *Value `json:"right,omitempty" yaml:"right,omitempty"`
	Args  []Expr `json:"args,omitempty" yaml:"args,omitempty"`
}

func Eval(e Expr, c beanctx.Request, input map[string]any) (bool, error) {
	if e.Op == "and" || e.Op == "or" {
		if len(e.Args) == 0 {
			return false, fmt.Errorf("%s requires arguments", e.Op)
		}
		for _, a := range e.Args {
			v, er := Eval(a, c, input)
			if er != nil {
				return false, er
			}
			if e.Op == "and" && !v {
				return false, nil
			}
			if e.Op == "or" && v {
				return true, nil
			}
		}
		return e.Op == "and", nil
	}
	if e.Op == "not" {
		if len(e.Args) != 1 {
			return false, fmt.Errorf("not requires one argument")
		}
		v, er := Eval(e.Args[0], c, input)
		return !v, er
	}
	l, er := resolve(e.Left, c, input)
	if er != nil {
		return false, er
	}
	if e.Op == "is_null" {
		return l == nil, nil
	}
	if e.Op == "is_not_null" {
		return l != nil, nil
	}
	r, er := resolve(e.Right, c, input)
	if er != nil {
		return false, er
	}
	switch e.Op {
	case "eq":
		return reflect.DeepEqual(l, r), nil
	case "ne":
		return !reflect.DeepEqual(l, r), nil
	case "gt", "gte", "lt", "lte":
		return compare(l, r, e.Op)
	case "in", "not_in":
		ok := contains(r, l)
		if e.Op == "not_in" {
			ok = !ok
		}
		return ok, nil
	case "contains":
		return strings.Contains(fmt.Sprint(l), fmt.Sprint(r)), nil
	case "starts_with":
		return strings.HasPrefix(fmt.Sprint(l), fmt.Sprint(r)), nil
	case "ends_with":
		return strings.HasSuffix(fmt.Sprint(l), fmt.Sprint(r)), nil
	default:
		return false, fmt.Errorf("unsupported expression operator %q", e.Op)
	}
}
func resolve(v *Value, c beanctx.Request, in map[string]any) (any, error) {
	if v == nil {
		return nil, fmt.Errorf("missing value")
	}
	switch v.Source {
	case "literal":
		return v.Literal, nil
	case "input":
		return in[v.Name], nil
	case "record":
		return c.Entity[v.Name], nil
	case "user":
		if c.User == nil {
			return nil, nil
		}
		if v.Name == "id" {
			return c.User.ID, nil
		}
		if v.Name == "email" {
			return c.User.Email, nil
		}
	case "tenant":
		return c.TenantID, nil
	case "route":
		return c.RouteParams[v.Name], nil
	case "context":
		return c.Values[v.Name], nil
	}
	return nil, fmt.Errorf("invalid value source %q", v.Source)
}
func compare(a, b any, op string) (bool, error) {
	af, aok := number(a)
	bf, bok := number(b)
	if !aok || !bok {
		return false, fmt.Errorf("comparison requires numbers")
	}
	switch op {
	case "gt":
		return af > bf, nil
	case "gte":
		return af >= bf, nil
	case "lt":
		return af < bf, nil
	default:
		return af <= bf, nil
	}
}
func number(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case float64:
		return x, true
	case float32:
		return float64(x), true
	}
	return 0, false
}
func contains(haystack, needle any) bool {
	v := reflect.ValueOf(haystack)
	if !v.IsValid() || v.Kind() != reflect.Slice {
		return false
	}
	for i := 0; i < v.Len(); i++ {
		if reflect.DeepEqual(v.Index(i).Interface(), needle) {
			return true
		}
	}
	return false
}
func Predicate(e Expr, input map[string]any) (dbal.Predicate, error) {
	return PredicateContext(e, input, beanctx.Request{})
}

func PredicateContext(e Expr, input map[string]any, c beanctx.Request) (dbal.Predicate, error) {
	if e.Op == "and" || e.Op == "or" || e.Op == "not" {
		children := []dbal.Predicate{}
		for _, a := range e.Args {
			p, er := PredicateContext(a, input, c)
			if er != nil {
				return p, er
			}
			children = append(children, p)
		}
		return dbal.Predicate{Op: dbal.Operator(e.Op), Children: children}, nil
	}
	if e.Left == nil || e.Left.Source != "record" {
		return dbal.Predicate{}, fmt.Errorf("database expression left side must be record field")
	}
	var val any
	if e.Right != nil {
		if e.Right.Source == "literal" {
			val = e.Right.Literal
		} else if e.Right.Source == "input" {
			val = input[e.Right.Name]
		} else {
			var er error
			val, er = resolve(e.Right, c, input)
			if er != nil {
				return dbal.Predicate{}, er
			}
		}
	}
	if e.Op == "in" || e.Op == "not_in" {
		xs, ok := val.([]any)
		if !ok {
			return dbal.Predicate{}, fmt.Errorf("in value must be list")
		}
		vs := make([]dbal.Value, len(xs))
		for i, value := range xs {
			vs[i] = value
		}
		val = vs
	}
	return dbal.Predicate{Op: dbal.Operator(e.Op), Column: e.Left.Name, Value: val}, nil
}
