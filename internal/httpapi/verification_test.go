package httpapi_test

import (
	"context"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/auth"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
)

func verificationBundle() definition.Bundle {
	return definition.Bundle{Name: "Verification", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": "public", "registration": true, "emailVerification": true}},
		{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
		{APIVersion: definition.APIVersion, Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup"}},
	}}
}
func TestEmailVerification(t *testing.T) {
	testEmailVerification(t, filepath.Join(t.TempDir(), "verification.db"))
}
func TestEmailVerificationPostgreSQL(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL")
	}
	testEmailVerification(t, databaseURL)
}
func testEmailVerification(t *testing.T, databaseURL string) {
	ctx := context.Background()
	inbox := &recoveryInbox{}
	runtime, err := bootstrap.OpenURLWithOptions(ctx, databaseURL, false, bootstrap.Options{AuthMail: recoveryMailer(t, inbox)})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err := runtime.HTTP.Auth.Create(ctx, "verify-existing@example.test", "test-password", []string{"authenticated"}, ""); err != nil {
		t.Fatal(err)
	}
	session, err := runtime.HTTP.Auth.Login(ctx, "verify-existing@example.test", "test-password")
	if err != nil {
		t.Fatal(err)
	}
	bundle := verificationBundle()
	publish := func() {
		t.Helper()
		if _, _, ds, err := runtime.Store.PublishBundle(ctx, "verification_contract", bundle); err != nil || len(ds) > 0 {
			t.Fatal(err, ds)
		}
	}
	publish()
	handler := runtime.HTTP.Handler()
	app, err := runtime.Store.ActiveApp(ctx, "verification_contract")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err != auth.ErrEmailUnverified {
		t.Fatal("old unverified session accepted", err)
	}
	if err := runtime.HTTP.Actions.RevokeAccountSessions(ctx, session.ID, "direct"); err != auth.ErrEmailUnverified {
		t.Fatal("direct Account Action bypassed verification", err)
	}
	login := func(email, password string) int {
		return serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": email, "password": password}, nil, "").Code
	}
	if login("verify-existing@example.test", "test-password") != 403 || login("verify-existing@example.test", "wrong-password") != 401 || login("missing@example.test", "test-password") != 401 {
		t.Fatal("credential/verification login boundary")
	}
	drain := func() {
		t.Helper()
		for i := 0; i < 2; i++ {
			if err := runtime.Outbox.RunOnce(ctx); err != nil {
				t.Fatal(err)
			}
		}
	}
	var replies []string
	for _, email := range []string{"verify-existing@example.test", "verify-missing@example.test"} {
		result := serve(t, handler, http.MethodPost, "/api/auth/verification/request", map[string]any{"email": email}, nil, "")
		if result.Code != 202 {
			t.Fatal(result.Code, result.Body.String())
		}
		replies = append(replies, result.Body.String())
	}
	if replies[0] != replies[1] {
		t.Fatal("resend reveals existence")
	}
	if len(inbox.messages) != 0 {
		t.Fatal("mail before committed worker")
	}
	drain()
	if len(inbox.messages) != 1 || inbox.messages[0].Purpose != "email_verify" {
		t.Fatal("verification delivery")
	}
	link, _ := url.Parse(inbox.messages[0].Link)
	token := strings.TrimPrefix(link.Fragment, "token=")
	if link.Query().Get("verification") != "confirm" {
		t.Fatal("wrong link purpose")
	}
	for _, input := range []map[string]any{{"token": token}, {"token": token, "password": "wrong-password"}, {"token": "tampered", "password": "test-password"}, {"token": token, "password": "test-password", "userId": "other"}} {
		if result := serve(t, handler, http.MethodPost, "/api/auth/verification/confirm", input, nil, ""); result.Code != 400 {
			t.Fatal("invalid confirmation accepted", result.Code)
		}
	}
	result := serve(t, handler, http.MethodPost, "/api/auth/verification/confirm", map[string]any{"token": token, "password": "test-password"}, nil, "")
	if result.Code != 200 || len(result.Result().Cookies()) != 0 {
		t.Fatal("verify response", result.Code, result.Body.String())
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, session.ID); err == nil {
		t.Fatal("verification revived old session")
	}
	if login("verify-existing@example.test", "test-password") != 200 {
		t.Fatal("verified user cannot login")
	}
	if err := runtime.HTTP.Actions.ConfirmEmail(ctx, app, action.EmailConfirmation{Token: token, Password: "test-password"}); err == nil {
		t.Fatal("verification replay accepted")
	}
	result = serve(t, handler, http.MethodPost, "/api/auth/verification/request", map[string]any{"email": "verify-existing@example.test"}, nil, "")
	if result.Body.String() != replies[0] {
		t.Fatal("verified resend reveals state")
	}
	drain()
	if len(inbox.messages) != 1 {
		t.Fatal("verified account sent another link")
	}
	// Registration and the encrypted request commit together; no login before verification.
	signup := map[string]any{"display_name": "Member", "email": "verify-member@example.test", "password": "member-password", "password_confirmation": "member-password"}
	if response := serve(t, handler, http.MethodPost, "/api/actions/signup", signup, nil, ""); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	if login("verify-member@example.test", "member-password") != 403 {
		t.Fatal("signup bypassed verification")
	}
	drain()
	if len(inbox.messages) != 2 || inbox.messages[1].To != "verify-member@example.test" {
		t.Fatal("signup did not enqueue verification")
	}
	memberLink, _ := url.Parse(inbox.messages[1].Link)
	memberToken := strings.TrimPrefix(memberLink.Fragment, "token=")
	// Queue then disable: no further delivery; old links and direct Actions are denied.
	if err := runtime.HTTP.Actions.RequestEmailVerification(ctx, app, "verify-member@example.test"); err != nil {
		t.Fatal(err)
	}
	bundle.Definitions[0].Spec["emailVerification"] = false
	publish()
	count := len(inbox.messages)
	drain()
	if count != len(inbox.messages) {
		t.Fatal("disabled feature delivered mail")
	}
	disabled, _ := runtime.Store.ActiveApp(ctx, "verification_contract")
	if err := runtime.HTTP.Actions.ConfirmEmail(ctx, disabled, action.EmailConfirmation{Token: memberToken, Password: "member-password"}); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatal("disabled direct confirmation", err)
	}
	for _, path := range []string{"/api/auth/verification/request", "/api/auth/verification/confirm"} {
		if result := serve(t, handler, http.MethodPost, path, map[string]any{}, nil, ""); result.Code != 404 {
			t.Fatal("disabled endpoint", result.Code)
		}
	}
	if login("verify-member@example.test", "member-password") != 200 {
		t.Fatal("disabled verification still blocks local login")
	}
}

func TestVerificationHostReadiness(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURLWithOptions(ctx, filepath.Join(t.TempDir(), "host.db"), false, bootstrap.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if _, _, _, err := runtime.Store.PublishBundle(ctx, "default", verificationBundle()); err == nil {
		t.Fatal("verification activated without mail")
	}
}
