package httpapi

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/scenariorun"
)

func streamStore(t *testing.T) (scenariorun.Store, scenariorun.Run) {
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
	run, err := store.Enqueue(context.Background(), scenariorun.Run{AppID: "demo", ReleaseID: "rel-1", Scenario: "s", Trigger: scenariorun.TriggerManual})
	if err != nil {
		t.Fatal(err)
	}
	return store, run
}

func TestStreamRunEventsReplaysHistoryAndEnds(t *testing.T) {
	store, run := streamStore(t)
	ctx := context.Background()
	if err := store.RecordEvent(ctx, run.ID, "", scenariorun.EventConsole, `{"text":"hello"}`); err != nil {
		t.Fatal(err)
	}
	run.Status = scenariorun.RunCompleted

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/scenario-runs/"+run.ID+"/events", nil)
	streamRunEvents(recorder, request, store, run.ID, run, 0)

	body := recorder.Body.String()
	if recorder.Header().Get("Content-Type") != "text/event-stream" {
		t.Fatalf("content-type=%q", recorder.Header().Get("Content-Type"))
	}
	if !strings.Contains(body, "id: 1\nevent: run_enqueued") || !strings.Contains(body, "id: 2\nevent: console_event\ndata: {\"text\":\"hello\"}") {
		t.Fatalf("missing replayed events:\n%s", body)
	}
	if !strings.HasSuffix(body, "event: end\ndata: {}\n\n") {
		t.Fatalf("stream did not terminate:\n%s", body)
	}
}

func TestStreamRunEventsResumesAfterSequence(t *testing.T) {
	store, run := streamStore(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if err := store.RecordEvent(ctx, run.ID, "", scenariorun.EventConsole, `{"n":`+strconv.Itoa(i)+`}`); err != nil {
			t.Fatal(err)
		}
	}
	run.Status = scenariorun.RunCompleted

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/scenario-runs/"+run.ID+"/events", nil)
	streamRunEvents(recorder, request, store, run.ID, run, 2)

	body := recorder.Body.String()
	if strings.Contains(body, "id: 1\n") || strings.Contains(body, "id: 2\n") {
		t.Fatalf("replay leaked earlier events:\n%s", body)
	}
	if !strings.Contains(body, "id: 3\nevent: console_event") || !strings.Contains(body, "id: 4\nevent: console_event") {
		t.Fatalf("missing resumed events:\n%s", body)
	}
}
