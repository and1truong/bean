package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/field"
)

func TestMultipartWebformStoresAndDownloadsFile(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "upload.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := definition.Bundle{Name: "upload", Definitions: []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "public_access"}, Spec: map[string]any{}},
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "restricted_access"}, Spec: map[string]any{"authenticated": true}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "asset"}, Spec: map[string]any{"policy": "public_access", "fields": []any{map[string]any{"name": "label", "type": "string", "required": true}, map[string]any{"name": "enabled", "type": "boolean", "required": true}, map[string]any{"name": "status", "type": "enum", "options": []any{"draft", "published"}, "required": true}, map[string]any{"name": "file", "type": "file", "required": true}, map[string]any{"name": "thumbnail", "type": "file"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "restricted_assets"}, Spec: map[string]any{"entity": "asset", "policy": "restricted_access", "fields": []any{"id", "file"}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "published_assets"}, Spec: map[string]any{"entity": "asset", "fields": []any{"id", "file"}, "filter": map[string]any{"left": map[string]any{"source": "record", "name": "status"}, "op": "eq", "right": map[string]any{"source": "literal", "literal": "published"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Webform", Metadata: definition.Metadata{Name: "upload_asset"}, Spec: map[string]any{"action": "asset_create", "elements": []any{map[string]any{"name": "label", "type": "text", "required": true}, map[string]any{"name": "enabled", "type": "checkbox", "required": true}, map[string]any{"name": "status", "type": "select", "required": true}, map[string]any{"name": "file", "type": "file", "required": true}, map[string]any{"name": "thumbnail", "type": "file"}}}},
	}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("label", "Plan")
	_ = writer.WriteField("enabled", "true")
	_ = writer.WriteField("status", "published")
	primaryContent := bytes.Repeat([]byte("a"), (1<<20)+1024)
	part, err := writer.CreateFormFile("file", "../plan.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(primaryContent)
	thumbnail, err := writer.CreateFormFile("thumbnail", "thumbnail.bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = thumbnail.Write(bytes.Repeat([]byte("b"), field.MaxFileBytes))
	_ = writer.Close()
	request := httptest.NewRequest(http.MethodPost, "/api/webforms/upload_asset/submit", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("upload status=%d body=%s", response.Code, response.Body.String())
	}
	var result struct{ Data map[string]any }
	decodeResponse(t, response, &result)
	fileID := result.Data["file"].(string)
	for _, path := range []string{"/api/files/" + fileID, "/api/files/" + fileID + "?view=restricted_assets"} {
		denied := httptest.NewRecorder()
		runtime.HTTP.Handler().ServeHTTP(denied, httptest.NewRequest(http.MethodGet, path, nil))
		if denied.Code != http.StatusNotFound {
			t.Fatalf("download without an authorized View leaked with status %d", denied.Code)
		}
	}
	publishedPath := "/api/files/" + fileID + "?view=published_assets"
	published := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(published, httptest.NewRequest(http.MethodGet, publishedPath, nil))
	if published.Code != http.StatusOK {
		t.Fatalf("published View download status=%d", published.Code)
	}
	if update := serve(t, runtime.HTTP.Handler(), http.MethodPost, "/api/actions/asset_update", map[string]any{"id": result.Data["id"], "status": "draft"}, nil, ""); update.Code != http.StatusOK {
		t.Fatalf("draft update status=%d body=%s", update.Code, update.Body.String())
	}
	draft := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(draft, httptest.NewRequest(http.MethodGet, publishedPath, nil))
	if draft.Code != http.StatusNotFound {
		t.Fatalf("View predicate bypassed with status %d", draft.Code)
	}
	if update := serve(t, runtime.HTTP.Handler(), http.MethodPost, "/api/actions/asset_update", map[string]any{"id": result.Data["id"], "status": "published"}, nil, ""); update.Code != http.StatusOK {
		t.Fatalf("publish update status=%d body=%s", update.Code, update.Body.String())
	}
	filePath := "/api/files/" + fileID + "?view=asset_list"
	bundle.Definitions[0].Spec = map[string]any{"condition": map[string]any{"left": map[string]any{"source": "record", "name": "enabled"}, "op": "eq", "right": map[string]any{"source": "literal", "literal": true}}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("typed-policy publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	download := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(download, httptest.NewRequest(http.MethodGet, filePath, nil))
	if download.Code != http.StatusOK || !bytes.Equal(download.Body.Bytes(), primaryContent) || !strings.Contains(download.Header().Get("Content-Disposition"), "plan.txt") || strings.Contains(download.Header().Get("Content-Disposition"), "..") {
		t.Fatalf("download status=%d headers=%v body length=%d", download.Code, download.Header(), download.Body.Len())
	}
	bundle.Definitions[0].Spec = map[string]any{"authenticated": true}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("protected publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	anonymous := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(anonymous, httptest.NewRequest(http.MethodGet, filePath, nil))
	if anonymous.Code != http.StatusNotFound {
		t.Fatalf("protected file leaked with status %d", anonymous.Code)
	}
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	login := serve(t, runtime.HTTP.Handler(), http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	authorizedRequest := httptest.NewRequest(http.MethodGet, filePath, nil)
	authorizedRequest.AddCookie(login.Result().Cookies()[0])
	authorized := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusOK || !bytes.Equal(authorized.Body.Bytes(), primaryContent) {
		t.Fatalf("authorized download status=%d body length=%d", authorized.Code, authorized.Body.Len())
	}
	bundle.Definitions[0].Spec = map[string]any{"authenticated": true, "redact": []any{"file"}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("redacted publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	redactedRequest := httptest.NewRequest(http.MethodGet, filePath, nil)
	redactedRequest.AddCookie(login.Result().Cookies()[0])
	redacted := httptest.NewRecorder()
	runtime.HTTP.Handler().ServeHTTP(redacted, redactedRequest)
	if redacted.Code != http.StatusNotFound {
		t.Fatalf("redacted file leaked with status %d", redacted.Code)
	}
}

func TestManifestKeepsAuthenticationNavigationForImplicitRequirements(t *testing.T) {
	entity := func(scope string) definition.Definition {
		return definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}, scope: true}}
	}
	cases := map[string][]definition.Definition{
		"owner":  {entity("owner")},
		"tenant": {entity("tenant")},
		"user condition": {
			{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "assigned"}, Spec: map[string]any{"condition": map[string]any{"left": map[string]any{"source": "record", "name": "assignee_id"}, "op": "eq", "right": map[string]any{"source": "user", "name": "id"}}}},
			{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"policy": "assigned", "fields": []any{map[string]any{"name": "assignee_id", "type": "string"}}}},
		},
	}
	for name, definitions := range cases {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "manifest.db"), false)
			if err != nil {
				t.Fatal(err)
			}
			defer runtime.DB.Close()
			if err = runtime.Store.SaveBundle(ctx, "default", definition.Bundle{Name: name, Definitions: definitions}); err != nil {
				t.Fatal(err)
			}
			if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
				t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
			}
			response := serve(t, runtime.HTTP.Handler(), http.MethodGet, "/api/system/manifest", nil, nil, "")
			var manifest map[string]any
			decodeResponse(t, response, &manifest)
			if manifest["authNavigation"] != true {
				t.Fatalf("manifest authNavigation=%v", manifest["authNavigation"])
			}
		})
	}
}

func TestPublicViewSearchIsCompilerDeclaredAndThemeIsExposed(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "search.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := definition.Bundle{Name: "search", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Theme", Metadata: definition.Metadata{Name: "default"}, Spec: map[string]any{"displayName": "Search Demo", "preset": "professional", "accent": "indigo"}},
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "public_access"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"policy": "public_access", "fields": []any{map[string]any{"name": "title", "type": "string", "required": true}, map[string]any{"name": "status", "type": "enum", "options": []any{"open", "closed"}, "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{"entity": "item", "fields": []any{"id", "title", "status"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "items"}, Spec: map[string]any{"type": "view", "view": "items", "presentation": map[string]any{"mode": "list", "titleField": "title", "searchFields": []any{"title"}}}},
		{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "home"}, Spec: map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"items"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "home"}, Spec: map[string]any{"route": "/", "panel": "home"}},
	}}
	if err = runtime.Store.EnsureApp(ctx, "default", "Bean"); err != nil {
		t.Fatal(err)
	}
	if _, _, diagnostics, publishErr := runtime.Store.PublishBundle(ctx, "default", bundle); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	// Republishing definitions must preserve the name of the active bundle.
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("republish=%v diagnostics=%v", publishErr, diagnostics)
	}
	if err = runtime.Store.LoadActive(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	handler := runtime.HTTP.Handler()
	for _, title := range []string{"Alpha role", "Beta role"} {
		response := serve(t, handler, http.MethodPost, "/api/actions/item_create", map[string]any{"title": title, "status": "open"}, nil, "")
		if response.Code != http.StatusOK {
			t.Fatalf("create status=%d body=%s", response.Code, response.Body.String())
		}
	}
	manifestResponse := serve(t, handler, http.MethodGet, "/api/system/manifest", nil, nil, "")
	var manifest map[string]any
	decodeResponse(t, manifestResponse, &manifest)
	if manifest["appName"] != "search" || manifest["appId"] != "default" {
		t.Fatalf("manifest appName=%v appId=%v", manifest["appName"], manifest["appId"])
	}
	theme := manifest["theme"].(map[string]any)
	if theme["DisplayName"] != "Search Demo" || theme["Accent"] != "indigo" {
		t.Fatalf("theme=%#v", theme)
	}

	search := serve(t, handler, http.MethodGet, "/api/views/items?_page=%2F&_block=items&q=beta", nil, nil, "")
	var result struct {
		Data []map[string]any `json:"data"`
	}
	decodeResponse(t, search, &result)
	if len(result.Data) != 1 || result.Data[0]["title"] != "Beta role" {
		t.Fatalf("search rows=%#v", result.Data)
	}
	unbound := serve(t, handler, http.MethodGet, "/api/views/items?q=alpha", nil, nil, "")
	if unbound.Code != http.StatusBadRequest {
		t.Fatalf("unbound search status=%d body=%s", unbound.Code, unbound.Body.String())
	}
}

