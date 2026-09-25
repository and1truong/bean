package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/scenarioexec"
	"github.com/beanruntime/bean/internal/scenariogen"
	"github.com/beanruntime/bean/internal/scenariorun"
	"github.com/beanruntime/bean/internal/scenariotrace"
)

// scenarios lists the compiled scenarios of the active release — the
// definition-level authoring surface stays on /api/admin/definitions.
func (s *Server) scenarios(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	write(w, 200, a.Scenarios)
}

// createRun enqueues a run of a compiled scenario and wakes the runner.
func (s *Server) createRun(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	if s.Runner == nil {
		problem(w, 503, "runner_unavailable", "Scenario runner is not configured.", requestID(r))
		return
	}
	var body struct {
		Scenario string `json:"scenario"`
		Trigger  string `json:"trigger"`
	}
	if !decode(w, r, &body) {
		return
	}
	a, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	if _, ok = a.Scenarios[body.Scenario]; !ok {
		problem(w, 422, "invalid", fmt.Sprintf("scenario %q is not defined in the active release", body.Scenario), requestID(r))
		return
	}
	trigger := body.Trigger
	if strings.TrimSpace(trigger) == "" {
		trigger = scenariorun.TriggerAPI
	}
	if !scenariorun.ValidTrigger(trigger) {
		problem(w, 422, "invalid", fmt.Sprintf("trigger %q is not one of manual, api, generated", trigger), requestID(r))
		return
	}
	run, err := s.Runner.Store.Enqueue(r.Context(), scenariorun.Run{
		AppID: a.AppID, ReleaseID: a.ReleaseID, Scenario: body.Scenario, Trigger: trigger,
	})
	if err != nil {
		respondError(w, r, err)
		return
	}
	go func() { _ = s.Runner.RunOnce(context.Background()) }()
	write(w, 201, run)
}

// runs lists runs newest first, filtered by scenario/status/app.
func (s *Server) runs(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	query := r.URL.Query()
	if status := query.Get("status"); status != "" && !scenariorun.ValidRunStatus(status) {
		problem(w, 422, "invalid", fmt.Sprintf("status %q is not a run status", status), requestID(r))
		return
	}
	filter := scenariorun.RunFilter{
		AppID:    query.Get("app"),
		Scenario: query.Get("scenario"),
		Status:   query.Get("status"),
	}
	if limit, err := strconv.Atoi(query.Get("limit")); err == nil && limit > 0 {
		filter.Limit = limit
	}
	list, err := s.runStore().List(r.Context(), filter)
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]any{"runs": list})
}

// runDetail returns a run with its sessions, steps, and artifacts.
func (s *Server) runDetail(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	run, ok := s.findRun(w, r)
	if !ok {
		return
	}
	store := s.runStore()
	sessions, err := store.Sessions(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	steps, err := store.Steps(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	artifacts, err := store.Artifacts(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]any{"run": run, "sessions": sessions, "steps": steps, "artifacts": artifacts})
}

// runControl drives pause/resume/stop on a run.
func (s *Server) runControl(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	if s.Runner == nil {
		problem(w, 503, "runner_unavailable", "Scenario runner is not configured.", requestID(r))
		return
	}
	runID := r.PathValue("id")
	var err error
	switch r.PathValue("control") {
	case "pause":
		err = s.Runner.RequestPause(r.Context(), runID)
	case "resume":
		err = s.Runner.Resume(r.Context(), runID)
	case "stop":
		err = s.Runner.Stop(r.Context(), runID)
	default:
		problem(w, 404, "not_found", "Unknown run control.", requestID(r))
		return
	}
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			problem(w, 404, "not_found", err.Error(), requestID(r))
			return
		}
		problem(w, 409, "conflict", err.Error(), requestID(r))
		return
	}
	write(w, 200, map[string]string{"run": runID, "control": r.PathValue("control")})
}

