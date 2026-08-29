package view

import (
	"context"
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/policy"
)

type Service struct{ DB dbal.Database }
type Params struct {
	Filter        map[string]any
	Limit, Offset int
}

func (s Service) Run(ctx context.Context, app *appir.App, name string, params Params, c beanctx.Request) ([]dbal.Row, error) {
	v, ok := app.Views[name]
	if !ok {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
	}
	e := app.Entities[v.Entity]
	predicates := []dbal.Predicate{}
	if v.Filter != nil {
		p, er := expr.Predicate(*v.Filter, params.Filter)
		if er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		predicates = append(predicates, p)
	}
	var redact []string
	if v.Policy != "" {
		p, ok := app.Policies[v.Policy]
		if !ok {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "View policy not found"}
		}
		injected, allowed := policy.Predicate(p, c)
		if !allowed {
			return nil, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
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
			return nil, &dbal.Error{Code: dbal.NotFound, Message: "View not found"}
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
	orders := []dbal.Order{}
	for _, o := range v.Sort {
		orders = append(orders, dbal.Order{Column: o.Field, Desc: o.Desc})
	}
	var where *dbal.Predicate
	if len(predicates) == 1 {
		where = &predicates[0]
	} else if len(predicates) > 1 {
		x := dbal.And(predicates...)
		where = &x
	}
	rows, er := s.DB.Select(ctx, dbal.Select{Table: v.Entity, Columns: v.Fields, Where: where, OrderBy: orders, Limit: limit, Offset: params.Offset})
	if er != nil {
		return nil, er
	}
	for _, row := range rows {
		policy.Redact(row, redact)
	}
	return rows, nil
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

var _ = fmt.Sprint
