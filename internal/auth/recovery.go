package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
)

func recoveryDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func invalidRecovery() error {
	return &dbal.Error{Code: dbal.InvalidQuery, Message: "reset link is invalid or expired"}
}

// IssueRecovery runs under the account lock and retains consumed rows as
// durable request receipts, so worker retries cannot reissue a used token.
func IssueRecovery(ctx context.Context, tx dbal.Transaction, id, email, appID, releaseID string, expires time.Time) (string, error) {
	user, err := userForAccount(ctx, tx, "email", normalizeEmail(email))
	if dbal.IsCode(err, dbal.NotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	existing, err := tx.Select(ctx, dbal.Select{Table: "bean_auth_token", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return "", err
	}
	if len(existing) > 0 {
		return "", nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	_, err = tx.Insert(ctx, dbal.Insert{Table: "bean_auth_token", Values: map[string]dbal.Value{"id": id, "digest": recoveryDigest(token), "user_id": user["id"], "app_id": appID, "release_id": releaseID, "purpose": "password_reset", "expires_at": expires.UTC().Format(time.RFC3339Nano)}})
	return token, err
}

func ResetWithToken(ctx context.Context, tx dbal.Transaction, appID, releaseID, token, password string, now time.Time) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(raw) != 32 || len(token) != 43 {
		return "", invalidRecovery()
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "digest", Value: recoveryDigest(token)}, dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: appID}, dbal.Predicate{Op: dbal.OpEQ, Column: "release_id", Value: releaseID}, dbal.Predicate{Op: dbal.OpEQ, Column: "purpose", Value: "password_reset"})
	query := dbal.Select{Table: "bean_auth_token", Where: &where, Limit: 1}
	tokens, err := tx.Select(ctx, query)
	if err != nil {
		return "", err
	}
	if len(tokens) != 1 {
		return "", invalidRecovery()
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
		return "", invalidRecovery()
	}
	expires, err := time.Parse(time.RFC3339Nano, fmt.Sprint(tokens[0]["expires_at"]))
	if err != nil || !expires.After(now) {
		return "", invalidRecovery()
	}
	if err := replacePassword(ctx, tx, user, password); err != nil {
		return "", err
	}
	return fmt.Sprint(user["id"]), nil
}

func invalidateRecovery(ctx context.Context, tx dbal.Transaction, userID any) error {
	_, err := tx.Update(ctx, dbal.Update{Table: "bean_auth_token", Values: map[string]dbal.Value{"consumed_at": time.Now().UTC().Format(time.RFC3339Nano)}, Where: dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "user_id", Value: userID}, dbal.Predicate{Op: dbal.OpIsNull, Column: "consumed_at"})})
	return err
}
