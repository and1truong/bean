package scenariorunner

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/browserapi"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/scenario"
	"github.com/beanruntime/bean/internal/scenarioexec"
	"github.com/beanruntime/bean/internal/scenariorun"
)

// fakeSession stands in for a browser session: each call blocks on the
// gate channel (when set) so tests can pin a run mid-node.
type fakeSession struct {
	gate   chan struct{}
	events chan browserapi.Event
	once   sync.Once
}

func newFakeSession(gate chan struct{}) *fakeSession {
	return &fakeSession{gate: gate, events: make(chan browserapi.Event, 16)}
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

// newRunner wires a runner against a throwaway sqlite store; gates[i]
// gates the i-th session the factory opens (nil = ungated).
func newRunner(t *testing.T, gates func(index int) chan struct{}) (scenariorun.Store, *Runner) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.ExecuteMigration(context.Background(), migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	store := scenariorun.Store{DB: db}
	opened := 0
	runner := &Runner{
		Store:       store,
		ArtifactDir: t.TempDir(),
		Sessions: func(context.Context) (browserapi.Session, error) {
			session := newFakeSession(gates(opened))
			opened++
			return session, nil
		},
		Scenario: func(context.Context, scenariorun.Run) (appir.Scenario, error) {
			return appir.Scenario{Name: "flow", Start: "nav", Nodes: []appir.ScenarioNode{
				{ID: "nav", Type: scenario.NodeNavigate, URL: "http://example.test/", Next: "wait"},
				{ID: "wait", Type: scenario.NodeWait, Condition: browserapi.ConditionNavigation},
			}}, nil
		},
	}
	return store, runner
}

func enqueue(t *testing.T, store scenariorun.Store) scenariorun.Run {
	t.Helper()
	run, err := store.Enqueue(context.Background(), scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "flow", Trigger: scenariorun.TriggerAPI})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func waitRunStatus(t *testing.T, store scenariorun.Store, id, want string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		run, found, err := store.Get(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if found && run.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	run, _, _ := store.Get(context.Background(), id)
	t.Fatalf("run did not reach %s: %+v", want, run)
}

// waitStepInFlight blocks until the walk has started a step for
// nodeID — RequestPause then deterministically parks at the NEXT
// boundary rather than racing the walk's first check.
func waitStepInFlight(t *testing.T, store scenariorun.Store, runID, nodeID string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		steps, err := store.Steps(context.Background(), runID)
		if err != nil {
			t.Fatal(err)
		}
		for _, step := range steps {
			if step.NodeID == nodeID {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("run %s never started a step for node %s", runID, nodeID)
}

func TestRunOnceExecutesPendingRun(t *testing.T) {
	store, runner := newRunner(t, func(int) chan struct{} { return nil })
	run := enqueue(t, store)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunCompleted)
	steps, err := store.Steps(context.Background(), run.ID)
	if err != nil || len(steps) != 2 {
		t.Fatalf("steps=%v %v", steps, err)
	}
	for _, step := range steps {
		if step.Status != scenariorun.StepPassed {
			t.Fatalf("step %+v", step)
		}
	}
}

func TestStopCancelsPendingRun(t *testing.T) {
	store, runner := newRunner(t, func(int) chan struct{} { return nil })
	run := enqueue(t, store)
	if err := runner.Stop(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	got, found, err := store.Get(context.Background(), run.ID)
	if err != nil || !found || got.Status != scenariorun.RunCancelled {
		t.Fatalf("run=%+v found=%v err=%v", got, found, err)
	}
	if err = runner.Stop(context.Background(), run.ID); err == nil {
		t.Fatal("expected error stopping a terminal run")
	}
}

func TestPauseAndResumeInFlightRun(t *testing.T) {
	first := make(chan struct{})
	calls := 0
	store, runner := newRunner(t, func(int) chan struct{} {
		calls++
		if calls == 1 {
			return first
		}
		return nil
	})
	run := enqueue(t, store)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunRunning)
	if err := runner.RequestPause(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	close(first)
	waitRunStatus(t, store, run.ID, scenariorun.RunPaused)
	if err := runner.Resume(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunCompleted)
}

func TestStopInFlightRun(t *testing.T) {
	gate := make(chan struct{})
	store, runner := newRunner(t, func(int) chan struct{} { return gate })
	run := enqueue(t, store)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunRunning)
	if err := runner.Stop(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunCancelled)
	close(gate)
}

// TestManualTakeoverAndResumeReusesSession drives the full control loop:
// pause mid-run, a human op on the parked browser, resume on the same
// session — and a stop on a parked run closes its held session.
func TestManualTakeoverAndResumeReusesSession(t *testing.T) {
	gate := make(chan struct{})
	store, runner := newRunner(t, func(int) chan struct{} { return gate })
	run := enqueue(t, store)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunRunning)
	// Pause only after 'nav' is in flight so the walk parks at the
	// next boundary ('wait'), not at the scenario start.
	waitStepInFlight(t, store, run.ID, "nav")
	if err := runner.RequestPause(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	close(gate)
	waitRunStatus(t, store, run.ID, scenariorun.RunPaused)

	result, err := runner.Manual(context.Background(), run.ID, scenarioexec.Manual{Op: scenarioexec.ManualSnapshot})
	if err != nil || result == "" {
		t.Fatalf("manual snapshot=%q err=%v", result, err)
	}
	if _, err = runner.Manual(context.Background(), run.ID, scenarioexec.Manual{Op: "bogus"}); err == nil {
		t.Fatal("expected unsupported manual op to error")
	}

	if err := runner.Resume(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunCompleted)

	sessions, err := store.Sessions(context.Background(), run.ID)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	events, err := store.Events(context.Background(), run.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	manual, resumePoint := 0, false
	for _, event := range events {
		if event.Kind == scenariorun.EventManualAction {
			manual++
		}
		if event.Kind == scenariorun.EventResumePoint && event.Payload == `{"resume_node":"wait"}` {
			resumePoint = true
		}
	}
	if manual != 2 || !resumePoint {
		t.Fatalf("manual=%d resumePoint=%v", manual, resumePoint)
	}
}

// TestStopClosesHeldSession: stopping a parked run cancels it and drops
// the held browser session — manual ops then find nothing.
func TestStopClosesHeldSession(t *testing.T) {
	gate := make(chan struct{})
	store, runner := newRunner(t, func(int) chan struct{} { return gate })
	run := enqueue(t, store)
	if err := runner.RunOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunRunning)
	if err := runner.RequestPause(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	close(gate)
	waitRunStatus(t, store, run.ID, scenariorun.RunPaused)
	if err := runner.Stop(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitRunStatus(t, store, run.ID, scenariorun.RunCancelled)
	if _, err := runner.Manual(context.Background(), run.ID, scenarioexec.Manual{Op: scenarioexec.ManualSnapshot}); err == nil {
		t.Fatal("manual op succeeded after stop")
	}
}
