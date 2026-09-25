package browserapi_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/browserapi"
)

func TestRefRoundTrip(t *testing.T) {
	ref := browserapi.Ref{Snapshot: "snap7", ID: "e12"}
	parsed, err := browserapi.ParseRef(ref.String())
	if err != nil || parsed != ref {
		t.Fatalf("parsed=%+v err=%v", parsed, err)
	}
	bare, err := browserapi.ParseRef("e3")
	if err != nil || bare.ID != "e3" || bare.Snapshot != "" {
		t.Fatalf("bare=%+v err=%v", bare, err)
	}
	for _, invalid := range []string{"", "x1", "e0", "e-1", "e", ":e1", "snap:", "snap:e1:x", "e99999"} {
		if _, err := browserapi.ParseRef(invalid); err == nil {
			t.Fatalf("invalid ref %q accepted", invalid)
		}
	}
}

func TestSnapshotMintsRefsInOrder(t *testing.T) {
	snapshot, err := browserapi.NewSnapshot("s1", "https://app/login", "Login", []browserapi.Node{
		{Role: "heading", Name: "Sign in"},
		{Role: "textbox", Name: "Email"},
		{Role: "textbox", Name: "Password"},
		{Role: "button", Name: "Continue"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for index, node := range snapshot.Nodes {
		if want := browserapi.MintRef(index + 1); node.Ref != want {
			t.Fatalf("node %d ref=%s want=%s", index, node.Ref, want)
		}
	}
	encoded := snapshot.Encode()
	for _, line := range []string{
		`url: https://app/login`,
		`title: Login`,
		`[ref=e1] heading "Sign in"`,
		`[ref=e2] textbox "Email"`,
		`[ref=e4] button "Continue"`,
	} {
		if !strings.Contains(encoded, line) {
			t.Fatalf("encoded missing %q:\n%s", line, encoded)
		}
	}
	// Canonical JSON is deterministic: nodes sort by ref number.
	out, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `"ref":"e1"`) {
		t.Fatalf("serialized=%s", out)
	}
}

func TestSnapshotResolveAndStaleDetection(t *testing.T) {
	snapshot, err := browserapi.NewSnapshot("s2", "https://app", "App", []browserapi.Node{
		{Role: "button", Name: "Save", Disabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	ref, err := snapshot.Resolve("e1")
	if err != nil || ref.Snapshot != "s2" {
		t.Fatalf("ref=%+v err=%v", ref, err)
	}
	node, err := snapshot.Node(ref)
	if err != nil || node.Role != "button" || !node.Disabled {
		t.Fatalf("node=%+v err=%v", node, err)
	}
	if _, err := snapshot.Resolve("e9"); !errors.Is(err, browserapi.ErrUnknownRef) {
		t.Fatalf("err=%v", err)
	}
	stale := browserapi.Ref{Snapshot: "s1", ID: "e1"}
	if _, err := snapshot.Node(stale); !errors.Is(err, browserapi.ErrStaleRef) {
		t.Fatalf("stale err=%v", err)
	}
}

func TestSnapshotBounds(t *testing.T) {
	if _, err := browserapi.NewSnapshot("", "u", "t", nil); err == nil {
		t.Fatal("empty id accepted")
	}
	oversized := browserapi.Node{Role: "textbox", Name: strings.Repeat("x", browserapi.MaxNameRunes+1)}
	if _, err := browserapi.NewSnapshot("s", "u", "t", []browserapi.Node{oversized}); err == nil {
		t.Fatal("oversized name accepted")
	}
}

func TestConditionValidation(t *testing.T) {
	valid := []browserapi.Condition{
		{Kind: browserapi.ConditionNavigation},
		{Kind: browserapi.ConditionNetworkIdle},
		{Kind: browserapi.ConditionRefVisible, Ref: browserapi.Ref{Snapshot: "s", ID: "e1"}},
		{Kind: browserapi.ConditionTextPresent, Text: "Done"},
		{Kind: browserapi.ConditionURLContains, Text: "/dashboard", TimeoutMillis: 5_000},
	}
	for _, condition := range valid {
		if err := condition.Validate(); err != nil {
			t.Fatalf("condition %+v: %v", condition, err)
		}
	}
	invalid := []browserapi.Condition{
		{Kind: "scroll_bottom"},
		{Kind: browserapi.ConditionRefVisible},
		{Kind: browserapi.ConditionTextPresent},
		{Kind: browserapi.ConditionNavigation, TimeoutMillis: -1},
		{Kind: browserapi.ConditionNavigation, TimeoutMillis: browserapi.MaxWaitMillis + 1},
	}
	for _, condition := range invalid {
		if err := condition.Validate(); err == nil {
			t.Fatalf("condition %+v accepted", condition)
		}
	}
	condition := browserapi.Condition{Kind: browserapi.ConditionNavigation}
	if condition.Timeout().Milliseconds() != browserapi.DefaultWaitMillis {
		t.Fatalf("timeout=%v", condition.Timeout())
	}
}