func TestPageViewDisplayEnforcesControlsPagerAndRouteBindings(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "display.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := definition.Bundle{Name: "display", Definitions: []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "public"}, Spec: map[string]any{}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"policy": "public", "fields": []any{
			map[string]any{"name": "title", "type": "string", "required": true},
			map[string]any{"name": "status", "type": "enum", "options": []any{"open", "closed"}, "required": true},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{
			"entity": "article", "fields": []any{"id", "title", "status"}, "policy": "public",
			"exposedFilters": map[string]any{"id": map[string]any{"field": "id", "operator": "eq"}, "status": map[string]any{"field": "status", "operator": "eq"}},
			"displays": map[string]any{
				"index":    map[string]any{"type": "page", "route": "/articles", "title": map[string]any{"text": "Articles"}, "renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "title", "label": "Article"}}}, "controls": []any{map[string]any{"filter": "status", "label": "Status", "widget": "select"}}, "pager": map[string]any{"type": "cursor", "pageSize": 1}},
				"snapshot": map[string]any{"type": "page", "route": "/article-snapshot", "renderer": map[string]any{"type": "detail", "titleField": "title"}, "pager": map[string]any{"type": "none", "pageSize": 2}},
				"detail":   map[string]any{"type": "page", "route": "/articles/:id", "bindings": map[string]any{"id": map[string]any{"source": "route", "name": "id", "required": true}}, "title": map[string]any{"field": "title", "fallback": "Article"}, "renderer": map[string]any{"type": "detail", "titleField": "title"}},
			},
		}},
	}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	handler := runtime.HTTP.Handler()
	ids := []string{}
	for _, input := range []map[string]any{{"title": "One", "status": "open"}, {"title": "Two", "status": "open"}, {"title": "Closed", "status": "closed"}} {
		created := serve(t, handler, http.MethodPost, "/api/actions/article_create", input, nil, "")
		var result struct{ Data map[string]any }
		decodeResponse(t, created, &result)
		ids = append(ids, result.Data["id"].(string))
	}
	pageResponse := serve(t, handler, http.MethodGet, "/api/system/page?path=%2Farticles", nil, nil, "")
	if pageResponse.Code != http.StatusOK || !strings.Contains(pageResponse.Body.String(), `"Type":"table"`) {
		t.Fatalf("page status=%d body=%s", pageResponse.Code, pageResponse.Body.String())
	}
	first := serve(t, handler, http.MethodGet, "/api/views/articles?_page=%2Farticles&_display=index&status=open&limit=200", nil, nil, "")
	var rows struct {
		Data       []map[string]any `json:"data"`
		NextCursor string           `json:"nextCursor"`
	}
	decodeResponse(t, first, &rows)
	if len(rows.Data) != 1 || rows.NextCursor == "" {
		t.Fatalf("rows=%v cursor=%q", rows.Data, rows.NextCursor)
	}
	snapshot := serve(t, handler, http.MethodGet, "/api/views/articles?_page=%2Farticle-snapshot&_display=snapshot&limit=200", nil, nil, "")
	decodeResponse(t, snapshot, &rows)
	if len(rows.Data) != 3 || rows.NextCursor != "" {
		t.Fatalf("snapshot rows=%v cursor=%q", rows.Data, rows.NextCursor)
	}
	if response := serve(t, handler, http.MethodGet, "/api/views/articles?_page=%2Farticles&_display=index&title=forged", nil, nil, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown filter status=%d body=%s", response.Code, response.Body.String())
	}
	detailPath := "/articles/" + ids[0]
	if response := serve(t, handler, http.MethodGet, "/api/views/articles?_page="+url.QueryEscape(detailPath)+"&_display=detail", nil, nil, ""); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"title":"One"`) {
		t.Fatalf("detail status=%d body=%s", response.Code, response.Body.String())
	}
	if response := serve(t, handler, http.MethodGet, "/api/views/articles?_page="+url.QueryEscape(detailPath)+"&_display=detail&id="+ids[1], nil, nil, ""); response.Code != http.StatusBadRequest {
		t.Fatalf("bound collision status=%d body=%s", response.Code, response.Body.String())
	}
}

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

func TestAdminRecordIncludesDerivedContextualMenu(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "books.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	bundle, err := examples.Load("books")
	if err != nil {
		t.Fatal(err)
	}
	bundle.Definitions = append(bundle.Definitions,
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "managers"}, Spec: map[string]any{"writeRoles": []any{"manager"}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "contextual_editors"}, Spec: map[string]any{"writeRoles": []any{"editor"}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "create_alternate_page"}, Spec: map[string]any{"entity": "page", "operation": "create", "policy": "contextual_editors", "input": map[string]any{"title": map[string]any{"type": "string", "required": true}, "body": map[string]any{"type": "text", "required": true}}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "create_second_page"}, Spec: map[string]any{"entity": "page", "operation": "create", "policy": "contextual_editors", "input": map[string]any{"title": map[string]any{"type": "string", "required": true}, "body": map[string]any{"type": "text", "required": true}}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "create_hidden_page"}, Spec: map[string]any{"entity": "page", "operation": "create", "policy": "managers", "input": map[string]any{"title": map[string]any{"type": "string", "required": true}, "body": map[string]any{"type": "text", "required": true}}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "alternate_pages"}, Spec: map[string]any{"entity": "page", "label": "Alternate Page", "labelField": "title", "view": "page_admin", "createAction": "create_alternate_page", "updateAction": "update_page", "deleteAction": "delete_page", "list": map[string]any{"columns": []any{"title", "updated_at"}}, "form": map[string]any{"fields": []any{"title", "body"}, "readonly": []any{"created_at", "updated_at", "version"}}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "second_pages"}, Spec: map[string]any{"entity": "page", "label": "Second Page", "labelField": "title", "view": "page_admin", "createAction": "create_second_page", "updateAction": "update_page", "deleteAction": "delete_page", "list": map[string]any{"columns": []any{"title", "updated_at"}}, "form": map[string]any{"fields": []any{"title", "body"}, "readonly": []any{"created_at", "updated_at", "version"}}}},
		definition.Definition{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "hidden_pages"}, Spec: map[string]any{"entity": "page", "label": "Hidden Page", "labelField": "title", "view": "page_admin", "createAction": "create_hidden_page", "updateAction": "update_page", "deleteAction": "delete_page", "list": map[string]any{"columns": []any{"title", "updated_at"}}, "form": map[string]any{"fields": []any{"title", "body"}, "readonly": []any{"created_at", "updated_at", "version"}}}},
	)
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish err=%v diagnostics=%v", publishErr, diagnostics)
	}
	handler := runtime.HTTP.Handler()
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	cookie, csrf := login.Result().Cookies()[0], session["csrfToken"].(string)
	if err = runtime.HTTP.Auth.Create(ctx, "editor@example.test", "test-password", []string{"editor"}, ""); err != nil {
		t.Fatal(err)
	}
	editorLogin := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "editor@example.test", "password": "test-password"}, nil, "")
	editorCookie := editorLogin.Result().Cookies()[0]
	createdBook := serve(t, handler, http.MethodPost, "/api/actions/create_book", map[string]any{"title": "Building Bean"}, cookie, csrf)
	var book struct {
		Data map[string]any `json:"data"`
	}
	decodeResponse(t, createdBook, &book)
	bookID := book.Data["id"].(string)

	type menuContext struct {
		Name    string           `json:"name"`
		Items   []map[string]any `json:"items"`
		Creates []struct {
			Resource string `json:"resource"`
			Entity   string `json:"entity"`
		} `json:"creates"`
	}
	read := func() (map[string]any, []menuContext) {
		t.Helper()
		response := serve(t, handler, http.MethodGet, "/api/admin/resources/books/"+bookID, nil, editorCookie, "")
		var result struct {
			Data    map[string]any `json:"data"`
			Context struct {
				Menus []menuContext `json:"menus"`
			} `json:"context"`
		}
		decodeResponse(t, response, &result)
		return result.Data, result.Context.Menus
	}
	data, menus := read()
	if data["title"] != "Building Bean" || len(menus) != 1 || menus[0].Name != "book_contents" || len(menus[0].Items) != 0 || len(menus[0].Creates) != 2 || menus[0].Creates[0].Resource != "alternate_pages" || menus[0].Creates[1].Resource != "second_pages" || menus[0].Creates[1].Entity != "page" {
		t.Fatalf("empty contextual response data=%v menus=%+v", data, menus)
	}
	createdPage := serve(t, handler, http.MethodPost, "/api/actions/create_page", map[string]any{"title": "Architecture", "body": "Immutable metadata", "_navigation": map[string]any{"placements": []any{map[string]any{"menu": "book_contents", "ownerId": bookID, "weight": 10}}}}, cookie, csrf)
	if createdPage.Code != http.StatusOK {
		t.Fatalf("create Page status=%d body=%s", createdPage.Code, createdPage.Body.String())
	}
	_, menus = read()
	if len(menus) != 1 || len(menus[0].Items) != 1 || menus[0].Items[0]["Label"] != "Architecture" {
		t.Fatalf("populated contextual response=%+v", menus)
	}
	missing := serve(t, handler, http.MethodGet, "/api/admin/resources/books/00000000-0000-4000-8000-000000000000", nil, editorCookie, "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("missing owner status=%d body=%s", missing.Code, missing.Body.String())
	}
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
	bundle := definition.Bundle{Name: "admin", Definitions: []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"label": "Article", "fields": []any{map[string]any{"name": "title", "label": "Title", "type": "string", "required": true}, map[string]any{"name": "status", "label": "Status", "type": "enum", "options": []any{"draft", "published"}, "required": true}, map[string]any{"name": "featured", "label": "Featured", "type": "boolean", "required": true}, map[string]any{"name": "attributes", "label": "Attributes", "type": "json", "required": true}}}}}}
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
	if err = runtime.HTTP.Auth.Create(ctx, "editor@example.test", "test-password", []string{"editor"}, ""); err != nil {
		t.Fatal(err)
	}
	editorLogin := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "editor@example.test", "password": "test-password"}, nil, "")
	editorCookie := editorLogin.Result().Cookies()[0]
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
		created := serve(t, handler, http.MethodPost, "/api/actions/article_create", map[string]any{"title": title, "status": "draft", "featured": true, "attributes": map[string]any{"source": "review"}}, cookie, csrf)
		if created.Code != http.StatusOK {
			t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
		}
	}
	publicPage := serve(t, handler, http.MethodGet, "/api/views/article_list?limit=1", nil, cookie, "")
	if publicPage.Code != http.StatusOK || publicPage.Header().Get("Bean-Next-Cursor") == "" {
		t.Fatalf("public View pagination header missing: status=%d headers=%v body=%s", publicPage.Code, publicPage.Header(), publicPage.Body.String())
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
	var recordResult struct {
		Data    map[string]any `json:"data"`
		Context struct {
			Menus []any `json:"menus"`
		} `json:"context"`
	}
	decodeResponse(t, record, &recordResult)
	attributes, attributesOK := recordResult.Data["attributes"].(map[string]any)
	if record.Code != http.StatusOK || recordResult.Data["title"] != "Beta" || recordResult.Data["featured"] != true || !attributesOK || attributes["source"] != "review" {
		t.Fatalf("record status=%v data=%v", record.Code, recordResult.Data)
	}
	if len(recordResult.Context.Menus) != 0 {
		t.Fatalf("ordinary Admin record context=%v", recordResult.Context.Menus)
	}
	history := serve(t, handler, http.MethodGet, "/api/admin/audit?entity=article&id="+id, nil, cookie, "")
	if history.Code != http.StatusOK || !strings.Contains(history.Body.String(), "article_create") {
		t.Fatalf("history status=%v", history.Code)
	}
	if unscoped := serve(t, handler, http.MethodGet, "/api/admin/audit", nil, editorCookie, ""); unscoped.Code != http.StatusForbidden {
		t.Fatalf("editor unscoped audit status=%v body=%s", unscoped.Code, unscoped.Body.String())
	}
	if scoped := serve(t, handler, http.MethodGet, "/api/admin/audit?entity=article&id="+id, nil, editorCookie, ""); scoped.Code != http.StatusOK || !strings.Contains(scoped.Body.String(), "article_create") {
		t.Fatalf("editor scoped audit status=%v body=%s", scoped.Code, scoped.Body.String())
	}
	if system := serve(t, handler, http.MethodGet, "/api/admin/audit?entity=bean_job&id=failed-job", nil, editorCookie, ""); system.Code != http.StatusForbidden {
		t.Fatalf("editor system audit status=%v body=%s", system.Code, system.Body.String())
	}
	invalid := serve(t, handler, http.MethodGet, "/api/admin/resources/article?filter.title=Beta", nil, cookie, "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("undeclared filter status=%v", invalid.Code)
	}
	bookingBundle, err := examples.Load("booking")
	if err != nil {
		t.Fatal(err)
	}
	compiled := compiler.Compile("default", 2, bookingBundle.Definitions)
	if len(compiled.Diagnostics) != 0 {
		t.Fatalf("booking diagnostics=%v", compiled.Diagnostics)
	}
	if err = runtime.DB.ExecuteMigration(ctx, []string{
		`CREATE TABLE resource (id TEXT PRIMARY KEY,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL,name TEXT NOT NULL)`,
		`CREATE TABLE booking (id TEXT PRIMARY KEY,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL,resource_id TEXT NOT NULL,start_at TEXT NOT NULL,end_at TEXT NOT NULL,requested_at TEXT NOT NULL,status TEXT NOT NULL,FOREIGN KEY(resource_id) REFERENCES resource(id))`,
	}); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Kernel.Activate(compiled.App); err != nil {
		t.Fatal(err)
	}
	resourceResponse := serve(t, handler, http.MethodPost, "/api/actions/resource_create", map[string]any{"name": "Concurrent room"}, cookie, csrf)
	var resourceResult struct {
		Data map[string]any `json:"data"`
	}
	decodeResponse(t, resourceResponse, &resourceResult)
	bookingInput := map[string]any{"resource_id": resourceResult.Data["id"], "start_at": time.Now().UTC().Add(time.Hour).Format(time.RFC3339), "end_at": time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339)}
	concurrentBookings := func(input map[string]any) chan *httptest.ResponseRecorder {
		start := make(chan struct{})
		responses := make(chan *httptest.ResponseRecorder, 2)
		var workers sync.WaitGroup
		for range 2 {
			workers.Add(1)
			go func() {
				defer workers.Done()
				<-start
				responses <- serve(t, handler, http.MethodPost, "/api/actions/book_resource", input, cookie, csrf)
			}()
		}
		close(start)
		workers.Wait()
		close(responses)
		return responses
	}
	succeeded := 0
	for response := range concurrentBookings(bookingInput) {
		if response.Code == http.StatusOK {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful concurrent bookings=%d", succeeded)
	}
	bookings, err := runtime.DB.Select(ctx, dbal.Select{Table: "booking", Columns: []string{"id", "requested_at"}, Limit: 10})
	if err != nil || len(bookings) != 1 || bookings[0]["requested_at"] == nil {
		t.Fatalf("bookings=%v err=%v", bookings, err)
	}
	invalidInterval := serve(t, handler, http.MethodPost, "/api/actions/book_resource", map[string]any{"resource_id": resourceResult.Data["id"], "start_at": time.Now().UTC().Add(6 * time.Hour).Format(time.RFC3339), "end_at": time.Now().UTC().Add(5 * time.Hour).Format(time.RFC3339)}, cookie, csrf)
	if invalidInterval.Code != http.StatusConflict {
		t.Fatalf("invalid interval status=%d body=%s", invalidInterval.Code, invalidInterval.Body.String())
	}
	if strings.HasPrefix(databaseURL, "postgres") {
		runtime.HTTP.Actions.DB = newIdempotencyBarrierDatabase(runtime.DB)
		idempotentInput := map[string]any{"resource_id": resourceResult.Data["id"], "start_at": time.Now().UTC().Add(3 * time.Hour).Format(time.RFC3339), "end_at": time.Now().UTC().Add(4 * time.Hour).Format(time.RFC3339), "_idempotencyKey": "concurrent-booking"}
		ids := map[string]bool{}
		for response := range concurrentBookings(idempotentInput) {
			var result struct {
				Data map[string]any `json:"data"`
			}
			decodeResponse(t, response, &result)
			id, ok := result.Data["id"].(string)
			if response.Code != http.StatusOK || !ok {
				t.Fatalf("idempotent booking status=%d body=%s", response.Code, response.Body.String())
			}
			ids[id] = true
		}
		bookings, err = runtime.DB.Select(ctx, dbal.Select{Table: "booking", Columns: []string{"id"}, Limit: 10})
		if err != nil || len(ids) != 1 || len(bookings) != 2 {
			t.Fatalf("idempotent ids=%v bookings=%v err=%v", ids, bookings, err)
		}
	}
}

type idempotencyBarrierDatabase struct {
	dbal.Database
	mu        sync.Mutex
	selected  int
	completed int
	ready     chan struct{}
}

func newIdempotencyBarrierDatabase(database dbal.Database) *idempotencyBarrierDatabase {
	return &idempotencyBarrierDatabase{Database: database, ready: make(chan struct{})}
}

func (d *idempotencyBarrierDatabase) Select(ctx context.Context, query dbal.Select) ([]dbal.Row, error) {
	d.mu.Lock()
	d.selected++
	waits := query.Table == "bean_idempotency" && d.selected <= 2
	d.mu.Unlock()
	rows, err := d.Database.Select(ctx, query)
	if !waits {
		return rows, err
	}
	d.mu.Lock()
	d.completed++
	if d.completed == 2 {
		close(d.ready)
	}
	d.mu.Unlock()
	<-d.ready
	return rows, err
}

type rejectAuditDatabase struct{ dbal.Database }

func (d rejectAuditDatabase) Transaction(ctx context.Context, operation func(dbal.Transaction) error) error {
	return d.Database.Transaction(ctx, func(tx dbal.Transaction) error {
		return operation(rejectAuditTransaction{Transaction: tx})
	})
}

type rejectAuditTransaction struct{ dbal.Transaction }

func (t rejectAuditTransaction) Insert(ctx context.Context, query dbal.Insert) (dbal.Result, error) {
	if query.Table == "bean_audit" {
		return dbal.Result{}, errors.New("injected audit failure")
	}
	return t.Transaction.Insert(ctx, query)
}

func TestSystemUserCreationRollsBackWhenAuditFails(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "admin-users.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	failing := rejectAuditDatabase{Database: runtime.DB}
	runtime.HTTP.Auth.DB = failing
	runtime.HTTP.Actions.DB = failing
	handler := runtime.HTTP.Handler()
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	response := serve(t, handler, http.MethodPost, "/api/admin/system/users", map[string]any{"email": "new@example.test", "password": "test-password", "roles": []string{"authenticated"}}, login.Result().Cookies()[0], session["csrfToken"].(string))
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_user", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "email", Value: "new@example.test"}, Limit: 1})
	if err != nil || len(rows) != 0 {
		t.Fatalf("user escaped failed audit transaction: rows=%v err=%v", rows, err)
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
