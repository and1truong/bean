// Package scenariorun persists the session-oriented run model for browser
// scenarios: Run -> Session -> StepExecution -> Artifact/Event. A run lasts
// seconds to minutes and outlives any single request, so every mutation is a
// short transaction; no database transaction is ever held across a run.
package scenariorun

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
)

// Run lifecycle statuses.
const (
	RunPending   = "pending"
	RunRunning   = "running"
	RunPaused    = "paused"
	RunCompleted = "completed"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
)

// Session (browser adapter session) statuses.
const (
	SessionStarting = "starting"
	SessionActive   = "active"
	SessionClosed   = "closed"
	SessionFailed   = "failed"
)

// StepExecution statuses.
const (
	StepPending = "pending"
	StepRunning = "running"
	StepPassed  = "passed"
	StepFailed  = "failed"
	StepSkipped = "skipped"
)

// Run trigger origins.
const (
	TriggerManual    = "manual"
	TriggerAPI       = "api"
	TriggerGenerated = "generated"
)

// Artifact kinds.
const (
	ArtifactScreenshot = "screenshot"
	ArtifactDOM        = "dom"
	ArtifactTrace      = "trace"
	ArtifactLog        = "log"
	ArtifactVideo      = "video"
)

// Event kinds recorded in the durable run log.
const (
	EventRunEnqueued      = "run_enqueued"
	EventRunClaimed       = "run_claimed"
	EventRunPaused        = "run_paused"
	EventRunResumed       = "run_resumed"
	EventRunFinished      = "run_finished"
	EventSessionOpened    = "session_opened"
	EventSessionUpdated   = "session_updated"
	EventSessionClosed    = "session_closed"
	EventStepStarted      = "step_started"
	EventStepFinished     = "step_finished"
	EventArtifactRecorded = "artifact_recorded"
	// Browser-level observations piped from the adapter into the run log:
	// normalized snapshot views, page console/pageerror output, network
	// request/response activity, and scenario assertion outcomes.
	EventBrowserSnapshot = "browser_snapshot"
	EventConsole         = "console_event"
	EventNetwork         = "network_event"
	EventAssertion       = "assertion_result"
	// EventPolicyBlocked records a navigation or request refused by the
	// host egress policy, with the refused URL and host in the payload.
	EventPolicyBlocked = "policy_blocked"
	// EventPolicyPause records an approval gate: a node type listed in
	// the execution policy paused the run before executing; resume is
	// the approval decision and this event makes it durable across walks.
	EventPolicyPause = "policy_pause"
	// EventSecretUsed records that a step resolved a secret by name —
	// the audit trail for credential use without logging the value.
	EventSecretUsed = "secret_used"
)

// Bounds shared with callers and the HTTP layer.
const (
	MaxErrorRunes    = 2048
	MaxOutputBytes   = 1 << 16
	MaxPayloadBytes  = 1 << 16
	MaxArtifactBytes = 32 << 20
	MaxEventsPerRun  = 4096
	MaxStepsPerRun   = 4096
	MaxListRows      = 500
)

var (
	runStatuses     = map[string]bool{RunPending: true, RunRunning: true, RunPaused: true, RunCompleted: true, RunFailed: true, RunCancelled: true}
	sessionStatuses = map[string]bool{SessionStarting: true, SessionActive: true, SessionClosed: true, SessionFailed: true}
	stepStatuses    = map[string]bool{StepPending: true, StepRunning: true, StepPassed: true, StepFailed: true, StepSkipped: true}
	triggers        = map[string]bool{TriggerManual: true, TriggerAPI: true, TriggerGenerated: true}
	artifactKinds   = map[string]bool{ArtifactScreenshot: true, ArtifactDOM: true, ArtifactTrace: true, ArtifactLog: true, ArtifactVideo: true}
	terminalRun     = map[string]bool{RunCompleted: true, RunFailed: true, RunCancelled: true}
	terminalStep    = map[string]bool{StepPassed: true, StepFailed: true, StepSkipped: true}
	terminalSession = map[string]bool{SessionClosed: true, SessionFailed: true}
	eventKinds      = map[string]bool{EventRunEnqueued: true, EventRunClaimed: true, EventRunPaused: true, EventRunResumed: true, EventRunFinished: true, EventSessionOpened: true, EventSessionUpdated: true, EventSessionClosed: true, EventStepStarted: true, EventStepFinished: true, EventArtifactRecorded: true, EventBrowserSnapshot: true, EventConsole: true, EventNetwork: true, EventAssertion: true, EventPolicyBlocked: true, EventPolicyPause: true, EventSecretUsed: true}
)

