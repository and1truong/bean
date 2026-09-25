// Package scenarioexec executes a compiled appir.Scenario against the
// Semantic Browser API, recording every step into the durable run model
// (internal/scenariorun). It is the seam where a scenario run becomes a
// browser session: one execution claims the run, opens one adapter session,
// walks the node graph, and finishes deterministically — completed, failed,
// cancelled, or paused — with no transaction ever held across a browser call.
//
// Retry semantics follow the run model: every node attempt is a separate
// StepExecution row (NextStepAttempt), a failed step either follows its
// onFail edge or fails the run, and a failing step captures a screenshot
// artifact when a session is live.
package scenarioexec

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/browserapi"
	"github.com/beanruntime/bean/internal/scenario"
	"github.com/beanruntime/bean/internal/scenariorun"
	"github.com/beanruntime/bean/internal/uid"
)

// ErrPaused is returned when execution reaches a pause node: the run is left
// in the paused state and a later Execute call resumes it from the last step.
var ErrPaused = errors.New("scenarioexec: run paused for takeover")

// SecretResolver resolves a scenario `secret` reference into fill text at
// execution time; secrets never enter AppIR or run artifacts.
type SecretResolver func(ctx context.Context, name string) (string, error)

// SessionFactory opens one browser session for a run.
type SessionFactory func(ctx context.Context) (browserapi.Session, error)

// Policy is the host-level security boundary for browser runs. It is
// deployment configuration, not app metadata: the boundary must hold even
// for scenarios a buggy or hostile definition produces.
type Policy struct {
	// AllowedDomains lists host suffixes ("example.test" covers
	// "app.example.test") the browser may reach; empty allows any host.
	// Navigation to another host or scheme fails the step with a
	// policy_blocked event; the adapter additionally refuses the request.
	AllowedDomains []string
	// MaxDuration bounds total run wall time; zero means unbounded. A
	// run that overruns finishes cancelled, not failed.
	MaxDuration time.Duration
	// PauseOn lists node types that pause the run for approval before
	// they execute — resume is the approval decision. Each gate is
	// consumed once per run via a policy_pause event so a resumed walk
	// passes nodes already approved.
	PauseOn []string
}

// Executor runs one scenario against the browser and the run store.
type Executor struct {
	Runs     scenariorun.Store
	Sessions SessionFactory
	Secrets  SecretResolver
	// ArtifactDir roots the files captured as run evidence (screenshots,
	// DOM snapshots, traces). Artifact rows store the path relative to it.
	// Empty defaults to bean-artifacts under the OS temp dir.
	ArtifactDir string
	Now         func() time.Time
	// PauseRequested, when set, asks the walk to stop cooperatively
	// before the next node: the run pauses as if a pause node were hit.
	PauseRequested func(runID string) bool
	// Policy bounds navigation domains, run duration, and approval
	// gates for the execution.
	Policy Policy
	// Held, when set, keeps paused runs' live walkers so a human can
	// drive the open browser session during takeover and resume can
	// continue on the same page state. Nil disables takeover: pause
	// closes the session and resume re-opens a fresh one.
	Held *Held
	// ClaimHeartbeat renews the run's claim timestamp at this interval
	// while executing — claims are never otherwise renewed, so a
	// healthy long run must beat inside the runner's stale-claim lease
	// or RecoverStale can kill a live run. Zero disables renewal.
	ClaimHeartbeat time.Duration
}

const conditionCheckMillis = int64(2000)