// runManual drives one human browser op on a paused run's held session —
// the takeover surface. The op lands in the run log as a manual_action
// event; the response carries the op's JSON result (e.g. an encoded
// snapshot for "snapshot", a base64 png for "screenshot").
func (s *Server) runManual(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	if s.Runner == nil {
		problem(w, 503, "runner_unavailable", "Scenario runner is not configured.", requestID(r))
		return
	}
	var body scenarioexec.Manual
	if !decode(w, r, &body) {
		return
	}
	result, err := s.Runner.Manual(r.Context(), r.PathValue("id"), body)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			problem(w, 404, "not_found", err.Error(), requestID(r))
			return
		}
		problem(w, 409, "conflict", err.Error(), requestID(r))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	_, _ = w.Write([]byte(result))
}

// runArtifact serves a persisted evidence file (screenshot, DOM
// snapshot, trace) recorded for the run.
func (s *Server) runArtifact(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	run, ok := s.findRun(w, r)
	if !ok {
		return
	}
	artifacts, err := s.runStore().Artifacts(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	var artifact *scenariorun.Artifact
	for i := range artifacts {
		if artifacts[i].ID == r.PathValue("artifact") {
			artifact = &artifacts[i]
			break
		}
	}
	if artifact == nil {
		problem(w, 404, "not_found", "Artifact not found.", requestID(r))
		return
	}
	dir := ""
	if s.Runner != nil {
		dir = s.Runner.ArtifactDir
	}
	root := scenarioexec.Executor{ArtifactDir: dir}.ArtifactRoot()
	rel := filepath.Clean(filepath.FromSlash(artifact.Ref))
	if filepath.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		problem(w, 422, "invalid", "Artifact reference is invalid.", requestID(r))
		return
	}
	path := filepath.Join(root, rel)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		problem(w, 404, "not_found", "Artifact file is missing.", requestID(r))
		return
	}
	if artifact.ContentType != "" {
		w.Header().Set("Content-Type", artifact.ContentType)
	}
	http.ServeFile(w, r, path)
}

type scenarioGenerateRequest struct {
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

// scenarioGenerate drafts a Scenario spec from a natural-language prompt. The
// draft is validated against the active release before it is returned — it is
// never saved; the caller persists it through the normal definition flow.
func (s *Server) scenarioGenerate(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	var input scenarioGenerateRequest
	if !decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Prompt) == "" {
		problem(w, 400, "invalid_request", "prompt is required", requestID(r))
		return
	}
	if s.Generator == nil {
		problem(w, 503, "not_configured", "Scenario generation is not configured (set BEAN_ANTHROPIC_API_KEY).", requestID(r))
		return
	}
	active, ok := s.Kernel.Active()
	if !ok {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = scenariogen.Slug(input.Prompt)
	}
	spec, err := scenariogen.Draft(r.Context(), s.Generator, active, name, input.Prompt)
	if err != nil {
		var draftErr *scenariogen.DraftError
		if errors.As(err, &draftErr) {
			write(w, 422, map[string]any{"valid": false, "diagnostics": draftErr.Diagnostics})
			return
		}
		problem(w, 502, "generation_failed", err.Error(), requestID(r))
		return
	}
	write(w, 200, map[string]any{"valid": true, "name": name, "spec": spec})
}

