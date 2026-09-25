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
	"errors"
	"fmt"
	"strings"
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

// Executor runs one scenario against the browser and the run store.
type Executor struct {
	Runs     scenariorun.Store
	Sessions SessionFactory
	Secrets  SecretResolver
	// StepTimeout bounds one step's browser calls; zero uses the node's own
	// timeout (scenario.DefaultTimeoutSeconds).
	Now func() time.Time
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
	defer session.Close(context.Background())
	nodes := make(map[string]appir.ScenarioNode, len(compiled.Nodes))
	for _, node := range compiled.Nodes {
		nodes[node.ID] = node
	}
	executor := &walker{exec: e, runID: runID, sessionID: sessionRow.ID, session: session, scenario: compiled, nodes: nodes}
	paused, failure := executor.walk(ctx)
	if paused {
		if pauseErr := e.Runs.Pause(ctx, runID, token); pauseErr != nil {
			return e.failRun(ctx, runID, token, pauseErr)
		}
		return ErrPaused
	}
	if failure == "" {
		return e.Runs.Finish(ctx, runID, token, scenariorun.RunCompleted, "")
	}
	if err := e.Runs.Finish(ctx, runID, token, scenariorun.RunFailed, bounded(failure, scenariorun.MaxErrorRunes)); err != nil {
		return err
	}
	return errors.New(failure)
}

func (e Executor) failRun(ctx context.Context, runID, token string, cause error) error {
	if finishErr := e.Runs.Finish(ctx, runID, token, scenariorun.RunFailed, bounded(cause.Error(), scenariorun.MaxErrorRunes)); finishErr != nil {
		return finishErr
	}
	return cause
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
}

// walk executes the scenario graph from the start node until it reaches a
// dead end (completed), a failure, or a pause. It reports (true, "") on a
// pause node, (false, "") on completion, or (false, message) on failure.
func (w *walker) walk(ctx context.Context) (bool, string) {
	nodeID := w.scenario.Start
	visited := 0
	for nodeID != "" {
		visited++
		if visited > scenariorun.MaxStepsPerRun {
			return false, fmt.Sprintf("step bound %d exceeded", scenariorun.MaxStepsPerRun)
		}
		node, found := w.nodes[nodeID]
		if !found {
			return false, fmt.Sprintf("node %q missing", nodeID)
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
		_ = w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepFailed, "", bounded(runErr.Error(), scenariorun.MaxErrorRunes))
		w.captureArtifact(ctx, step.ID)
		if node.OnFail != "" {
			return node.OnFail, nil
		}
		return "", fmt.Errorf("step %s failed: %w", node.ID, runErr)
	}
	if node.Type != scenario.NodeBranch {
		w.lastPass = true
	}
	_ = w.exec.Runs.FinishStep(ctx, step.ID, scenariorun.StepPassed, bounded(output, scenariorun.MaxOutputBytes), "")
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
	return nil
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
		if !strings.Contains(extracted.Value, node.Text) {
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
	if !result.Met {
		return "", fmt.Errorf("assertion %q not met", node.Assertion)
	}
	return "{}", nil
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

func (w *walker) captureArtifact(ctx context.Context, stepID string) {
	capture, err := w.session.Screenshot(ctx)
	if err != nil || len(capture.Bytes) == 0 {
		return
	}
	_, _ = w.exec.Runs.RecordArtifact(ctx, scenariorun.Artifact{
		RunID: w.runID, StepID: stepID,
		Kind: scenariorun.ArtifactScreenshot, ContentType: capture.ContentType,
		Size: int64(len(capture.Bytes)), Ref: "inline",
	})
}