// Execute claims the run, walks the scenario graph, and finishes the
// run. Pausing parks the walk instead of finishing: with Held set the
// live walker (open session, event pump, resolved secrets, page state)
// is retained so takeover can drive the browser and the next Execute
// continues from the recorded resume point on the same session. The
// caller decides what to do with ErrPaused.
func (e Executor) Execute(ctx context.Context, runID string, compiled appir.Scenario) error {
	token := uid.New()
	claimed, err := e.Runs.Claim(ctx, runID, token)
	if err != nil {
		return err
	}
	if !claimed {
		return fmt.Errorf("scenarioexec: run %s not claimable", runID)
	}
	if e.ClaimHeartbeat > 0 {
		stopHeartbeat := startClaimHeartbeat(e.Runs, runID, token, e.ClaimHeartbeat)
		defer stopHeartbeat()
	}
	var executor *walker
	if e.Held != nil {
		executor = e.Held.take(runID)
	}
	if executor == nil {
		sessionRow, err := e.Runs.OpenSession(ctx, scenariorun.Session{RunID: runID, Adapter: "playwright"})
		if err != nil {
			return e.failRun(ctx, runID, token, err)
		}
		session, err := e.Sessions(ctx)
		if err != nil {
			_ = e.Runs.UpdateSession(ctx, sessionRow.ID, scenariorun.SessionFailed, "", err.Error())
			return e.failRun(ctx, runID, token, err)
		}
		if err = e.Runs.UpdateSession(ctx, sessionRow.ID, scenariorun.SessionActive, "", ""); err != nil {
			return e.failRun(ctx, runID, token, err)
		}
		nodes := make(map[string]appir.ScenarioNode, len(compiled.Nodes))
		for _, node := range compiled.Nodes {
			nodes[node.ID] = node
		}
		executor = &walker{exec: e, runID: runID, sessionID: sessionRow.ID, session: session, scenario: compiled, nodes: nodes}
		// A resume without a held session (process restart, or no Held
		// configured) still honours the recorded resume point: the walk
		// continues at the boundary node on a fresh browser session —
		// page state is lost but steps do not re-run.
		executor.loadResumePoint(ctx)
	} else {
		executor.exec = e
		executor.scenario = compiled
	}
	for _, nodeType := range e.Policy.PauseOn {
		if executor.pauseOn == nil {
			executor.pauseOn = map[string]bool{}
		}
		executor.pauseOn[nodeType] = true
	}
	if len(executor.pauseOn) > 0 {
		executor.loadPausedNodes(ctx)
	}
	execCtx := ctx
	var deadline context.CancelFunc
	if e.Policy.MaxDuration > 0 {
		execCtx, deadline = context.WithTimeout(ctx, e.Policy.MaxDuration)
	}
	if !executor.pumpStarted {
		executor.events.Add(1)
		executor.pumpStarted = true
		go executor.pumpEvents()
	}
	paused, failure := executor.walk(execCtx)
	if deadline != nil {
		deadline()
	}
	if failure != "" && execCtx.Err() == nil {
		executor.captureTrace(context.Background())
	}
	if paused {
		if e.Held != nil {
			e.Held.put(runID, executor)
		}
		if pauseErr := e.Runs.Pause(ctx, runID, token); pauseErr != nil {
			// Pause could not be recorded: the run cannot stay parked on
			// a live session, so take the walker back out of Held and
			// close it before finishing failed.
			if e.Held != nil {
				e.Held.take(runID)
			}
			_ = executor.session.Close(context.Background())
			executor.events.Wait()
			return e.failRun(ctx, runID, token, pauseErr)
		}
		if e.Held == nil {
			_ = executor.session.Close(context.Background())
			executor.events.Wait()
		}
		return ErrPaused
	}
	// Close before finishing so the event pump drains every late console and
	// network observation into the log ahead of the terminal event.
	_ = executor.session.Close(context.Background())
	executor.events.Wait()
	if execCtx.Err() != nil {
		cause := "cancelled"
		if e.Policy.MaxDuration > 0 && errors.Is(execCtx.Err(), context.DeadlineExceeded) {
			cause = fmt.Sprintf("run exceeded time limit %s", e.Policy.MaxDuration)
		}
		return e.Runs.Finish(context.WithoutCancel(ctx), runID, token, scenariorun.RunCancelled, cause)
	}
	if failure == "" {
		return e.Runs.Finish(ctx, runID, token, scenariorun.RunCompleted, "")
	}
	if err := e.Runs.Finish(ctx, runID, token, scenariorun.RunFailed, bounded(failure, scenariorun.MaxErrorRunes)); err != nil {
		return err
	}
	return errors.New(failure)
}

// startClaimHeartbeat renews the run's claim every interval until the
// returned cancel fires — a healthy long execution keeps claimed_at
// fresh so stale recovery only kills genuinely abandoned claims.
func startClaimHeartbeat(store scenariorun.Store, runID, token string, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = store.TouchClaim(ctx, runID, token)
			}
		}
	}()
	return cancel
}

// failRun finishes the run failed. The store write runs on a detached
// context: a cancelled run context (a stop landing mid-setup) must not
// take the terminal row down with it — and when the context was
// cancelled, the run reports cancelled rather than failed.
func (e Executor) failRun(ctx context.Context, runID, token string, cause error) error {
	status := scenariorun.RunFailed
	if ctx.Err() != nil {
		status = scenariorun.RunCancelled
	}
	if finishErr := e.Runs.Finish(context.WithoutCancel(ctx), runID, token, status, bounded(cause.Error(), scenariorun.MaxErrorRunes)); finishErr != nil {
		return finishErr
	}
	return cause
}

func jsonString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func bounded(value string, max int) string {
	runes := []rune(value)
	if len(runes) > max {
		return string(runes[:max])
	}
	return value
}

type walker struct {
	exec        Executor
	runID       string
	sessionID   string
	session     browserapi.Session
	scenario    appir.Scenario
	lastPass    bool
	snapshot    *browserapi.Snapshot
	nodes       map[string]appir.ScenarioNode
	events      sync.WaitGroup
	pumpStarted bool
	pauseOn     map[string]bool
	paused      map[string]bool
	resumeNode  string
	resumeDone  bool
	opMu        sync.Mutex
	manualDone  bool
	secretMu    sync.RWMutex
	secrets     []string
}

// Held retains paused runs' walkers while a human takes over the
// browser. It is process-local: the live session cannot outlive the
// runner process, while the durable resume_point event still lets a
// restarted runner continue the walk logically on a fresh session.
type Held struct {
	mu      sync.Mutex
	walkers map[string]*walker
}

func (h *Held) take(runID string) *walker {
	h.mu.Lock()
	w := h.walkers[runID]
	delete(h.walkers, runID)
	h.mu.Unlock()
	if w != nil {
		// Wait out any in-flight manual op, then bar further takeover
		// ops — the resumed walk now drives the session.
		w.opMu.Lock()
		w.manualDone = true
		w.opMu.Unlock()
	}
	return w
}

func (h *Held) put(runID string, w *walker) {
	// The walk parked, so the session is idle again — takeover ops may
	// resume. Runs before the map insert so a resumed run can never
	// observe manualDone=false while executing.
	w.opMu.Lock()
	w.manualDone = false
	w.opMu.Unlock()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.walkers == nil {
		h.walkers = map[string]*walker{}
	}
	h.walkers[runID] = w
}