func valid(set map[string]bool, value string) bool { return set[value] }

func blank(value string) bool { return strings.TrimSpace(value) == "" }

func boundedRunes(value string, max int) bool { return utf8.RuneCountInString(value) <= max }

// Run is one durable execution of a Scenario against a pinned release.
type Run struct {
	ID         string
	AppID      string
	ReleaseID  string
	Scenario   string
	Trigger    string
	Status     string
	Error      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	ClaimToken string
	ClaimedAt  time.Time
}

// Session is one browser-adapter session bound to a run.
type Session struct {
	ID        string
	RunID     string
	Adapter   string
	Status    string
	Ref       string
	Error     string
	CreatedAt time.Time
	ClosedAt  time.Time
}

// StepExecution is one attempt at one scenario node.
type StepExecution struct {
	ID         string
	RunID      string
	SessionID  string
	NodeID     string
	Attempt    int64
	Status     string
	Output     string
	Error      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
}

// Artifact is durable evidence (screenshot, DOM, trace, ...) for a run or step.
type Artifact struct {
	ID          string
	RunID       string
	StepID      string
	Kind        string
	ContentType string
	Size        int64
	Ref         string
	CreatedAt   time.Time
}

// Event is one persisted entry in the run log consumed by the stream layer.
type Event struct {
	ID        string
	RunID     string
	StepID    string
	Sequence  int64
	Kind      string
	Payload   string
	CreatedAt time.Time
}

// Store persists run model state through short transactions only.
type Store struct {
	DB  dbal.Database
	Now func() time.Time
}

func (s Store) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

// writeMu serializes write transactions against the bean_run_* tables. The
// store is a value type used from several goroutines at once (the executor
// walker and the event pump, for instance), and two overlapping write
// transactions can hit a snapshot conflict that busy_timeout cannot wait
// out, so contention is excluded instead.
var writeMu sync.Mutex

func (s Store) write(ctx context.Context, fn func(dbal.Transaction) error) error {
	writeMu.Lock()
	defer writeMu.Unlock()
	return s.DB.Transaction(ctx, fn)
}

func parseTimestamp(value any) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, fmt.Sprint(value))
	return t.UTC()
}

func integer(value any) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(v)
	case json.Number:
		n, _ := v.Int64()
		return n
	default:
		return 0
	}
}

