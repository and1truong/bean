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
	contentfilter "github.com/beanruntime/bean/internal/filter"
	"github.com/beanruntime/bean/internal/policy"
)

type Service struct{ DB dbal.Database }

type Reader interface {
	Select(context.Context, dbal.Select) ([]dbal.Row, error)
}

type ReadOptions struct {
	Params
	ExpressionValues   map[string]any
	ExtraPredicate     *expr.Expr
	ApplyExposedFilter bool
}

type Params struct {
	Filter        map[string]any
	ExactFilters  map[string]any
	Search        string
	RecordID      string
	SearchFields  []string
	Sort          []appir.Sort
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
	return ReadPage(ctx, s.DB, app, name, ReadOptions{Params: params, ExpressionValues: params.Filter, ApplyExposedFilter: true}, c)
}

func ReadPage(ctx context.Context, reader Reader, app *appir.App, name string, options ReadOptions, c beanctx.Request) (Result, error) {
	params := options.Params
	v, ok := app.Views[name]
	if !ok {
		return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
	}
	e, ok := app.Entities[v.Entity]
	if !ok {
		return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View entity is invalid"}
	}
	predicates := []dbal.Predicate{}
	joined := len(v.Relationships) > 0
	if v.Filter != nil {
		p, er := expr.PredicateContext(*v.Filter, options.ExpressionValues, c)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, qualifyPredicateColumns(p, v.Entity, joined))
	}
	if v.ContextFilter != nil {
		p, er := expr.PredicateContext(*v.ContextFilter, options.ExpressionValues, c)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, qualifyPredicateColumns(p, v.Entity, joined))
	}
	if options.ExtraPredicate != nil {
		p, er := expr.PredicateContext(*options.ExtraPredicate, options.ExpressionValues, c)
		if er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, qualifyPredicateColumns(p, v.Entity, joined))
	}
	if options.ApplyExposedFilter {
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
			predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: qualify(column, v.Entity, joined), Value: value})
		}
	}
	available := map[string]bool{}
	for _, name := range v.Fields {
		available[name] = true
	}
	for _, aggregate := range v.Aggregates {
		available[aggregate.Alias] = true
	}
	for name, value := range params.ExactFilters {
		if !available[name] {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "admin filter field is not selected by the View"}
		}
		definition, ok := entityField(e, name)
		if !ok {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "admin filter field is invalid"}
		}
		value = coerce(value, definition.Type)
		if er := field.Validate(definition, value); er != nil {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: name, Value: value})
	}
	if params.RecordID != "" {
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: qualify("id", v.Entity, joined), Value: params.RecordID})
	}
	if params.Search != "" {
		search := make([]dbal.Predicate, 0, len(params.SearchFields))
		for _, name := range params.SearchFields {
			if !available[name] {
				return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "admin search field is not selected by the View"}
			}
			search = append(search, dbal.Predicate{Op: dbal.OpContains, Column: qualify(name, v.Entity, joined), Value: params.Search})
		}
		if len(search) > 0 {
			predicates = append(predicates, dbal.Or(search...))
		}
	}
	var redact []string
	policyName := v.Policy
	if policyName == "" {
		policyName = e.Policy
	}
	if policyName != "" {
		p, ok := app.Policies[policyName]
		if !ok {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View policy not found"}
		}
		injected, allowed := policy.Predicate(p, c)
		if !allowed {
			return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
		}
		if injected != nil {
			predicates = append(predicates, qualifyPredicateColumns(*injected, v.Entity, joined))
		}
		redact = p.Redact
	}
	if policyName == "" && e.Owner {
		if c.User == nil {
			return Result{}, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: c.User.ID})
	}
	if policyName == "" && e.Tenant {
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
	aggregateAliases := map[string]bool{}
	for _, aggregate := range v.Aggregates {
		aggregateAliases[aggregate.Alias] = true
	}
	orders := []dbal.Order{}
	sortDefinitions := v.Sort
	if len(params.Sort) > 0 {
		sortDefinitions = params.Sort
	}
	aggregateSort := false
	for _, o := range sortDefinitions {
		if !available[o.Field] {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "admin sort field is not selected by the View"}
		}
		column := o.Field
		if aggregateAliases[column] {
			aggregateSort = true
		} else {
			column = qualify(column, v.Entity, joined)
		}
		orders = append(orders, dbal.Order{Column: column, Desc: o.Desc})
	}
	if aggregateSort {
		for _, group := range v.GroupBy {
			if !orderedBy(orders, group) {
				orders = append(orders, dbal.Order{Column: qualify(group, v.Entity, joined)})
			}
		}
	}
	if len(v.GroupBy) == 0 && len(v.Aggregates) == 0 && !orderedBy(orders, "id") {
		orders = append(orders, dbal.Order{Column: qualify("id", v.Entity, joined)})
	}
	if params.Cursor != "" {
		if aggregateSort {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor pagination is not supported for aggregate-sorted Views; use offset pagination"}
		}
		decoded, er := decodeCursor(params.Cursor)
		if er != nil || decoded.Version != app.FormatVersion || decoded.View != name || decoded.Signature != signature(v, params) {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor is invalid or incompatible with this View"}
		}
		if params.Offset != 0 {
			return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "cursor and offset cannot be combined"}
		}
		offset = 0
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
		aggregates = append(aggregates, dbal.Aggregate{Function: function, Column: qualify(aggregate.Field, v.Entity, joined), Alias: aggregate.Alias})
	}
	var where *dbal.Predicate
	if len(predicates) == 1 {
		where = &predicates[0]
	} else if len(predicates) > 1 {
		x := dbal.And(predicates...)
		where = &x
	}
	columns := qualifyAll(storedFields(v.Fields, e), v.Entity, joined)
	hiddenCursorFields := map[string]bool{}
	if !aggregateSort {
		for _, order := range orders {
			if !containsColumn(columns, order.Column) {
				columns = append(columns, order.Column)
				hiddenCursorFields[strings.ReplaceAll(order.Column, ".", "__")] = true
			}
		}
	}
	rows, er := reader.Select(ctx, dbal.Select{Table: v.Entity, Columns: columns, Joins: joins, Where: where, GroupBy: qualifyAll(v.GroupBy, v.Entity, joined), Aggregates: aggregates, OrderBy: orders, Limit: limit + 1, Offset: offset})
	if er != nil {
		return Result{}, er
	}
	next := ""
	if len(rows) > limit {
		rows = rows[:limit]
		if !aggregateSort {
			last := rows[len(rows)-1]
			values := make([]any, len(orders))
			for i, order := range orders {
				key := strings.ReplaceAll(order.Column, ".", "__")
				values[i] = last[key]
			}
			next = encodeCursor(cursor{Version: app.FormatVersion, View: name, Signature: signature(v, params), Values: values})
		}
	}
	for _, row := range rows {
		for fieldName := range hiddenCursorFields {
			delete(row, fieldName)
		}
		for _, selected := range v.Fields {
			compiled := qualify(selected, v.Entity, len(v.Relationships) > 0)
			encoded := strings.ReplaceAll(compiled, ".", "__")
			if encoded != selected {
				value, exists := row[encoded]
				if !exists {
					continue
				}
				row[selected] = value
				delete(row, encoded)
				parts := strings.Split(selected, ".")
				if len(parts) == 2 {
					if _, exists = row[parts[1]]; !exists {
						row[parts[1]] = value
					}
				}
			}
		}
		for _, definition := range e.Fields {
			if value, ok := row[definition.Name]; ok {
				decoded := field.Decode(definition, value)
				if definition.Type == "richtext" {
					if _, filtered := v.FieldFilters[definition.Name]; !filtered {
						if source, textual := decoded.(string); textual {
							decoded = contentfilter.SanitizeHTML(source)
						}
					}
				}
				row[definition.Name] = decoded
			}
		}
		if er = loadToMany(ctx, reader, e, v.Fields, row); er != nil {
			return Result{}, er
		}
		policy.Redact(row, redact)
		for fieldName, filterName := range v.FieldFilters {
			value, exists := row[fieldName]
			if !exists || value == nil {
				continue
			}
			source, ok := value.(string)
			if !ok {
				return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View filter field is not textual"}
			}
			definition, exists := app.Filters[filterName]
			if !exists {
				return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View Filter is invalid"}
			}
			formatted, filterErr := contentfilter.Apply(definition, source)
			if filterErr != nil {
				return Result{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "View Filter is invalid"}
			}
			row[fieldName] = formatted
			parts := strings.Split(fieldName, ".")
			if len(parts) == 2 && row[parts[1]] == source {
				row[parts[1]] = formatted
			}
		}
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
func entityField(entity appir.Entity, name string) (appir.Field, bool) {
	for _, definition := range entity.Fields {
		if definition.Name == name {
			return definition, true
		}
	}
	for _, definition := range []appir.Field{{Name: "id", Type: "uuid"}, {Name: "created_at", Type: "datetime"}, {Name: "updated_at", Type: "datetime"}, {Name: "version", Type: "integer"}} {
		if definition.Name == name {
			return definition, true
		}
	}
	return appir.Field{}, false
}
func toManyRelation(field appir.Field) bool {
	return field.Relation != nil && (field.Relation.Kind == "one-to-many" || field.Relation.Kind == "many-to-many")
}

func storedFields(names []string, entity appir.Entity) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		definition, exists := entityField(entity, name)
		if exists && toManyRelation(definition) {
			continue
		}
		out = append(out, name)
	}
	return out
}

