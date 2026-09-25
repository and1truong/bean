package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/scenariogen"
	"github.com/beanruntime/bean/internal/scenariorun"
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

// TestScenarioRunRepairDraftsFailedRun exercises POST
// /api/scenario-runs/{id}/repair: on a failed run the authored spec plus the
// failure summary drive scenariogen.Repair, and the response carries the
// node-level diff for review — nothing is saved.
func TestScenarioRunRepairDraftsFailedRun(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })
	ctx := context.Background()
	runtime.HTTP.Generator = &fakeScenarioGenerator{specs: []map[string]any{{
		"title": "Smoke", "start": "nav",
		"nodes": []any{
			map[string]any{"id": "nav", "type": "navigate", "url": "http://example.test/", "next": "wait"},
			map[string]any{"id": "wait", "type": "wait", "condition": "navigation", "next": "verify"},
			map[string]any{"id": "verify", "type": "assert", "assertion": "text_present", "text": "Welcome"},
		},
	}}}

	run, err := runtime.Runs.Store.Enqueue(ctx, scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "smoke", Trigger: scenariorun.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	if ok, claimErr := runtime.Runs.Store.Claim(ctx, run.ID, "token"); !ok || claimErr != nil {
		t.Fatalf("claim=%v %v", ok, claimErr)
	}
	step, err := runtime.Runs.Store.StartStep(ctx, scenariorun.StepExecution{RunID: run.ID, NodeID: "nav", Attempt: 1, Status: scenariorun.StepRunning})
	if err != nil {
		t.Fatal(err)
	}
	if err = runtime.Runs.Store.FinishStep(ctx, step.ID, scenariorun.StepFailed, "", "element not found"); err != nil {
		t.Fatal(err)
	}
	if err = runtime.Runs.Store.Finish(ctx, run.ID, "token", scenariorun.RunFailed, "step nav failed"); err != nil {
		t.Fatal(err)
	}

	response := serve(t, handler, http.MethodPost, "/api/scenario-runs/"+run.ID+"/repair", nil, cookie, csrf)
	if response.Code != http.StatusOK {
		t.Fatalf("repair=%d %s", response.Code, response.Body.String())
	}
	var body map[string]any
	decodeResponse(t, response, &body)
	if body["valid"] != true || body["name"] != "smoke" {
		t.Fatalf("body=%s", response.Body.String())
	}
	spec, _ := body["spec"].(map[string]any)
	if len(spec["nodes"].([]any)) != 3 {
		t.Fatalf("spec=%v", spec)
	}
	diff, _ := body["diff"].([]any)
	if len(diff) == 0 || !strings.Contains(fmt.Sprint(diff), "node verify added") || !strings.Contains(fmt.Sprint(diff), "node wait changed") {
		t.Fatalf("diff=%v", diff)
	}
	definitions := serve(t, handler, http.MethodGet, "/api/admin/definitions", nil, cookie, "")
	var list []map[string]any
	decodeResponse(t, definitions, &list)
	for _, item := range list {
		if item["kind"] == "Scenario" {
			specMap, _ := item["spec"].(map[string]any)
			if len(specMap["nodes"].([]any)) != 2 {
				t.Fatalf("saved scenario changed: %v", specMap)
			}
		}
	}
}

func TestScenarioRunRepairRequiresFailedRun(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })
	run, err := runtime.Runs.Store.Enqueue(context.Background(), scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "smoke", Trigger: scenariorun.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	response := serve(t, handler, http.MethodPost, "/api/scenario-runs/"+run.ID+"/repair", nil, cookie, csrf)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("pending run repair=%d %s", response.Code, response.Body.String())
	}
}
