package httpapi_test

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/demoseed"
)

func TestExplorePreviewCompilesExecutesAndSavesOrdinaryView(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "explore.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := definition.Bundle{Name: "explore", Definitions: []definition.Definition{{
		APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{
			"fields": []any{
				map[string]any{"name": "name", "type": "string", "required": true},
				map[string]any{"name": "stage", "type": "enum", "required": true, "options": []any{"applied", "interview"}},
			},
		},
	}, {
		APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "explore_seed"},
		Spec: map[string]any{"entities": map[string]any{"candidate": map[string]any{"count": 2, "profile": "people"}}},
	}}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	active, ok := runtime.Kernel.Active()
	if !ok {
		t.Fatal("missing active application")
	}
	if _, err = demoseed.Run(ctx, runtime.DB, active, 42); err != nil {
		t.Fatal(err)
	}
	handler := runtime.HTTP.Handler()
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	cookie, csrf := login.Result().Cookies()[0], session["csrfToken"].(string)
	spec := map[string]any{
		"entity": "candidate", "fields": []any{"id", "name", "stage"},
		"search":         map[string]any{"fields": []any{"name"}},
		"exposedFilters": map[string]any{"stage": map[string]any{"field": "stage", "operator": "eq"}},
		"sort":           []any{map[string]any{"field": "name"}},
	}
	request := map[string]any{"name": "candidate_records", "spec": spec, "search": "Jordan", "filter": map[string]any{"stage": "interview"}, "limit": 25}
	if denied := serve(t, handler, http.MethodPost, "/api/admin/explore/preview", request, cookie, ""); denied.Code != http.StatusForbidden {
		t.Fatalf("preview without CSRF status=%d", denied.Code)
	}
	preview := serve(t, handler, http.MethodPost, "/api/admin/explore/preview", request, cookie, csrf)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview status=%d body=%s", preview.Code, preview.Body.String())
	}
	var result struct {
		Valid bool
		Data  []map[string]any
	}
	decodeResponse(t, preview, &result)
	if !result.Valid || len(result.Data) != 1 || result.Data[0]["name"] != "Jordan Smith 2" {
		t.Fatalf("preview=%+v", result)
	}

	definitions := serve(t, handler, http.MethodGet, "/api/admin/definitions", nil, cookie, "")
	var before []definition.Definition
	decodeResponse(t, definitions, &before)
	if len(before) != 2 {
		t.Fatalf("preview persisted a definition: %+v", before)
	}
	draftToken := definitions.Header().Get("ETag")
	if draftToken == "" {
		t.Fatal("definitions response omitted draft ETag")
	}
	candidateDefinition := definition.Definition{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Namespace: "default", Name: "candidate_records"}, Spec: spec}
	saved := serveWithHeaders(t, handler, http.MethodPost, "/api/admin/definitions", candidateDefinition, cookie, csrf, map[string]string{"If-Match": draftToken})
	if saved.Code != http.StatusOK {
		t.Fatalf("save status=%d body=%s", saved.Code, saved.Body.String())
	}
	stale := serveWithHeaders(t, handler, http.MethodPost, "/api/admin/definitions", candidateDefinition, cookie, csrf, map[string]string{"If-Match": draftToken})
	if stale.Code != http.StatusConflict {
		t.Fatalf("stale save status=%d body=%s", stale.Code, stale.Body.String())
	}
	definitions = serve(t, handler, http.MethodGet, "/api/admin/definitions", nil, cookie, "")
	var after []definition.Definition
	decodeResponse(t, definitions, &after)
	if len(after) != 3 {
		t.Fatalf("saved definitions=%+v", after)
	}
	validation := serve(t, handler, http.MethodPost, "/api/admin/releases/validate", nil, cookie, csrf)
	var validationResult struct {
		Valid   bool
		Changes []struct{ Operation, Path string }
	}
	decodeResponse(t, validation, &validationResult)
	if !validationResult.Valid || len(validationResult.Changes) != 1 || validationResult.Changes[0].Operation != "add" || validationResult.Changes[0].Path != "definitions.View.candidate_records" {
		t.Fatalf("validation=%+v", validationResult)
	}
}

func TestExplorePreviewReturnsCompilerDiagnostics(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "explore-invalid.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle := definition.Bundle{Name: "explore", Definitions: []definition.Definition{{
		APIVersion: definition.APIVersion,
		Kind:       "Entity",
		Metadata:   definition.Metadata{Name: "candidate"},
		Spec:       map[string]any{"fields": []any{map[string]any{"name": "score", "type": "integer"}}},
	}}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	handler := runtime.HTTP.Handler()
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	response := serve(t, handler, http.MethodPost, "/api/admin/explore/preview", map[string]any{
		"name": "candidate_records", "spec": map[string]any{"entity": "candidate", "fields": []any{"id", "score"}, "search": map[string]any{"fields": []any{"score"}}},
	}, login.Result().Cookies()[0], session["csrfToken"].(string))
	if response.Code != http.StatusBadRequest || response.Body.String() == "" {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}
