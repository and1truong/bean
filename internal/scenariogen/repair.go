package scenariogen

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
)

// maxFailureRunes bounds the failure context handed to the model.
const maxFailureRunes = 4096

// Repair drafts a corrected scenario spec for a failed run: the authored
// original and a bounded failure summary go into the prompt, the output runs
// through the same generate -> compile -> feedback loop as Draft, and the
// result is stamped with repair provenance. The spec is a candidate only —
// it replaces the saved scenario solely through the normal definition flow.
func Repair(ctx context.Context, gen Generator, active *appir.App, name string, original map[string]any, failure string) (map[string]any, error) {
	spec, err := draftLoop(ctx, gen, active, name, repairPrompt(original, failure))
	if err != nil {
		return nil, err
	}
	provenance := "Agent-proposed repair of a failed run — review the diff before saving."
	if existing, _ := spec["description"].(string); existing != "" && !strings.HasPrefix(existing, "Agent-") {
		provenance += "\n\n" + existing
	}
	spec["description"] = provenance
	return spec, nil
}

func repairPrompt(original map[string]any, failure string) string {
	encoded, _ := json.Marshal(original)
	if len(failure) > maxFailureRunes {
		failure = failure[:maxFailureRunes]
	}
	return fmt.Sprintf("Repair this browser-test scenario. It failed in a real run — fix the node(s) that failed or add missing waits; keep the rest of the graph unchanged. Return only the corrected scenario spec as JSON.\n\nCurrent spec:\n%s\n\nFailure:\n%s", string(encoded), failure)
}

// maxDiffLines bounds the node-diff summary reported to the reviewer.
const maxDiffLines = 50

// DiffSpecs summarizes how proposed differs from original as a graph diff —
// added and removed node ids, per-node changed fields, and changed top-level
// fields — so a reviewer sees the repair before approving it.
func DiffSpecs(original, proposed map[string]any) []string {
	diff := []string{}
	for _, field := range []string{"title", "description", "start"} {
		before, _ := original[field].(string)
		after, _ := proposed[field].(string)
		if before != after {
			diff = append(diff, fmt.Sprintf("%s changed", field))
		}
	}
	beforeNodes := nodesByID(original)
	afterNodes := nodesByID(proposed)
	for id, after := range afterNodes {
		before, exists := beforeNodes[id]
		if !exists {
			diff = append(diff, "node "+id+" added")
			continue
		}
		fields := []string{}
		keys := map[string]bool{}
		for key := range before {
			keys[key] = true
		}
		for key := range after {
			keys[key] = true
		}
		for key := range keys {
			if key == "id" {
				continue
			}
			left, _ := json.Marshal(before[key])
			right, _ := json.Marshal(after[key])
			if string(left) != string(right) {
				fields = append(fields, key)
			}
		}
		sort.Strings(fields)
		if len(fields) > 0 {
			diff = append(diff, "node "+id+" changed "+strings.Join(fields, ", "))
		}
	}
	for id := range beforeNodes {
		if _, exists := afterNodes[id]; !exists {
			diff = append(diff, "node "+id+" removed")
		}
	}
	sort.Strings(diff)
	if len(diff) > maxDiffLines {
		diff = append(diff[:maxDiffLines], fmt.Sprintf("… %d more changes", len(diff)-maxDiffLines))
	}
	return diff
}

func nodesByID(spec map[string]any) map[string]map[string]any {
	nodes := map[string]map[string]any{}
	list, _ := spec["nodes"].([]any)
	for _, item := range list {
		node, _ := item.(map[string]any)
		id, _ := node["id"].(string)
		if id != "" {
			nodes[id] = node
		}
	}
	return nodes
}
