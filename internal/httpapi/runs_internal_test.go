package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/browserapi"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/scenariorun"
)

// fakeSession stands in for a browser session in HTTP tests: each call
// waits on the gate channel (when set) so a run can be pinned mid-node.
type fakeSession struct {
	gate   chan struct{}
	events chan browserapi.Event
	once   sync.Once
}

func (f *fakeSession) await(ctx context.Context) error {
	if f.gate == nil {
		return nil
	}
	select {
	case <-f.gate:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (f *fakeSession) Open(ctx context.Context, url string) (browserapi.Result, error) {
	if err := f.await(ctx); err != nil {
		return browserapi.Result{}, err
	}
	return browserapi.Result{URL: url}, nil
}
func (f *fakeSession) Snapshot(ctx context.Context) (browserapi.Snapshot, error) {
	return browserapi.Snapshot{ID: "snap"}, f.await(ctx)
}
func (f *fakeSession) Click(ctx context.Context, _ browserapi.Ref) (browserapi.Result, error) {
	return browserapi.Result{}, f.await(ctx)
}
func (f *fakeSession) Fill(ctx context.Context, _ browserapi.Ref, _ string) (browserapi.Result, error) {
	return browserapi.Result{}, f.await(ctx)
}
func (f *fakeSession) Select(ctx context.Context, _ browserapi.Ref, _ string) (browserapi.Result, error) {
	return browserapi.Result{}, f.await(ctx)
}
func (f *fakeSession) Press(ctx context.Context, _ string) (browserapi.Result, error) {
	return browserapi.Result{}, f.await(ctx)
}
func (f *fakeSession) Wait(ctx context.Context, condition browserapi.Condition) (browserapi.WaitResult, error) {
	if err := f.await(ctx); err != nil {
		return browserapi.WaitResult{}, err
	}
	return browserapi.WaitResult{Condition: condition, Met: true}, nil
}
func (f *fakeSession) Extract(ctx context.Context, _ browserapi.Ref, _, _ string) (browserapi.Extraction, error) {
	return browserapi.Extraction{}, f.await(ctx)
}
func (f *fakeSession) Screenshot(context.Context) (browserapi.Capture, error) {
	return browserapi.Capture{ContentType: "image/png", Bytes: []byte{0x89, 0x50}}, nil
}
func (f *fakeSession) Trace(context.Context) (browserapi.Capture, error) {
	return browserapi.Capture{ContentType: "application/zip", Bytes: []byte("PK\x03\x04")}, nil
}
func (f *fakeSession) Events() <-chan browserapi.Event { return f.events }
func (f *fakeSession) Close(context.Context) error {
	f.once.Do(func() { close(f.events) })
	return nil
}

var _ browserapi.Session = (*fakeSession)(nil)

// runFixture builds a running runtime with one published scenario; gates
// controls the i-th browser session the runner opens (nil = ungated).
func runFixture(t *testing.T, gates func(index int) chan struct{}) (*bootstrap.Runtime, http.Handler, *http.Cookie, string) {
	t.Helper()
	ctx := context.Background()
	runtime, err := bootstrap.Open(ctx, filepath.Join(t.TempDir(), "runs.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { runtime.DB.Close() })
	if err = runtime.HTTP.Auth.Bootstrap(ctx, "admin@example.test", "test-password"); err != nil {
		t.Fatal(err)
	}
	bundle := definition.Bundle{Name: "scenarios", Definitions: []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Scenario", Metadata: definition.Metadata{Name: "smoke"}, Spec: map[string]any{
			"title": "Smoke", "start": "nav",
			"nodes": []any{
				map[string]any{"id": "nav", "type": "navigate", "url": "http://example.test/", "next": "wait"},
				map[string]any{"id": "wait", "type": "wait", "condition": "navigation"},
			},
		}},
	}}
	if err = runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := runtime.Store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) > 0 {
		t.Fatalf("publish err=%v diagnostics=%v", publishErr, diagnostics)
	}
	opened := 0
	runtime.Runs.Sessions = func(context.Context) (browserapi.Session, error) {
		session := &fakeSession{gate: gates(opened), events: make(chan browserapi.Event, 16)}
		opened++
		return session, nil
	}
	runtime.Runs.ArtifactDir = t.TempDir()
	handler := runtime.HTTP.Handler()
	login := serve(t, handler, http.MethodPost, "/api/auth/login", map[string]any{"email": "admin@example.test", "password": "test-password"}, nil, "")
	var session map[string]any
	decodeResponse(t, login, &session)
	return runtime, handler, login.Result().Cookies()[0], session["csrfToken"].(string)
}

