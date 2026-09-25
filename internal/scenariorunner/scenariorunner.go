// Package scenariorunner drives pending scenario runs through
// scenarioexec on the in-process browser adapter and mediates run
// control: a claimed run can be asked to pause cooperatively between
// nodes, stopped, or resumed once paused. Claimed work is tracked by
// run id so HTTP control lands on the goroutine actually executing it.
package scenariorunner

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/scenarioexec"
	"github.com/beanruntime/bean/internal/scenariorun"
	"github.com/beanruntime/bean/internal/uid"
)

// DefaultMaxConcurrent bounds simultaneous executions when
// Runner.MaxConcurrent is unset — each run owns a Chromium sidecar
// (hundreds of MB), so pending runs queue across ticks rather than
// bursting.
const DefaultMaxConcurrent = 4

// DefaultStaleClaimLease bounds how long a run may hold its claim when
// no Policy.MaxDuration is configured — claims are never renewed, so a
// crashed runner's runs recover once the lease lapses.
const DefaultStaleClaimLease = 2 * time.Hour

// Runner claims pending runs and executes them against the compiled
// scenario resolved by Scenario.
type Runner struct {
	Store       scenariorun.Store
	Sessions    scenarioexec.SessionFactory
	Secrets     scenarioexec.SecretResolver
	ArtifactDir string
	// Policy is the host-level security boundary applied to every run
	// the runner executes.
	Policy scenarioexec.Policy
	// Scenario resolves the compiled graph a pending run executes.
	Scenario func(ctx context.Context, run scenariorun.Run) (appir.Scenario, error)
	// MaxConcurrent caps in-flight executions; pending runs beyond the
	// cap wait for later ticks. Zero applies DefaultMaxConcurrent.
	MaxConcurrent int
	// StaleClaimLease bounds claim age before a running run is recovered
	// failed; zero applies twice Policy.MaxDuration, or
	// DefaultStaleClaimLease when no max duration is configured.
	StaleClaimLease time.Duration

	mu      sync.Mutex
	handles map[string]*handle
	held    *scenarioexec.Held
}

// heldSessions lazily creates the registry of parked (paused) live
// sessions; shared by every executor the runner spawns.
func (r *Runner) heldSessions() *scenarioexec.Held {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.held == nil {
		r.held = &scenarioexec.Held{}
	}
	return r.held
}

// handle tracks one in-flight execution for control delivery.
type handle struct {
	cancel context.CancelFunc
	pause  atomic.Bool
}

func (r *Runner) handle(id string) *handle {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.handles == nil {
		return nil
	}
	return r.handles[id]
}

// PauseRequested reports whether an in-flight run was asked to pause;
// scenarioexec consults it before each node.
func (r *Runner) PauseRequested(id string) bool {
	h := r.handle(id)
	return h != nil && h.pause.Load()
}

// RunOnce claims every pending run not already executing in this
// process and launches it in its own goroutine. Call it periodically
// (the serve loop ticks it alongside the job runner) and after enqueue
// or resume for prompt pickup.
func (r *Runner) RunOnce(ctx context.Context) error {
	// Recover runs a crashed runner abandoned — a claim that outlived
	// its lease is failed rather than parked forever.
	_, _ = r.Store.RecoverStale(ctx, r.staleClaimLease())
	runs, err := r.Store.List(ctx, scenariorun.RunFilter{Status: scenariorun.RunPending})
	if err != nil {
		return err
	}
	for _, run := range runs {
		r.launch(ctx, run)
	}
	return nil
}

func (r *Runner) staleClaimLease() time.Duration {
	if r.StaleClaimLease > 0 {
		return r.StaleClaimLease
	}
	if r.Policy.MaxDuration > 0 {
		return 2 * r.Policy.MaxDuration
	}
	return DefaultStaleClaimLease
}

func (r *Runner) maxConcurrent() int {
	if r.MaxConcurrent > 0 {
		return r.MaxConcurrent
	}
	return DefaultMaxConcurrent
}

