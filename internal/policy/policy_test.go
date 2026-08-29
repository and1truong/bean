package policy_test

import (
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/policy"
)

func TestEnforcementMatrix(t *testing.T) {
	user := &beanctx.User{ID: "00000000-0000-4000-8000-000000000001", Roles: []string{"member"}}
	other := "00000000-0000-4000-8000-000000000002"
	tenant := "00000000-0000-4000-8000-00000000000a"
	condition := &expr.Expr{Op: "eq", Left: &expr.Value{Source: "record", Name: "published"}, Right: &expr.Value{Source: "literal", Literal: true}}
	tests := []struct {
		name   string
		policy appir.Policy
		write  bool
		ctx    beanctx.Request
		row    map[string]any
		want   bool
	}{
		{"anonymous read", appir.Policy{}, false, beanctx.Request{}, nil, true},
		{"authenticated denial", appir.Policy{Authenticated: true}, false, beanctx.Request{}, nil, false},
		{"authenticated", appir.Policy{Authenticated: true}, false, beanctx.Request{User: user}, nil, true},
		{"read role", appir.Policy{ReadRoles: []string{"member"}}, false, beanctx.Request{User: user}, nil, true},
		{"write role distinct", appir.Policy{WriteRoles: []string{"editor"}}, true, beanctx.Request{User: user}, nil, false},
		{"owner", appir.Policy{Owner: true}, true, beanctx.Request{User: user}, map[string]any{"owner_id": user.ID}, true},
		{"known foreign owner", appir.Policy{Owner: true}, true, beanctx.Request{User: user}, map[string]any{"owner_id": other}, false},
		{"tenant", appir.Policy{Tenant: true}, false, beanctx.Request{TenantID: tenant}, map[string]any{"tenant_id": tenant}, true},
		{"foreign tenant", appir.Policy{Tenant: true}, false, beanctx.Request{TenantID: tenant}, map[string]any{"tenant_id": other}, false},
		{"field condition", appir.Policy{Condition: condition}, false, beanctx.Request{}, map[string]any{"published": true}, true},
		{"field condition denied", appir.Policy{Condition: condition}, false, beanctx.Request{}, map[string]any{"published": false}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.Can(test.policy, test.write, test.ctx, test.row); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
}

func TestListPredicateIncludesCondition(t *testing.T) {
	p := appir.Policy{Condition: &expr.Expr{Op: "eq", Left: &expr.Value{Source: "record", Name: "published"}, Right: &expr.Value{Source: "literal", Literal: true}}}
	predicate, allowed := policy.Predicate(p, beanctx.Request{})
	if !allowed || predicate == nil || len(predicate.Children) != 1 || predicate.Children[0].Column != "published" {
		t.Fatalf("predicate=%+v allowed=%v", predicate, allowed)
	}
}
