package valuesource

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	beanctx "github.com/beanruntime/bean/internal/context"
)

type Context string

const (
	Expression Context = "expression"
	Action     Context = "action"
	Block      Context = "block"
	Page       Context = "page"
)

const (
	Literal = "literal"
	Input   = "input"
	Record  = "record"
	Result  = "result"
	Request = "context"
	Now     = "now"
	Tenant  = "tenant"
	User    = "user"
	Route   = "route"
	Query   = "query"
)

var allowed = map[Context]map[string]bool{
	Expression: {Literal: true, Input: true, Record: true, User: true, Tenant: true, Route: true, Request: true},
	Action:     {Literal: true, Input: true, Record: true, Result: true, Request: true, Now: true, Tenant: true, User: true},
	Block:      {Request: true, Tenant: true, User: true},
	Page:       {Route: true, Query: true, Tenant: true, User: true},
}

func Allows(context Context, source string) bool {
	return allowed[context][source]
}

func Names(context Context) []string {
	names := make([]string, 0, len(allowed[context]))
	for name := range allowed[context] {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func IsLiteral(source string) bool { return source == Literal }

type Environment struct {
	Request beanctx.Request
	Literal any
	Input   map[string]any
	Record  map[string]any
	Results map[string]any
	Context map[string]any
	Route   map[string]string
	Query   map[string]string
}

func Resolve(context Context, source, path string, environment Environment) (any, error) {
	if !Allows(context, source) {
		return nil, fmt.Errorf("unsupported %s value source %q", context, source)
	}
	switch source {
	case Literal:
		return environment.Literal, nil
	case Input:
		return lookup(environment.Input, path), nil
	case Record:
		record := environment.Record
		if record == nil {
			record = environment.Request.Entity
		}
		return lookup(record, path), nil
	case Result:
		return resolvePath(environment.Results, path)
	case Request:
		values := environment.Context
		if values == nil {
			values = environment.Request.Values
		}
		return lookup(values, path), nil
	case Now:
		values := environment.Context
		if values == nil {
			values = environment.Request.Values
		}
		return lookup(values, "now"), nil
	case Tenant:
		return environment.Request.TenantID, nil
	case User:
		if environment.Request.User == nil {
			return nil, nil
		}
		if path == "id" {
			return environment.Request.User.ID, nil
		}
		if path == "email" && context != Block {
			return environment.Request.User.Email, nil
		}
		if path == "display_name" && context == Action {
			return environment.Request.User.DisplayName, nil
		}
		return nil, fmt.Errorf("unsupported %s user value %q", context, path)
	case Route:
		route := environment.Route
		if route == nil {
			route = environment.Request.RouteParams
		}
		return route[path], nil
	case Query:
		return environment.Query[path], nil
	default:
		return nil, fmt.Errorf("unsupported %s value source %q", context, source)
	}
}

func lookup(values map[string]any, path string) any {
	if values == nil {
		return nil
	}
	return values[path]
}

func resolvePath(results map[string]any, path string) (any, error) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 || parts[0] == "" {
		return nil, fmt.Errorf("result path is missing")
	}
	var value any = results[parts[0]]
	for _, part := range parts[1:] {
		if value == nil {
			return nil, nil
		}
		reflected := reflect.ValueOf(value)
		if reflected.Kind() == reflect.Pointer {
			reflected = reflected.Elem()
		}
		switch reflected.Kind() {
		case reflect.Map:
			key := reflect.ValueOf(part)
			if !key.Type().AssignableTo(reflected.Type().Key()) {
				return nil, fmt.Errorf("result path %q cannot index a map", path)
			}
			item := reflected.MapIndex(key)
			if !item.IsValid() {
				return nil, nil
			}
			value = item.Interface()
		case reflect.Slice, reflect.Array:
			index, err := strconv.Atoi(part)
			if err != nil || index < 0 || index >= reflected.Len() {
				return nil, nil
			}
			value = reflected.Index(index).Interface()
		default:
			return nil, fmt.Errorf("result path %q cannot traverse %T", path, value)
		}
	}
	return value, nil
}
