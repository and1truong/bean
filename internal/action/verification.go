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

func (s Service) verificationAvailable(app *appir.App) error {
	if app == nil || !app.EmailVerificationEnabled() {
		return &dbal.Error{Code: dbal.NotFound, Message: "email verification is not enabled"}
	}
	if s.AuthMail == nil {
		return &dbal.Error{Code: dbal.Unavailable, Message: "auth email delivery is not configured"}
	}
	return nil
}
func (s Service) RequestEmailVerification(ctx context.Context, app *appir.App, email string) error {
	if err := s.verificationAvailable(app); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error { return s.queueVerification(ctx, tx, app, email) })
}
func (s Service) queueVerification(ctx context.Context, tx dbal.Transaction, app *appir.App, email string) error {
	if err := s.verificationAvailable(app); err != nil {
		return err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	if !authmail.ValidEmail(email) {
		return &dbal.Error{Code: dbal.InvalidQuery, Message: "enter a valid email address"}
	}
	message := authmail.Envelope{ID: uid.New(), AppID: app.AppID, ReleaseID: app.ReleaseID, Email: email, Expires: s.now().Add(15 * time.Minute)}
	payload, err := s.AuthMail.Seal(authmail.VerificationRequestTopic, message)
	if err != nil {
		return err
	}
	_, err = event.Enqueue(ctx, tx, authmail.VerificationRequestTopic, payload, event.Options{ID: message.ID, MaxAttempts: 3, RetryDelay: 30 * time.Second})
	return err
}
func (s Service) deliverVerification(ctx context.Context, app *appir.App, topic string, message authmail.Envelope) error {
	if app == nil || !app.EmailVerificationEnabled() || app.AppID != message.AppID || app.ReleaseID != message.ReleaseID || !message.Expires.After(s.now()) {
		return nil
	}
	if topic == authmail.VerificationDeliveryTopic {
		return s.AuthMail.SendVerification(ctx, message)
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		token, err := auth.IssueVerification(ctx, tx, message.ID, message.Email, message.AppID, message.ReleaseID, message.Expires)
		if err != nil {
			return &dbal.Error{Code: dbal.Unavailable, Message: "auth verification preparation failed"}
		}
		if token == "" {
			return nil
		}
		message.Token = token
		sealed, err := s.AuthMail.Seal(authmail.VerificationDeliveryTopic, message)
		if err != nil {
			return err
		}
		if _, err := event.Enqueue(ctx, tx, authmail.VerificationDeliveryTopic, sealed, event.Options{MaxAttempts: 3, RetryDelay: 30 * time.Second}); err != nil {
			return &dbal.Error{Code: dbal.Unavailable, Message: "auth verification delivery enqueue failed"}
		}
		return nil
	})
}

type EmailConfirmation struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s Service) ConfirmEmail(ctx context.Context, app *appir.App, input EmailConfirmation) error {
	if err := s.verificationAvailable(app); err != nil {
		return err
	}
	return s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		userID, err := auth.VerifyEmail(ctx, tx, app.AppID, app.ReleaseID, input.Token, input.Password, s.now())
		if err != nil {
			return err
		}
		if err := auth.RevokeSessions(ctx, tx, userID); err != nil {
			return err
		}
		return audit.Write(ctx, tx, audit.Entry{UserID: userID, Action: "system_email_verify", EntityType: "bean_user", EntityID: userID, Changed: []string{"email_verified_at", "sessions"}, Success: true})
	})
}