func text(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func nullable(value any) any {
	if value == nil || fmt.Sprint(value) == "" {
		return nil
	}
	return value
}

// Enqueue registers a new pending run and records the enqueued event.
func (s Store) Enqueue(ctx context.Context, run Run) (Run, error) {
	if blank(run.AppID) || blank(run.ReleaseID) || blank(run.Scenario) {
		return Run{}, fmt.Errorf("run requires app_id, release_id, and scenario")
	}
	if !valid(triggers, run.Trigger) {
		return Run{}, fmt.Errorf("invalid run trigger %q", run.Trigger)
	}
	if run.ID == "" {
		run.ID = uid.New()
	}
	run.Status = RunPending
	run.CreatedAt = s.now()
	return run, s.write(ctx, func(tx dbal.Transaction) error {
		if _, err := tx.Insert(ctx, dbal.Insert{Table: "bean_run", Values: runValues(run)}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, run.ID, "", EventRunEnqueued, fmt.Sprintf(`{"scenario":%q,"trigger":%q}`, run.Scenario, run.Trigger))
	})
}

// Claim atomically moves a pending run to running under token. The second
// result reports whether the claim succeeded; a false result means another
// runner won or the run left the pending state.
func (s Store) Claim(ctx context.Context, id, token string) (bool, error) {
	now := s.now()
	claimed := false
	err := s.write(ctx, func(tx dbal.Transaction) error {
		result, err := tx.Update(ctx, dbal.Update{Table: "bean_run", Values: map[string]dbal.Value{"status": RunRunning, "claim_token": token, "claimed_at": timestamp(now), "started_at": timestamp(now)}, Where: dbal.And(
			dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
			dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunPending},
		)})
		if err != nil {
			return err
		}
		claimed = result.Affected == 1
		if claimed {
			return s.appendEvent(ctx, tx, id, "", EventRunClaimed, fmt.Sprintf(`{"claim_token":%q}`, token))
		}
		return nil
	})
	return claimed, err
}

// Pause moves a running run to paused, preserving it for a later resume.
func (s Store) Pause(ctx context.Context, id, token string) error {
	return s.transition(ctx, id, dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunRunning},
		dbal.Predicate{Op: dbal.OpEQ, Column: "claim_token", Value: token},
	), map[string]dbal.Value{"status": RunPaused, "claim_token": nil, "claimed_at": nil}, EventRunPaused, "{}")
}

// Cancel stops a run that has not started (pending) or is parked
// (paused). Cancelling a claimed running run goes through the runner
// holding its claim; Cancel only covers unclaimed states.
func (s Store) Cancel(ctx context.Context, id string) error {
	return s.transition(ctx, id, dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
		dbal.Predicate{Op: dbal.OpIn, Column: "status", Value: []dbal.Value{RunPending, RunPaused}},
	), map[string]dbal.Value{"status": RunCancelled, "finished_at": timestamp(s.now())}, EventRunFinished, fmt.Sprintf(`{"status":%q}`, RunCancelled))
}

// Resume returns a paused run to pending so a runner can claim it again.
func (s Store) Resume(ctx context.Context, id string) error {
	return s.transition(ctx, id, dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunPaused},
	), map[string]dbal.Value{"status": RunPending}, EventRunResumed, "{}")
}

// Finish moves the claimed running run to a terminal status and releases the
// claim. Still-active sessions and running steps are closed/failed in the same
// transaction so nothing is left hanging.
func (s Store) Finish(ctx context.Context, id, token, status, cause string) error {
	if !valid(terminalRun, status) {
		return fmt.Errorf("invalid terminal run status %q", status)
	}
	if !boundedRunes(cause, MaxErrorRunes) {
		return fmt.Errorf("run error exceeds %d runes", MaxErrorRunes)
	}
	now := s.now()
	sessionStatus := SessionClosed
	stepStatus := StepSkipped
	if status == RunFailed {
		sessionStatus = SessionFailed
		stepStatus = StepFailed
	}
	return s.write(ctx, func(tx dbal.Transaction) error {
		result, err := tx.Update(ctx, dbal.Update{Table: "bean_run", Values: map[string]dbal.Value{"status": status, "error": nullable(cause), "claim_token": nil, "claimed_at": nil, "finished_at": timestamp(now)}, Where: dbal.And(
			dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
			dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunRunning},
			dbal.Predicate{Op: dbal.OpEQ, Column: "claim_token", Value: token},
		), ExpectedRows: 1})
		if err != nil {
			return err
		}
		if result.Affected == 0 {
			return fmt.Errorf("run %q is not running under this claim", id)
		}
		if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_session", Values: map[string]dbal.Value{"status": sessionStatus, "closed_at": timestamp(now)}, Where: dbal.And(
			dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: id},
			dbal.Predicate{Op: dbal.OpIn, Column: "status", Value: []dbal.Value{SessionStarting, SessionActive}},
		)}); err != nil {
			return err
		}
		if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_step", Values: map[string]dbal.Value{"status": stepStatus, "finished_at": timestamp(now)}, Where: dbal.And(
			dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: id},
			dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: StepRunning},
		)}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, id, "", EventRunFinished, fmt.Sprintf(`{"status":%q}`, status))
	})
}