// Close drops the run's held walker, closing the browser session and
// draining its event pump. Used when a parked run is cancelled.
func (h *Held) Close(runID string) {
	h.mu.Lock()
	w := h.walkers[runID]
	delete(h.walkers, runID)
	h.mu.Unlock()
	if w == nil {
		return
	}
	// Let an in-flight manual op finish, then bar new ones before
	// tearing the session down.
	w.opMu.Lock()
	w.manualDone = true
	w.opMu.Unlock()
	_ = w.session.Close(context.Background())
	w.events.Wait()
	_ = w.exec.Runs.UpdateSession(context.Background(), w.sessionID, scenariorun.SessionClosed, "", "")
}

// Manual executes one human-driven browser op on a held (paused) run's
// live session and records it as a manual_action event.
func (h *Held) Manual(ctx context.Context, runID string, op Manual) (string, error) {
	h.mu.Lock()
	w := h.walkers[runID]
	h.mu.Unlock()
	if w == nil {
		return "", fmt.Errorf("scenarioexec: run %s has no live browser session (restart while paused?)", runID)
	}
	return w.Manual(ctx, op)
}

// pumpEvents pipes adapter page observations into the run log until the
// session's event channel closes (session end). Every event is its own
// short transaction, so streaming never holds a lock across a browser call.
func (w *walker) pumpEvents() {
	defer w.events.Done()
	for event := range w.session.Events() {
		kind := scenariorun.EventConsole
		if event.Kind == browserapi.EventRequest || event.Kind == browserapi.EventResponse || event.Kind == browserapi.EventRequestFailed || event.Kind == browserapi.EventRequestBlocked {
			kind = scenariorun.EventNetwork
		}
		_ = w.exec.Runs.RecordEvent(context.Background(), w.runID, "", kind, w.scrub(string(event.Data)))
	}
}

// walk executes the scenario graph until it reaches a dead end
// (completed), a failure, or a pause. A resumed walk starts at the
// recorded resume point — the node the pause parked on — rather than
// the scenario start. It reports (true, "") on a pause, (false, "") on
// completion, or (false, message) on failure.
func (w *walker) walk(ctx context.Context) (bool, string) {
	// A pause that parked past the last node resumes to completion —
	// every step already ran.
	if w.resumeDone {
		return false, ""
	}
	nodeID := w.resumeNode
	if nodeID == "" {
		nodeID = w.scenario.Start
	}
	visited := 0
	for nodeID != "" {
		visited++
		if w.exec.PauseRequested != nil && w.exec.PauseRequested(w.runID) {
			return w.pauseAt(ctx, nodeID), ""
		}
		if visited > scenariorun.MaxStepsPerRun {
			return false, fmt.Sprintf("step bound %d exceeded", scenariorun.MaxStepsPerRun)
		}
		node, found := w.nodes[nodeID]
		if !found {
			return false, fmt.Sprintf("node %q missing", nodeID)
		}
		if w.pauseOn[node.Type] && !w.paused[node.ID] {
			_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventPolicyPause, fmt.Sprintf(`{"node":%q,"type":%q}`, node.ID, node.Type))
			return w.pauseAt(ctx, nodeID), ""
		}
		next, err := w.executeNode(ctx, node)
		if errors.Is(err, ErrPaused) {
			// A pause node completes its step at the boundary; resume
			// continues at its outgoing edge.
			if node.Type == scenario.NodePause {
				return w.pauseAt(ctx, node.Next), ""
			}
			return w.pauseAt(ctx, nodeID), ""
		}
		if err != nil {
			return false, err.Error()
		}
		nodeID = next
	}
	return false, ""
}

// pauseAt parks the walk on nodeID: the node is the resume point — what
// executes next after resume — recorded durably so a restarted process
// can still continue the walk logically.
func (w *walker) pauseAt(ctx context.Context, nodeID string) bool {
	w.resumeNode = nodeID
	w.resumeDone = nodeID == ""
	_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventResumePoint, fmt.Sprintf(`{"resume_node":%q}`, nodeID))
	return true
}

// loadResumePoint restores the boundary a previous walk paused on.
// Only a fresh walker calls it; a held walker already carries its
// in-memory resume point.
func (w *walker) loadResumePoint(ctx context.Context) {
	events, err := w.exec.Runs.Events(ctx, w.runID, 0)
	if err != nil {
		return
	}
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Kind != scenariorun.EventResumePoint {
			continue
		}
		var payload struct {
			ResumeNode string `json:"resume_node"`
		}
		if json.Unmarshal([]byte(events[i].Payload), &payload) == nil {
			w.resumeNode = payload.ResumeNode
			w.resumeDone = payload.ResumeNode == ""
		}
		break
	}
	// Rehydrate the last-step outcome so the first branch evaluated
	// after resume sees the status of the step that actually ran last,
	// not the fresh walker's default.
	steps, err := w.exec.Runs.Steps(ctx, w.runID)
	if err != nil {
		return
	}
	for i := len(steps) - 1; i >= 0; i-- {
		switch steps[i].Status {
		case scenariorun.StepPassed:
			w.lastPass = true
			return
		case scenariorun.StepFailed:
			w.lastPass = false
			return
		}
	}
}

func (w *walker) timeout(node appir.ScenarioNode) time.Duration {
	seconds := node.TimeoutSeconds
	if seconds <= 0 {
		seconds = scenario.DefaultTimeoutSeconds
	}
	if seconds > scenario.MaxTimeoutSeconds {
		seconds = scenario.MaxTimeoutSeconds
	}
	return time.Duration(seconds) * time.Second
}

