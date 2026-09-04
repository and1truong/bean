package action_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/authmail"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
)

type discardRecoveryMail struct{}

func (discardRecoveryMail) Send(context.Context, authmail.Message) error { return nil }
func TestRecoveryRollbackExpiryAndWorkerReplay(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURLWithOptions(ctx, filepath.Join(t.TempDir(), "recovery.db"), false, bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	mail, err := authmail.New(authmail.Config{Address: "localhost:587", From: "bean@example.test", Origin: "https://example.test", Key: base64.StdEncoding.EncodeToString(make([]byte, 32))}, discardRecoveryMail{})
	if err != nil {
		t.Fatal(err)
	}
	svc := action.Service{DB: runtime.DB, AuthMail: mail}
	app := appir.Empty()
	app.AppID = "test"
	app.ReleaseID = "release"
	app.Authentication = &appir.Authentication{Preset: "internal", PasswordRecovery: true}
	if err := runtime.HTTP.Auth.Create(ctx, "recovery@example.test", "old-password", nil, ""); err != nil {
		t.Fatal(err)
	}
	session, err := runtime.HTTP.Auth.Login(ctx, "recovery@example.test", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestPasswordRecovery(ctx, app, "recovery@example.test"); err != nil {
		t.Fatal(err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 10})
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(fmt.Sprint(rows[0]["payload"])), &payload); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if err := svc.DeliverAuthMail(ctx, app, authmail.RequestTopic, payload); err != nil {
			t.Fatal(err)
		}
	}
	deliveries, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "topic", Value: authmail.DeliveryTopic}, Limit: 10})
	if err != nil || len(deliveries) != 1 {
		t.Fatal("worker replay duplicated delivery", err)
	}
	sealed := map[string]any{}
	if err := json.Unmarshal([]byte(fmt.Sprint(deliveries[0]["payload"])), &sealed); err != nil {
		t.Fatal(err)
	}
	message, err := mail.Open(authmail.DeliveryTopic, sealed)
	if err != nil {
		t.Fatal(err)
	}
	input := action.RecoveryReset{Token: message.Token, Password: "new-password", Confirmation: "new-password"}
	expired := svc
	expired.Now = func() time.Time { return message.Expires.Add(time.Second) }
	if err := expired.ResetPasswordWithToken(ctx, app, input); err == nil {
		t.Fatal("expired token accepted")
	}
	wrong := *app
	wrong.ReleaseID = "different"
	if err := svc.ResetPasswordWithToken(ctx, &wrong, input); err == nil {
		t.Fatal("wrong-release token accepted")
	}
	failing := svc
	failing.DB = auditFailureDB{runtime.DB}
	if err := failing.ResetPasswordWithToken(ctx, app, input); err == nil {
		t.Fatal("audit failure committed reset")
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err != nil {
		t.Fatal("rollback revoked session", err)
	}
	if _, err := runtime.HTTP.Auth.Login(ctx, "recovery@example.test", "old-password"); err != nil {
		t.Fatal("rollback changed password", err)
	}
	results := make(chan error, 2)
	for attempt := 0; attempt < 2; attempt++ {
		go func() { results <- svc.ResetPasswordWithToken(ctx, app, input) }()
	}
	successes := 0
	for attempt := 0; attempt < 2; attempt++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent redemptions committed %d times", successes)
	}
	if err := svc.DeliverAuthMail(ctx, app, authmail.RequestTopic, payload); err != nil {
		t.Fatal(err)
	}
	tokens, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_auth_token", Limit: 10})
	if err != nil || len(tokens) != 1 {
		t.Fatal(err)
	}
	if tokens[0]["consumed_at"] == nil || strings.Contains(fmt.Sprint(tokens), message.Token) {
		t.Fatal("consumed receipt lost or plaintext token persisted")
	}
	if err := svc.ResetPasswordWithToken(ctx, app, input); err == nil {
		t.Fatal("retry resurrected consumed token")
	}
}
