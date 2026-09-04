package action

import (
	"context"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/audit"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/authmail"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/uid"
)

func (s Service) recoveryAvailable(app *appir.App) error {
	if app == nil || !app.PasswordRecoveryEnabled() {
		return &dbal.Error{Code: dbal.NotFound, Message: "password recovery is not enabled"}
	}
	if s.AuthMail == nil {
		return &dbal.Error{Code: dbal.Unavailable, Message: "auth email delivery is not configured"}
	}
	return nil
}

// RequestPasswordRecovery does not read account existence; every valid address
// follows the same durable encrypted enqueue path and gets the same response.
func (s Service) RequestPasswordRecovery(ctx context.Context, app *appir.App, email string) error {
	if err := s.recoveryAvailable(app); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !authmail.ValidEmail(email) {
		return &dbal.Error{Code: dbal.InvalidQuery, Message: "enter a valid email address"}
	}
	message := authmail.Envelope{ID: uid.New(), AppID: app.AppID, ReleaseID: app.ReleaseID, Email: email, Expires: s.now().Add(15 * time.Minute)}
	payload, err := s.AuthMail.Seal(authmail.RequestTopic, message)
	if err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		_, err := event.Enqueue(ctx, tx, authmail.RequestTopic, payload, event.Options{ID: message.ID, MaxAttempts: 3, RetryDelay: 30 * time.Second})
		return err
	})
}

// DeliverAuthMail is called only by the committed outbox worker. Decryption is
// authenticated per topic; release replacement/disable discards stale intents.
func (s Service) DeliverAuthMail(ctx context.Context, app *appir.App, topic string, payload map[string]any) error {
	if s.AuthMail == nil {
		return authmail.ErrConfiguration
	}
	message, err := s.AuthMail.Open(topic, payload)
	if err != nil {
		return err
	}
	if app == nil || !app.PasswordRecoveryEnabled() || app.AppID != message.AppID || app.ReleaseID != message.ReleaseID || !message.Expires.After(s.now()) {
		return nil
	}
	switch topic {
	case authmail.RequestTopic:
		return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
			token, err := auth.IssueRecovery(ctx, tx, message.ID, message.Email, message.AppID, message.ReleaseID, message.Expires)
			if err != nil {
				return &dbal.Error{Code: dbal.Unavailable, Message: "auth recovery preparation failed"}
			}
			if token == "" {
				return nil
			}
			message.Token = token
			sealed, err := s.AuthMail.Seal(authmail.DeliveryTopic, message)
			if err != nil {
				return err
			}
			_, err = event.Enqueue(ctx, tx, authmail.DeliveryTopic, sealed, event.Options{MaxAttempts: 3, RetryDelay: 30 * time.Second})
			if err != nil {
				return &dbal.Error{Code: dbal.Unavailable, Message: "auth recovery delivery enqueue failed"}
			}
			return nil
		})
	case authmail.DeliveryTopic:
		return s.AuthMail.Send(ctx, message)
	default:
		return authmail.ErrEnvelope
	}
}

type RecoveryReset struct {
	Token        string `json:"token"`
	Password     string `json:"password"`
	Confirmation string `json:"confirmation"`
}

func (s Service) ResetPasswordWithToken(ctx context.Context, app *appir.App, input RecoveryReset) error {
	if err := s.recoveryAvailable(app); err != nil {
		return err
	}
	if err := auth.ValidateNewPassword(input.Password, input.Confirmation); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		userID, err := auth.ResetWithToken(ctx, tx, app.AppID, app.ReleaseID, input.Token, input.Password, s.now())
		if err != nil {
			return err
		}
		if err := auth.RevokeSessions(ctx, tx, userID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{UserID: userID, Action: "system_password_recovery", EntityType: "bean_user", EntityID: userID, Changed: []string{"credentials_or_sessions"}, Success: true})
	})
}