// RecoverStale deterministically fails runs whose claim lease expired (the
// holder crashed), closing their sessions and running steps so nothing hangs.
// Paused runs are unclaimed and stay resumable.
func (s Store) RecoverStale(ctx context.Context, lease time.Duration) (int, error) {
	now := s.now()
	stale := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunRunning},
		dbal.Predicate{Op: dbal.OpLTE, Column: "claimed_at", Value: timestamp(now.Add(-lease))},
	)
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run", Columns: []string{"id"}, Where: &stale, Limit: MaxListRows})
	if err != nil {
		return 0, err
	}
	recovered := 0
	for _, row := range rows {
		id := text(row["id"])
		err = s.write(ctx, func(tx dbal.Transaction) error {
			result, err := tx.Update(ctx, dbal.Update{Table: "bean_run", Values: map[string]dbal.Value{"status": RunFailed, "error": "runner claim expired", "claim_token": nil, "claimed_at": nil, "finished_at": timestamp(now)}, Where: staleFor(id, now, lease), ExpectedRows: 1})
			if err != nil {
				return err
			}
			if result.Affected == 0 {
				return nil
			}
			if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_session", Values: map[string]dbal.Value{"status": SessionFailed, "error": "runner claim expired", "closed_at": timestamp(now)}, Where: dbal.And(
				dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: id},
				dbal.Predicate{Op: dbal.OpIn, Column: "status", Value: []dbal.Value{SessionStarting, SessionActive}},
			)}); err != nil {
				return err
			}
			if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_step", Values: map[string]dbal.Value{"status": StepFailed, "error": "runner claim expired", "finished_at": timestamp(now)}, Where: dbal.And(
				dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: id},
				dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: StepRunning},
			)}); err != nil {
				return err
			}
			return s.appendEvent(ctx, tx, id, "", EventRunFinished, `{"status":"failed","cause":"claim_expired"}`)
		})
		if err != nil {
			return recovered, err
		}
		recovered++
	}
	return recovered, nil
}

func staleFor(id string, now time.Time, lease time.Duration) dbal.Predicate {
	return dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: RunRunning},
		dbal.Predicate{Op: dbal.OpLTE, Column: "claimed_at", Value: timestamp(now.Add(-lease))},
	)
}

func (s Store) transition(ctx context.Context, id string, where dbal.Predicate, values map[string]dbal.Value, eventKind, payload string) error {
	return s.write(ctx, func(tx dbal.Transaction) error {
		if _, err := tx.Update(ctx, dbal.Update{Table: "bean_run", Values: values, Where: where, ExpectedRows: 1}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, id, "", eventKind, payload)
	})
}

// OpenSession registers a browser session bound to a run.
func (s Store) OpenSession(ctx context.Context, session Session) (Session, error) {
	if blank(session.RunID) || blank(session.Adapter) {
		return Session{}, fmt.Errorf("session requires run_id and adapter")
	}
	if session.ID == "" {
		session.ID = uid.New()
	}
	if session.Status == "" {
		session.Status = SessionStarting
	}
	if !valid(sessionStatuses, session.Status) {
		return Session{}, fmt.Errorf("invalid session status %q", session.Status)
	}
	session.CreatedAt = s.now()
	return session, s.write(ctx, func(tx dbal.Transaction) error {
		if _, err := tx.Insert(ctx, dbal.Insert{Table: "bean_run_session", Values: sessionValues(session)}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, session.RunID, "", EventSessionOpened, fmt.Sprintf(`{"session":%q,"adapter":%q}`, session.ID, session.Adapter))
	})
}

