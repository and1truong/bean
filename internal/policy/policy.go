package policy

import (
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/expr"
)

func Can(p appir.Policy, write bool, c beanctx.Request, record map[string]any) bool {
	if p.Authenticated && c.User == nil {
		return false
	}
	roles := p.ReadRoles
	if write {
		roles = p.WriteRoles
	}
	if len(roles) > 0 && !hasRole(c, roles) {
		return false
	}
	bypassOwner := hasRole(c, p.BypassOwnerRoles)
	if p.Owner && !bypassOwner {
		owned := c.User != nil && (record == nil || record["owner_id"] == c.User.ID)
		public := !write && p.OwnerOrPublic && record != nil && record[p.PublicField] == p.PublicValue
		if !owned && !public {
			return false
		}
	}
	if p.Tenant {
		if c.TenantID == "" || record != nil && record["tenant_id"] != c.TenantID {
			return false
		}
	}
	if p.Condition != nil {
		cc := c
		cc.Entity = record
		ok, e := expr.Eval(*p.Condition, cc, nil)
		return e == nil && ok
	}
	return true
}
func Predicate(p appir.Policy, c beanctx.Request) (*dbal.Predicate, bool) {
	ps := []dbal.Predicate{}
	if p.Authenticated && c.User == nil {
		return nil, false
	}
	if len(p.ReadRoles) > 0 && !hasRole(c, p.ReadRoles) {
		return nil, false
	}
	bypassOwner := hasRole(c, p.BypassOwnerRoles)
	if p.Owner && !bypassOwner {
		if p.OwnerOrPublic {
			public := dbal.Predicate{Op: dbal.OpEQ, Column: p.PublicField, Value: p.PublicValue}
			if c.User == nil {
				ps = append(ps, public)
			} else {
				ps = append(ps, dbal.Or(dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: c.User.ID}, public))
			}
		} else if c.User == nil {
			return nil, false
		} else {
			ps = append(ps, dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: c.User.ID})
		}
	}
	if p.Tenant {
		if c.TenantID == "" {
			return nil, false
		}
		ps = append(ps, dbal.Predicate{Op: dbal.OpEQ, Column: "tenant_id", Value: c.TenantID})
	}
	if p.Condition != nil {
		condition, e := expr.PredicateContext(*p.Condition, nil, c)
		if e != nil {
			return nil, false
		}
		ps = append(ps, condition)
	}
	if len(ps) == 0 {
		return nil, true
	}
	x := dbal.And(ps...)
	return &x, true
}
func Redact(row dbal.Row, fields []string) {
	for _, f := range fields {
		delete(row, f)
	}
}
func hasRole(c beanctx.Request, want []string) bool {
	if c.User == nil {
		return false
	}
	for _, got := range c.User.Roles {
		for _, w := range want {
			if got == w {
				return true
			}
		}
	}
	return false
}
