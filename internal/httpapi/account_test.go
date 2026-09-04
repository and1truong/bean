package httpapi_test

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
)

func TestAccountPasswordAndSessionActions(t *testing.T) {
	testAccountActions(t, filepath.Join(t.TempDir(), "account.db"))
}
func TestAccountPasswordAndSessionActionsPostgreSQL(t *testing.T) {
	url := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if url == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL")
	}
	testAccountActions(t, url)
}
func testAccountActions(t *testing.T, url string) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, url, false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	handler := runtime.HTTP.Handler()
	for _, email := range []string{"account-a@example.test", "account-b@example.test"} {
		if err := runtime.HTTP.Auth.Create(ctx, email, "old-password", []string{"authenticated"}, ""); err != nil {
			t.Fatal(err)
		}
	}
	login := func(email, password string) (*http.Cookie, string) {
		t.Helper()
		result := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": email, "password": password}, nil, "")
		if result.Code != 200 {
			t.Fatalf("login: %d", result.Code)
		}
		var session map[string]any
		decodeResponse(t, result, &session)
		return result.Result().Cookies()[0], session["csrfToken"].(string)
	}
	a, csrf := login("account-a@example.test", "old-password")
	other, _ := login("account-a@example.test", "old-password")
	b, bCSRF := login("account-b@example.test", "old-password")
	input := map[string]any{"currentPassword": "old-password", "password": "new-password", "confirmation": "new-password"}
	for _, path := range []string{"/api/auth/password", "/api/auth/sessions/revoke"} {
		if result := serve(t, handler, http.MethodPost, path, input, nil, ""); result.Code != 401 {
			t.Fatal("anonymous accepted", result.Code)
		}
		if result := serve(t, handler, http.MethodPost, path, input, a, ""); result.Code != 403 {
			t.Fatal("missing CSRF accepted", result.Code)
		}
	}
	input["currentPassword"] = "wrong-password"
	if result := serve(t, handler, http.MethodPost, "/api/auth/password", input, a, csrf); result.Code == 200 {
		t.Fatal("wrong current password accepted")
	}
	input["currentPassword"] = "old-password"
	input["confirmation"] = "different"
	if result := serve(t, handler, http.MethodPost, "/api/auth/password", input, a, csrf); result.Code == 200 {
		t.Fatal("confirmation mismatch accepted")
	}
	input["confirmation"] = "new-password"
	input["userId"] = "someone-else"
	if result := serve(t, handler, http.MethodPost, "/api/auth/password", input, a, csrf); result.Code == 200 {
		t.Fatal("client-selected user accepted")
	}
	delete(input, "userId")
	result := serveWithHeaders(t, handler, http.MethodPost, "/api/auth/password", input, a, csrf, map[string]string{"Idempotency-Key": "password-secret-key"})
	if result.Code != 200 || strings.Contains(result.Body.String(), "password") {
		t.Fatalf("change: %d %s", result.Code, result.Body.String())
	}
	replays, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_idempotency", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "action", Value: "system_password_change"}, Limit: 1})
	if err != nil || len(replays) != 0 {
		t.Fatalf("account Action used idempotency storage: %v", err)
	}
	if len(result.Result().Cookies()) != 1 || result.Result().Cookies()[0].MaxAge != -1 {
		t.Fatal("cookie not cleared")
	}
	for _, cookie := range []*http.Cookie{a, other} {
		if _, err := runtime.HTTP.Auth.Current(ctx, cookie.Value); err == nil {
			t.Fatal("old session survived")
		}
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, b.Value); err != nil {
		t.Fatal("other account session revoked", err)
	}
	if _, err := runtime.HTTP.Auth.Login(ctx, "account-a@example.test", "old-password"); err == nil {
		t.Fatal("old password accepted")
	}
	fresh, token := login("account-a@example.test", "new-password")
	another, _ := login("account-a@example.test", "new-password")
	if result := serve(t, handler, http.MethodPost, "/api/auth/sessions/revoke", map[string]any{}, fresh, token); result.Code != 200 {
		t.Fatalf("revoke: %d", result.Code)
	}
	for _, cookie := range []*http.Cookie{fresh, another} {
		if _, err := runtime.HTTP.Auth.Current(ctx, cookie.Value); err == nil {
			t.Fatal("session revocation incomplete")
		}
	}
	if _, err := runtime.HTTP.Auth.Current(ctx, b.Value); err != nil {
		t.Fatal(err)
	}
	// Invalid account submissions exhaust only the account limiter, not login.
	limited := false
	for attempt := 0; attempt < 11; attempt++ {
		response := serve(t, handler, http.MethodPost, "/api/auth/password", map[string]any{"password": "short", "confirmation": "short"}, b, bCSRF)
		if response.Code == http.StatusTooManyRequests {
			limited = true
			break
		}
	}
	if !limited {
		t.Fatal("account mutations are not throttled")
	}
	login("account-b@example.test", "old-password")
}
