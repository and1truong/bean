// Package browserplaywright implements the Semantic Browser API
// (internal/browserapi) over a Playwright sidecar child process. Bean spawns
// `bun sidecar.mjs` inside a module directory that provides the playwright
// dependency, talks newline-delimited JSON RPC over stdio, and owns the
// process lifecycle: health-check at spawn, deterministic teardown at Close,
// and a dead session (never a hang) when the child exits unexpectedly.
//
// One sidecar process hosts one browser context for the lifetime of one
// Session — browser state (cookies, tabs, current URL) persists across the
// steps of a run and dies with the process, which is exactly what the run
// model's crash-recovery contract assumes.
package browserplaywright

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/beanruntime/bean/internal/browserapi"
)

// Adapter spawns Playwright sidecar sessions.
type Adapter struct {
	// Dir is the module directory containing sidecar.mjs and the playwright
	// dependency (browser/ in the repository, or a provisioned directory in
	// deployment). Defaults to "browser".
	Dir string
	// Command is the runtime executable. Defaults to "bun".
	Command string
	// Env overlays the spawned process environment.
	Env []string
	// AllowedDomains, when non-empty, installs the egress boundary on
	// the browser context: only requests to these hosts or their
	// subdomains are routed; the rest abort with a request_blocked event.
	AllowedDomains []string
}

// NewSession spawns a sidecar, health-checks it, and returns the session.
func (a Adapter) NewSession(ctx context.Context) (browserapi.Session, error) {
	dir := a.Dir
	if dir == "" {
		dir = "browser"
	}
	script, err := filepath.Abs(filepath.Join(dir, "sidecar.mjs"))
	if err != nil {
		return nil, err
	}
	if _, err = os.Stat(script); err != nil {
		return nil, fmt.Errorf("browserplaywright: sidecar not found: %w", err)
	}
	command := a.Command
	if command == "" {
		command = "bun"
	}
	cmd := exec.Command(command, script)
	cmd.Dir = filepath.Dir(script)
	cmd.Env = append(os.Environ(), a.Env...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("browserplaywright: spawn: %w", err)
	}
	session := &session{
		cmd:     cmd,
		stdin:   stdin,
		pending: map[uint64]chan response{},
		done:    make(chan struct{}),
		events:  make(chan browserapi.Event, 1024),
	}
	go session.readLoop(stdout)
	go session.waitLoop()
	if err = session.call(ctx, "ping", nil, &struct {
		OK bool `json:"ok"`
	}{}); err != nil {
		session.Close(context.Background())
		return nil, fmt.Errorf("browserplaywright: health check: %w", err)
	}
	if len(a.AllowedDomains) > 0 {
		if err = session.call(ctx, "configure", map[string]any{"allowed_domains": a.AllowedDomains}, &struct {
			OK bool `json:"ok"`
		}{}); err != nil {
			session.Close(context.Background())
			return nil, fmt.Errorf("browserplaywright: configure: %w", err)
		}
	}
	return session, nil
}

type wireRequest struct {
	ID     uint64 `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

type wireResponse struct {
	ID     uint64          `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *wireError      `json:"error"`
	Event  *wireEvent      `json:"event"`
}

