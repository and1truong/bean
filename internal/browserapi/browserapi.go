// Package browserapi defines the Semantic Browser API: the runtime-agnostic
// boundary that both saved scenario executions and interactive browser agents
// call. Callers address page elements through snapshot-scoped semantic refs —
// never adapter-specific selectors — so a Playwright, CDP, or future adapter
// implementation cannot leak into either caller.
//
// Ref stability contract: every Ref is minted by one Snapshot and carries that
// snapshot's ID. Interactions must be issued with a Ref from the most recent
// Snapshot; adapters return ErrStaleRef for a ref minted by an older snapshot
// (the page moved on and re-resolution is the caller's responsibility) and
// ErrUnknownRef for a ref that the snapshot never contained.
package browserapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Bounds shared by every adapter and caller.
const (
	MaxURLBytes       = 1 << 16
	MaxNameRunes      = 512
	MaxValueRunes     = 4096
	MaxAttrRunes      = 512
	MaxAttrsPerNode   = 16
	MaxNodesPerSnap   = 4096
	MaxKeyRunes       = 64
	MaxTextRunes      = 4096
	MaxExtractRunes   = 1 << 16
	MaxCaptureBytes   = 32 << 20
	DefaultWaitMillis = 30_000
	MaxWaitMillis     = 300_000
	refPrefix         = "e"
)

var (
	// ErrStaleRef reports a ref minted by an older snapshot. The page has
	// changed; the caller must take a fresh Snapshot and re-resolve.
	ErrStaleRef = errors.New("browserapi: ref belongs to an older snapshot")
	// ErrUnknownRef reports a ref that the current snapshot never contained.
	ErrUnknownRef = errors.New("browserapi: ref not present in snapshot")
)

// Ref addresses one node in one snapshot generation. The serialized form
// "<snapshot>:e<n>" (e.g. "snap7:e12") is the contract stored on scenario
// step executions and agent transcripts.
type Ref struct {
	Snapshot string `json:"snapshot"`
	ID       string `json:"id"`
}

func (r Ref) String() string {
	if r.Snapshot == "" {
		return r.ID
	}
	return r.Snapshot + ":" + r.ID
}

// ParseRef decodes the serialized form produced by String.
func ParseRef(value string) (Ref, error) {
	snapshot, id, found := strings.Cut(value, ":")
	if !found {
		return Ref{ID: value}, validateRefID(value)
	}
	if snapshot == "" {
		return Ref{}, fmt.Errorf("browserapi: empty snapshot in ref %q", value)
	}
	return Ref{Snapshot: snapshot, ID: id}, validateRefID(id)
}

func validateRefID(id string) error {
	if !strings.HasPrefix(id, refPrefix) {
		return fmt.Errorf("browserapi: malformed ref %q", id)
	}
	n, err := strconv.Atoi(strings.TrimPrefix(id, refPrefix))
	if err != nil || n < 1 || n > MaxNodesPerSnap {
		return fmt.Errorf("browserapi: malformed ref %q", id)
	}
	return nil
}

// MintRef formats the ref ID for the n-th node (1-based) of a snapshot.
func MintRef(index int) string { return refPrefix + strconv.Itoa(index) }

// Condition kinds for Wait and for assert/branch checks. The vocabulary is a
// closed set: adapters reject anything else rather than interpreting it.
const (
	ConditionNavigation  = "navigation"
	ConditionNetworkIdle = "network_idle"
	ConditionRefVisible  = "ref_visible"
	ConditionRefHidden   = "ref_hidden"
	ConditionTextPresent = "text_present"
	ConditionURLEquals   = "url_equals"
	ConditionURLContains = "url_contains"
)

var conditionKinds = map[string]bool{
	ConditionNavigation: true, ConditionNetworkIdle: true,
	ConditionRefVisible: true, ConditionRefHidden: true, ConditionTextPresent: true,
	ConditionURLEquals: true, ConditionURLContains: true,
}

// ConditionNeedsRef reports whether the condition targets an element ref.
func ConditionNeedsRef(kind string) bool {
	return kind == ConditionRefVisible || kind == ConditionRefHidden
}

// ConditionNeedsText reports whether the condition reads an expected text or URL.
func ConditionNeedsText(kind string) bool {
	return kind == ConditionTextPresent || kind == ConditionURLEquals || kind == ConditionURLContains
}

