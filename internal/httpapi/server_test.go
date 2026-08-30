package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
)

func TestAdminResourceAPIRequiresAdminAndUsesCompiledRuntime(t *testing.T) {
	testAdminResourceAPI(t, filepath.Join(t.TempDir(), "admin.db"))
}

func TestAdminResourceAPIPostgreSQLParity(t *testing.T) {
	databaseURL := os.Getenv("BEAN_TEST_POSTGRES_URL")
	if databaseURL == "" {
		t.Skip("set BEAN_TEST_POSTGRES_URL to run PostgreSQL parity")
	}
	testAdminResourceAPI(t, databaseURL)
}

func testAdminResourceAPI(t *testing.T, databaseURL string) {
	t.Helper()
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, databaseURL, false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	bundle := definition.Bundle{Name: "admin", Definitions: []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"label": "Article", "fields": []any{map[string]any{"name": "title", "label": "Title", "type": "string", "required": true}, map[string]any{"name": "status", "label": "Status", "type": "enum", "options": []any{"draft", "published"}, "required": true}}}}}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish err=%v diagnostics=%v", publishErr, diagnostics)
	}
	handler := runtime.HTTP.Handler()
	if response := serve(t, handler, http.MethodGet, "/api/admin/manifest", nil, nil, ""); response.Code != http.StatusForbidden {
		t.Fatalf("anonymous manifest status=%v", response.Code)
	}
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	csrf := session["csrfToken"].(string)
	cookie := login.Result().Cookies()[0]
	users := serve(t, handler, http.MethodGet, "/api/admin/system/users", nil, cookie, "")
	if users.Code != http.StatusOK || strings.Contains(users.Body.String(), "password_hash") || strings.Contains(users.Body.String(), "csrf_token") {
		t.Fatalf("unsafe system users response: status=%d body=%s", users.Code, users.Body.String())
	}
	if _, err = runtime.DB.Insert(ctx, dbal.Insert{Table: "bean_job", Values: map[string]dbal.Value{"id": "failed-job", "name": "test", "run_at": "2026-08-30T00:00:00Z", "status": "failed", "payload": "{}", "attempts": 5, "retry_delay": 60, "max_attempts": 5, "last_error": "offline"}}); err != nil {
		t.Fatal(err)
	}
	withoutCSRF := serve(t, handler, http.MethodPost, "/api/admin/system/jobs/failed-job/retry", map[string]any{}, cookie, "")
	if withoutCSRF.Code != http.StatusForbidden {
		t.Fatalf("system mutation without CSRF status=%d", withoutCSRF.Code)
	}
	retried := serve(t, handler, http.MethodPost, "/api/admin/system/jobs/failed-job/retry", map[string]any{}, cookie, csrf)
	if retried.Code != http.StatusOK {
		t.Fatalf("system retry status=%d body=%s", retried.Code, retried.Body.String())
	}
	systemAudit := serve(t, handler, http.MethodGet, "/api/admin/audit?entity=bean_job&id=failed-job", nil, cookie, "")
	if systemAudit.Code != http.StatusOK || !strings.Contains(systemAudit.Body.String(), "system_job_retry") {
		t.Fatalf("system audit status=%d body=%s", systemAudit.Code, systemAudit.Body.String())
	}

	manifest := serve(t, handler, http.MethodGet, "/api/admin/manifest", nil, cookie, "")
	if manifest.Code != http.StatusOK || !strings.Contains(manifest.Body.String(), "adminResources") {
		t.Fatalf("manifest status=%v", manifest.Code)
	}
	for _, title := range []string{"Alpha", "Beta"} {
		created := serve(t, handler, http.MethodPost, "/api/actions/article_create", map[string]any{"title": title, "status": "draft"}, cookie, csrf)
		if created.Code != http.StatusOK {
			t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
		}
	}
	list := serve(t, handler, http.MethodGet, "/api/admin/resources/article?q=bet&sort=title&direction=desc", nil, cookie, "")
	var result struct {
		Data []map[string]any `json:"data"`
	}
	decodeResponse(t, list, &result)
	if len(result.Data) != 1 || result.Data[0]["title"] != "Beta" {
		t.Fatalf("filtered rows=%v", result.Data)
	}
	id := result.Data[0]["id"].(string)
	record := serve(t, handler, http.MethodGet, "/api/admin/resources/article/"+id, nil, cookie, "")
	if record.Code != http.StatusOK || !strings.Contains(record.Body.String(), "Beta") {
		t.Fatalf("record status=%v", record.Code)
	}
	history := serve(t, handler, http.MethodGet, "/api/admin/audit?entity=article&id="+id, nil, cookie, "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), "article_create") {
		t.Fatalf("history status=%v", history.Code)
	}
	invalid := serve(t, handler, http.MethodGet, "/api/admin/resources/article?filter.title=Beta", nil, cookie, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("undeclared filter status=%v", invalid.Code)
	}
}

func serve(t *testing.T, handler http.Handler, method, path string, body any, cookie *http.Cookie, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	encoded, _ := json.Marshal(body)
	request := httptest.NewRequest(method, path, bytes.NewReader(encoded))
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if csrf != "" {
		request.Header.Set("X-CSRF-Token", csrf)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}
func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.NewDecoder(response.Body).Decode(out); err != nil {
		t.Fatal(err)
	}
}