// wireEvent is the unsolicited {"event":...} frame the sidecar pushes for
// page observations (console, network, page errors) while a session lives.
type wireEvent struct {
	Kind string          `json:"kind"`
	Time time.Time       `json:"time"`
	Data json.RawMessage `json:"data"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CallError is one sidecar-level failure. Code is the protocol code
// ("stale_ref", "unknown_ref", "timeout", "invalid", "crash", "internal").
type CallError struct {
	Code    string
	Message string
}

func (e *CallError) Error() string {
	return fmt.Sprintf("browserplaywright: %s: %s", e.Code, e.Message)
}

// ErrSessionDead reports that the sidecar process exited. The run model maps
// this to a deterministically failed session.
var ErrSessionDead = errors.New("browserplaywright: sidecar exited")

var _ error = (*CallError)(nil)

type response struct {
	result json.RawMessage
	err    error
}

type session struct {
	cmd   *exec.Cmd
	stdin io.Writer

	mu      sync.Mutex
	nextID  uint64
	pending map[uint64]chan response
	dead    error // set once the process exits or the read loop ends
	done    chan struct{}
	events  chan browserapi.Event
}

func (s *session) call(ctx context.Context, method string, params any, out any) error {
	if err := s.fatal(); err != nil {
		return err
	}
	s.mu.Lock()
	s.nextID++
	id := s.nextID
	channel := make(chan response, 1)
	s.pending[id] = channel
	s.mu.Unlock()
	envelope, err := json.Marshal(wireRequest{ID: id, Method: method, Params: params})
	if err != nil {
		return err
	}
	s.mu.Lock()
	if _, err = s.stdin.Write(append(envelope, '\n')); err != nil {
		delete(s.pending, id)
		s.mu.Unlock()
		return fmt.Errorf("browserplaywright: write: %w", err)
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pending, id)
		s.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case reply := <-channel:
		if reply.err != nil {
			return reply.err
		}
		if out != nil && len(reply.result) > 0 {
			return json.Unmarshal(reply.result, out)
		}
		return nil
	case <-s.done:
		if err = s.fatal(); err != nil {
			return err
		}
		return ErrSessionDead
	}
}

func (s *session) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 1<<20), 128<<20)
	for scanner.Scan() {
		var message wireResponse
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			continue
		}
		if message.Event != nil {
			event := browserapi.Event{Kind: message.Event.Kind, Data: message.Event.Data, Time: message.Event.Time}
			select {
			case s.events <- event:
			default: // a slow consumer must not stall the read loop
			}
			continue
		}
		s.mu.Lock()
		channel, found := s.pending[message.ID]
		if found {
			delete(s.pending, message.ID)
		}
		s.mu.Unlock()
		if !found {
			continue
		}
		if message.Error != nil {
			channel <- response{err: &CallError{Code: message.Error.Code, Message: message.Error.Message}}
		} else {
			channel <- response{result: message.Result}
		}
	}
	s.fail(ErrSessionDead)
	close(s.events)
}

func (s *session) waitLoop() {
	err := s.cmd.Wait()
	if err != nil {
		s.fail(fmt.Errorf("%w: %v", ErrSessionDead, err))
	} else {
		s.fail(ErrSessionDead)
	}
}

func (s *session) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dead == nil {
		s.dead = err
		close(s.done)
	}
	for id, channel := range s.pending {
		channel <- response{err: err}
		delete(s.pending, id)
	}
}

func (s *session) fatal() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dead
}

func (s *session) Open(ctx context.Context, url string) (browserapi.Result, error) {
	var result struct {
		URL       string `json:"url"`
		Navigated bool   `json:"navigated"`
	}
	if err := s.call(ctx, "open", map[string]any{"url": url}, &result); err != nil {
		return browserapi.Result{}, translate(err)
	}
	return browserapi.Result{URL: result.URL, Navigated: result.Navigated}, nil
}

func (s *session) Snapshot(ctx context.Context) (browserapi.Snapshot, error) {
	var raw struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Title string `json:"title"`
		Nodes []struct {
			Role     string            `json:"role"`
			Name     string            `json:"name"`
			Value    string            `json:"value"`
			Depth    int               `json:"depth"`
			Disabled bool              `json:"disabled"`
			Checked  *bool             `json:"checked"`
			Attrs    map[string]string `json:"attrs"`
		} `json:"nodes"`
	}
	if err := s.call(ctx, "snapshot", nil, &raw); err != nil {
		return browserapi.Snapshot{}, translate(err)
	}
	nodes := make([]browserapi.Node, len(raw.Nodes))
	for index, node := range raw.Nodes {
		nodes[index] = browserapi.Node{
			Role: node.Role, Name: node.Name, Value: node.Value, Depth: node.Depth,
			Disabled: node.Disabled, Checked: node.Checked, Attrs: node.Attrs,
		}
	}
	snapshot, err := browserapi.NewSnapshot(raw.ID, raw.URL, raw.Title, nodes)
	if err != nil {
		return browserapi.Snapshot{}, err
	}
	return snapshot, nil
}

func (s *session) interact(ctx context.Context, method string, params map[string]any) (browserapi.Result, error) {
	var result struct {
		URL       string `json:"url"`
		Navigated bool   `json:"navigated"`
	}
	if err := s.call(ctx, method, params, &result); err != nil {
		return browserapi.Result{}, translate(err)
	}
	return browserapi.Result{URL: result.URL, Navigated: result.Navigated}, nil
}

func (s *session) Click(ctx context.Context, ref browserapi.Ref) (browserapi.Result, error) {
	return s.interact(ctx, "click", map[string]any{"snapshot": ref.Snapshot, "id": ref.ID})
}

func (s *session) Fill(ctx context.Context, ref browserapi.Ref, text string) (browserapi.Result, error) {
	return s.interact(ctx, "fill", map[string]any{"snapshot": ref.Snapshot, "id": ref.ID, "text": text})
}

func (s *session) Select(ctx context.Context, ref browserapi.Ref, value string) (browserapi.Result, error) {
	return s.interact(ctx, "select", map[string]any{"snapshot": ref.Snapshot, "id": ref.ID, "value": value})
}

func (s *session) Press(ctx context.Context, key string) (browserapi.Result, error) {
	return s.interact(ctx, "press", map[string]any{"key": key})
}

func (s *session) Wait(ctx context.Context, condition browserapi.Condition) (browserapi.WaitResult, error) {
	var result struct {
		Met bool   `json:"met"`
		URL string `json:"url"`
	}
	params := map[string]any{
		"kind":           condition.Kind,
		"text":           condition.Text,
		"timeout_millis": condition.Timeout().Milliseconds(),
		"snapshot":       condition.Ref.Snapshot,
		"id":             condition.Ref.ID,
	}
	if err := s.call(ctx, "wait", params, &result); err != nil {
		return browserapi.WaitResult{}, translate(err)
	}
	return browserapi.WaitResult{Condition: condition, Met: result.Met, URL: result.URL}, nil
}

func (s *session) Extract(ctx context.Context, ref browserapi.Ref, as string, attribute string) (browserapi.Extraction, error) {
	var result struct {
		Value string `json:"value"`
	}
	params := map[string]any{"snapshot": ref.Snapshot, "id": ref.ID, "as": as, "attribute": attribute}
	if err := s.call(ctx, "extract", params, &result); err != nil {
		return browserapi.Extraction{}, translate(err)
	}
	return browserapi.Extraction{Ref: ref, As: as, Value: result.Value}, nil
}

func (s *session) Screenshot(ctx context.Context) (browserapi.Capture, error) {
	var result struct {
		ContentType string `json:"content_type"`
		BytesBase64 string `json:"bytes_base64"`
	}
	if err := s.call(ctx, "screenshot", nil, &result); err != nil {
		return browserapi.Capture{}, translate(err)
	}
	bytes, err := base64.StdEncoding.DecodeString(result.BytesBase64)
	if err != nil {
		return browserapi.Capture{}, err
	}
	return browserapi.Capture{ContentType: result.ContentType, Bytes: bytes}, nil
}

// Trace stops the session's tracing run and returns the zip. Only the first
// call captures; the trace also ends when the session closes.
func (s *session) Trace(ctx context.Context) (browserapi.Capture, error) {
	var result struct {
		ContentType string `json:"content_type"`
		BytesBase64 string `json:"bytes_base64"`
	}
	if err := s.call(ctx, "trace", nil, &result); err != nil {
		return browserapi.Capture{}, translate(err)
	}
	bytes, err := base64.StdEncoding.DecodeString(result.BytesBase64)
	if err != nil {
		return browserapi.Capture{}, err
	}
	return browserapi.Capture{ContentType: result.ContentType, Bytes: bytes}, nil
}

// Events returns the channel of page observations for this session. It is
// closed when the session ends (Close or process exit).
func (s *session) Events() <-chan browserapi.Event {
	return s.events
}

func (s *session) Close(ctx context.Context) error {
	if s.fatal() != nil {
		return nil
	}
	_ = s.call(ctx, "close", nil, nil)
	_ = s.stdinClose()
	s.fail(ErrSessionDead)
	if s.cmd.Process != nil {
		_ = s.cmd.Process.Kill()
	}
	return nil
}

func (s *session) stdinClose() error {
	if closer, ok := s.stdin.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// translate maps protocol error codes onto the contract's error semantics.
func translate(err error) error {
	var callErr *CallError
	if !errors.As(err, &callErr) {
		return err
	}
	switch callErr.Code {
	case "stale_ref":
		return fmt.Errorf("%w: %s", browserapi.ErrStaleRef, callErr.Message)
	case "unknown_ref":
		return fmt.Errorf("%w: %s", browserapi.ErrUnknownRef, callErr.Message)
	default:
		return err
	}
}