// Condition is one normalized wait/assert check.
type Condition struct {
	Kind          string `json:"kind"`
	Ref           Ref    `json:"ref,omitempty"`
	Text          string `json:"text,omitempty"`
	TimeoutMillis int64  `json:"timeout_millis,omitempty"`
}

// Validate checks the closed condition contract before an adapter sees it.
func (c Condition) Validate() error {
	if !conditionKinds[c.Kind] {
		return fmt.Errorf("browserapi: invalid condition kind %q", c.Kind)
	}
	if ConditionNeedsRef(c.Kind) && c.Ref.ID == "" {
		return fmt.Errorf("browserapi: condition %q requires ref", c.Kind)
	}
	if ConditionNeedsText(c.Kind) && strings.TrimSpace(c.Text) == "" {
		return fmt.Errorf("browserapi: condition %q requires text", c.Kind)
	}
	if c.TimeoutMillis < 0 || c.TimeoutMillis > MaxWaitMillis {
		return fmt.Errorf("browserapi: condition timeout %d out of bounds", c.TimeoutMillis)
	}
	return nil
}

// Timeout returns the effective wait deadline.
func (c Condition) Timeout() time.Duration {
	if c.TimeoutMillis <= 0 {
		return time.Duration(DefaultWaitMillis) * time.Millisecond
	}
	return time.Duration(c.TimeoutMillis) * time.Millisecond
}

// Node is one accessibility entry inside a Snapshot. Role is the semantic role
// (button, textbox, link, heading, ...), Name the accessible name, Value the
// current control value when one exists.
type Node struct {
	Ref      string            `json:"ref"`
	Role     string            `json:"role"`
	Name     string            `json:"name"`
	Value    string            `json:"value,omitempty"`
	Depth    int               `json:"depth"`
	Disabled bool              `json:"disabled,omitempty"`
	Checked  *bool             `json:"checked,omitempty"`
	Attrs    map[string]string `json:"attrs,omitempty"`
}

// Line renders the canonical diffable form: `[ref=e12] button "Continue"`.
func (n Node) Line() string {
	var b strings.Builder
	fmt.Fprintf(&b, "[ref=%s] %s %q", n.Ref, n.Role, n.Name)
	if n.Value != "" {
		fmt.Fprintf(&b, " value=%q", n.Value)
	}
	if n.Disabled {
		b.WriteString(" disabled")
	}
	if n.Checked != nil {
		fmt.Fprintf(&b, " checked=%t", *n.Checked)
	}
	if len(n.Attrs) > 0 {
		keys := make([]string, 0, len(n.Attrs))
		for key := range n.Attrs {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fmt.Fprintf(&b, " %s=%q", key, n.Attrs[key])
		}
	}
	return b.String()
}

// Snapshot is a normalized, deterministic page view. Nodes are ordered by ref
// number, so Encode output is stable enough to diff across runs.
type Snapshot struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title"`
	Nodes []Node `json:"nodes"`
}

// NewSnapshot builds a normalized snapshot from unordered nodes, minting refs
// in document order as supplied by the adapter.
func NewSnapshot(id, url, title string, nodes []Node) (Snapshot, error) {
	if strings.TrimSpace(id) == "" {
		return Snapshot{}, fmt.Errorf("browserapi: snapshot requires id")
	}
	if len(nodes) > MaxNodesPerSnap {
		return Snapshot{}, fmt.Errorf("browserapi: snapshot of %d nodes exceeds bound %d", len(nodes), MaxNodesPerSnap)
	}
	snapshot := Snapshot{ID: id, URL: url, Title: title, Nodes: make([]Node, len(nodes))}
	for index, node := range nodes {
		node.Ref = MintRef(index + 1)
		if utf8.RuneCountInString(node.Name) > MaxNameRunes ||
			utf8.RuneCountInString(node.Value) > MaxValueRunes ||
			len(node.Attrs) > MaxAttrsPerNode {
			return Snapshot{}, fmt.Errorf("browserapi: node %s exceeds bounds", node.Ref)
		}
		for key, value := range node.Attrs {
			if utf8.RuneCountInString(key) > MaxAttrRunes || utf8.RuneCountInString(value) > MaxValueRunes {
				return Snapshot{}, fmt.Errorf("browserapi: node %s attr %q exceeds bounds", node.Ref, key)
			}
		}
		snapshot.Nodes[index] = node
	}
	return snapshot, nil
}

