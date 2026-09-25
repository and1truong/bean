package scenariogen_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/scenariogen"
)

type fakeGenerator struct {
	specs     []map[string]any
	err       error
	feedbacks []string
}

func (f *fakeGenerator) Generate(_ context.Context, request scenariogen.Request) (map[string]any, error) {
	f.feedbacks = append(f.feedbacks, request.Feedback)
	if f.err != nil {
		return nil, f.err
	}
	if len(f.specs) == 0 {
		return map[string]any{}, nil
	}
	spec := f.specs[0]
	f.specs = f.specs[1:]
	return spec, nil
}

func validSpec() map[string]any {
	return map[string]any{
		"title": "Login failure", "start": "nav",
		"nodes": []any{
			map[string]any{"id": "nav", "type": "navigate", "url": "http://example.test/login", "next": "assert"},
			map[string]any{"id": "assert", "type": "assert", "assertion": "text_present", "text": "Invalid password"},
		},
	}
}

func TestDraftRetriesWithDiagnosticsFeedback(t *testing.T) {
	gen := &fakeGenerator{specs: []map[string]any{
		{"start": "nav", "nodes": []any{map[string]any{"id": "nav", "type": "nonsense"}}},
		validSpec(),
	}}
	spec, err := scenariogen.Draft(context.Background(), gen, appir.Empty(), "login_failure", "test invalid login")
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if len(gen.feedbacks) != 2 || gen.feedbacks[1] == "" {
		t.Fatalf("expected feedback on retry: %v", gen.feedbacks)
	}
	if description, _ := spec["description"].(string); !strings.Contains(description, "test invalid login") {
		t.Fatalf("provenance missing: %v", spec["description"])
	}
}

func TestDraftFailsAfterMaxAttempts(t *testing.T) {
	gen := &fakeGenerator{specs: []map[string]any{
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
		{"nodes": []any{map[string]any{"type": "nonsense"}}},
	}}
	_, err := scenariogen.Draft(context.Background(), gen, appir.Empty(), "draft", "prompt")
	var draftErr *scenariogen.DraftError
	if !errors.As(err, &draftErr) || len(draftErr.Diagnostics) == 0 {
		t.Fatalf("err=%v diagnostics=%v", err, draftErr)
	}
	if len(gen.feedbacks) != scenariogen.MaxAttempts {
		t.Fatalf("attempts=%d", len(gen.feedbacks))
	}
}

func TestDraftPropagatesGeneratorError(t *testing.T) {
	gen := &fakeGenerator{err: errors.New("provider down")}
	if _, err := scenariogen.Draft(context.Background(), gen, appir.Empty(), "draft", "prompt"); err == nil || strings.Contains(err.Error(), "compile") {
		t.Fatalf("err=%v", err)
	}
}

func TestSlug(t *testing.T) {
	if slug := scenariogen.Slug("Test login with an invalid password"); slug != "test_login_with_an_invalid_password" {
		t.Fatalf("slug=%q", slug)
	}
	if slug := scenariogen.Slug("123 numeric start"); !strings.HasPrefix(slug, "generated_") {
		t.Fatalf("slug=%q", slug)
	}
	if slug := scenariogen.Slug(""); slug != "generated_" {
		t.Fatalf("slug=%q", slug)
	}
}

func TestAnthropicGenerateParsesFencedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "key" {
			t.Errorf("missing api key")
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if system, _ := body["system"].(string); !strings.Contains(system, "navigate") {
			t.Errorf("system prompt missing vocabulary")
		}
		json.NewEncoder(w).Encode(map[string]any{"content": []map[string]any{
			{"type": "text", "text": "Here is the spec:\n```json\n" + `{"title":"T","start":"nav","nodes":[{"id":"nav","type":"navigate","url":"http://x/"}]}` + "\n```"},
		}})
	}))
	defer server.Close()
	gen := &scenariogen.Anthropic{APIKey: "key", Model: "m", Endpoint: server.URL}
	spec, err := gen.Generate(context.Background(), scenariogen.Request{Prompt: "p"})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if spec["start"] != "nav" {
		t.Fatalf("spec=%v", spec)
	}
}
