package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/scenariorun"
)

// runEvents serves the durable run log: the full history as JSON, or a live
// text/event-stream a client can subscribe to mid-run and resume by sequence.
func (s *Server) runEvents(w http.ResponseWriter, r *http.Request) {
	if !s.editor(w, r) {
		return
	}
	runID := r.PathValue("id")
	store := scenariorun.Store{DB: s.Actions.DB}
	run, found, err := store.Get(r.Context(), runID)
	if err != nil {
		respondError(w, r, err)
		return
	}
	if !found {
		problem(w, 404, "not_found", "Run not found.", requestID(r))
		return
	}
	after := int64(0)
	if lastEventID := r.Header.Get("Last-Event-ID"); lastEventID != "" {
		if parsed, err := strconv.ParseInt(lastEventID, 10, 64); err == nil && parsed > 0 {
			after = parsed
		}
	} else if parsed, err := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64); err == nil && parsed > 0 {
		after = parsed
	}
	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		streamRunEvents(w, r, store, run.ID, run, after)
		return
	}
	events, err := store.Events(r.Context(), run.ID, after)
	if err != nil {
		respondError(w, r, err)
		return
	}
	write(w, 200, map[string]any{"run": run.ID, "status": run.Status, "events": events})
}

// streamRunEvents replays persisted events after `after`, then polls the run
// log until the run reaches a terminal state and the backlog drains. Every
// SSE frame carries the event sequence as its id so a reconnecting client
// resumes exactly where it left off — never duplicated, never skipped.
func streamRunEvents(w http.ResponseWriter, r *http.Request, store scenariorun.Store, runID string, run scenariorun.Run, after int64) {
	headers := w.Header()
	headers.Set("Content-Type", "text/event-stream")
	headers.Set("Cache-Control", "no-cache")
	headers.Set("Connection", "keep-alive")
	w.WriteHeader(200)
	controller := http.NewResponseController(w)
	_ = controller.Flush()

	ctx := r.Context()
	sequence := after
	finished := terminalRun(run.Status)
	poll := time.NewTimer(0)
	heartbeat := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-poll.C:
		}
		events, err := store.Events(ctx, runID, sequence)
		if err != nil {
			return
		}
		for _, event := range events {
			_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", event.Sequence, event.Kind, event.Payload)
			sequence = event.Sequence
		}
		if len(events) > 0 {
			_ = controller.Flush()
		}
		if finished {
			_, _ = fmt.Fprint(w, "event: end\ndata: {}\n\n")
			_ = controller.Flush()
			return
		}
		if time.Since(heartbeat) >= 15*time.Second {
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			_ = controller.Flush()
			heartbeat = time.Now()
		}
		if refresh, found, err := store.Get(ctx, runID); err == nil && found {
			run = refresh
			finished = terminalRun(run.Status)
		}
		poll.Reset(200 * time.Millisecond)
	}
}

func terminalRun(status string) bool {
	switch status {
	case scenariorun.RunCompleted, scenariorun.RunFailed, scenariorun.RunCancelled:
		return true
	default:
		return false
	}
}
