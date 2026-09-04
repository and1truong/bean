package httpapi_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/authmail"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
)

type recoveryInbox struct {
	messages []authmail.Message
	fail     bool
}

func (s *recoveryInbox) Send(_ context.Context, message authmail.Message) error {
	if s.fail {
		return fmt.Errorf("provider leaked %s", message.Link)
	}
	s.messages = append(s.messages, message)
	return nil
}
func recoveryMailer(t *testing.T, inbox *recoveryInbox) *authmail.Service {
	t.Helper()
	service, err := authmail.New(authmail.Config{Address: "localhost:587", From: "bean@example.test", Origin: "https://accounts.example.test", Key: base64.StdEncoding.EncodeToString(make([]byte, 32))}, inbox)
	if err != nil {
		t.Fatal(err)
	}
	return service
}
func recoveryBundle() definition.Bundle {
	return definition.Bundle{Name: "Recovery", Definitions: []definition.Definition{{APIVersion: definition.APIVersion, Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": "internal", "passwordRecovery": true}}}}
}

func TestEmailPasswordRecovery(t *testing.T) {
	testEmailRecovery(t, filepath.Join(t.TempDir(), "recovery.db"))
}
func TestEmailPasswordRecoveryPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL")
	}
	testEmailRecovery(t, databaseURL)
}
func testEmailRecovery(t *testing.T, databaseURL string) {
	ctx := context.Background()
	inbox := &recoveryInbox{}
	runtime, err := bootstrap.OpenURLWithOptions(ctx, databaseURL, false, bootstrap.Options{AuthMail: recoveryMailer(t, inbox)})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := recoveryBundle()
	if _, _, ds, err := runtime.Store.PublishBundle(ctx, "default", bundle); err != nil || len(ds) > 0 {
		t.Fatalf("publish: %v %v", err, ds)
	}
	if err := runtime.HTTP.Auth.Create(ctx, "recovery-user@example.test", "old-password", []string{"authenticated"}, ""); err != nil {
		t.Fatal(err)
	}
	session, err := runtime.HTTP.Auth.Login(ctx, "recovery-user@example.test", "old-password")
	if err != nil {
		t.Fatal(err)
	}
	handler := runtime.HTTP.Handler()
	var responses []string
	for _, email := range []string{" RECOVERY-USER@EXAMPLE.TEST ", "missing-recovery@example.test"} {
		response := serve(t, handler, http.MethodPost, "/api/auth/recovery/request", map[string]any{"email": email}, nil, "")
		if response.Code != 202 {
			t.Fatalf("request: %d %s", response.Code, response.Body.String())
		}
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("cacheable recovery")
		}
		responses = append(responses, response.Body.String())
	}
	if responses[0] != responses[1] {
		t.Fatal("request response reveals account existence")
	}
	if len(inbox.messages) != 0 {
		t.Fatal("SMTP ran in request transaction")
	}
	drain := func() {
		t.Helper()
		for attempt := 0; attempt < 2; attempt++ {
			if err := runtime.Outbox.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	drain()
	if len(inbox.messages) != 1 {
		t.Fatalf("mail count %d", len(inbox.messages))
	}
	link, err := url.Parse(inbox.messages[0].Link)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimPrefix(link.Fragment, "token=")
	if link.Host != "accounts.example.test" || strings.Contains(link.RawQuery, token) || len(token) != 43 {
		t.Fatal("unsafe recovery link")
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Limit: 1000})
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		payload := fmt.Sprint(row["payload"])
		if strings.Contains(payload, token) || strings.Contains(payload, "recovery-user@example.test") {
			t.Fatal("outbox plaintext secret")
		}
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err != nil {
		t.Fatal("request revoked session", err)
	}
	app, err := runtime.Store.ActiveApp(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"token": token, "password": "reset-password", "confirmation": "reset-password"}
	input["token"] = "invalid"
	if result := serve(t, handler, http.MethodPost, "/api/auth/recovery/reset", input, nil, ""); result.Code != 400 {
		t.Fatal("bad token accepted")
	}
	input["token"] = token
	result := serve(t, handler, http.MethodPost, "/api/auth/recovery/reset", input, nil, "")
	if result.Code != 200 || len(result.Result().Cookies()) != 0 {
		t.Fatalf("reset response: %d %s", result.Code, result.Body.String())
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err == nil {
		t.Fatal("reset kept session")
	}
	if _, err := runtime.HTTP.Auth.Login(ctx, "recovery-user@example.test", "reset-password"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.HTTP.Actions.ResetPasswordWithToken(ctx, app, action.RecoveryReset{Token: token, Password: "another-password", Confirmation: "another-password"}); err == nil {
		t.Fatal("token replay accepted")
	}
	// A fresh token is invalidated by host recovery as well.
	if err := runtime.HTTP.Actions.RequestPasswordRecovery(ctx, app, "recovery-user@example.test"); err != nil {
		t.Fatal(err)
	}
	drain()
	nextLink, _ := url.Parse(inbox.messages[len(inbox.messages)-1].Link)
	nextToken := strings.TrimPrefix(nextLink.Fragment, "token=")
	if err := runtime.HTTP.Actions.ResetAccountPassword(ctx, "recovery-user@example.test", "host-password"); err != nil {
		t.Fatal(err)
	}
	if err := runtime.HTTP.Actions.ResetPasswordWithToken(ctx, app, action.RecoveryReset{Token: nextToken, Password: "another-password", Confirmation: "another-password"}); err == nil {
		t.Fatal("host reset left recovery token valid")
	}
	// Delivery failures persist only sanitized errors, then retry the same intent.
	inbox.fail = true
	if err := runtime.HTTP.Actions.RequestPasswordRecovery(ctx, app, "recovery-user@example.test"); err != nil {
		t.Fatal(err)
	}
	drain()
	pending, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "last_error", Value: authmail.ErrDelivery.Error()}, Limit: 10})
	if err != nil || len(pending) != 1 {
		t.Fatalf("sanitized retry state: %v %v", pending, err)
	}
	inbox.fail = false
	if _, err := runtime.DB.Update(ctx, dbal.Update{Table: "bean_outbox", Values: map[string]dbal.Value{"next_attempt_at": "2000-01-01T00:00:00Z"}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: pending[0]["id"]}, ExpectedRows: 1}); err != nil {
		t.Fatal(err)
	}
	beforeRetry := len(inbox.messages)
	drain()
	if len(inbox.messages) != beforeRetry+1 {
		t.Fatal("committed delivery did not retry")
	}
	// Disable and republish: APIs and direct Actions fail closed, including queued intents.
	if err := runtime.HTTP.Actions.RequestPasswordRecovery(ctx, app, "recovery-user@example.test"); err != nil {
		t.Fatal(err)
	}
	bundle.Definitions[0].Spec["passwordRecovery"] = false
	if _, _, ds, err := runtime.Store.PublishBundle(ctx, "default", bundle); err != nil || len(ds) > 0 {
		t.Fatal(err, ds)
	}
	count := len(inbox.messages)
	drain()
	if len(inbox.messages) != count {
		t.Fatal("disabled release delivered queued recovery")
	}
	disabled, _ := runtime.Store.ActiveApp(ctx, "default")
	if err := runtime.HTTP.Actions.RequestPasswordRecovery(ctx, disabled, "recovery-user@example.test"); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatal("disabled direct Action", err)
	}
	for _, path := range []string{"/api/auth/recovery/request", "/api/auth/recovery/reset"} {
		if result := serve(t, handler, http.MethodPost, path, input, nil, ""); result.Code != 404 {
			t.Fatal("disabled endpoint", result.Code)
		}
	}
}

func TestRecoveryPublicationRequiresHostDelivery(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "host.db")
	runtime, err := bootstrap.OpenURLWithOptions(ctx, path, false, bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := runtime.Store.PublishBundle(ctx, "default", recoveryBundle()); err == nil {
		t.Fatal("enabled recovery published without host delivery")
	}
	runtime.DB.Close()
	configured, err := bootstrap.OpenURLWithOptions(ctx, path, false, bootstrap.Options{AuthMail: recoveryMailer(t, &recoveryInbox{})})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ds, err := configured.Store.PublishBundle(ctx, "default", recoveryBundle()); err != nil || len(ds) > 0 {
		t.Fatal(err, ds)
	}
	configured.DB.Close()
	if runtime, err := bootstrap.OpenURLWithOptions(ctx, path, false, bootstrap.Options{}); err == nil {
		runtime.DB.Close()
		t.Fatal("enabled recovery started without host delivery")
	}
	inspection, err := bootstrap.OpenInspection(ctx, path)
	if err != nil {
		t.Fatal("offline inspection required credentials", err)
	}
	inspection.DB.Close()
}