func (w *walker) executeNode(ctx context.Context, node appir.ScenarioNode) (string, error) {
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	stepCtx, cancel := context.WithTimeout(ctx, w.timeout(node))
	defer cancel()
	attempt, err := w.exec.Runs.NextStepAttempt(ctx, w.runID, node.ID)
	if err != nil {
		return "", err
	}
	step, err := w.exec.Runs.StartStep(ctx, scenariorun.StepExecution{
		RunID: w.runID, SessionID: w.sessionID, NodeID: node.ID, Attempt: attempt,
	})
	if err != nil {
		return "", err
	}
	output, runErr := w.runNode(stepCtx, node)
	if node.Type == scenario.NodePause {
		if runErr != nil {
			_ = w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepFailed, "", runErr.Error())
		} else {
			_ = w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepPassed, "paused", "")
		}
		return "", ErrPaused
	}
	if runErr != nil {
		w.lastPass = false
		if err := w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepFailed, "", bounded(runErr.Error(), scenariorun.MaxErrorRunes)); err != nil {
			return "", err
		}
		w.captureArtifact(ctx, step.ID)
		if node.OnFail != "" {
			return node.OnFail, nil
		}
		return "", fmt.Errorf("step %s failed: %w", node.ID, runErr)
	}
	if node.Type != scenario.NodeBranch {
		w.lastPass = true
	}
	if err := w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepPassed, bounded(output, scenariorun.MaxOutputBytes), ""); err != nil {
		return "", err
	}
	switch node.Type {
	case scenario.NodeBranch:
		return w.branchTarget(ctx, node)
	case scenario.NodeLoop:
		return w.loopTarget(ctx, node)
	default:
		return node.Next, nil
	}
}

func (w *walker) runNode(ctx context.Context, node appir.ScenarioNode) (string, error) {
	switch node.Type {
	case scenario.NodeNavigate:
		if host, allowed := w.hostAllowed(node.URL); !allowed {
			_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventPolicyBlocked, fmt.Sprintf(`{"url":%q,"host":%q}`, node.URL, host))
			return "", fmt.Errorf("navigation to %q is not in the browser policy allowlist", node.URL)
		}
		result, err := w.session.Open(ctx, node.URL)
		w.snapshot = nil
		return fmt.Sprintf(`{"url":%q}`, result.URL), err
	case scenario.NodeClick:
		ref, err := w.resolveRefFor(ctx, node.Ref, clickRoles, "clickable")
		if err != nil {
			return "", err
		}
		_, err = w.session.Click(ctx, ref)
		w.snapshot = nil
		return "{}", err
	case scenario.NodeFill:
		ref, err := w.resolveRefFor(ctx, node.Ref, fillRoles, "fillable")
		if err != nil {
			return "", err
		}
		text := node.Text
		if node.Secret != "" {
			if w.exec.Secrets == nil {
				return "", fmt.Errorf("node %s requires a secret resolver", node.ID)
			}
			resolved, err := w.exec.Secrets(ctx, node.Secret)
			if err != nil {
				return "", err
			}
			text = resolved
			w.registerSecret(resolved)
			_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventSecretUsed, fmt.Sprintf(`{"node":%q,"name":%q}`, node.ID, node.Secret))
		}
		_, err = w.session.Fill(ctx, ref, text)
		return "{}", err
	case scenario.NodeSelect:
		ref, err := w.resolveRefFor(ctx, node.Ref, selectRoles, "selectable")
		if err != nil {
			return "", err
		}
		_, err = w.session.Select(ctx, ref, node.Value)
		return "{}", err
	case scenario.NodePress:
		_, err := w.session.Press(ctx, node.Key)
		w.snapshot = nil
		return "{}", err
	case scenario.NodeWait:
		timeout := w.timeout(node)
		condition, err := w.condition(ctx, node.Condition, node.Ref, node.Text, timeout)
		if err != nil {
			return "", err
		}
		result, err := w.session.Wait(ctx, condition)
		if err != nil {
			return "", fmt.Errorf("wait for %s %q was not satisfied within %s (%w)", node.Condition, node.Text, timeout, err)
		}
		if !result.Met {
			return "", fmt.Errorf("condition %q not met", node.Condition)
		}
		return "{}", nil
	case scenario.NodeAssert:
		return w.assert(ctx, node)
	case scenario.NodeExtract:
		return w.extract(ctx, node)
	case scenario.NodeBranch:
		return "{}", nil // target chosen by executeNode
	case scenario.NodeLoop:
		return "{}", nil // target chosen by executeNode
	case scenario.NodePause:
		return "", nil
	case scenario.NodeScript, scenario.NodeAPICall:
		return "", fmt.Errorf("node type %q not implemented in this slice", node.Type)
	default:
		return "", fmt.Errorf("unknown node type %q", node.Type)
	}
}

// actionableRoles bounds the snapshot roles an interaction ref may land on:
// a name like "Sign in" can match a heading or landmark above the actual
// control, and clicking the wrong ancestor passes the step without doing
// anything — refs for click/fill/select resolve only against compatible
// actionable roles, never against headings, landmarks, or images.
var (
	clickRoles  = []string{"button", "link", "checkbox", "radio", "option", "menuitem", "menuitemcheckbox", "menuitemradio", "tab", "switch", "treeitem", "gridcell", "textbox", "searchbox", "combobox", "listbox", "slider", "spinbutton"}
	fillRoles   = []string{"textbox", "searchbox", "spinbutton", "combobox"}
	selectRoles = []string{"combobox", "listbox"}
)