func waitHTTPRunStatus(t *testing.T, runtime *bootstrap.Runtime, id, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, found, err := runtime.Runs.Store.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if found && run.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _, _ := runtime.Runs.Store.Get(context.Background(), id)
	t.Fatalf("run did not reach %s: %+v", want, run)
}

func runIDOf(t *testing.T, response *httptest.ResponseRecorder) string {
	t.Helper()
	var created map[string]any
	decodeResponse(t, response, &created)
	id, _ := created["ID"].(string)
	if id == "" {
		t.Fatalf("no run id in %s", response.Body.String())
	}
	return id
}

func TestScenarioRunHTTPDrivesLifecycle(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return nil })

	scenarios := serve(t, handler, http.MethodGet, "/api/scenarios", nil, cookie, "")
	if scenarios.Code != http.StatusOK || !strings.Contains(scenarios.Body.String(), "smoke") {
		t.Fatalf("scenarios=%d %s", scenarios.Code, scenarios.Body.String())
	}
	missing := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "missing"}, cookie, csrf)
	if missing.Code != http.StatusUnprocessableEntity {
		t.Fatalf("missing scenario=%d %s", missing.Code, missing.Body.String())
	}
	created := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "smoke"}, cookie, csrf)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	id := runIDOf(t, created)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunCompleted)

	detail := serve(t, handler, http.MethodGet, "/api/scenario-runs/"+id, nil, cookie, "")
	var body struct {
		Run   map[string]any   `json:"run"`
		Steps []map[string]any `json:"steps"`
	}
	decodeResponse(t, detail, &body)
	if detail.Code != http.StatusOK || len(body.Steps) != 2 {
		t.Fatalf("detail=%d %s", detail.Code, detail.Body.String())
	}
	for _, step := range body.Steps {
		if step["Status"] != scenariorun.StepPassed {
			t.Fatalf("step=%+v", step)
		}
	}
	events := serve(t, handler, http.MethodGet, "/api/scenario-runs/"+id+"/events", nil, cookie, "")
	if events.Code != http.StatusOK || !strings.Contains(events.Body.String(), "step_finished") {
		t.Fatalf("events=%d %s", events.Code, events.Body.String())
	}
	list := serve(t, handler, http.MethodGet, "/api/scenario-runs?scenario=smoke", nil, cookie, "")
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), id) {
		t.Fatalf("list=%d %s", list.Code, list.Body.String())
	}
}

func TestScenarioRunHTTPPauseResumeStop(t *testing.T) {
	gate := make(chan struct{})
	runtime, handler, cookie, csrf := runFixture(t, func(index int) chan struct{} {
		if index == 0 {
			return gate
		}
		return nil
	})
	created := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "smoke"}, cookie, csrf)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	id := runIDOf(t, created)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunRunning)

	pause := serve(t, handler, http.MethodPost, fmt.Sprintf("/api/scenario-runs/%s/pause", id), map[string]any{}, cookie, csrf)
	if pause.Code != http.StatusOK {
		t.Fatalf("pause=%d %s", pause.Code, pause.Body.String())
	}
	close(gate)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunPaused)

	resume := serve(t, handler, http.MethodPost, fmt.Sprintf("/api/scenario-runs/%s/resume", id), map[string]any{}, cookie, csrf)
	if resume.Code != http.StatusOK {
		t.Fatalf("resume=%d %s", resume.Code, resume.Body.String())
	}
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunCompleted)
}

func TestScenarioRunHTTPStopAndGuards(t *testing.T) {
	runtime, handler, cookie, csrf := runFixture(t, func(int) chan struct{} { return make(chan struct{}) })

	unauthenticated := serve(t, handler, http.MethodGet, "/api/scenario-runs", nil, nil, "")
	if unauthenticated.Code != http.StatusForbidden {
		t.Fatalf("unauthenticated=%d", unauthenticated.Code)
	}
	noCSRF := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "smoke"}, cookie, "")
	if noCSRF.Code != http.StatusForbidden {
		t.Fatalf("no csrf=%d", noCSRF.Code)
	}
	unknown := serve(t, handler, http.MethodPost, "/api/scenario-runs/some-id/rewind", map[string]any{}, cookie, csrf)
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown control=%d", unknown.Code)
	}
	created := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "smoke"}, cookie, csrf)
	id := runIDOf(t, created)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunRunning)
	stop := serve(t, handler, http.MethodPost, fmt.Sprintf("/api/scenario-runs/%s/stop", id), map[string]any{}, cookie, csrf)
	if stop.Code != http.StatusOK {
		t.Fatalf("stop=%d %s", stop.Code, stop.Body.String())
	}
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunCancelled)
}