// UpdateSession transitions a session, recording its adapter ref and error.
func (s Store) UpdateSession(ctx context.Context, id, status, ref, cause string) error {
	if !valid(sessionStatuses, status) {
		return fmt.Errorf("invalid session status %q", status)
	}
	if !boundedRunes(cause, MaxErrorRunes) {
		return fmt.Errorf("session error exceeds %d runes", MaxErrorRunes)
	}
	values := map[string]dbal.Value{"status": status, "error": nullable(cause)}
	if ref != "" {
		values["ref"] = ref
	}
	var runID string
	return s.write(ctx, func(tx dbal.Transaction) error {
		rows, err := tx.Select(ctx, dbal.Select{Table: "bean_run_session", Columns: []string{"run_id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
		if err != nil || len(rows) != 1 {
			return fmt.Errorf("session %q not found", id)
		}
		runID = text(rows[0]["run_id"])
		if valid(terminalSession, status) {
			values["closed_at"] = timestamp(s.now())
		}
		if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_session", Values: values, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1}); err != nil {
			return err
		}
		eventKind := EventSessionUpdated
		if valid(terminalSession, status) {
			eventKind = EventSessionClosed
		}
		return s.appendEvent(ctx, tx, runID, "", eventKind, fmt.Sprintf(`{"session":%q,"status":%q}`, id, status))
	})
}

// NextStepAttempt returns the next attempt ordinal for a node in a run.
func (s Store) NextStepAttempt(ctx context.Context, runID, nodeID string) (int64, error) {
	where := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID},
		dbal.Predicate{Op: dbal.OpEQ, Column: "node_id", Value: nodeID},
	)
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_step", Columns: []string{"attempt"}, Where: &where, OrderBy: []dbal.Order{{Column: "attempt", Desc: true}}, Limit: 1})
	if err != nil || len(rows) == 0 {
		return 1, err
	}
	return integer(rows[0]["attempt"]) + 1, nil
}

// StartStep records a running attempt at one scenario node.
func (s Store) StartStep(ctx context.Context, step StepExecution) (StepExecution, error) {
	if blank(step.RunID) || blank(step.NodeID) {
		return StepExecution{}, fmt.Errorf("step requires run_id and node_id")
	}
	if step.ID == "" {
		step.ID = uid.New()
	}
	if step.Attempt <= 0 {
		return StepExecution{}, fmt.Errorf("step attempt must be positive")
	}
	count, err := s.stepCount(ctx, step.RunID)
	if err != nil {
		return StepExecution{}, err
	}
	if count >= MaxStepsPerRun {
		return StepExecution{}, fmt.Errorf("run %q exceeds %d step executions", step.RunID, MaxStepsPerRun)
	}
	step.Status = StepRunning
	step.CreatedAt = s.now()
	step.StartedAt = step.CreatedAt
	return step, s.write(ctx, func(tx dbal.Transaction) error {
		if _, err := tx.Insert(ctx, dbal.Insert{Table: "bean_run_step", Values: stepValues(step)}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, step.RunID, step.ID, EventStepStarted, fmt.Sprintf(`{"node":%q,"attempt":%d}`, step.NodeID, step.Attempt))
	})
}

