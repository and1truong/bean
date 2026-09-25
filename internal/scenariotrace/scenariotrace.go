// Package scenariotrace converts a recorded run — its StepExecution rows and
// manual_action events — into an editable Scenario definition spec. The saved
// test replays the path the browser actually took: executed primitive nodes
// clone from the compiled scenario, takeover ops become nodes of the same
// kind, and control-flow nodes (branch/loop/pause) drop out — a trace is
// linear by nature.
package scenariotrace

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	beanscenario "github.com/beanruntime/bean/internal/scenario"
	"github.com/beanruntime/bean/internal/scenariogen"
	"github.com/beanruntime/bean/internal/scenariorun"
)

// recorded pairs one observed action with the time it happened.
type recorded struct {
	at   int64
	node map[string]any
}

// Spec builds a Scenario spec from one run's trace. Nodes are renamed
// step_1..step_n and chained linearly through next; the last node has no next.
// Refs emit the element's semantic name (stable across sessions) when the
// trace recorded one, falling back to the raw ref otherwise.
func Spec(run scenariorun.Run, scenario appir.Scenario, steps []scenariorun.StepExecution, events []scenariorun.Event) (map[string]any, error) {
	byID := map[string]appir.ScenarioNode{}
	for _, node := range scenario.Nodes {
		byID[node.ID] = node
	}
	items := []recorded{}
	for _, step := range steps {
		if step.Status != scenariorun.StepPassed && step.Status != scenariorun.StepFailed {
			continue
		}
		node, ok := byID[step.NodeID]
		if !ok || !primitive(node.Type) {
			continue
		}
		items = append(items, recorded{at: step.CreatedAt.UnixNano(), node: cloneNode(node)})
	}
	for _, event := range events {
		if event.Kind != scenariorun.EventManualAction {
			continue
		}
		node := manualNode(event)
		if node == nil {
			continue
		}
		items = append(items, recorded{at: event.CreatedAt.UnixNano(), node: node})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].at < items[j].at })
	nodes := dedup(items)
	for index := range nodes {
		nodes[index]["id"] = fmt.Sprintf("step_%d", index+1)
		if index+1 < len(nodes) {
			nodes[index]["next"] = fmt.Sprintf("step_%d", index+2)
		}
	}
	spec := map[string]any{
		"title":       scenarioTitle(run, scenario),
		"description": fmt.Sprintf("Saved from run %s of scenario %s.", run.ID, run.Scenario),
		"nodes":       nodes,
	}
	if len(nodes) > 0 {
		spec["start"] = nodes[0]["id"]
	}
	return spec, nil
}

// primitive reports whether a node type replays as a recorded trace step —
// control flow has no single recorded path and is dropped.
func primitive(nodeType string) bool {
	switch nodeType {
	case beanscenario.NodeBranch, beanscenario.NodeLoop, beanscenario.NodePause:
		return false
	}
	return beanscenario.ValidNodeType(nodeType)
}

// cloneNode copies a compiled node's behavior fields (never its edges).
func cloneNode(node appir.ScenarioNode) map[string]any {
	out := map[string]any{"type": node.Type}
	put := func(field, value string) {
		if value != "" {
			out[field] = value
		}
	}
	put("label", node.Label)
	put("url", node.URL)
	put("ref", node.Ref)
	put("text", node.Text)
	put("secret", node.Secret)
	put("value", node.Value)
	put("key", node.Key)
	put("condition", node.Condition)
	put("assertion", node.Assertion)
	put("as", node.As)
	put("attribute", node.Attribute)
	put("script", node.Script)
	put("action", node.Action)
	if len(node.Input) > 0 {
		out["input"] = node.Input
	}
	if node.TimeoutSeconds > 0 {
		out["timeoutSeconds"] = node.TimeoutSeconds
	}
	return out
}

// manualNode rebuilds a node from one manual_action event payload; nil for
// failed ops and observational ops (snapshot/screenshot) that move nothing.
func manualNode(event scenariorun.Event) map[string]any {
	var payload struct {
		Op        string `json:"op"`
		Ref       string `json:"ref"`
		Name      string `json:"name"`
		URL       string `json:"url"`
		Text      string `json:"text"`
		Secret    string `json:"secret"`
		Value     string `json:"value"`
		Key       string `json:"key"`
		Condition string `json:"condition"`
		As        string `json:"as"`
		Attribute string `json:"attribute"`
		OK        bool   `json:"ok"`
	}
	if err := json.Unmarshal([]byte(event.Payload), &payload); err != nil || !payload.OK {
		return nil
	}
	ref := payload.Name
	if ref == "" {
		ref = payload.Ref
	}
	node := map[string]any{}
	put := func(field, value string) {
		if value != "" {
			node[field] = value
		}
	}
	switch payload.Op {
	case "navigate":
		node["type"] = beanscenario.NodeNavigate
		put("url", payload.URL)
	case "click":
		node["type"] = beanscenario.NodeClick
		put("ref", ref)
	case "fill":
		node["type"] = beanscenario.NodeFill
		put("ref", ref)
		if payload.Secret != "" {
			put("secret", payload.Secret)
		} else {
			put("text", payload.Text)
		}
	case "select":
		node["type"] = beanscenario.NodeSelect
		put("ref", ref)
		put("value", payload.Value)
	case "press":
		node["type"] = beanscenario.NodePress
		put("key", payload.Key)
	case "wait":
		node["type"] = beanscenario.NodeWait
		put("condition", payload.Condition)
		put("ref", ref)
		put("text", payload.Text)
	case "extract":
		node["type"] = beanscenario.NodeExtract
		put("ref", ref)
		put("as", payload.As)
		put("attribute", payload.Attribute)
	default:
		return nil
	}
	node["label"] = "manual " + payload.Op
	return node
}

// dedup collapses consecutive identical nodes — the recorded trace often
// repeats a wait or assert around an action the human retried.
func dedup(items []recorded) []map[string]any {
	nodes := []map[string]any{}
	for _, item := range items {
		if len(nodes) > 0 {
			previous, _ := json.Marshal(nodes[len(nodes)-1])
			current, _ := json.Marshal(item.node)
			if string(previous) == string(current) {
				continue
			}
		}
		nodes = append(nodes, item.node)
	}
	return nodes
}

func scenarioTitle(run scenariorun.Run, scenario appir.Scenario) string {
	if scenario.Title != "" {
		return scenario.Title + " (saved run)"
	}
	return "Saved run " + scenariogen.Slug(run.Scenario)
}
