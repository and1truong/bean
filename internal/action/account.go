package action

import (
	"context"
	"fmt"

	"github.com/beanruntime/bean/internal/audit"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/dbal"
)

// Account Actions are built-in identity operations, independent of application
// metadata. They accept only a session proof, never a client-selected user ID.
// They deliberately do not use generic input hashing or idempotency storage.
type PasswordChange struct {
	CurrentPassword string `json:"currentPassword"`
	Password        string `json:"password"`
	Confirmation    string `json:"confirmation"`
}

func (s Service) ChangeAccountPassword(ctx context.Context, sessionID, requestID string, input PasswordChange) error {
	if err := auth.ValidateNewPassword(input.Password, input.Confirmation); err != nil {
		return err
	}
	return s.accountMutation(ctx, sessionID, requestID, "system_password_change", func(tx dbal.Transaction, user dbal.Row) error {
		return auth.ChangePassword(ctx, tx, user, input.CurrentPassword, input.Password)
	})
}

func (s Service) RevokeAccountSessions(ctx context.Context, sessionID, requestID string) error {
	return s.accountMutation(ctx, sessionID, requestID, "system_sessions_revoke", func(dbal.Transaction, dbal.Row) error { return nil })
}

func (s Service) accountMutation(ctx context.Context, sessionID, requestID, operation string, mutate func(dbal.Transaction, dbal.Row) error) error {
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		user, err := auth.AccountSession(ctx, tx, sessionID)
		if err != nil {
			return err
		}
		if err := mutate(tx, user); err != nil {
			return err
		}
		userID := fmt.Sprint(user["id"])
		if err := auth.RevokeSessions(ctx, tx, userID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{RequestID: requestID, UserID: userID, Action: operation, EntityType: "bean_user", EntityID: userID, Changed: []string{"credentials_or_sessions"}, Success: true})
	})
}

// ResetAccountPassword is a host-only recovery Action. The caller must already
// possess database access; no HTTP or application Action dispatch exposes it.
func (s Service) ResetAccountPassword(ctx context.Context, email, password string) error {
	if err := auth.ValidateNewPassword(password, password); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		user, err := auth.HostResetPassword(ctx, tx, email, password)
		if err != nil {
			return err
		}
		userID := fmt.Sprint(user["id"])
		if err := auth.RevokeSessions(ctx, tx, userID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{UserID: "host_operator", Action: "system_password_reset", EntityType: "bean_user", EntityID: userID, Changed: []string{"credentials_or_sessions"}, Success: true})
	})
}
