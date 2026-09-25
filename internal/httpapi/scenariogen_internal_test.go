package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/beanruntime/bean/internal/scenariogen"
)

type fakeScenarioGenerator struct {
	specs []map[string]any
	calls int
}

func (f *fakeScenarioGenerator) Generate(_ context.Context, _ scenariogen.Request) (map[string]any, error) {
	f.calls++
	if len(f.specs) == 0 {
		return map[string]any{}, nil
	}
	spec := f.specs[0]
	f.specs = f.specs[1:]
	return spec, nil
}

func TestScenarioGenerateDraftsWithoutSaving(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })
	runtime.HTTP.Generator = &fakeScenarioGenerator{specs: []map[string]any{{
		"title": "Invalid login", "start": "nav",
		"nodes": []any{
			map[string]any{"id": "nav", "type": "navigate", "url": "http://example.test/login", "next": "assert"},
			map[string]any{"id": "assert", "type": "assert", "assertion": "text_present", "text": "Invalid password"},
		},
	}}}

	response := serve(t, handler, http.MethodPost, "/api/scenario-generate", map[string]any{"prompt": "Test login with an invalid password"}, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("generate=%d %s", response.Code, response.Body.String())
	}
	var body map[string]any
	decodeResponse(t, response, &body)
	if body["name"] != "test_login_with_an_invalid_password" {
		t.Fatalf("name=%v", body["name"])
	}
	spec, _ := body["spec"].(map[string]any)
	if spec["start"] != "nav" {
		t.Fatalf("spec=%v", spec)
	}
	if description, _ := spec["description"].(string); description == "" {
		t.Fatalf("provenance missing: %v", spec)
	}

	// The draft is a candidate only — no definition was persisted.
	definitions := serve(t, handler, http.MethodGet, "/api/admin/definitions", nil, cookie, "")
	var list []map[string]any
	decodeResponse(t, definitions, &list)
	scenarios := 0
	for _, item := range list {
		if item["kind"] == "Scenario" {
			scenarios++
		}
	}
	if scenarios != 1 {
		t.Fatalf("saved scenarios=%d definitions=%s", scenarios, definitions.Body.String())
	}
}

func TestScenarioGenerateRejectsInvalidOutput(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })
	gen := &fakeScenarioGenerator{specs: []map[string]any{
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
	}}
	runtime.HTTP.Generator = gen

	response := serve(t, handler, http.MethodPost, "/api/scenario-generate", map[string]any{"prompt": "anything"}, cookie, csrf)
	if response.Code != http.StatusUnprocessableEntity {
		t.Fatalf("generate=%d %s", response.Code, response.Body.String())
	}
	if gen.calls != scenariogen.MaxAttempts {
		t.Fatalf("attempts=%d", gen.calls)
	}
	var body map[string]any
	decodeResponse(t, response, &body)
	if diagnostics, _ := body["diagnostics"].([]any); len(diagnostics) == 0 {
		t.Fatalf("body=%s", response.Body.String())
	}
}

func TestScenarioGenerateNeedsPromptAndGenerator(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })

	missing := serve(t, handler, http.MethodPost, "/api/scenario-generate", map[string]any{"prompt": "  "}, cookie, csrf)
	if missing.Code != http.StatusBadRequest {
		t.Fatalf("empty prompt=%d", missing.Code)
	}
	unconfigured := serve(t, handler, http.MethodPost, "/api/scenario-generate", map[string]any{"prompt": "test"}, cookie, csrf)
	if unconfigured.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured=%d %s", unconfigured.Code, unconfigured.Body.String())
	}
	var _ = runtime
}
