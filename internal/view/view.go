package view

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/policy"
)

type Service struct{ DB dbal.Database }
type Params struct {
	Filter        map[string]any
	Limit, Offset int
	Cursor        string
}
type Result struct {
	Rows       []dbal.Row
	NextCursor string
}

type cursor struct {
	Version, View, Signature string
	Values                   []any
}

func (s Service) Run(ctx context.Context, app *appir.App, name string, params Params, c beanctx.Request) ([]dbal.Row, error) {
	result, e := s.RunPage(ctx, app, name, params, c)
	return result.Rows, e
}

func (s Service) RunPage(ctx context.Context, app *appir.App, name string, params Params, c beanctx.Request) (Result, error) {
	v, ok := app.Views[name]
	if !ok {
		return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
	}
	e, ok := app.Entities[v.Entity]
	if !ok {
		return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View entity is invalid"}
	}
	predicates := []dbal.Predicate{}
	if v.Filter != nil {
		p, er := expr.PredicateContext(*v.Filter, params.Filter, c)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, p)
	}
	if v.ContextFilter != nil {
		p, er := expr.PredicateContext(*v.ContextFilter, params.Filter, c)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, p)
	}
	for name, definition := range v.ExposedFilters {
		value, supplied := params.Filter[name]
		if !supplied || value == "" {
			continue
		}
		value = coerce(value, definition.Type)
		if er := field.Validate(definition, value); er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		column := definition.Name
		if column == "" {
			column = name
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: column, Value: value})
	}
	var redact []string
	if v.Policy != "" {
		p, ok := app.Policies[v.Policy]
		if !ok {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View policy not found"}
		}
		injected, allowed := policy.Predicate(p, c)
		if !allowed {
			return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
		}
		if injected != nil {
			predicates = append(predicates, *injected)
		}
		redact = p.Redact
	}
	if v.Policy == "" && e.Owner && c.User != nil {
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: c.User.ID})
	}
	if v.Policy == "" && e.Tenant {
		if c.TenantID == "" {
			return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "tenant_id", Value: c.TenantID})
	}
	if e.SoftDelete {
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpIsNull, Column: "deleted_at"})
	}
	limit := params.Limit
	if limit <= 0 {
		limit = v.DefaultLimit
	}
	if limit <= 0 {
		limit = 50
	}
	max := v.MaxLimit
	if max <= 0 || max > 200 {
		max = 200
	}
	if limit > max {
		limit = max
	}
	offset := params.Offset
	var decoded cursor
	if params.Cursor != "" {
		var er error
		decoded, er = decodeCursor(params.Cursor)
		if er != nil || decoded.Version != app.FormatVersion || decoded.View != name || decoded.Signature != signature(v) {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor is invalid or incompatible with this View"}
		}
		if params.Offset != 0 {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor and offset cannot be combined"}
		}
		offset = 0
	}
	orders := []dbal.Order{}
	for _, o := range v.Sort {
		orders = append(orders, dbal.Order{Column: qualify(o.Field, v.Entity, len(v.Relationships) > 0), Desc: o.Desc})
	}
	if len(v.GroupBy) == 0 && !orderedBy(orders, "id") {
		orders = append(orders, dbal.Order{Column: qualify("id", v.Entity, len(v.Relationships) > 0)})
	}
	if params.Cursor != "" {
		cursorPredicate, er := keysetPredicate(orders, decoded.Values)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor is invalid or incompatible with this View"}
		}
		predicates = append(predicates, cursorPredicate)
	}
	joins := []dbal.Join{}
	for _, relationship := range v.Relationships {
		fieldDefinition := relationField(e, relationship.RelationField)
		if fieldDefinition != nil && toManyRelation(*fieldDefinition) {
			linkAlias := relationship.Name + "_links"
			joins = append(joins,
				dbal.Join{Table: e.Name + "_" + fieldDefinition.Name, Alias: linkAlias, Type: relationship.Type, Left: e.Name + ".id", Right: linkAlias + "." + e.Name + "_id"},
				dbal.Join{Table: relationship.Entity, Alias: relationship.Name, Type: relationship.Type, Left: linkAlias + "." + relationship.Entity + "_id", Right: relationship.Name + "." + relationship.TargetField},
			)
		} else {
			joins = append(joins, dbal.Join{Table: relationship.Entity, Alias: relationship.Name, Type: relationship.Type, Left: v.Entity + "." + relationship.LocalField, Right: relationship.Name + "." + relationship.TargetField})
		}
	}
	aggregates := []dbal.Aggregate{}
	for _, aggregate := range v.Aggregates {
		function := aggregate.Function
		if function == "average" {
			function = "avg"
		}
		aggregates = append(aggregates, dbal.Aggregate{Function: function, Column: aggregate.Field, Alias: aggregate.Alias})
	}
	var where *dbal.Predicate
	if len(predicates) == 1 {
		where = &predicates[0]
	} else if len(predicates) > 1 {
		x := dbal.And(predicates...)
		where = &x
	}
	rows, er := s.DB.Select(ctx, dbal.Select{Table: v.Entity, Columns: qualifyAll(v.Fields, v.Entity, len(v.Relationships) > 0), Joins: joins, Where: where, GroupBy: qualifyAll(v.GroupBy, v.Entity, len(v.Relationships) > 0), Aggregates: aggregates, OrderBy: orders, Limit: limit + 1, Offset: offset})
	if er != nil {
		return Result{}, er
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		values := make([]any, len(orders))
		for i, order := range orders {
			parts := strings.Split(order.Column, ".")
			values[i] = last[parts[len(parts)-1]]
		}
		next = encodeCursor(cursor{Version: app.FormatVersion, View: name, Signature: signature(v), Values: values})
	}
	for _, row := range rows {
		policy.Redact(row, redact)
	}
	return Result{Rows: rows, NextCursor: next}, nil
}