func (r *Runner) launch(ctx context.Context, run scenariorun.Run) {
	r.mu.Lock()
	if r.handles == nil {
		r.handles = map[string]*handle{}
	}
	if _, exists := r.handles[run.ID]; exists {
		r.mu.Unlock()
		return
	}
	if len(r.handles) >= r.maxConcurrent() {
		r.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(context.Background())
	h := &handle{cancel: cancel}
	r.handles[run.ID] = h
	r.mu.Unlock()
	go r.execute(runCtx, run, h)
}

func (r *Runner) execute(ctx context.Context, run scenariorun.Run, h *handle) {
	defer func() {
		cancel := h.cancel
		r.mu.Lock()
		delete(r.handles, run.ID)
		r.mu.Unlock()
		cancel()
	}()
	compiled, err := r.Scenario(ctx, run)
	if err != nil {
		r.fail(run.ID, err)
		return
	}
	executor := scenarioexec.Executor{
		Runs:           r.Store,
		Sessions:       r.Sessions,
		Secrets:        r.Secrets,
		ArtifactDir:    r.ArtifactDir,
		PauseRequested: r.PauseRequested,
		Policy:         r.Policy,
		Held:           r.heldSessions(),
		// Heartbeat comfortably inside the stale-claim lease so a
		// healthy long run is never recovered as abandoned.
		ClaimHeartbeat: r.staleClaimLease() / 4,
	}
	if err := executor.Execute(ctx, run.ID, compiled); err != nil && !errors.Is(err, scenarioexec.ErrPaused) {
		r.fail(run.ID, err)
	}
}

// fail claims and finishes a run whose execution never got going (the
// scenario failed to resolve) or returned with its claim still held.
func (r *Runner) fail(runID string, cause error) {
	token := uid.New()
	ctx := context.Background()
	if claimed, err := r.Store.Claim(ctx, runID, token); err != nil || !claimed {
		return
	}
	_ = r.Store.Finish(ctx, runID, token, scenariorun.RunFailed, fmt.Sprintf("run failed: %s", cause))
}

// RequestPause asks an in-flight run to pause before its next node.
// Pausing is cooperative: the executor stores the pause before
// returning, so the run row flips only once execution actually stops.
func (r *Runner) RequestPause(ctx context.Context, runID string) error {
	run, found, err := r.Store.Get(ctx, runID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("run %q not found", runID)
	}
	if run.Status != scenariorun.RunRunning {
		return fmt.Errorf("run %q is %s", runID, run.Status)
	}
	if h := r.handle(runID); h != nil {
		h.pause.Store(true)
		return nil
	}
	return fmt.Errorf("run %q is not executing on this server", runID)
}

// Manual executes one human-driven browser op on a paused run's held
// session — the takeover surface. The op is recorded as a manual_action
// event so it lands in the run timeline marked as manual.
func (r *Runner) Manual(ctx context.Context, runID string, op scenarioexec.Manual) (string, error) {
	run, found, err := r.Store.Get(ctx, runID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("run %q not found", runID)
	}
	if run.Status != scenariorun.RunPaused {
		return "", fmt.Errorf("run %q is %s — takeover needs a paused run", runID, run.Status)
	}
	return r.heldSessions().Manual(ctx, runID, op)
}

// Resume moves a paused run back to pending for the next RunOnce. When
// the runner still holds the paused session the walk continues on the
// same browser at the recorded resume point; otherwise the resume point
// event lets the walk continue logically on a fresh session.
func (r *Runner) Resume(ctx context.Context, runID string) error {
	if err := r.Store.Resume(ctx, runID); err != nil {
		return err
	}
	return r.RunOnce(ctx)
}

// Stop cancels a pending or paused run immediately, or asks an
// in-flight run to stop: the executor observes the cancelled context
// and finishes the run cancelled itself.
func (r *Runner) Stop(ctx context.Context, runID string) error {
	run, found, err := r.Store.Get(ctx, runID)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("run %q not found", runID)
	}
	switch run.Status {
	case scenariorun.RunPaused:
		r.heldSessions().Close(runID)
		return r.Store.Cancel(ctx, runID)
	case scenariorun.RunPending:
		return r.Store.Cancel(ctx, runID)
	case scenariorun.RunRunning:
		if h := r.handle(runID); h != nil {
			h.cancel()
			return nil
		}
		return fmt.Errorf("run %q is not executing on this server", runID)
	default:
		return fmt.Errorf("run %q is already %s", runID, run.Status)
	}
}
