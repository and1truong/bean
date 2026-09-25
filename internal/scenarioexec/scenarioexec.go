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
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
}

const conditionCheckMillis = int64(2000)

// Execute claims the run, opens a session, walks the scenario graph, and
// finishes the run. The caller decides what to do with ErrPaused.
func (e Executor) Execute(ctx context.Context, runID string, compiled appir.Scenario) error {
	token := uid.New()
	claimed, err := e.Runs.Claim(ctx, runID, token)
	if err != nil {
		return err
	}
	if !claimed {
		return fmt.Errorf("scenarioexec: run %s not claimable", runID)
	}
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
	executor := &walker{exec: e, runID: runID, sessionID: sessionRow.ID, session: session, scenario: compiled, nodes: nodes}
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
	executor.events.Add(1)
	go executor.pumpEvents()
	paused, failure := executor.walk(execCtx)
	if deadline != nil {
		deadline()
	}
	if failure != "" && execCtx.Err() == nil {
		executor.captureTrace(context.Background())
	}
	// Close before finishing so the event pump drains every late console and
	// network observation into the log ahead of the terminal event.
	_ = session.Close(context.Background())
	executor.events.Wait()
	if paused {
		if pauseErr := e.Runs.Pause(ctx, runID, token); pauseErr != nil {
			return e.failRun(ctx, runID, token, pauseErr)
		}
		return ErrPaused
	}
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
	exec      Executor
	runID     string
	sessionID string
	session   browserapi.Session
	scenario  appir.Scenario
	lastPass  bool
	snapshot  *browserapi.Snapshot
	nodes     map[string]appir.ScenarioNode
	events    sync.WaitGroup
	pauseOn   map[string]bool
	paused    map[string]bool
	secretMu  sync.RWMutex
	secrets   []string
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

// walk executes the scenario graph from the start node until it reaches a
// dead end (completed), a failure, or a pause. It reports (true, "") on a
// pause node, (false, "") on completion, or (false, message) on failure.
func (w *walker) walk(ctx context.Context) (bool, string) {
	nodeID := w.scenario.Start
	visited := 0
	for nodeID != "" {
		visited++
		if w.exec.PauseRequested != nil && w.exec.PauseRequested(w.runID) {
			return true, ""
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
			return true, ""
		}
		next, err := w.executeNode(ctx, node)
		if errors.Is(err, ErrPaused) {
			return true, ""
		}
		if err != nil {
			return false, err.Error()
		}
		nodeID = next
	}
	return false, ""
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
		ref, err := w.resolveRef(ctx, node.Ref)
		if err != nil {
			return "", err
		}
		_, err = w.session.Click(ctx, ref)
		w.snapshot = nil
		return "{}", err
	case scenario.NodeFill:
		ref, err := w.resolveRef(ctx, node.Ref)
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
		ref, err := w.resolveRef(ctx, node.Ref)
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
		condition, err := w.condition(ctx, node.Condition, node.Ref, node.Text, w.timeout(node))
		if err != nil {
			return "", err
		}
		result, err := w.session.Wait(ctx, condition)
		if err != nil {
			return "", err
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

// resolveRef maps a scenario `ref` (semantic element name or raw ref id) onto
// a live browserapi.Ref from the current snapshot.
func (w *walker) resolveRef(ctx context.Context, target string) (browserapi.Ref, error) {
	if err := w.refreshSnapshot(ctx); err != nil {
		return browserapi.Ref{}, err
	}
	nodes := w.snapshot.Nodes
	// Raw ref id (e.g. "e12") resolves positionally.
	if len(target) > 1 && target[0] == 'e' {
		if ref, err := w.snapshot.Resolve(target); err == nil {
			return ref, nil
		}
	}
	for _, node := range nodes {
		if strings.EqualFold(node.Name, target) {
			return w.snapshot.Resolve(node.Ref)
		}
	}
	for _, node := range nodes {
		if strings.Contains(strings.ToLower(node.Name), strings.ToLower(target)) {
			return w.snapshot.Resolve(node.Ref)
		}
	}
	return browserapi.Ref{}, fmt.Errorf("no element matches ref %q", target)
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
			return "", fmt.Errorf("ref %s text %q does not contain %q", node.Ref, extracted.Value, node.Text)
		}
		return fmt.Sprintf(`{"actual":%q}`, extracted.Value), nil
	}
	condition, err := w.condition(ctx, node.Assertion, node.Ref, node.Text, w.timeout(node))
	if err != nil {
		return "", err
	}
	result, err := w.session.Wait(ctx, condition)
	if err != nil {
		return "", err
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

// hostAllowed applies Policy.AllowedDomains to a navigation target. When
// no allowlist is configured every URL passes; otherwise only http(s)
// hosts equal to or beneath a listed domain are allowed — other schemes
// (file:, javascript:, data:) are refused outright.
func (w *walker) hostAllowed(raw string) (string, bool) {
	if len(w.exec.Policy.AllowedDomains) == 0 {
		return "", true
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return host, false
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
	kind := node.As
	if kind == "" {
		kind = browserapi.ExtractText
	}
	extracted, err := w.session.Extract(ctx, ref, kind, node.Attribute)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{"as":%q,"value":%q}`, kind, extracted.Value), nil
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