func coerce(value any, fieldType string) any {
	s, ok := value.(string)
	if !ok {
		return value
	}
	switch fieldType {
	case "integer", "money":
		if v, e := strconv.ParseInt(s, 10, 64); e == nil {
			return v
		}
	case "boolean":
		if v, e := strconv.ParseBool(s); e == nil {
			return v
		}
	}
	return value
}

func relationField(entity appir.Entity, name string) *appir.Field {
	for _, field := range entity.Fields {
		if field.Name == name {
			copy := field
			return &copy
		}
	}
	return nil
}
func toManyRelation(field appir.Field) bool {
	return field.Relation != nil && (field.Relation.Kind == "one-to-many" || field.Relation.Kind == "many-to-many")
}

func orderedBy(orders []dbal.Order, field string) bool {
	for _, order := range orders {
		if order.Column == field || strings.HasSuffix(order.Column, "."+field) {
			return true
		}
	}
	return false
}

func keysetPredicate(orders []dbal.Order, values []any) (dbal.Predicate, error) {
	if len(orders) == 0 || len(orders) != len(values) {
		return dbal.Predicate{}, fmt.Errorf("cursor ordering mismatch")
	}
	branches := make([]dbal.Predicate, 0, len(orders))
	for i, order := range orders {
		parts := make([]dbal.Predicate, 0, i+1)
		for previous := 0; previous < i; previous++ {
			parts = append(parts, dbal.Predicate{Op: dbal.OpEQ, Column: orders[previous].Column, Value: values[previous]})
		}
		op := dbal.OpGT
		if order.Desc {
			op = dbal.OpLT
		}
		parts = append(parts, dbal.Predicate{Op: op, Column: order.Column, Value: values[i]})
		branches = append(branches, dbal.And(parts...))
	}
	return dbal.Or(branches...), nil
}

func qualify(name, entity string, joined bool) string {
	if joined && !strings.Contains(name, ".") {
		return entity + "." + name
	}
	return name
}
func qualifyAll(names []string, entity string, joined bool) []string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = qualify(name, entity, joined)
	}
	return out
}

func signature(v appir.View) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func encodeCursor(value cursor) string {
	b, _ := json.Marshal(value)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(value string) (cursor, error) {
	var out cursor
	b, e := base64.RawURLEncoding.DecodeString(value)
	if e != nil {
		return out, e
	}
	e = json.Unmarshal(b, &out)
	if e != nil || len(out.Values) == 0 {
		return out, fmt.Errorf("invalid cursor")
	}
	return out, nil
}
func Display(app *appir.App, route string) (string, appir.Display, bool) {
	for name, v := range app.Views {
		for _, d := range v.Displays {
			if d.Route == route {
				return name, d, true
			}
		}
	}
	return "", appir.Display{}, false
}