// resolveRef maps a scenario `ref` (semantic element name or raw ref id) onto
// a live browserapi.Ref from the current snapshot.
func (w *walker) resolveRef(ctx context.Context, target string) (browserapi.Ref, error) {
	return w.resolveRefFor(ctx, target, nil, "usable")
}

// resolveRefFor resolves target like resolveRef but, when roles is non-empty,
// restricts matches to actionable snapshot nodes of those roles and rejects
// ambiguous or disabled targets. verb names the interaction for errors.
func (w *walker) resolveRefFor(ctx context.Context, target string, roles []string, verb string) (browserapi.Ref, error) {
	if err := w.refreshSnapshot(ctx); err != nil {
		return browserapi.Ref{}, err
	}
	compatible := func(node browserapi.Node) bool {
		if roles == nil {
			return true
		}
		if node.Disabled {
			return false
		}
		for _, role := range roles {
			if node.Role == role {
				return true
			}
		}
		return false
	}
	nodes := w.snapshot.Nodes
	// Raw ref id (e.g. "e12") resolves positionally.
	if len(target) > 1 && target[0] == 'e' {
		if ref, err := w.snapshot.Resolve(target); err == nil {
			index, _ := strconv.Atoi(strings.TrimPrefix(target, "e"))
			if resolved := nodes[index-1]; !compatible(resolved) {
				if resolved.Disabled {
					return browserapi.Ref{}, fmt.Errorf("ref %s (%s %q) is disabled", target, resolved.Role, resolved.Name)
				}
				return browserapi.Ref{}, fmt.Errorf("ref %s resolves to %s %q — not a %s element", target, resolved.Role, resolved.Name, verb)
			}
			return ref, nil
		}
	}
	var exact, partial, wrongRole []browserapi.Node
	for _, node := range nodes {
		if strings.EqualFold(node.Name, target) {
			if compatible(node) {
				exact = append(exact, node)
			} else {
				wrongRole = append(wrongRole, node)
			}
			continue
		}
		if strings.Contains(strings.ToLower(node.Name), strings.ToLower(target)) && compatible(node) {
			partial = append(partial, node)
		}
	}
	matches := exact
	if len(matches) == 0 {
		matches = partial
	}
	switch len(matches) {
	case 1:
		return w.snapshot.Resolve(matches[0].Ref)
	case 0:
		if len(wrongRole) > 0 {
			first := wrongRole[0]
			return browserapi.Ref{}, fmt.Errorf("ref %q matched %s %q — not a %s element", target, first.Role, first.Name, verb)
		}
		return browserapi.Ref{}, fmt.Errorf("no element matches ref %q", target)
	default:
		candidates := make([]string, 0, len(matches))
		for _, node := range matches {
			candidates = append(candidates, fmt.Sprintf("%s %s %q", node.Ref, node.Role, node.Name))
		}
		return browserapi.Ref{}, fmt.Errorf("ref %q is ambiguous between %s", target, strings.Join(candidates, ", "))
	}
}

func (w *walker) refreshSnapshot(ctx context.Context) error {
	if w.snapshot != nil {
		return nil
	}
	snapshot, err := w.session.Snapshot(ctx)
	if err != nil {
		return err
	}
	w.snapshot = &snapshot
	w.recordSnapshot(ctx)
	return nil
}

// recordSnapshot logs the normalized page view; the encoded tree rides in
// the payload while it fits the event bound, else only metadata is kept.
// Form values rendered into the tree are scrubbed of resolved secrets.
func (w *walker) recordSnapshot(ctx context.Context) {
	encoded := w.scrub(w.snapshot.Encode())
	payload := fmt.Sprintf(`{"snapshot":%q,"nodes":%d,"tree":%s}`, w.snapshot.ID, len(w.snapshot.Nodes), jsonString(encoded))
	if len(payload) > scenariorun.MaxPayloadBytes {
		payload = fmt.Sprintf(`{"snapshot":%q,"nodes":%d}`, w.snapshot.ID, len(w.snapshot.Nodes))
	}
	_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventBrowserSnapshot, payload)
}

func (w *walker) condition(ctx context.Context, kind, ref, text string, timeout time.Duration) (browserapi.Condition, error) {
	condition := browserapi.Condition{
		Kind: kind, Text: text,
		TimeoutMillis: timeout.Milliseconds(),
	}
	if browserapi.ConditionNeedsRef(kind) {
		resolved, err := w.resolveRef(ctx, ref)
		if err != nil {
			return browserapi.Condition{}, err
		}
		condition.Ref = resolved
	}
	return condition, condition.Validate()
}

func (w *walker) assert(ctx context.Context, node appir.ScenarioNode) (string, error) {
	if node.Assertion == scenario.AssertRefText {
		ref, err := w.resolveRef(ctx, node.Ref)
		if err != nil {
			return "", err
		}
		extracted, err := w.session.Extract(ctx, ref, browserapi.ExtractText, "")
		if err != nil {
			return "", err
		}
		met := strings.Contains(extracted.Value, node.Text)
		w.recordAssertion(ctx, node, met, fmt.Sprintf(`{"actual":%q}`, extracted.Value))
		if !met {
			return "", fmt.Errorf("ref %s text %q does not contain %q", node.Ref, w.scrub(extracted.Value), node.Text)
		}
		return fmt.Sprintf(`{"actual":%q}`, w.scrub(extracted.Value)), nil
	}
	timeout := w.timeout(node)
	condition, err := w.condition(ctx, node.Assertion, node.Ref, node.Text, timeout)
	if err != nil {
		return "", err
	}
	result, err := w.session.Wait(ctx, condition)
	if err != nil {
		// A failed check is an outcome, not an infrastructure error: record the
		// assertion with what was expected and what the wait observed so the
		// run report and event explorer carry it — never just "context
		// deadline exceeded". The write detaches from the expired wait
		// context so the event survives the timeout that produced it.
		expected := fmt.Sprintf("%s %q", node.Assertion, node.Text)
		w.recordAssertion(context.WithoutCancel(ctx), node, false, fmt.Sprintf(`{"expected":%q,"error":%q}`, expected, err.Error()))
		return "", fmt.Errorf("assertion not met: %s was not satisfied within %s (%w)", expected, timeout, err)
	}
	w.recordAssertion(ctx, node, result.Met, "{}")
	if !result.Met {
		return "", fmt.Errorf("assertion %q not met", node.Assertion)
	}
	return "{}", nil
}

