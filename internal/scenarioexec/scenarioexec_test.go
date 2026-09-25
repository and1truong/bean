package scenarioexec_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/browserapi"
	"github.com/beanruntime/bean/internal/browserplaywright"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/scenarioexec"
	"github.com/beanruntime/bean/internal/scenariorun"
)

const loginPage = `<!doctype html><html><head><title>Sign in</title></head><body>
<main><h1>Sign in</h1>
<form id="login" action="/done" method="get">
<label for="email">Email</label><input id="email" name="email" type="email">
<label for="password">Password</label><input id="password" name="password" type="password">
<button id="submit" type="submit">Continue</button>
</form></main></body></html>`

const donePage = `<!doctype html><html><head><title>Done</title></head><body><main><h1>Welcome</h1><p id="banner">Login accepted</p></main><script>console.log("bean done page");fetch("/ping");</script></body></html>`

const failPage = `<!doctype html><html><head><title>Sign in</title></head><body><main><h1>Sign in</h1><p id="error">Invalid credentials</p></main></body></html>`

func newExecutor(t *testing.T) (*httptest.Server, scenariorun.Store, scenarioexec.Executor, string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("content-type", "text/html")
		switch r.URL.Path {
		case "/done":
			fmt.Fprint(w, donePage)
		case "/failed-login":
			fmt.Fprint(w, failPage)
		default:
			fmt.Fprint(w, loginPage)
		}
	}))
	t.Cleanup(server.Close)
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.ExecuteMigration(context.Background(), migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	store := scenariorun.Store{DB: db}
	if _, err = os.Stat("../../browser/sidecar.mjs"); err != nil {
		t.Skipf("sidecar source unavailable: %v", err)
	}
	artifactDir := filepath.Join(t.TempDir(), "artifacts")
	executor := scenarioexec.Executor{
		Runs:        store,
		ArtifactDir: artifactDir,
		Sessions: func(ctx context.Context) (browserapi.Session, error) {
			return (browserplaywright.Adapter{Dir: "../../browser"}).NewSession(ctx)
		},
		Secrets: func(ctx context.Context, name string) (string, error) {
			if name == "PASSWORD" {
				return "s3cret", nil
			}
			return "", fmt.Errorf("unknown secret %q", name)
		},
	}
	return server, store, executor, artifactDir
}