// FinishStep records a terminal status, output, and error for a running step.
func (s Store) FinishStep(ctx context.Context, id, status, output, cause string) error {
	if !valid(terminalStep, status) {
		return fmt.Errorf("invalid terminal step status %q", status)
	}
	if len(output) > MaxOutputBytes {
		return fmt.Errorf("step output exceeds %d bytes", MaxOutputBytes)
	}
	if !boundedRunes(cause, MaxErrorRunes) {
		return fmt.Errorf("step error exceeds %d runes", MaxErrorRunes)
	}
	var runID string
	return s.write(ctx, func(tx dbal.Transaction) error {
		rows, err := tx.Select(ctx, dbal.Select{Table: "bean_run_step", Columns: []string{"run_id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
		if err != nil || len(rows) != 1 {
			return fmt.Errorf("step %q not found", id)
		}
		runID = text(rows[0]["run_id"])
		if _, err = tx.Update(ctx, dbal.Update{Table: "bean_run_step", Values: map[string]dbal.Value{"status": status, "output": nullable(output), "error": nullable(cause), "finished_at": timestamp(s.now())}, Where: dbal.And(
			dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id},
			dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: StepRunning},
		), ExpectedRows: 1}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, runID, id, EventStepFinished, fmt.Sprintf(`{"status":%q}`, status))
	})
}

func (s Store) stepCount(ctx context.Context, runID string) (int64, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_step", Columns: []string{"id"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}, Limit: MaxStepsPerRun + 1})
	if err != nil {
		return 0, err
	}
	return int64(len(rows)), nil
}

// RecordArtifact stores durable evidence metadata for a run or step.
func (s Store) RecordArtifact(ctx context.Context, artifact Artifact) (Artifact, error) {
	if blank(artifact.RunID) || !valid(artifactKinds, artifact.Kind) {
		return Artifact{}, fmt.Errorf("artifact requires run_id and a valid kind")
	}
	if artifact.Size < 0 || artifact.Size > MaxArtifactBytes {
		return Artifact{}, fmt.Errorf("artifact size exceeds %d bytes", MaxArtifactBytes)
	}
	if blank(artifact.Ref) {
		return Artifact{}, fmt.Errorf("artifact requires a storage ref")
	}
	if artifact.ID == "" {
		artifact.ID = uid.New()
	}
	artifact.CreatedAt = s.now()
	return artifact, s.write(ctx, func(tx dbal.Transaction) error {
		if _, err := tx.Insert(ctx, dbal.Insert{Table: "bean_run_artifact", Values: artifactValues(artifact)}); err != nil {
			return err
		}
		return s.appendEvent(ctx, tx, artifact.RunID, artifact.StepID, EventArtifactRecorded, fmt.Sprintf(`{"artifact":%q,"kind":%q}`, artifact.ID, artifact.Kind))
	})
}

// RecordEvent appends one observation to the run log in its own short
// transaction; use it for events that are not themselves state mutations
// (browser snapshots, console/network activity, assertion outcomes).
func (s Store) RecordEvent(ctx context.Context, runID, stepID, kind, payload string) error {
	return s.write(ctx, func(tx dbal.Transaction) error {
		return s.appendEvent(ctx, tx, runID, stepID, kind, payload)
	})
}

// appendEvent assigns the next per-run sequence inside the caller's
// transaction so the persisted log stays gap-free per run.
func (s Store) appendEvent(ctx context.Context, tx dbal.Transaction, runID, stepID, kind, payload string) error {
	if !valid(eventKinds, kind) {
		return fmt.Errorf("invalid event kind %q", kind)
	}
	if len(payload) > MaxPayloadBytes {
		return fmt.Errorf("event payload exceeds %d bytes", MaxPayloadBytes)
	}
	rows, err := tx.Select(ctx, dbal.Select{Table: "bean_run_event", Columns: []string{"sequence"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}, OrderBy: []dbal.Order{{Column: "sequence", Desc: true}}, Limit: 1})
	if err != nil {
		return err
	}
	sequence := int64(1)
	if len(rows) == 1 {
		sequence = integer(rows[0]["sequence"]) + 1
	}
	if sequence > MaxEventsPerRun {
		return fmt.Errorf("run %q exceeds %d events", runID, MaxEventsPerRun)
	}
	_, err = tx.Insert(ctx, dbal.Insert{Table: "bean_run_event", Values: map[string]dbal.Value{"id": uid.New(), "run_id": runID, "step_id": nullable(stepID), "sequence": sequence, "kind": kind, "payload": payload, "created_at": timestamp(s.now())}})
	return err
}

// Get returns a run by ID.
func (s Store) Get(ctx context.Context, id string) (Run, bool, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil || len(rows) == 0 {
		return Run{}, false, err
	}
	return runFrom(rows[0]), true, nil
}

// RunFilter scopes bounded run listings.
type RunFilter struct {
	AppID    string
	Scenario string
	Status   string
	Limit    int
}

// List returns runs matching the filter, newest first.
func (s Store) List(ctx context.Context, filter RunFilter) ([]Run, error) {
	where := dbal.And()
	if filter.AppID != "" {
		where.Children = append(where.Children, dbal.Predicate{Op: dbal.OpEQ, Column: "app_id", Value: filter.AppID})
	}
	if filter.Scenario != "" {
		where.Children = append(where.Children, dbal.Predicate{Op: dbal.OpEQ, Column: "scenario", Value: filter.Scenario})
	}
	if filter.Status != "" {
		if !valid(runStatuses, filter.Status) {
			return nil, fmt.Errorf("invalid run status filter %q", filter.Status)
		}
		where.Children = append(where.Children, dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: filter.Status})
	}
	limit := filter.Limit
	if limit <= 0 || limit > MaxListRows {
		limit = MaxListRows
	}
	var wherePtr *dbal.Predicate
	if len(where.Children) > 0 {
		wherePtr = &where
	}
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run", Where: wherePtr, OrderBy: []dbal.Order{{Column: "created_at", Desc: true}}, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]Run, 0, len(rows))
	for _, row := range rows {
		out = append(out, runFrom(row))
	}
	return out, nil
}