// Resolve returns the snapshot-scoped Ref for a raw ID, or ErrUnknownRef.
func (s Snapshot) Resolve(id string) (Ref, error) {
	if err := validateRefID(id); err != nil {
		return Ref{}, err
	}
	index, _ := strconv.Atoi(strings.TrimPrefix(id, refPrefix))
	if index < 1 || index > len(s.Nodes) {
		return Ref{}, ErrUnknownRef
	}
	return Ref{Snapshot: s.ID, ID: id}, nil
}

// Node returns the node behind a ref minted by this snapshot.
func (s Snapshot) Node(ref Ref) (Node, error) {
	if ref.Snapshot != "" && ref.Snapshot != s.ID {
		return Node{}, ErrStaleRef
	}
	index, err := strconv.Atoi(strings.TrimPrefix(ref.ID, refPrefix))
	if err != nil || index < 1 || index > len(s.Nodes) {
		return Node{}, ErrUnknownRef
	}
	return s.Nodes[index-1], nil
}

// Encode renders the canonical line format used for debugging diffs.
func (s Snapshot) Encode() string {
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s\ntitle: %s\n", s.URL, s.Title)
	for _, node := range s.Nodes {
		b.WriteString(node.Line())
		b.WriteString("\n")
	}
	return b.String()
}

// MarshalJSON keeps serialization canonical: nodes sorted by ref number.
func (s Snapshot) MarshalJSON() ([]byte, error) {
	type wire struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Title string `json:"title"`
		Nodes []Node `json:"nodes"`
	}
	nodes := append([]Node(nil), s.Nodes...)
	sort.SliceStable(nodes, func(i, j int) bool {
		return refIndex(nodes[i].Ref) < refIndex(nodes[j].Ref)
	})
	return json.Marshal(wire{ID: s.ID, URL: s.URL, Title: s.Title, Nodes: nodes})
}

func refIndex(ref string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(ref, refPrefix))
	return n
}

// Result is the common outcome of a mutating interaction: the location the
// page settled on and whether the interaction caused navigation.
type Result struct {
	URL       string `json:"url"`
	Navigated bool   `json:"navigated,omitempty"`
}

// WaitResult reports whether a condition was met before its deadline.
type WaitResult struct {
	Condition Condition `json:"condition"`
	Met       bool      `json:"met"`
	URL       string    `json:"url"`
}

// Extraction is the value pulled by an extract primitive.
type Extraction struct {
	Ref   Ref    `json:"ref"`
	As    string `json:"as"` // text | value | attribute
	Value string `json:"value"`
}

// Extraction kinds.
const (
	ExtractText      = "text"
	ExtractValue     = "value"
	ExtractAttribute = "attribute"
)

// Capture is binary evidence returned by a session (screenshot, trace, ...).
// Persistence as a run artifact is the run layer's job.
type Capture struct {
	ContentType string `json:"content_type"`
	Bytes       []byte `json:"bytes"`
}

// Session is the Semantic Browser API surface. One Session is one browser
// context bound to one caller (a scenario StepExecution driver or an agent).
// Every method takes and returns only the serialized contract types defined in
// this package — adapter types never cross this boundary.
type Session interface {
	// Open navigates to url and waits for the initial load.
	Open(ctx context.Context, url string) (Result, error)
	// Snapshot returns the current normalized page view, minting a new
	// snapshot generation and invalidating refs from older ones.
	Snapshot(ctx context.Context) (Snapshot, error)
	// Click activates the element behind ref.
	Click(ctx context.Context, ref Ref) (Result, error)
	// Fill replaces the value of the editable element behind ref.
	Fill(ctx context.Context, ref Ref, text string) (Result, error)
	// Select picks option value on the select element behind ref.
	Select(ctx context.Context, ref Ref, value string) (Result, error)
	// Press dispatches a normalized key (e.g. "Enter", "Tab").
	Press(ctx context.Context, key string) (Result, error)
	// Wait blocks until the condition is met or its timeout elapses.
	Wait(ctx context.Context, condition Condition) (WaitResult, error)
	// Extract reads a value from the element behind ref.
	Extract(ctx context.Context, ref Ref, as string, attribute string) (Extraction, error)
	// Screenshot captures the current viewport.
	Screenshot(ctx context.Context) (Capture, error)
	// Close releases the browser context. Idempotent.
	Close(ctx context.Context) error
}