func (w *walker) recordAssertion(ctx context.Context, node appir.ScenarioNode, met bool, detail string) {
	_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventAssertion,
		fmt.Sprintf(`{"node":%q,"assertion":%q,"met":%t,"detail":%s}`, node.ID, node.Assertion, met, w.scrub(detail)))
}

// loadPausedNodes marks policy-gated nodes that already paused this run
// once: a policy_pause event is the durable consent record, so a resumed
// walk passes a node its approval already covered.
func (w *walker) loadPausedNodes(ctx context.Context) {
	w.paused = map[string]bool{}
	events, err := w.exec.Runs.Events(ctx, w.runID, 0)
	if err != nil {
		return
	}
	for _, event := range events {
		if event.Kind != scenariorun.EventPolicyPause {
			continue
		}
		var payload struct {
			Node string `json:"node"`
		}
		if json.Unmarshal([]byte(event.Payload), &payload) == nil {
			w.paused[payload.Node] = true
		}
	}
}

// hostAllowed applies Policy.AllowedDomains to a navigation target.
// Only http(s) URLs ever pass — file:, javascript:, and data: schemes
// are refused outright — and with an allowlist configured the host
// must equal or sit beneath a listed domain.
func (w *walker) hostAllowed(raw string) (string, bool) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return host, false
	}
	if len(w.exec.Policy.AllowedDomains) == 0 {
		return host, true
	}
	for _, domain := range w.exec.Policy.AllowedDomains {
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return host, true
		}
	}
	return host, false
}

// registerSecret adds a resolved secret to the scrub set so later event
// payloads and artifacts cannot carry it.
func (w *walker) registerSecret(value string) {
	w.secretMu.Lock()
	defer w.secretMu.Unlock()
	w.secrets = append(w.secrets, value)
}

// scrub replaces resolved secret values with a mask. Short values are
// left alone — masking a 3-character string would mangle ordinary text
// while still leaking nothing of value.
func (w *walker) scrub(text string) string {
	w.secretMu.RLock()
	defer w.secretMu.RUnlock()
	for _, secret := range w.secrets {
		if len(secret) >= 4 {
			text = strings.ReplaceAll(text, secret, "****")
		}
	}
	return text
}