func enqueue(t *testing.T, store scenariorun.Store, scenario string) scenariorun.Run {
	t.Helper()
	run, err := store.Enqueue(context.Background(), scenariorun.Run{
		AppID: "demo", ReleaseID: "rel-1", Scenario: scenario, Trigger: scenariorun.TriggerManual,
	})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func loginScenario(url, next string) appir.Scenario {
	return appir.Scenario{
		Name: "login", Start: "open_login",
		Nodes: []appir.ScenarioNode{
			{ID: "open_login", Type: "navigate", URL: url, Next: "fill_email"},
			{ID: "fill_email", Type: "fill", Ref: "Email", Text: "runner@bean.dev", Next: "fill_password"},
			{ID: "fill_password", Type: "fill", Ref: "Password", Secret: "PASSWORD", Next: "submit"},
			{ID: "submit", Type: "click", Ref: "Continue", Next: next},
		},
	}
}

func TestExecuteCompletesRunThroughRealChromium(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := loginScenario(server.URL+"/login", "check_done")
	compiled.Nodes = append(compiled.Nodes,
		appir.ScenarioNode{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done", Next: "check_banner"},
		appir.ScenarioNode{ID: "check_banner", Type: "assert", Assertion: "text_present", Text: "Login accepted"},
	)
	run := enqueue(t, store, "login")
	if err := executor.Execute(ctx, run.ID, compiled); err != nil {
		t.Fatalf("execute: %v", err)
	}
	persisted, found, err := store.Get(ctx, run.ID)
	if err != nil || !found || persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}
	steps, err := store.Steps(ctx, run.ID)
	if err != nil || len(steps) != 6 {
		t.Fatalf("steps=%d", len(steps))
	}
	for _, step := range steps {
		if step.Status != scenariorun.StepPassed || step.Attempt != 1 {
			t.Fatalf("step %+v", step)
		}
	}
	sessions, err := store.Sessions(ctx, run.ID)
	if err != nil || len(sessions) != 1 || sessions[0].Status != scenariorun.SessionClosed {
		t.Fatalf("sessions=%+v", sessions)
	}
	events, err := store.Events(ctx, run.ID, 0)
	if err != nil || len(events) == 0 {
		t.Fatalf("events=%v", err)
	}
	kinds := map[string]int{}
	for _, event := range events {
		kinds[event.Kind]++
		if event.Sequence <= 0 {
			t.Fatalf("event out of order: %+v", event)
		}
	}
	for _, want := range []string{
		scenariorun.EventStepStarted, scenariorun.EventStepFinished,
		scenariorun.EventBrowserSnapshot, scenariorun.EventAssertion,
		scenariorun.EventConsole, scenariorun.EventNetwork,
	} {
		if kinds[want] == 0 {
			t.Fatalf("no %s events in %v", want, kinds)
		}
	}
}

func TestExecuteFailsRunDeterministicallyAndCapturesDiagnosisBundle(t *testing.T) {
	server, store, executor, artifactDir := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "missing_button", Start: "open_login",
		Nodes: []appir.ScenarioNode{
			{ID: "open_login", Type: "navigate", URL: server.URL + "/login", Next: "click_ghost"},
			{ID: "click_ghost", Type: "click", Ref: "NoSuchButton", TimeoutSeconds: 5},
		},
	}
	run := enqueue(t, store, "missing_button")
	if err := executor.Execute(ctx, run.ID, compiled); err == nil {
		t.Fatal("run with unresolvable ref succeeded")
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunFailed || persisted.Error == "" {
		t.Fatalf("run=%+v", persisted)
	}
	steps, _ := store.Steps(ctx, run.ID)
	if steps[1].Status != scenariorun.StepFailed {
		t.Fatalf("step=%+v", steps[1])
	}
	artifacts, _ := store.Artifacts(ctx, run.ID)
	byKind := map[string]scenariorun.Artifact{}
	for _, artifact := range artifacts {
		byKind[artifact.Kind] = artifact
	}
	for _, want := range []string{scenariorun.ArtifactScreenshot, scenariorun.ArtifactDOM, scenariorun.ArtifactTrace} {
		artifact, ok := byKind[want]
		if !ok {
			t.Fatalf("no %s artifact in %+v", want, artifacts)
		}
		info, err := os.Stat(filepath.Join(artifactDir, artifact.Ref))
		if err != nil || info.Size() == 0 || info.Size() != artifact.Size {
			t.Fatalf("%s artifact file missing: ref=%q size=%d err=%v", want, artifact.Ref, artifact.Size, err)
		}
	}
	sessions, _ := store.Sessions(ctx, run.ID)
	if sessions[0].Status != scenariorun.SessionClosed && sessions[0].Status != scenariorun.SessionFailed {
		t.Fatalf("session leaked: %+v", sessions[0])
	}
}

func TestPauseLeavesRunResumable(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := loginScenario(server.URL+"/login", "checkpoint")
	compiled.Nodes = append(compiled.Nodes,
		appir.ScenarioNode{ID: "checkpoint", Type: "pause", Next: "reopen"},
	)
	run := enqueue(t, store, "login")
	if err := executor.Execute(ctx, run.ID, compiled); !errors.Is(err, scenarioexec.ErrPaused) {
		t.Fatalf("execute err=%v", err)
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunPaused {
		t.Fatalf("run=%+v", persisted)
	}
	if err := store.Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	// Resume continues at the recorded boundary (checkpoint's outgoing
	// edge) on a fresh session — already-run nodes are not re-executed, so
	// the boundary must be self-sufficient (re-navigate before asserting).
	resumed := loginScenario(server.URL+"/login", "")
	resumed.Nodes = append(resumed.Nodes,
		appir.ScenarioNode{ID: "reopen", Type: "navigate", URL: server.URL + "/done", Next: "check_done"},
		appir.ScenarioNode{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done"},
	)
	if err := executor.Execute(ctx, run.ID, resumed); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	persisted, _, _ = store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}
	steps, err := store.Steps(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) != 7 {
		t.Fatalf("steps=%v", steps)
	}
	for _, step := range steps {
		if step.Attempt != 1 {
			t.Fatalf("node %s re-ran on resume: %+v", step.NodeID, step)
		}
	}
}

// TestTakeoverHoldsSessionForManualOps covers the takeover loop: a pause
// node parks the walk with the browser still open, human ops run against
// the live session and land in the log as manual_action events, and
// resume continues on the same session at the recorded boundary.
func TestTakeoverHoldsSessionForManualOps(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	executor.Held = &scenarioexec.Held{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := loginScenario(server.URL+"/login", "done_nav")
	compiled.Nodes = append(compiled.Nodes, appir.ScenarioNode{ID: "done_nav", Type: "navigate", URL: server.URL + "/done"})
	// Park between the two fills.
	for i, node := range compiled.Nodes {
		if node.ID == "fill_email" {
			compiled.Nodes[i].Next = "human"
		}
	}
	compiled.Nodes = append(compiled.Nodes, appir.ScenarioNode{ID: "human", Type: "pause", Next: "fill_password"})
	run := enqueue(t, store, "login")
	if err := executor.Execute(ctx, run.ID, compiled); !errors.Is(err, scenarioexec.ErrPaused) {
		t.Fatalf("execute err=%v", err)
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunPaused {
		t.Fatalf("run=%+v", persisted)
	}

	snapshot, err := executor.Held.Manual(ctx, run.ID, scenarioexec.Manual{Op: scenarioexec.ManualSnapshot})
	if err != nil || !strings.Contains(snapshot, "Sign in") {
		t.Fatalf("manual snapshot=%q err=%v", snapshot, err)
	}
	if _, err = executor.Held.Manual(ctx, run.ID, scenarioexec.Manual{Op: scenarioexec.ManualFill, Ref: "Password", Secret: "PASSWORD"}); err != nil {
		t.Fatalf("manual secret fill: %v", err)
	}
	if _, err = executor.Held.Manual(ctx, run.ID, scenarioexec.Manual{Op: "bogus"}); err == nil {
		t.Fatal("expected unsupported manual op to error")
	}
	if _, err = executor.Held.Manual(ctx, "other-run", scenarioexec.Manual{Op: scenarioexec.ManualSnapshot}); err == nil {
		t.Fatal("expected manual op on non-held run to error")
	}

	if err := store.Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if err := executor.Execute(ctx, run.ID, compiled); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	persisted, _, _ = store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}

	// Takeover reuses the parked session — one session row for the run.
	sessions, err := store.Sessions(ctx, run.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	steps, err := store.Steps(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		if step.Attempt != 1 {
			t.Fatalf("node %s re-ran on resume: %+v", step.NodeID, step)
		}
	}
	events, err := store.Events(ctx, run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	var manual, resumePoint int
	for _, event := range events {
		if event.Kind == scenariorun.EventManualAction {
			manual++
		}
		if event.Kind == scenariorun.EventResumePoint {
			if !strings.Contains(event.Payload, `"resume_node":"fill_password"`) {
				t.Fatalf("resume_point payload=%s", event.Payload)
			}
			resumePoint++
		}
	}
	// Three manual ops logged — snapshot + fill + the rejected bogus op.
	if manual != 3 || resumePoint != 1 {
		t.Fatalf("manual=%d resumePoint=%d", manual, resumePoint)
	}
}

// TestResumeWithoutHeldSessionContinuesAtBoundary covers the degraded
// path: the runner process lost the parked session (or none was held),
// so resume opens a fresh session and still starts at the boundary node
// rather than the scenario start.
func TestResumeWithoutHeldSessionContinuesAtBoundary(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	executor.Held = &scenarioexec.Held{}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "login", Start: "open_login",
		Nodes: []appir.ScenarioNode{
			{ID: "open_login", Type: "navigate", URL: server.URL + "/login", Next: "human"},
			{ID: "human", Type: "pause", Next: "done_nav"},
			{ID: "done_nav", Type: "navigate", URL: server.URL + "/done", Next: "check_done"},
			{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done"},
		},
	}
	run := enqueue(t, store, "login")
	if err := executor.Execute(ctx, run.ID, compiled); !errors.Is(err, scenarioexec.ErrPaused) {
		t.Fatalf("execute err=%v", err)
	}
	if err := store.Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	// A different executor with an empty registry — the parked session
	// is gone, so resume opens a second session at the boundary.
	resumer := executor
	resumer.Held = &scenarioexec.Held{}
	if err := resumer.Execute(ctx, run.ID, compiled); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	sessions, err := store.Sessions(ctx, run.ID)
	if err != nil || len(sessions) != 2 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	steps, err := store.Steps(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range steps {
		if step.NodeID == "open_login" && step.Attempt != 1 {
			t.Fatalf("boundary-before node re-ran: %+v", step)
		}
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}
}

func TestExecuteRejectsUnclaimableRun(t *testing.T) {
	_, store, executor, _ := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := executor.Execute(ctx, "missing-run", appir.Scenario{}); err == nil {
		t.Fatal("claimed a missing run")
	}
	run := enqueue(t, store, "s")
	if claimed, _ := store.Claim(ctx, run.ID, "other"); !claimed {
		t.Fatal("setup claim failed")
	}
	if err := executor.Execute(ctx, run.ID, appir.Scenario{}); err == nil {
		t.Fatal("claimed an already-claimed run")
	}
}

func TestPolicyBlocksDisallowedNavigation(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	executor.Policy = scenarioexec.Policy{AllowedDomains: []string{"allowed.example"}}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run := enqueue(t, store, "blocked")
	err := executor.Execute(ctx, run.ID, appir.Scenario{
		Name: "blocked", Start: "open",
		Nodes: []appir.ScenarioNode{{ID: "open", Type: "navigate", URL: server.URL + "/login"}},
	})
	if err == nil {
		t.Fatal("run to a disallowed host succeeded")
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunFailed {
		t.Fatalf("run=%+v", persisted)
	}
	events, _ := store.Events(ctx, run.ID, 0)
	blocked := false
	for _, event := range events {
		if event.Kind == scenariorun.EventPolicyBlocked && strings.Contains(event.Payload, "127.0.0.1") {
			blocked = true
		}
	}
	if !blocked {
		t.Fatalf("no policy_blocked event in %+v", events)
	}
}

func TestSecretFillNeverLeaksIntoLogsOrArtifacts(t *testing.T) {
	server, store, executor, artifactDir := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "login", Start: "open_login",
		Nodes: []appir.ScenarioNode{
			{ID: "open_login", Type: "navigate", URL: server.URL + "/login", Next: "fill_password"},
			{ID: "fill_password", Type: "fill", Ref: "Password", Secret: "PASSWORD", Next: "check"},
			{ID: "check", Type: "assert", Assertion: "ref_text", Ref: "Password", Text: "never"},
		},
	}
	run := enqueue(t, store, "login")
	if err := executor.Execute(ctx, run.ID, compiled); err == nil {
		t.Fatal("assert should have failed")
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunFailed {
		t.Fatalf("run=%+v", persisted)
	}
	events, _ := store.Events(ctx, run.ID, 0)
	secretUsed := false
	for _, event := range events {
		if strings.Contains(event.Payload, "s3cret") {
			t.Fatalf("secret leaked into %s event: %s", event.Kind, event.Payload)
		}
		if event.Kind == scenariorun.EventSecretUsed {
			secretUsed = true
		}
	}
	if !secretUsed {
		t.Fatal("no secret_used audit event")
	}
	artifacts, _ := store.Artifacts(ctx, run.ID)
	if len(artifacts) == 0 {
		t.Fatal("no diagnosis artifacts")
	}
	for _, artifact := range artifacts {
		content, err := os.ReadFile(filepath.Join(artifactDir, artifact.Ref))
		if err != nil {
			continue
		}
		if strings.Contains(string(content), "s3cret") {
			t.Fatalf("secret leaked into %s artifact %s", artifact.Kind, artifact.Ref)
		}
	}
}

func TestPolicyPauseGateRequiresResume(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	executor.Policy = scenarioexec.Policy{PauseOn: []string{"wait"}}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "gated", Start: "open",
		Nodes: []appir.ScenarioNode{
			{ID: "open", Type: "navigate", URL: server.URL + "/login", Next: "hold"},
			{ID: "hold", Type: "wait", Condition: "navigation"},
		},
	}
	run := enqueue(t, store, "gated")
	if err := executor.Execute(ctx, run.ID, compiled); !errors.Is(err, scenarioexec.ErrPaused) {
		t.Fatalf("expected ErrPaused, got %v", err)
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunPaused {
		t.Fatalf("run=%+v", persisted)
	}
	// Resuming walks again; the consumed gate lets the wait node execute.
	if err := store.Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if err := executor.Execute(ctx, run.ID, compiled); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	persisted, _, _ = store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}
	events, _ := store.Events(ctx, run.ID, 0)
	pauses := 0
	for _, event := range events {
		if event.Kind == scenariorun.EventPolicyPause {
			pauses++
		}
	}
	if pauses != 1 {
		t.Fatalf("policy_pause events=%d", pauses)
	}
}

func TestPolicyMaxDurationCancelsRun(t *testing.T) {
	_, store, executor, _ := newExecutor(t)
	executor.Policy = scenarioexec.Policy{MaxDuration: time.Nanosecond}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run := enqueue(t, store, "bounded")
	_ = executor.Execute(ctx, run.ID, appir.Scenario{
		Name: "bounded", Start: "open",
		Nodes: []appir.ScenarioNode{{ID: "open", Type: "navigate", URL: "http://example.test/"}},
	})
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCancelled && persisted.Status != scenariorun.RunFailed {
		t.Fatalf("run=%+v", persisted)
	}
}

const conflictPage = `<!doctype html><html><head><title>Sign in</title></head><body>
<main><h1>Sign in</h1>
<form id="login" action="/done" method="get">
<button id="submit" type="submit">Sign in</button>
</form></main></body></html>`

const ambiguousPage = `<!doctype html><html><head><title>Duplicate</title></head><body>
<main><button id="a">Sign in</button><button id="b">Sign in</button></main></body></html>`

func serve(t *testing.T, pages map[string]string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, ok := pages[r.URL.Path]
		if !ok {
			page = pages["/"]
		}
		w.Header().Set("content-type", "text/html")
		fmt.Fprint(w, page)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestClickRefResolvesToTheActionableElement(t *testing.T) {
	_, store, executor, _ := newExecutor(t)
	// A heading and a submit button share the accessible name "Sign in" —
	// the click must land on the button, not the non-interactive ancestor.
	server := serve(t, map[string]string{"/": conflictPage, "/done": donePage})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "conflict", Start: "open",
		Nodes: []appir.ScenarioNode{
			{ID: "open", Type: "navigate", URL: server.URL + "/", Next: "submit"},
			{ID: "submit", Type: "click", Ref: "Sign in", Next: "check_done"},
			{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done"},
		},
	}
	run := enqueue(t, store, "conflict")
	if err := executor.Execute(ctx, run.ID, compiled); err != nil {
		t.Fatalf("execute: %v", err)
	}
	persisted, _, _ := store.Get(ctx, run.ID)
	if persisted.Status != scenariorun.RunCompleted {
		t.Fatalf("run=%+v", persisted)
	}
}

func TestClickRefRejectsNonActionableMatch(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// loginPage's only "Sign in" is a heading — clicking it must fail loudly
	// instead of silently hitting a non-interactive element.
	compiled := appir.Scenario{
		Name: "heading_click", Start: "open",
		Nodes: []appir.ScenarioNode{
			{ID: "open", Type: "navigate", URL: server.URL + "/login", Next: "submit"},
			{ID: "submit", Type: "click", Ref: "Sign in"},
		},
	}
	run := enqueue(t, store, "heading_click")
	if err := executor.Execute(ctx, run.ID, compiled); err == nil {
		t.Fatal("click on a heading succeeded")
	}
	steps, _ := store.Steps(ctx, run.ID)
	if steps[1].Status != scenariorun.StepFailed || !strings.Contains(steps[1].Error, "not a clickable element") {
		t.Fatalf("step=%+v", steps[1])
	}
}

func TestClickRefRejectsAmbiguousMatch(t *testing.T) {
	_, store, executor, _ := newExecutor(t)
	server := serve(t, map[string]string{"/": ambiguousPage})
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "ambiguous", Start: "open",
		Nodes: []appir.ScenarioNode{
			{ID: "open", Type: "navigate", URL: server.URL + "/", Next: "submit"},
			{ID: "submit", Type: "click", Ref: "Sign in"},
		},
	}
	run := enqueue(t, store, "ambiguous")
	if err := executor.Execute(ctx, run.ID, compiled); err == nil {
		t.Fatal("ambiguous click succeeded")
	}
	steps, _ := store.Steps(ctx, run.ID)
	if steps[1].Status != scenariorun.StepFailed || !strings.Contains(steps[1].Error, "ambiguous") {
		t.Fatalf("step=%+v", steps[1])
	}
}

func TestAssertionTimeoutRecordsOutcomeNotInfrastructureError(t *testing.T) {
	server, store, executor, _ := newExecutor(t)
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	compiled := appir.Scenario{
		Name: "missing_text", Start: "open",
		Nodes: []appir.ScenarioNode{
			{ID: "open", Type: "navigate", URL: server.URL + "/login", Next: "check_missing"},
			{ID: "check_missing", Type: "assert", Assertion: "text_present", Text: "This sentence intentionally does not exist", TimeoutSeconds: 1},
		},
	}
	run := enqueue(t, store, "missing_text")
	if err := executor.Execute(ctx, run.ID, compiled); err == nil {
		t.Fatal("unmet assertion succeeded")
	}
	// The failed check lands in the event stream as a failed assertion —
	// expected text and observation included — never as bare infrastructure noise.
	events, _ := store.Events(ctx, run.ID, 0)
	var assertion string
	for _, event := range events {
		if event.Kind == scenariorun.EventAssertion {
			assertion = event.Payload
		}
	}
	if !strings.Contains(assertion, `"met":false`) || !strings.Contains(assertion, "This sentence intentionally does not exist") || !strings.Contains(assertion, "expected") {
		t.Fatalf("assertion event=%s", assertion)
	}
	steps, _ := store.Steps(ctx, run.ID)
	if steps[1].Status != scenariorun.StepFailed || !strings.Contains(steps[1].Error, "This sentence intentionally does not exist") || !strings.Contains(steps[1].Error, "within 1s") {
		t.Fatalf("step error=%q", steps[1].Error)
	}
}