func TestScenarioRunArtifactDownload(t *testing.T) {
	runtime, handler, cookie, _ := runFixture(t, func(int) chan struct{} { return nil })
	ctx := context.Background()
	run, err := runtime.Runs.Store.Enqueue(ctx, scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "smoke", Trigger: scenariorun.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	root := runtime.Runs.ArtifactDir
	dir := filepath.Join(root, run.ID)
	if err = os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "shot.png"), []byte("PNGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}
	artifact, err := runtime.Runs.Store.RecordArtifact(ctx, scenariorun.Artifact{RunID: run.ID, Kind: scenariorun.ArtifactScreenshot, ContentType: "image/png", Size: 7, Ref: filepath.ToSlash(filepath.Join(run.ID, "shot.png"))})
	if err != nil {
		t.Fatal(err)
	}
	download := serve(t, handler, http.MethodGet, fmt.Sprintf("/api/scenario-runs/%s/artifacts/%s", run.ID, artifact.ID), nil, cookie, "")
	if download.Code != http.StatusOK || download.Body.String() != "PNGDATA" || download.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("download=%d %s", download.Code, download.Body.String())
	}

	bad, err := runtime.Runs.Store.RecordArtifact(ctx, scenariorun.Artifact{RunID: run.ID, Kind: scenariorun.ArtifactScreenshot, ContentType: "image/png", Size: 1, Ref: "../escape"})
	if err != nil {
		t.Fatal(err)
	}
	escape := serve(t, handler, http.MethodGet, fmt.Sprintf("/api/scenario-runs/%s/artifacts/%s", run.ID, bad.ID), nil, cookie, "")
	if escape.Code != http.StatusUnprocessableEntity {
		t.Fatalf("escape=%d", escape.Code)
	}
}

// TestScenarioRunHTTPManualTakeover exercises POST /manual: on a paused
// run a human op drives the parked browser session and lands in the
// event log as manual_action; the op's JSON result comes straight back.
func TestScenarioRunHTTPManualTakeover(t *testing.T) {
	gate := make(chan struct{})
	runtime, handler, cookie, csrf := runFixture(t, func(index int) chan struct{} {
		if index == 0 {
			return gate
		}
		return nil
	})
	created := serve(t, handler, http.MethodPost, "/api/scenario-runs", map[string]any{"scenario": "smoke"}, cookie, csrf)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d %s", created.Code, created.Body.String())
	}
	id := runIDOf(t, created)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunRunning)

	// Manual ops on a running (unpaused) run are rejected.
	running := serve(t, handler, http.MethodPost, "/api/scenario-runs/"+id+"/manual", map[string]any{"op": "snapshot"}, cookie, csrf)
	if running.Code != http.StatusConflict {
		t.Fatalf("manual on running=%d %s", running.Code, running.Body.String())
	}

	pause := serve(t, handler, http.MethodPost, fmt.Sprintf("/api/scenario-runs/%s/pause", id), map[string]any{}, cookie, csrf)
	if pause.Code != http.StatusOK {
		t.Fatalf("pause=%d %s", pause.Code, pause.Body.String())
	}
	close(gate)
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunPaused)

	snapshot := serve(t, handler, http.MethodPost, "/api/scenario-runs/"+id+"/manual", map[string]any{"op": "snapshot"}, cookie, csrf)
	if snapshot.Code != http.StatusOK || !strings.Contains(snapshot.Body.String(), "snapshot") {
		t.Fatalf("manual snapshot=%d %s", snapshot.Code, snapshot.Body.String())
	}
	bogus := serve(t, handler, http.MethodPost, "/api/scenario-runs/"+id+"/manual", map[string]any{"op": "bogus"}, cookie, csrf)
	if bogus.Code != http.StatusConflict {
		t.Fatalf("manual bogus=%d %s", bogus.Code, bogus.Body.String())
	}
	missing := serve(t, handler, http.MethodPost, "/api/scenario-runs/missing-run/manual", map[string]any{"op": "snapshot"}, cookie, csrf)
	if missing.Code != http.StatusNotFound {
		t.Fatalf("manual missing run=%d %s", missing.Code, missing.Body.String())
	}

	resume := serve(t, handler, http.MethodPost, fmt.Sprintf("/api/scenario-runs/%s/resume", id), map[string]any{}, cookie, csrf)
	if resume.Code != http.StatusOK {
		t.Fatalf("resume=%d %s", resume.Code, resume.Body.String())
	}
	waitHTTPRunStatus(t, runtime, id, scenariorun.RunCompleted)

	events, err := runtime.Runs.Store.Events(context.Background(), id, 0)
	if err != nil {
		t.Fatal(err)
	}
	manual := 0
	for _, event := range events {
		if event.Kind == scenariorun.EventManualAction {
			manual++
		}
	}
	if manual != 2 {
		t.Fatalf("manual_action events=%d", manual)
	}
}
