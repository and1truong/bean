package scenariotrace_test

import (
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/scenariorun"
	"github.com/beanruntime/bean/internal/scenariotrace"
)

func TestSpecReplaysExecutedPathWithManualOps(t *testing.T) {
	scenario := appir.Scenario{
		Name: "login", Title: "Login", Start: "nav",
		Nodes: []appir.ScenarioNode{
			{ID: "nav", Type: "navigate", URL: "http://app.test/login", Next: "branch"},
			{ID: "branch", Type: "branch", Branches: []appir.ScenarioBranch{{Condition: "last_step_passed", Next: "assert"}}},
			{ID: "assert", Type: "assert", Assertion: "text_present", Text: "Welcome"},
			{ID: "hold", Type: "pause"},
		},
	}
	run := scenariorun.Run{ID: "run_1", Scenario: "login"}
	base := time.Unix(1000, 0)
	steps := []scenariorun.StepExecution{
		{NodeID: "nav", Status: scenariorun.StepPassed, CreatedAt: base},
		{NodeID: "branch", Status: scenariorun.StepPassed, CreatedAt: base.Add(time.Second)},
		{NodeID: "hold", Status: scenariorun.StepSkipped, CreatedAt: base.Add(2 * time.Second)},
		{NodeID: "ghost", Status: scenariorun.StepPassed, CreatedAt: base.Add(3 * time.Second)},
	}
	events := []scenariorun.Event{
		{Kind: scenariorun.EventManualAction, CreatedAt: base.Add(4 * time.Second), Payload: `{"op":"fill","ref":"e3","name":"Email","text":"a@b.c","ok":true,"result":"{}"}`},
		{Kind: scenariorun.EventManualAction, CreatedAt: base.Add(5 * time.Second), Payload: `{"op":"click","ref":"e4","name":"Sign in","ok":false,"result":"boom"}`},
		{Kind: scenariorun.EventManualAction, CreatedAt: base.Add(6 * time.Second), Payload: `{"op":"snapshot","ok":true,"result":"{}"}`},
		{Kind: scenariorun.EventBrowserSnapshot, CreatedAt: base.Add(7 * time.Second), Payload: `{}`},
	}
	spec, err := scenariotrace.Spec(run, scenario, steps, events)
	if err != nil {
		t.Fatal(err)
	}
	if spec["title"] != "Login (saved run)" || spec["start"] != "step_1" {
		t.Fatalf("spec=%v", spec)
	}
	nodes, _ := spec["nodes"].([]map[string]any)
	if len(nodes) != 2 {
		t.Fatalf("nodes=%v", nodes)
	}
	if nodes[0]["type"] != "navigate" || nodes[0]["url"] != "http://app.test/login" || nodes[0]["next"] != "step_2" {
		t.Fatalf("nav=%v", nodes[0])
	}
	// The branch control node, the skipped pause, the unknown ghost step,
	// the failed manual click, and observational ops all drop out.
	if nodes[1]["type"] != "fill" || nodes[1]["ref"] != "Email" || nodes[1]["text"] != "a@b.c" || nodes[1]["label"] != "manual fill" {
		t.Fatalf("fill=%v", nodes[1])
	}
	if _, hasNext := nodes[1]["next"]; hasNext {
		t.Fatalf("last node keeps a next edge: %v", nodes[1])
	}
}

func TestSpecDedupsConsecutiveIdenticalNodes(t *testing.T) {
	scenario := appir.Scenario{
		Name: "poll", Start: "wait",
		Nodes: []appir.ScenarioNode{
			{ID: "wait_a", Type: "wait", Condition: "text_present", Text: "Done"},
			{ID: "wait_b", Type: "wait", Condition: "text_present", Text: "Done"},
			{ID: "click", Type: "click", Ref: "Save"},
		},
	}
	run := scenariorun.Run{ID: "run_2", Scenario: "poll"}
	base := time.Unix(1000, 0)
	steps := []scenariorun.StepExecution{
		{NodeID: "wait_a", Status: scenariorun.StepPassed, CreatedAt: base},
		{NodeID: "wait_b", Status: scenariorun.StepPassed, CreatedAt: base.Add(time.Second)},
		{NodeID: "click", Status: scenariorun.StepPassed, CreatedAt: base.Add(2 * time.Second)},
	}
	spec, err := scenariotrace.Spec(run, scenario, steps, nil)
	if err != nil {
		t.Fatal(err)
	}
	nodes, _ := spec["nodes"].([]map[string]any)
	if len(nodes) != 2 || nodes[0]["type"] != "wait" || nodes[1]["type"] != "click" {
		t.Fatalf("nodes=%v", nodes)
	}
}
