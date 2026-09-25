package scenarioexec_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
		appir.ScenarioNode{ID: "checkpoint", Type: "pause", Next: "check_done"},
		appir.ScenarioNode{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done"},
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
	// Resume drives the remaining nodes under a fresh session.
	resumed := loginScenario(server.URL+"/login", "")
	resumed.Nodes = append(resumed.Nodes,
		appir.ScenarioNode{ID: "check_done", Type: "assert", Assertion: "url_contains", Text: "/done"},
	)
	if err := executor.Execute(ctx, run.ID, resumed); err != nil {
		t.Fatalf("resume execute: %v", err)
	}
	persisted, _, _ = store.Get(ctx, run.ID)
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