// Steps returns step executions of a run in creation order.
func (s Store) Steps(ctx context.Context, runID string) ([]StepExecution, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_step", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}, OrderBy: []dbal.Order{{Column: "created_at"}}, Limit: MaxStepsPerRun})
	if err != nil {
		return nil, err
	}
	out := make([]StepExecution, 0, len(rows))
	for _, row := range rows {
		out = append(out, stepFrom(row))
	}
	return out, nil
}

// Sessions returns browser sessions of a run in creation order.
func (s Store) Sessions(ctx context.Context, runID string) ([]Session, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_session", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}, OrderBy: []dbal.Order{{Column: "created_at"}}, Limit: MaxListRows})
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(rows))
	for _, row := range rows {
		out = append(out, sessionFrom(row))
	}
	return out, nil
}

// Events returns the persisted run log at or after a sequence, ordered.
func (s Store) Events(ctx context.Context, runID string, afterSequence int64) ([]Event, error) {
	where := dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}
	if afterSequence > 0 {
		where = dbal.And(where, dbal.Predicate{Op: dbal.OpGT, Column: "sequence", Value: afterSequence})
	}
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_event", Where: &where, OrderBy: []dbal.Order{{Column: "sequence"}}, Limit: MaxEventsPerRun})
	if err != nil {
		return nil, err
	}
	out := make([]Event, 0, len(rows))
	for _, row := range rows {
		out = append(out, eventFrom(row))
	}
	return out, nil
}

// Artifacts returns artifact metadata of a run in creation order.
func (s Store) Artifacts(ctx context.Context, runID string) ([]Artifact, error) {
	rows, err := s.DB.Select(ctx, dbal.Select{Table: "bean_run_artifact", Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "run_id", Value: runID}, OrderBy: []dbal.Order{{Column: "created_at"}}, Limit: MaxListRows})
	if err != nil {
		return nil, err
	}
	out := make([]Artifact, 0, len(rows))
	for _, row := range rows {
		out = append(out, artifactFrom(row))
	}
	return out, nil
}