type rowSelector interface {
	Select(context.Context, dbal.Select) ([]dbal.Row, error)
}

func loadToMany(ctx context.Context, selector rowSelector, entity appir.Entity, selected []string, row dbal.Row) error {
	for _, name := range selected {
		definition, exists := entityField(entity, name)
		if !exists || !toManyRelation(definition) {
			continue
		}
		entityColumn := entity.Name + "_id"
		targetColumn := definition.Relation.Entity + "_id"
		links, err := selector.Select(ctx, dbal.Select{Table: entity.Name + "_" + definition.Name, Columns: []string{targetColumn}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: entityColumn, Value: row["id"]}, OrderBy: []dbal.Order{{Column: targetColumn}}, Limit: 10000})
		if err != nil {
			return err
		}
		values := make([]any, len(links))
		for index := range links {
			values[index] = links[index][targetColumn]
		}
		row[name] = values
	}
	return nil
}

func containsColumn(columns []string, wanted string) bool {
	for _, column := range columns {
		if column == wanted {
			return true
		}
	}
	return false
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

func qualifyPredicateColumns(predicate dbal.Predicate, entity string, joined bool) dbal.Predicate {
	predicate.Column = qualify(predicate.Column, entity, joined)
	for index := range predicate.Children {
		predicate.Children[index] = qualifyPredicateColumns(predicate.Children[index], entity, joined)
	}
	return predicate
}

func signature(v appir.View, params Params) string {
	b, _ := json.Marshal(struct {
		View         appir.View
		ExactFilters map[string]any
		Search       string
		SearchFields []string
		Sort         []appir.Sort
	}{v, params.ExactFilters, params.Search, params.SearchFields, params.Sort})
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