func (w *walker) extract(ctx context.Context, node appir.ScenarioNode) (string, error) {
	ref, err := w.resolveRef(ctx, node.Ref)
	if err != nil {
		return "", err
	}
	// `attribute` selects the extraction kind (text/value/attribute),
	// `name` is the HTML attribute read for the attribute kind
	// (data-testid, aria-label, ...), and `as` is purely the result
	// binding name.
	kind := node.Attribute
	if kind == "" {
		kind = browserapi.ExtractText
	}
	attribute := ""
	if kind == browserapi.ExtractAttribute {
		attribute = node.Name
	}
	extracted, err := w.session.Extract(ctx, ref, kind, attribute)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"as":%q,"value":%q}`, node.As, w.scrub(extracted.Value)), nil
}

// branchTarget evaluates the node's branch edges in order; the first true
// edge wins, else `next` — the same semantics the compiler validated.
func (w *walker) branchTarget(ctx context.Context, node appir.ScenarioNode) (string, error) {
	for _, branch := range node.Branches {
		met, err := w.evaluate(ctx, branch.Condition, branch.Ref, branch.Text)
		if err != nil {
			return "", err
		}
		if met {
			return branch.Next, nil
		}
	}
	return node.Next, nil
}

// loopTarget runs the body subgraph until the until-condition is met or
// MaxIterations is reached, then follows `next`.
func (w *walker) loopTarget(ctx context.Context, node appir.ScenarioNode) (string, error) {
	iterations := node.MaxIterations
	if iterations <= 0 {
		iterations = scenario.DefaultIterations
	}
	for iteration := 0; iteration < iterations; iteration++ {
		met, err := w.evaluate(ctx, node.Until, node.Ref, node.Text)
		if err != nil {
			return "", err
		}
		if met {
			return node.Next, nil
		}
		next := node.Body
		for next != "" {
			body, found := w.nodes[next]
			if !found {
				return "", fmt.Errorf("loop %s body node %q missing", node.ID, next)
			}
			// Body nodes honour the same boundaries as the outer walk:
			// pause requests and PauseOn-gated types park the run at the
			// loop node — resume then re-runs the loop from its start.
			if w.exec.PauseRequested != nil && w.exec.PauseRequested(w.runID) {
				return "", ErrPaused
			}
			if w.pauseOn[body.Type] && !w.paused[body.ID] {
				_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventPolicyPause, fmt.Sprintf(`{"node":%q,"type":%q}`, body.ID, body.Type))
				return "", ErrPaused
			}
			next, err = w.executeNode(ctx, body)
			if err != nil {
				return "", err
			}
		}
	}
	met, err := w.evaluate(ctx, node.Until, node.Ref, node.Text)
	if err != nil {
		return "", err
	}
	if !met {
		return "", fmt.Errorf("loop %s exhausted %d iterations", node.ID, iterations)
	}
	return node.Next, nil
}

// evaluate checks one branch/loop condition without waiting.
func (w *walker) evaluate(ctx context.Context, kind, ref, text string) (bool, error) {
	switch kind {
	case scenario.BranchLastStepPassed:
		return w.lastPass, nil
	case scenario.BranchLastStepFailed:
		return !w.lastPass, nil
	case scenario.BranchURLEquals, scenario.BranchURLContains, scenario.BranchTextPresent,
		scenario.BranchRefVisible, scenario.BranchRefHidden:
		condition, err := w.condition(ctx, kind, ref, text, time.Duration(conditionCheckMillis)*time.Millisecond)
		if err != nil {
			return false, nil // an unresolvable ref means the condition is not met
		}
		result, err := w.session.Wait(ctx, condition)
		if err != nil {
			return false, nil // timeout/unknown-ref means not met
		}
		return result.Met, nil
	default:
		return false, fmt.Errorf("unsupported condition %q", kind)
	}
}

// ArtifactRoot is the directory run evidence files live under; artifact
// rows store paths relative to it so a later file server can resolve them.
func (e Executor) ArtifactRoot() string {
	if e.ArtifactDir != "" {
		return e.ArtifactDir
	}
	return filepath.Join(os.TempDir(), "bean-artifacts")
}

// writeArtifact persists bytes as a run evidence file and records the row.
func (w *walker) writeArtifact(ctx context.Context, stepID, kind, contentType, ext string, bytes []byte) {
	if len(bytes) == 0 {
		return
	}
	dir := filepath.Join(w.exec.ArtifactRoot(), w.runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	name := fmt.Sprintf("%s-%s.%s", stepID, kind, ext)
	if stepID == "" {
		name = fmt.Sprintf("run-%s.%s", kind, ext)
	}
	rel := filepath.Join(w.runID, name)
	if err := os.WriteFile(filepath.Join(dir, name), bytes, 0o644); err != nil {
		return
	}
	_, _ = w.exec.Runs.RecordArtifact(ctx, scenariorun.Artifact{
		RunID: w.runID, StepID: stepID,
		Kind: kind, ContentType: contentType,
		Size: int64(len(bytes)), Ref: filepath.ToSlash(rel),
	})
}

// captureArtifact persists the diagnosis bundle for a failed step: the
// viewport screenshot and a fresh DOM snapshot of the post-failure state.
func (w *walker) captureArtifact(ctx context.Context, stepID string) {
	capture, err := w.session.Screenshot(ctx)
	if err == nil {
		w.writeArtifact(ctx, stepID, scenariorun.ArtifactScreenshot, capture.ContentType, "png", capture.Bytes)
	}
	if snapshot, err := w.session.Snapshot(ctx); err == nil {
		w.writeArtifact(ctx, stepID, scenariorun.ArtifactDOM, "text/plain", "txt", []byte(w.scrub(snapshot.Encode())))
	}
}

// captureTrace stops the session's trace on a failed run and stores the zip.
func (w *walker) captureTrace(ctx context.Context) {
	capture, err := w.session.Trace(ctx)
	if err == nil {
		w.writeArtifact(ctx, "", scenariorun.ArtifactTrace, capture.ContentType, "zip", capture.Bytes)
	}
}

// Manual op names a human may drive on a held (paused) run's browser
// session during takeover. The set mirrors the primitive node types —
// control-flow node types (branch/loop/pause) have no manual form.
type Manual struct {
	Op             string `json:"op"`
	Ref            string `json:"ref,omitempty"`
	URL            string `json:"url,omitempty"`
	Text           string `json:"text,omitempty"`
	Secret         string `json:"secret,omitempty"`
	Key            string `json:"key,omitempty"`
	Value          string `json:"value,omitempty"`
	Condition      string `json:"condition,omitempty"`
	Attribute      string `json:"attribute,omitempty"`
	As             string `json:"as,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`
}

const (
	ManualNavigate   = "navigate"
	ManualClick      = "click"
	ManualFill       = "fill"
	ManualSelect     = "select"
	ManualPress      = "press"
	ManualWait       = "wait"
	ManualSnapshot   = "snapshot"
	ManualScreenshot = "screenshot"
	ManualExtract    = "extract"
)

