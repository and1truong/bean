package scenariorun_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/scenariorun"
)

func newStore(t *testing.T, now time.Time) (*sqlite.DB, scenariorun.Store, *time.Time) {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "runs.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err = db.ExecuteMigration(context.Background(), migration.MetadataSchema()); err != nil {
		t.Fatal(err)
	}
	current := now
	return db, scenariorun.Store{DB: db, Now: func() time.Time { return current }}, &current
}

func enqueue(t *testing.T, store scenariorun.Store) scenariorun.Run {
	t.Helper()
	run, err := store.Enqueue(context.Background(), scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "login_invalid_password", Trigger: scenariorun.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	return run
}

func TestRunLifecyclePersistsAcrossStoreInstances(t *testing.T) {
	ctx := context.Background()
	db, store, current := newStore(t, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	run := enqueue(t, store)
	if run.Status != scenariorun.RunPending || run.ID == "" {
		t.Fatalf("run=%+v", run)
	}
	if claimed, err := store.Claim(ctx, "missing-id", "tok-x"); claimed || err != nil {
		t.Fatalf("missing claim: %v %v", claimed, err)
	}
	if claimed, err := store.Claim(ctx, run.ID, "tok-a"); !claimed || err != nil {
		t.Fatalf("first claim: %v %v", claimed, err)
	}
	if claimed, err := store.Claim(ctx, run.ID, "tok-b"); claimed || err != nil {
		t.Fatalf("second claim: %v %v", claimed, err)
	}
	session, err := store.OpenSession(ctx, scenariorun.Session{RunID: run.ID, Adapter: "playwright"})
	if err != nil || session.Status != scenariorun.SessionStarting {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	if err = store.UpdateSession(ctx, session.ID, scenariorun.SessionActive, "ws://sidecar/1", ""); err != nil {
		t.Fatal(err)
	}
	attempt, err := store.NextStepAttempt(ctx, run.ID, "open_login")
	if err != nil || attempt != 1 {
		t.Fatalf("attempt=%d err=%v", attempt, err)
	}
	step, err := store.StartStep(ctx, scenariorun.StepExecution{RunID: run.ID, SessionID: session.ID, NodeID: "open_login", Attempt: attempt})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.RecordArtifact(ctx, scenariorun.Artifact{RunID: run.ID, StepID: step.ID, Kind: scenariorun.ArtifactScreenshot, ContentType: "image/png", Size: 1024, Ref: "blob://shot-1"}); err != nil {
		t.Fatal(err)
	}
	if err = store.FinishStep(ctx, step.ID, scenariorun.StepPassed, `{"url":"/login"}`, ""); err != nil {
		t.Fatal(err)
	}
	if err = store.Pause(ctx, run.ID, "tok-a"); err != nil {
		t.Fatal(err)
	}
	if err = store.Resume(ctx, run.ID); err != nil {
		t.Fatal(err)
	}
	if claimed, err := store.Claim(ctx, run.ID, "tok-c"); !claimed || err != nil {
		t.Fatalf("resume claim: %v %v", claimed, err)
	}
	if err = store.Finish(ctx, run.ID, "tok-c", scenariorun.RunCompleted, ""); err != nil {
		t.Fatal(err)
	}
	// A fresh Store on the same database sees durable state.
	reopened := scenariorun.Store{DB: db}
	persisted, found, err := reopened.Get(ctx, run.ID)
	if err != nil || !found || persisted.Status != scenariorun.RunCompleted || persisted.Scenario != "login_invalid_password" {
		t.Fatalf("persisted=%+v found=%v err=%v", persisted, found, err)
	}
	steps, err := reopened.Steps(ctx, run.ID)
	if err != nil || len(steps) != 1 || steps[0].Status != scenariorun.StepPassed || steps[0].Attempt != 1 || steps[0].Output == "" {
		t.Fatalf("steps=%v err=%v", steps, err)
	}
	sessions, err := reopened.Sessions(ctx, run.ID)
	if err != nil || len(sessions) != 1 || sessions[0].Status != scenariorun.SessionClosed || sessions[0].ClosedAt.IsZero() {
		t.Fatalf("sessions=%v err=%v", sessions, err)
	}
	artifacts, err := reopened.Artifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 1 || artifacts[0].Kind != scenariorun.ArtifactScreenshot {
		t.Fatalf("artifacts=%v err=%v", artifacts, err)
	}
	events, err := reopened.Events(ctx, run.ID, 0)
	if err != nil || len(events) != 11 {
		t.Fatalf("events=%v err=%v", len(events), err)
	}
	for index, event := range events {
		if event.Sequence != int64(index+1) {
			t.Fatalf("event sequences not ordered: %v", events)
		}
	}
	if events[0].Kind != scenariorun.EventRunEnqueued || events[10].Kind != scenariorun.EventRunFinished {
		t.Fatalf("event log=%v", events)
	}
	_ = current
}

func TestFinishRejectsWrongClaimAndInvalidStatus(t *testing.T) {
	ctx := context.Background()
	_, store, _ := newStore(t, time.Now())
	run := enqueue(t, store)
	if claimed, err := store.Claim(ctx, run.ID, "tok-a"); !claimed || err != nil {
		t.Fatal(err)
	}
	if err := store.Finish(ctx, run.ID, "tok-b", scenariorun.RunCompleted, ""); err == nil {
		t.Fatal("finish under a foreign claim succeeded")
	}
	if err := store.Finish(ctx, run.ID, "tok-a", "hanging", ""); err == nil {
		t.Fatal("non-terminal status accepted")
	}
	if err := store.Pause(ctx, run.ID, "tok-b"); err == nil {
		t.Fatal("pause under a foreign claim succeeded")
	}
	run, found, err := store.Get(ctx, run.ID)
	if err != nil || !found || run.Status != scenariorun.RunRunning {
		t.Fatalf("run=%+v", run)
	}
}

func TestRecoverStaleDeterministicallyFailsExpiredClaims(t *testing.T) {
	ctx := context.Background()
	db, store, current := newStore(t, time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC))
	stuck := enqueue(t, store)
	if claimed, err := store.Claim(ctx, stuck.ID, "tok-dead"); !claimed || err != nil {
		t.Fatal(err)
	}
	session, err := store.OpenSession(ctx, scenariorun.Session{RunID: stuck.ID, Adapter: "playwright"})
	if err != nil {
		t.Fatal(err)
	}
	step, err := store.StartStep(ctx, scenariorun.StepExecution{RunID: stuck.ID, SessionID: session.ID, NodeID: "open_login", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	paused := enqueue(t, store)
	if claimed, err := store.Claim(ctx, paused.ID, "tok-live"); !claimed || err != nil {
		t.Fatal(err)
	}
	if err = store.Pause(ctx, paused.ID, "tok-live"); err != nil {
		t.Fatal(err)
	}
	pending := enqueue(t, store)
	// Crash: process restarts 10 minutes later.
	*current = current.Add(10 * time.Minute)
	restarted := scenariorun.Store{DB: db, Now: store.Now}
	recovered, err := restarted.RecoverStale(ctx, time.Minute)
	if err != nil || recovered != 1 {
		t.Fatalf("recovered=%d err=%v", recovered, err)
	}
	row, found, err := restarted.Get(ctx, stuck.ID)
	if err != nil || !found || row.Status != scenariorun.RunFailed || row.Error == "" || !row.FinishedAt.After(step.StartedAt) {
		t.Fatalf("stale run=%+v", row)
	}
	sessions, err := restarted.Sessions(ctx, stuck.ID)
	if err != nil || sessions[0].Status != scenariorun.SessionFailed {
		t.Fatalf("sessions=%v", sessions)
	}
	steps, err := restarted.Steps(ctx, stuck.ID)
	if err != nil || steps[0].Status != scenariorun.StepFailed {
		t.Fatalf("steps=%v", steps)
	}
	row, _, _ = restarted.Get(ctx, paused.ID)
	if row.Status != scenariorun.RunPaused {
		t.Fatalf("paused run touched: %+v", row)
	}
	row, _, _ = restarted.Get(ctx, pending.ID)
	if row.Status != scenariorun.RunPending {
		t.Fatalf("pending run touched: %+v", row)
	}
	// Recovery is idempotent.
	if recovered, err = restarted.RecoverStale(ctx, time.Minute); err != nil || recovered != 0 {
		t.Fatalf("second recovery: %d %v", recovered, err)
	}
}

func TestEnqueueValidatesAndStepAttemptsSequence(t *testing.T) {
	ctx := context.Background()
	_, store, _ := newStore(t, time.Now())
	if _, err := store.Enqueue(ctx, scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "s", Trigger: "cron"}); err == nil {
		t.Fatal("invalid trigger accepted")
	}
	if _, err := store.Enqueue(ctx, scenariorun.Run{Scenario: "s", Trigger: scenariorun.TriggerManual}); err == nil {
		t.Fatal("missing ids accepted")
	}
	run := enqueue(t, store)
	step, err := store.StartStep(ctx, scenariorun.StepExecution{RunID: run.ID, NodeID: "fill_email", Attempt: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = store.FinishStep(ctx, step.ID, scenariorun.StepFailed, "", "timeout"); err != nil {
		t.Fatal(err)
	}
	attempt, err := store.NextStepAttempt(ctx, run.ID, "fill_email")
	if err != nil || attempt != 2 {
		t.Fatalf("retry attempt=%d err=%v", attempt, err)
	}
	retry, err := store.StartStep(ctx, scenariorun.StepExecution{RunID: run.ID, NodeID: "fill_email", Attempt: attempt})
	if err != nil {
		t.Fatal(err)
	}
	// Same (run,node,attempt) is unique.
	if _, err = store.StartStep(ctx, scenariorun.StepExecution{RunID: run.ID, NodeID: "fill_email", Attempt: 2}); err == nil {
		t.Fatal("duplicate attempt accepted")
	}
	if err = store.FinishStep(ctx, retry.ID, scenariorun.StepPassed, "{}", ""); err != nil {
		t.Fatal(err)
	}
	steps, err := store.Steps(ctx, run.ID)
	if err != nil || len(steps) != 2 || steps[0].Status != scenariorun.StepFailed || steps[1].Status != scenariorun.StepPassed {
		t.Fatalf("steps=%v", steps)
	}
	if _, err = store.RecordArtifact(ctx, scenariorun.Artifact{RunID: run.ID, Kind: "memory-dump", Ref: "x"}); err == nil {
		t.Fatal("invalid artifact kind accepted")
	}
	if err = store.FinishStep(ctx, step.ID, scenariorun.StepSkipped, "", ""); err == nil {
		t.Fatal("finished step re-finished")
	}
}
