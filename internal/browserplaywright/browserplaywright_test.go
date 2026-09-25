package browserplaywright_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/browserapi"
	"github.com/beanruntime/bean/internal/browserplaywright"
)

const loginPage = `<!doctype html><html><head><title>Sign in</title></head><body>
<main>
<h1>Sign in</h1>
<form id="login" action="/done" method="get">
<label for="email">Email</label>
<input id="email" name="email" type="email" placeholder="you@example.com">
<label for="password">Password</label>
<input id="password" name="password" type="password">
<label for="role">Role</label>
<select id="role" name="role"><option value="viewer">Viewer</option><option value="admin">Admin</option></select>
<button id="submit" type="submit">Continue</button>
</form>
</main>
<script>console.log("bean login page");fetch("/ping");</script>
</body></html>`

const donePage = `<!doctype html><html><head><title>Done</title></head><body><main><h1>Welcome</h1><p id="banner">Login accepted</p></main></body></html>`

func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ping" {
			w.WriteHeader(204)
			return
		}
		w.Header().Set("content-type", "text/html")
		if r.URL.Path == "/done" {
			fmt.Fprint(w, donePage)
			return
		}
		fmt.Fprint(w, loginPage)
	}))
	t.Cleanup(server.Close)
	return server
}

func newSession(t *testing.T) (browserapi.Session, context.Context) {
	t.Helper()
	if _, err := os.Stat("../../browser/sidecar.mjs"); err != nil {
		t.Skipf("sidecar source unavailable: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	session, err := (browserplaywright.Adapter{Dir: "../../browser"}).NewSession(ctx)
	if err != nil {
		t.Skipf("playwright sidecar unavailable in this environment: %v", err)
	}
	t.Cleanup(func() { session.Close(context.Background()) })
	return session, ctx
}

func refOf(t *testing.T, snapshot browserapi.Snapshot, name string) browserapi.Ref {
	t.Helper()
	for _, node := range snapshot.Nodes {
		if node.Name == name {
			ref, err := snapshot.Resolve(node.Ref)
			if err != nil {
				t.Fatal(err)
			}
			return ref
		}
	}
	t.Fatalf("no node named %q in snapshot:\n%s", name, snapshot.Encode())
	return browserapi.Ref{}
}

func TestSessionDrivesRealChromiumEndToEnd(t *testing.T) {
	server := newServer(t)
	session, ctx := newSession(t)

	result, err := session.Open(ctx, server.URL+"/login")
	if err != nil || result.URL == "" {
		t.Fatalf("open: %v %+v", err, result)
	}

	snapshot, err := session.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	encoded := snapshot.Encode()
	for _, want := range []string{`textbox "Email"`, `textbox "Password"`, `combobox "Role"`, `button "Continue"`, `heading "Sign in"`} {
		if !strings.Contains(encoded, want) {
			t.Fatalf("snapshot missing %q:\n%s", want, encoded)
		}
	}
	// Same snapshot twice on an unchanged page is deterministic.
	second, err := session.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var linesA, linesB []string
	for _, n := range snapshot.Nodes {
		linesA = append(linesA, n.Line())
	}
	for _, n := range second.Nodes {
		linesB = append(linesB, n.Line())
	}
	if len(linesA) != len(linesB) {
		t.Fatalf("node count differs: %d vs %d", len(linesA), len(linesB))
	}
	for index := range linesA {
		if linesA[index] != linesB[index] {
			t.Fatalf("snapshot not deterministic at %d: %q vs %q", index, linesA[index], linesB[index])
		}
	}
	// second snapshot minted a new generation — old refs are stale.
	stale := browserapi.Ref{Snapshot: snapshot.ID, ID: "e1"}
	if _, err = session.Click(ctx, stale); !errors.Is(err, browserapi.ErrStaleRef) {
		t.Fatalf("stale ref err=%v", err)
	}

	if _, err = session.Fill(ctx, refOf(t, second, "Email"), "runner@bean.dev"); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Fill(ctx, refOf(t, second, "Password"), "s3cret"); err != nil {
		t.Fatal(err)
	}
	if _, err = session.Select(ctx, refOf(t, second, "Role"), "admin"); err != nil {
		t.Fatal(err)
	}
	extracted, err := session.Extract(ctx, refOf(t, second, "Email"), "value", "")
	if err != nil || extracted.Value != "runner@bean.dev" {
		t.Fatalf("extract=%+v err=%v", extracted, err)
	}
	if _, err = session.Click(ctx, refOf(t, second, "Continue")); err != nil {
		t.Fatal(err)
	}
	waited, err := session.Wait(ctx, browserapi.Condition{Kind: browserapi.ConditionURLContains, Text: "/done"})
	if err != nil || !waited.Met {
		t.Fatalf("wait=%+v err=%v", waited, err)
	}
	waited, err = session.Wait(ctx, browserapi.Condition{Kind: browserapi.ConditionTextPresent, Text: "Login accepted"})
	if err != nil || !waited.Met {
		t.Fatalf("wait=%+v err=%v", waited, err)
	}
	capture, err := session.Screenshot(ctx)
	if err != nil || capture.ContentType != "image/png" || len(capture.Bytes) < 1000 {
		t.Fatalf("capture=%d bytes err=%v", len(capture.Bytes), err)
	}
	if err = session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if err = session.Close(ctx); err != nil {
		t.Fatal("close not idempotent")
	}
}

func TestUnknownRefAndWaitTimeout(t *testing.T) {
	server := newServer(t)
	session, ctx := newSession(t)
	if _, err := session.Open(ctx, server.URL); err != nil {
		t.Fatal(err)
	}
	snapshot, err := session.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = session.Click(ctx, browserapi.Ref{Snapshot: snapshot.ID, ID: "e999"}); !errors.Is(err, browserapi.ErrUnknownRef) {
		t.Fatalf("unknown ref err=%v", err)
	}
	_, err = session.Wait(ctx, browserapi.Condition{Kind: browserapi.ConditionTextPresent, Text: "never-appears", TimeoutMillis: 500})
	if err == nil {
		t.Fatal("wait did not time out")
	}
	var callErr *browserplaywright.CallError
	if !errors.As(err, &callErr) || callErr.Code != "timeout" {
		t.Fatalf("timeout error=%v", err)
	}
}

func TestSessionStreamsPageEventsAndTrace(t *testing.T) {
	server := newServer(t)
	session, ctx := newSession(t)

	deadline := time.Now().Add(30 * time.Second)
	var consoleSeen, networkSeen bool
	if _, err := session.Open(ctx, server.URL+"/login"); err != nil {
		t.Fatal(err)
	}
	for !(consoleSeen && networkSeen) && time.Now().Before(deadline) {
		select {
		case event, ok := <-session.Events():
			if !ok {
				t.Fatal("events channel closed mid-session")
			}
			switch event.Kind {
			case browserapi.EventConsole, browserapi.EventPageError:
				if strings.Contains(string(event.Data), "bean login page") {
					consoleSeen = true
				}
			case browserapi.EventRequest, browserapi.EventResponse, browserapi.EventRequestFailed:
				if strings.Contains(string(event.Data), "/ping") {
					networkSeen = true
				}
			}
		case <-ctx.Done():
			t.Fatal("timed out waiting for page events")
		}
	}
	if !consoleSeen || !networkSeen {
		t.Fatalf("console=%v network=%v", consoleSeen, networkSeen)
	}

	capture, err := session.Trace(ctx)
	if err != nil || capture.ContentType != "application/zip" || len(capture.Bytes) < 4 || string(capture.Bytes[:2]) != "PK" {
		t.Fatalf("trace=%d bytes ct=%q err=%v", len(capture.Bytes), capture.ContentType, err)
	}
	if _, err = session.Trace(ctx); err == nil {
		t.Fatal("second trace succeeded")
	}
}

func TestSidecarCrashFailsSessionInsteadOfHanging(t *testing.T) {
	// A sidecar that exits during the health check must surface as a spawn
	// error, never a hang.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := (browserplaywright.Adapter{Dir: "testdata/crasher"}).NewSession(ctx); err == nil {
		t.Fatal("crashing sidecar produced a live session")
	}
	// An abrupt Close leaves every later call failing fast.
	session, err := (browserplaywright.Adapter{Dir: "../../browser"}).NewSession(ctx)
	if err != nil {
		t.Skipf("sidecar unavailable: %v", err)
	}
	if err = session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	callCtx, callCancel := context.WithTimeout(ctx, 3*time.Second)
	defer callCancel()
	if _, err = session.Snapshot(callCtx); err == nil {
		t.Fatal("call on dead session succeeded")
	}
}

func TestAllowedDomainsBoundary(t *testing.T) {
	server := newServer(t)
	host := "127.0.0.1"
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Matching allowlist: navigation flows normally.
	allowed, err := (browserplaywright.Adapter{Dir: "../../browser", AllowedDomains: []string{host}}).NewSession(ctx)
	if err != nil {
		t.Skipf("sidecar unavailable: %v", err)
	}
	defer allowed.Close(context.Background())
	if _, err := allowed.Open(ctx, server.URL+"/login"); err != nil {
		t.Fatalf("allowed navigation blocked: %v", err)
	}

	// Non-matching allowlist: the request is aborted client-side and the
	// boundary emits a request_blocked observation.
	blocked, err := (browserplaywright.Adapter{Dir: "../../browser", AllowedDomains: []string{"other.example"}}).NewSession(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close(context.Background())
	if _, err := blocked.Open(ctx, server.URL+"/login"); err == nil {
		t.Fatal("navigation to a non-allowlisted host succeeded")
	}
	deadline := time.After(10 * time.Second)
	for {
		select {
		case event := <-blocked.Events():
			if event.Kind == browserapi.EventRequestBlocked {
				return
			}
		case <-deadline:
			t.Fatal("no request_blocked event")
		}
	}
}