// Manual executes one human-driven op on the held session and records it
// as a manual_action event in the run log — the takeover audit trail and
// the input #34 (exploration → saved test) later replays. Manual actions
// are events, never StepExecution rows; a manual op moves the browser,
// so scenario assertions after resume evaluate the state the human left.
func (w *walker) Manual(ctx context.Context, op Manual) (string, error) {
	w.opMu.Lock()
	defer w.opMu.Unlock()
	if w.manualDone {
		return "", fmt.Errorf("scenarioexec: run %s resumed or closed — the session is no longer held", w.runID)
	}
	result, name, err := w.manualOp(ctx, op)
	outcome := result
	if err != nil {
		outcome = err.Error()
	}
	// Bound the result so the assembled payload always fits
	// MaxPayloadBytes — a full-size snapshot result would otherwise
	// push the event past the store limit and drop the audit record
	// entirely. Escaping inflates the encoded result, so cap the raw
	// outcome well below the limit and fall back to a stub if the
	// assembled payload is still oversized.
	encoded := jsonString(bounded(w.scrub(outcome), scenariorun.MaxPayloadBytes/8))
	payload := fmt.Sprintf(`{"op":%q,"ref":%q,"name":%q,"url":%q,"text":%q,"secret":%q,"value":%q,"key":%q,"condition":%q,"as":%q,"attribute":%q,"ok":%t,"result":%s}`, op.Op, op.Ref, name, op.URL, op.Text, op.Secret, op.Value, op.Key, op.Condition, op.As, op.Attribute, err == nil, encoded)
	if len(payload) > scenariorun.MaxPayloadBytes {
		payload = fmt.Sprintf(`{"op":%q,"ref":%q,"name":%q,"ok":%t,"result":"truncated"}`, op.Op, op.Ref, name, err == nil)
	}
	_ = w.exec.Runs.RecordEvent(context.Background(), w.runID, "", scenariorun.EventManualAction, payload)
	return result, err
}

// elementName maps a resolved snapshot ref back to the element's semantic
// name — the stable identifier a saved scenario replays by, since raw
// snapshot refs go stale between sessions.
func (w *walker) elementName(ref browserapi.Ref) string {
	if w.snapshot == nil {
		return ""
	}
	for _, node := range w.snapshot.Nodes {
		if node.Ref == ref.ID {
			return node.Name
		}
	}
	return ""
}

// manualOp dispatches the takeover primitive, returning the JSON result and
// the semantic name of the element touched (for the run log). The same
// policy boundary applies as for scenario nodes: navigations check the
// allowlist, and a secret fill resolves through the configured resolver
// (audit event recorded, value scrubbed from logs).
func (w *walker) manualOp(ctx context.Context, op Manual) (string, string, error) {
	switch op.Op {
	case ManualNavigate:
		host, allowed := w.hostAllowed(op.URL)
		if !allowed {
			_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventPolicyBlocked, fmt.Sprintf(`{"url":%q,"host":%q}`, op.URL, host))
			return "", "", fmt.Errorf("navigation to %q is not in the browser policy allowlist", op.URL)
		}
		result, err := w.session.Open(ctx, op.URL)
		w.snapshot = nil
		return fmt.Sprintf(`{"url":%q}`, result.URL), "", err
	case ManualClick:
		ref, err := w.resolveRef(ctx, op.Ref)
		if err != nil {
			return "", "", err
		}
		name := w.elementName(ref)
		_, err = w.session.Click(ctx, ref)
		w.snapshot = nil
		return "{}", name, err
	case ManualFill:
		ref, err := w.resolveRef(ctx, op.Ref)
		if err != nil {
			return "", "", err
		}
		name := w.elementName(ref)
		text := op.Text
		if op.Secret != "" {
			if w.exec.Secrets == nil {
				return "", "", fmt.Errorf("manual fill with secret %q requires a secret resolver", op.Secret)
			}
			resolved, err := w.exec.Secrets(ctx, op.Secret)
			if err != nil {
				return "", "", err
			}
			text = resolved
			w.registerSecret(resolved)
			_ = w.exec.Runs.RecordEvent(ctx, w.runID, "", scenariorun.EventSecretUsed, fmt.Sprintf(`{"name":%q,"manual":true}`, op.Secret))
		}
		_, err = w.session.Fill(ctx, ref, text)
		return "{}", name, err
	case ManualSelect:
		ref, err := w.resolveRef(ctx, op.Ref)
		if err != nil {
			return "", "", err
		}
		name := w.elementName(ref)
		_, err = w.session.Select(ctx, ref, op.Value)
		return "{}", name, err
	case ManualPress:
		_, err := w.session.Press(ctx, op.Key)
		w.snapshot = nil
		return "{}", "", err
	case ManualWait:
		name := ""
		if op.Ref != "" {
			if ref, rerr := w.resolveRef(ctx, op.Ref); rerr == nil {
				name = w.elementName(ref)
			}
		}
		condition, err := w.condition(ctx, op.Condition, op.Ref, op.Text, w.timeout(appir.ScenarioNode{TimeoutSeconds: op.TimeoutSeconds}))
		if err != nil {
			return "", "", err
		}
		result, err := w.session.Wait(ctx, condition)
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf(`{"met":%t,"url":%q}`, result.Met, result.URL), name, nil
	case ManualSnapshot:
		snapshot, err := w.session.Snapshot(ctx)
		if err != nil {
			return "", "", err
		}
		w.snapshot = &snapshot
		w.recordSnapshot(ctx)
		return fmt.Sprintf(`{"snapshot":%s}`, jsonString(w.scrub(snapshot.Encode()))), "", nil
	case ManualScreenshot:
		capture, err := w.session.Screenshot(ctx)
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf(`{"content_type":%q,"png":%q}`, capture.ContentType, base64.StdEncoding.EncodeToString(capture.Bytes)), "", nil
	case ManualExtract:
		ref, err := w.resolveRef(ctx, op.Ref)
		if err != nil {
			return "", "", err
		}
		name := w.elementName(ref)
		kind := op.As
		if kind == "" {
			kind = browserapi.ExtractText
		}
		extracted, err := w.session.Extract(ctx, ref, kind, op.Attribute)
		if err != nil {
			return "", name, err
		}
		return fmt.Sprintf(`{"as":%q,"value":%q}`, kind, w.scrub(extracted.Value)), name, nil
	default:
		return "", "", fmt.Errorf("unsupported manual op %q", op.Op)
	}
}