// repairRun drafts a corrected scenario spec for a failed run: the authored
// spec plus a bounded failure summary (run error, failing steps, recent
// events) drive scenariogen.Repair — the same compile-checked draft loop as
// scenario-generate. The response carries a node-level graph diff for review;
// the saved scenario is replaced only through the normal definition flow.
func (s *Server) repairRun(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	run, ok := s.findRun(w, r)
	if !ok {
		return
	}
	if run.Status != scenariorun.RunFailed {
		problem(w, 400, "invalid_request", "Repair is only available for failed runs.", requestID(r))
		return
	}
	if s.Generator == nil {
		problem(w, 503, "not_configured", "Scenario generation is not configured (set BEAN_ANTHROPIC_API_KEY).", requestID(r))
		return
	}
	active, exists := s.Kernel.Active()
	if !exists {
		problem(w, 503, "not_ready", "No active release.", requestID(r))
		return
	}
	defs, err := s.Store.Draft(r.Context(), "default")
	if err != nil {
		respondError(w, r, err)
		return
	}
	var original map[string]any
	for _, item := range defs {
		if item.Kind == "Scenario" && item.Metadata.Name == run.Scenario {
			original = item.Spec
		}
	}
	if original == nil {
		problem(w, 404, "not_found", "Scenario definition not found.", requestID(r))
		return
	}
	steps, err := s.runStore().Steps(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	spec, err := scenariogen.Repair(r.Context(), s.Generator, active, run.Scenario, original, failureContext(run, steps))
	if err != nil {
		var draftErr *scenariogen.DraftError
		if errors.As(err, &draftErr) {
			write(w, 422, map[string]any{"valid": false, "diagnostics": draftErr.Diagnostics})
			return
		}
		problem(w, 502, "generation_failed", err.Error(), requestID(r))
		return
	}
	write(w, 200, map[string]any{"valid": true, "name": run.Scenario, "spec": spec, "diff": scenariogen.DiffSpecs(original, spec)})
}

// failureContext summarizes a failed run for the repair prompt: the run-level
// error plus each failed step's node, error, and output.
func failureContext(run scenariorun.Run, steps []scenariorun.StepExecution) string {
	var b strings.Builder
	if run.Error != "" {
		fmt.Fprintf(&b, "run error: %s\n", run.Error)
	}
	for _, step := range steps {
		if step.Status != scenariorun.StepFailed {
			continue
		}
		fmt.Fprintf(&b, "step %q failed: %s", step.NodeID, step.Error)
		if step.Output != "" {
			fmt.Fprintf(&b, " (output: %s)", step.Output)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// saveAsTest converts a run's recorded trace — executed primitive steps
// plus manual takeover ops — into an editable Scenario spec draft. Like
// scenario-generate the draft is returned, never saved; the caller reviews
// it in the graph editor and persists through the normal definition flow.
func (s *Server) saveAsTest(w http.ResponseWriter, r *http.Request) {
	if !s.editorMutation(w, r) {
		return
	}
	run, ok := s.findRun(w, r)
	if !ok {
		return
	}
	// Resolve the scenario from the release the run executed under —
	// a later activation must not change which compiled graph the
	// recorded step NodeIDs map to. Fall back to the active release
	// (or none) only when the pinned one is gone.
	var scenario appir.Scenario
	if pinned, err := s.Store.AppByRelease(r.Context(), run.ReleaseID); err == nil {
		scenario = pinned.Scenarios[run.Scenario]
	} else if active, exists := s.Kernel.Active(); exists {
		scenario = active.Scenarios[run.Scenario]
	}
	store := s.runStore()
	steps, err := store.Steps(r.Context(), run.ID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	events, err := store.Events(r.Context(), run.ID, 0)
	if err != nil {
		respondError(w, r, err)
		return
	}
	spec, err := scenariotrace.Spec(run, scenario, steps, events)
	if err != nil {
		respondError(w, r, err)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		name = scenariogen.Slug(run.Scenario + " saved run")
	}
	// Compile-check the draft like scenario-generate does — `valid`
	// reflects whether the spec saves cleanly, while the draft itself
	// is still returned for the editor to repair. The draft lands in
	// current definitions and publishes into the next release, so
	// validity is checked against the active application, not the
	// release the run was pinned to.
	var app *appir.App
	if active, exists := s.Kernel.Active(); exists {
		app = active
	} else if pinned, err := s.Store.AppByRelease(r.Context(), run.ReleaseID); err == nil {
		app = pinned
	}
	result := compiler.CompileScenarioCandidate(app, name, spec)
	if len(result.Diagnostics) > 0 {
		write(w, 200, map[string]any{"valid": false, "name": name, "spec": spec, "diagnostics": result.Diagnostics})
		return
	}
	write(w, 200, map[string]any{"valid": true, "name": name, "spec": spec})
}

func (s *Server) runStore() scenariorun.Store {
	return scenariorun.Store{DB: s.Actions.DB}
}

func (s *Server) findRun(w http.ResponseWriter, r *http.Request) (scenariorun.Run, bool) {
	run, found, err := s.runStore().Get(r.Context(), r.PathValue("id"))
	if err != nil {
		respondError(w, r, err)
		return scenariorun.Run{}, false
	}
	if !found {
		problem(w, 404, "not_found", "Run not found.", requestID(r))
		return scenariorun.Run{}, false
	}
	return run, true
}
