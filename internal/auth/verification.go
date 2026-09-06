package auth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"golang.org/x/crypto/bcrypt"
)

var ErrEmailUnverified = errors.New("verify your email before signing in")

func (s Service) CheckVerified(user dbal.Row) error {
	if s.VerificationRequired != nil && s.VerificationRequired() && user["email_verified_at"] == nil {
		return ErrEmailUnverified
	}
	return nil
}
func IssueVerification(ctx context.Context, tx dbal.Transaction, id, email, appID, releaseID string, expires time.Time) (string, error) {
	return issueEmailToken(ctx, tx, id, email, appID, releaseID, "email_verify", expires)
}
func invalidVerification() error {
	return &dbal.Error{Code: dbal.InvalidQuery, Message: "verification link or password is invalid or expired"}
}

// VerifyEmail requires both mailbox possession and the account password. This
// prevents an unsolicited click from activating an attacker-created account.
func VerifyEmail(ctx context.Context, tx dbal.Transaction, appID, releaseID, token, password string, now time.Time) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || len(token) != 43 {
		return "", invalidVerification()
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "digest", Value: recoveryDigest(token)}, dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, dbal.Predicate{Op: dbal.OpEQ, Column: "release_id", Value: releaseID}, dbal.Predicate{Op: dbal.OpEQ, Column: "purpose", Value: "email_verify"})
	query := dbal.Select{Table: "bean_auth_token", Where: &where, Limit: 1}
	tokens, err := tx.Select(ctx, query)
	if err != nil {
		return "", err
	}
	if len(tokens) != 1 {
		return "", invalidVerification()
	}
	user, err := userForAccount(ctx, tx, "id", fmt.Sprint(tokens[0]["user_id"]))
	if err != nil {
		return "", err
	}
	tokens, err = tx.Select(ctx, query)
	if err != nil {
		return "", err
	}
	if len(tokens) != 1 || tokens[0]["consumed_at"] != nil {
		return "", invalidVerification()
	}
	expires, err := time.Parse(time.RFC3339Nano, fmt.Sprint(tokens[0]["expires_at"]))
	if err != nil || !expires.After(now) || bcrypt.CompareHashAndPassword([]byte(fmt.Sprint(user["password_hash"])), []byte(password)) != nil {
		return "", invalidVerification()
	}
	stamp := now.UTC().Format(time.RFC3339Nano)
	if _, err := tx.Update(ctx, dbal.Update{Table: "bean_user", Values: map[string]dbal.Value{"email_verified_at": stamp}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: user["id"]}, ExpectedRows: 1}); err != nil {
		return "", err
	}
	if _, err := tx.Update(ctx, dbal.Update{Table: "bean_auth_token", Values: map[string]dbal.Value{"consumed_at": stamp}, Where: dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "user_id", Value: user["id"]}, dbal.Predicate{Op: dbal.OpEQ, Column: "purpose", Value: "email_verify"}, dbal.Predicate{Op: dbal.OpIsNull, Column: "consumed_at"})}); err != nil {
		return "", err
	}
	return fmt.Sprint(user["id"]), nil
}
