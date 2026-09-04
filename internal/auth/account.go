package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"golang.org/x/crypto/bcrypt"
)

// LockUser serializes credential/session mutations with session creation. The
// compare-and-swap also rejects a stale password read on concurrent resets.
func LockUser(ctx context.Context, tx dbal.Transaction, user dbal.Row) error {
	_, err := tx.Update(ctx, dbal.Update{Table: "bean_user", Values: map[string]dbal.Value{"password_hash": user["password_hash"]}, Where: dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: user["id"]}, dbal.Predicate{Op: dbal.OpEQ, Column: "password_hash", Value: user["password_hash"]}), ExpectedRows: 1})
	return err
}

func userForAccount(ctx context.Context, tx dbal.Transaction, column, value string) (dbal.Row, error) {
	rows, err := tx.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: column, Value: value}, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "account not found"}
	}
	if err := LockUser(ctx, tx, rows[0]); err != nil {
		return nil, err
	}
	return rows[0], nil
}

// AccountSession revalidates the session under the user lock, so an in-flight
// request cannot reuse a session revoked by another account Action.
func AccountSession(ctx context.Context, tx dbal.Transaction, sessionID string) (dbal.Row, error) {
	query := dbal.Select{Table: "bean_session", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: sessionID}, Limit: 1}
	rows, err := tx.Select(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "session not found"}
	}
	user, err := userForAccount(ctx, tx, "id", fmt.Sprint(rows[0]["user_id"]))
	if err != nil {
		return nil, err
	}
	rows, err = tx.Select(ctx, query)
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "session not found"}
	}
	expires, err := time.Parse(time.RFC3339Nano, fmt.Sprint(rows[0]["expires_at"]))
	if err != nil || !expires.After(time.Now()) {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "session expired"}
	}
	return user, nil
}

func ValidateNewPassword(password, confirmation string) error {
	if password != confirmation {
		return &dbal.Error{Code: dbal.InvalidQuery, Message: "password confirmation does not match"}
	}
	if len(password) < 10 || len(password) > 72 {
		return &dbal.Error{Code: dbal.InvalidQuery, Message: "password must be between 10 and 72 bytes"}
	}
	return nil
}

func ChangePassword(ctx context.Context, tx dbal.Transaction, user dbal.Row, current, password string) error {
	if bcrypt.CompareHashAndPassword([]byte(fmt.Sprint(user["password_hash"])), []byte(current)) != nil {
		return &dbal.Error{Code: dbal.Conflict, Message: "current password is incorrect"}
	}
	return replacePassword(ctx, tx, user, password)
}

// HostResetPassword is for a trusted host operator with database access, not a
// remote request or application role. It never creates a missing account.
func HostResetPassword(ctx context.Context, tx dbal.Transaction, email, password string) (dbal.Row, error) {
	user, err := userForAccount(ctx, tx, "email", normalizeEmail(email))
	if err != nil {
		return nil, err
	}
	return user, replacePassword(ctx, tx, user, password)
}

func replacePassword(ctx context.Context, tx dbal.Transaction, user dbal.Row, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = tx.Update(ctx, dbal.Update{Table: "bean_user", Values: map[string]dbal.Value{"password_hash": string(hash)}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: user["id"]}, ExpectedRows: 1})
	return err
}

func RevokeSessions(ctx context.Context, tx dbal.Transaction, userID string) error {
	_, err := tx.Delete(ctx, dbal.Delete{Table: "bean_session", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "user_id", Value: userID}})
	return err
}