func runValues(r Run) map[string]dbal.Value {
	return map[string]dbal.Value{"id": r.ID, "app_id": r.AppID, "release_id": r.ReleaseID, "scenario": r.Scenario, "trigger_kind": r.Trigger, "status": r.Status, "error": nullable(r.Error), "claim_token": nullable(r.ClaimToken), "claimed_at": nullableTime(r.ClaimedAt), "started_at": nullableTime(r.StartedAt), "finished_at": nullableTime(r.FinishedAt), "created_at": timestamp(r.CreatedAt)}
}

func sessionValues(s Session) map[string]dbal.Value {
	return map[string]dbal.Value{"id": s.ID, "run_id": s.RunID, "adapter": s.Adapter, "status": s.Status, "ref": nullable(s.Ref), "error": nullable(s.Error), "created_at": timestamp(s.CreatedAt), "closed_at": nullableTime(s.ClosedAt)}
}

func stepValues(s StepExecution) map[string]dbal.Value {
	return map[string]dbal.Value{"id": s.ID, "run_id": s.RunID, "session_id": nullable(s.SessionID), "node_id": s.NodeID, "attempt": s.Attempt, "status": s.Status, "output": nullable(s.Output), "error": nullable(s.Error), "created_at": timestamp(s.CreatedAt), "started_at": nullableTime(s.StartedAt), "finished_at": nullableTime(s.FinishedAt)}
}

func artifactValues(a Artifact) map[string]dbal.Value {
	return map[string]dbal.Value{"id": a.ID, "run_id": a.RunID, "step_id": nullable(a.StepID), "kind": a.Kind, "content_type": a.ContentType, "size": a.Size, "ref": a.Ref, "created_at": timestamp(a.CreatedAt)}
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return timestamp(value)
}

func runFrom(row dbal.Row) Run {
	return Run{ID: text(row["id"]), AppID: text(row["app_id"]), ReleaseID: text(row["release_id"]), Scenario: text(row["scenario"]), Trigger: text(row["trigger_kind"]), Status: text(row["status"]), Error: text(row["error"]), ClaimToken: text(row["claim_token"]), ClaimedAt: parseTimestamp(row["claimed_at"]), CreatedAt: parseTimestamp(row["created_at"]), StartedAt: parseTimestamp(row["started_at"]), FinishedAt: parseTimestamp(row["finished_at"])}
}

func sessionFrom(row dbal.Row) Session {
	return Session{ID: text(row["id"]), RunID: text(row["run_id"]), Adapter: text(row["adapter"]), Status: text(row["status"]), Ref: text(row["ref"]), Error: text(row["error"]), CreatedAt: parseTimestamp(row["created_at"]), ClosedAt: parseTimestamp(row["closed_at"])}
}

func stepFrom(row dbal.Row) StepExecution {
	return StepExecution{ID: text(row["id"]), RunID: text(row["run_id"]), SessionID: text(row["session_id"]), NodeID: text(row["node_id"]), Attempt: integer(row["attempt"]), Status: text(row["status"]), Output: text(row["output"]), Error: text(row["error"]), CreatedAt: parseTimestamp(row["created_at"]), StartedAt: parseTimestamp(row["started_at"]), FinishedAt: parseTimestamp(row["finished_at"])}
}

func artifactFrom(row dbal.Row) Artifact {
	return Artifact{ID: text(row["id"]), RunID: text(row["run_id"]), StepID: text(row["step_id"]), Kind: text(row["kind"]), ContentType: text(row["content_type"]), Size: integer(row["size"]), Ref: text(row["ref"]), CreatedAt: parseTimestamp(row["created_at"])}
}

func eventFrom(row dbal.Row) Event {
	return Event{ID: text(row["id"]), RunID: text(row["run_id"]), StepID: text(row["step_id"]), Sequence: integer(row["sequence"]), Kind: text(row["kind"]), Payload: text(row["payload"]), CreatedAt: parseTimestamp(row["created_at"])}
}
