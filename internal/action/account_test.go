package action_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
)

type auditFailureDB struct{ dbal.Database }
type auditFailureTx struct{ dbal.Transaction }

func (d auditFailureDB) Transaction(ctx context.Context, fn func(dbal.Transaction) error) error {
	return d.Database.Transaction(ctx, func(tx dbal.Transaction) error { return fn(auditFailureTx{tx}) })
}
func (tx auditFailureTx) Insert(ctx context.Context, in dbal.Insert) (dbal.Result, error) {
	if in.Table == "bean_audit" {
		return dbal.Result{}, errors.New("injected audit failure")
	}
	return tx.Transaction.Insert(ctx, in)
}

type beforeTransactionDB struct {
	dbal.Database
	before func()
}

func (d beforeTransactionDB) Transaction(ctx context.Context, fn func(dbal.Transaction) error) error {
	d.before()
	return d.Database.Transaction(ctx, fn)
}

func TestAccountActionsRollbackAndStaleLogin(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, filepath.Join(t.TempDir(), "account.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	service := runtime.HTTP.Auth
	if err := service.Create(ctx, "a@example.test", "old-password", []string{"authenticated"}, ""); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(ctx, "a@example.test", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	failing := action.Service{DB: auditFailureDB{runtime.DB}}
	if err := failing.ChangeAccountPassword(ctx, session.ID, "request", action.PasswordChange{CurrentPassword: "old-password", Password: "new-password", Confirmation: "new-password"}); err == nil {
		t.Fatal("audit failure committed")
	}
	if err := failing.RevokeAccountSessions(ctx, session.ID, "request"); err == nil {
		t.Fatal("audit failure committed revocation")
	}
	if err := failing.ResetAccountPassword(ctx, "a@example.test", "reset-password"); err == nil {
		t.Fatal("audit failure committed reset")
	}
	if _, err := service.Current(ctx, session.ID); err != nil {
		t.Fatal("rollback revoked session", err)
	}
	if _, err := service.Login(ctx, "a@example.test", "old-password"); err != nil {
		t.Fatal("rollback changed password", err)
	}
	stale := auth.Service{DB: beforeTransactionDB{runtime.DB, func() {
		if err := runtime.HTTP.Actions.ResetAccountPassword(ctx, "a@example.test", "reset-password"); err != nil {
			t.Fatal(err)
		}
	}}}
	if _, err := stale.Login(ctx, "a@example.test", "old-password"); err == nil {
		t.Fatal("old-password login survived concurrent reset")
	}
	if _, err := service.Current(ctx, session.ID); err == nil {
		t.Fatal("reset kept old session")
	}
	if err := runtime.HTTP.Actions.RevokeAccountSessions(ctx, session.ID, "stale"); err == nil {
		t.Fatal("stale session authorized Action")
	}
	if _, err := service.Login(ctx, "a@example.test", "reset-password"); err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_audit", Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		for _, value := range row {
			if text, ok := value.(string); ok && (strings.Contains(text, "old-password") || strings.Contains(text, "reset-password") || strings.Contains(text, "new-password")) {
				t.Fatal("credential in audit")
			}
		}
	}
}
