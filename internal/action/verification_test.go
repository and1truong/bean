package action_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/authmail"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
)

type outboxFailureDB struct{ dbal.Database }
type outboxFailureTx struct{ dbal.Transaction }

func (d outboxFailureDB) Transaction(ctx context.Context, fn func(dbal.Transaction) error) error {
	return d.Database.Transaction(ctx, func(tx dbal.Transaction) error { return fn(outboxFailureTx{tx}) })
}
func (tx outboxFailureTx) Insert(ctx context.Context, input dbal.Insert) (dbal.Result, error) {
	if input.Table == "bean_outbox" {
		return dbal.Result{}, errors.New("injected outbox failure")
	}
	return tx.Transaction.Insert(ctx, input)
}

func TestVerificationTokenAndRegistrationTransactions(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURLWithOptions(ctx, filepath.Join(t.TempDir(), "verification.db"), false, bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	mail, err := authmail.New(authmail.Config{Address: "localhost:587", From: "bean@example.test", Origin: "https://example.test", Key: base64.StdEncoding.EncodeToString(make([]byte, 32))}, discardRecoveryMail{})
	if err != nil {
		t.Fatal(err)
	}
	svc := action.Service{DB: runtime.DB, Auth: runtime.HTTP.Auth, AuthMail: mail}
	app := appir.Empty()
	app.AppID = "test"
	app.ReleaseID = "release"
	app.Authentication = &appir.Authentication{Preset: "internal", EmailVerification: true, PasswordRecovery: true}
	if err := svc.Auth.Create(ctx, "verify@example.test", "test-password", nil, ""); err != nil {
		t.Fatal(err)
	}
	session, err := svc.Auth.Login(ctx, "verify@example.test", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestEmailVerification(ctx, app, "verify@example.test"); err != nil {
		t.Fatal(err)
	}
	issue := func(requestTopic, deliveryTopic string) authmail.Envelope {
		t.Helper()
		rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "topic", Value: requestTopic}, Limit: 10})
		if err != nil || len(rows) != 1 {
			t.Fatal("request count", err, len(rows))
		}
		payload := map[string]any{}
		if err := json.Unmarshal([]byte(fmt.Sprint(rows[0]["payload"])), &payload); err != nil {
			t.Fatal(err)
		}
		for attempt := 0; attempt < 2; attempt++ {
			if err := svc.DeliverAuthMail(ctx, app, requestTopic, payload); err != nil {
				t.Fatal(err)
			}
		}
		deliveries, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "topic", Value: deliveryTopic}, Limit: 10})
		if err != nil || len(deliveries) != 1 {
			t.Fatal("worker replay", err, len(deliveries))
		}
		sealed := map[string]any{}
		if err := json.Unmarshal([]byte(fmt.Sprint(deliveries[0]["payload"])), &sealed); err != nil {
			t.Fatal(err)
		}
		message, err := mail.Open(deliveryTopic, sealed)
		if err != nil {
			t.Fatal(err)
		}
		return message
	}
	verification := issue(authmail.VerificationRequestTopic, authmail.VerificationDeliveryTopic)
	input := action.EmailConfirmation{Token: verification.Token, Password: "test-password"}
	expired := svc
	expired.Now = func() time.Time { return verification.Expires.Add(time.Second) }
	if err := expired.ConfirmEmail(ctx, app, input); err == nil {
		t.Fatal("expired verification accepted")
	}
	wrongRelease := *app
	wrongRelease.ReleaseID = "other"
	if err := svc.ConfirmEmail(ctx, &wrongRelease, input); err == nil {
		t.Fatal("wrong release accepted")
	}
	if err := svc.RequestPasswordRecovery(ctx, app, "verify@example.test"); err != nil {
		t.Fatal(err)
	}
	recovery := issue(authmail.RequestTopic, authmail.DeliveryTopic)
	if err := svc.ConfirmEmail(ctx, app, action.EmailConfirmation{Token: recovery.Token, Password: "test-password"}); err == nil {
		t.Fatal("reset token verified email")
	}
	if err := svc.ResetPasswordWithToken(ctx, app, action.RecoveryReset{Token: verification.Token, Password: "new-password", Confirmation: "new-password"}); err == nil {
		t.Fatal("verification token reset password")
	}
	failing := svc
	failing.DB = auditFailureDB{runtime.DB}
	if err := failing.ConfirmEmail(ctx, app, input); err == nil {
		t.Fatal("audit failure committed")
	}
	if _, err := svc.Auth.Current(ctx, session.ID); err != nil {
		t.Fatal("rollback revoked session", err)
	}
	users, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: "verify@example.test"}, Limit: 1})
	if err != nil || users[0]["email_verified_at"] != nil {
		t.Fatal("rollback verified email", err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- svc.ConfirmEmail(ctx, app, input) }()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent confirmations", successes)
	}
	if _, err := svc.Auth.Current(ctx, session.ID); err == nil {
		t.Fatal("verification kept old session")
	}
	// Registration rolls back if verification cannot be queued.
	result := compiler.Compile("test", 1, []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": "public", "registration": true, "emailVerification": true}},
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
		{APIVersion: definition.APIVersion, Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup"}},
	})
	if len(result.Diagnostics) > 0 {
		t.Fatal(result.Diagnostics)
	}
	result.App.ReleaseID = "release"
	failing = svc
	failing.DB = outboxFailureDB{runtime.DB}
	if _, err := failing.Execute(ctx, result.App, "signup", map[string]any{"email": "rollback@example.test", "display_name": "Rollback", "password": "test-password", "password_confirmation": "test-password"}, beanctx.Request{}); err == nil {
		t.Fatal("registration survived queue failure")
	}
	users, err = runtime.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: "rollback@example.test"}, Limit: 1})
	if err != nil || len(users) != 0 {
		t.Fatal("orphaned registration", err)
	}
}
