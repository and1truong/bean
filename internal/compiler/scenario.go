package compiler

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
	beanscenario "github.com/beanruntime/bean/internal/scenario"
)

var scenarioNodeID = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
var scenarioBinding = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// scenarioAttrName matches HTML attribute names (data-testid,
// aria-label, http-equiv, ...) — broader than the binding charset.
var scenarioAttrName = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:.\-]*$`)

func scenarioDiagnostic(name, path, message string) definition.Diagnostic {
	return definition.NewDiagnostic(definition.RuleScenario, "Scenario", name, path, message)
}

func scenarioDefinitionKind() definitionKind {
	kind := mappedDefinitionKind(appir.Scenario{}, func(app *appir.App) map[string]appir.Scenario { return app.Scenarios }, normalizeScenario)
	base := kind.Compile
	kind.Compile = func(app *appir.App, source definition.Definition) []definition.Diagnostic {
		if diagnostics := validateScenarioSource(source); len(diagnostics) > 0 {
			return diagnostics
		}
		return base(app, source)
	}
	kind.References = scenarioReferences
	kind.ReferenceCandidates = true
	kind.Validate = validateScenarios
	return kind
}

func normalizeScenario(name string, value *appir.Scenario) {
	value.Name = name
	if value.Start == "" && len(value.Nodes) > 0 {
		value.Start = value.Nodes[0].ID
	}
	for index := range value.Nodes {
		node := &value.Nodes[index]
		if node.Type == beanscenario.NodeWait && node.TimeoutSeconds == 0 {
			node.TimeoutSeconds = beanscenario.DefaultTimeoutSeconds
		}
		if node.Type == beanscenario.NodeLoop && node.MaxIterations == 0 {
			node.MaxIterations = beanscenario.DefaultIterations
		}
		if node.Type == beanscenario.NodeExtract && node.Attribute == "" {
			node.Attribute = beanscenario.DefaultAttribute
		}
	}
}

// validateScenarioSource enforces the per-node-type field contract on the raw
// spec before decoding, mirroring validateBlockContentSource.
func validateScenarioSource(source definition.Definition) []definition.Diagnostic {
	name := source.Metadata.Name
	out := []definition.Diagnostic{}
	rawNodes, present := source.Spec["nodes"]
	if !present {
		return nil
	}
	nodes, ok := rawNodes.([]any)
	if !ok {
		return []definition.Diagnostic{scenarioDiagnostic(name, "spec.nodes", "must be a list of nodes")}
	}
	for index, rawNode := range nodes {
		path := fmt.Sprintf("spec.nodes.%d", index)
		node, ok := rawNode.(map[string]any)
		if !ok {
			out = append(out, scenarioDiagnostic(name, path, "must be a mapping"))
			continue
		}
		nodeType, _ := node["type"].(string)
		if nodeType == "" {
			out = append(out, requiredDiagnostic("Scenario", name, path+".type", "is required"))
		} else if !beanscenario.ValidNodeType(nodeType) {
			out = append(out, scenarioDiagnostic(name, path+".type", "has no supported Scenario node type"))
		}
		for _, field := range sortedSpecKeys(node) {
			if field == "type" {
				continue
			}
			if !beanscenario.AllowedFields(nodeType, field) {
				if beanscenario.ValidNodeType(nodeType) {
					out = append(out, scenarioDiagnostic(name, path+"."+field, "is not supported by a "+nodeType+" node"))
				} else {
					out = append(out, scenarioDiagnostic(name, path+"."+field, "is not supported by a Scenario node"))
				}
			}
		}
		if !beanscenario.ValidNodeType(nodeType) {
			continue
		}
		for _, field := range beanscenario.RequiredFields(nodeType) {
			if value, exists := node[field]; !exists || value == nil {
				out = append(out, requiredDiagnostic("Scenario", name, path+"."+field, "is required"))
			}
		}
		if nodeType == beanscenario.NodeBranch {
			out = append(out, validateScenarioBranchSource(name, path, node["branches"])...)
		}
	}
	return out
}

func sortedSpecKeys(node map[string]any) []string {
	fields := make([]string, 0, len(node))
	for field := range node {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

func validateScenarioBranchSource(name, path string, rawBranches any) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	branches, ok := rawBranches.([]any)
	if !ok {
		if rawBranches != nil {
			return []definition.Diagnostic{scenarioDiagnostic(name, path+".branches", "must be a list of branches")}
		}
		return nil
	}
	allowed := map[string]bool{"condition": true, "ref": true, "text": true, "next": true}
	for index, rawBranch := range branches {
		branchPath := fmt.Sprintf("%s.branches.%d", path, index)
		branch, ok := rawBranch.(map[string]any)
		if !ok {
			out = append(out, scenarioDiagnostic(name, branchPath, "must be a mapping"))
			continue
		}
		for field := range branch {
			if !allowed[field] {
				out = append(out, scenarioDiagnostic(name, branchPath+"."+field, "is not supported by a branch edge"))
			}
		}
		for _, field := range []string{"condition", "next"} {
			if value, exists := branch[field]; !exists || value == nil {
				out = append(out, requiredDiagnostic("Scenario", name, branchPath+"."+field, "is required"))
			}
		}
	}
	return out
}

func validateScenarios(app *appir.App, _ *validationState) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if len(app.Scenarios) > beanscenario.MaxScenarios {
		out = append(out, scenarioDiagnostic("", "spec", fmt.Sprintf("application exceeds %d Scenario definitions", beanscenario.MaxScenarios)))
	}
	encodedSize := 0
	for _, name := range keys(app.Scenarios) {
		item := app.Scenarios[name]
		encoded, _ := json.Marshal(item)
		encodedSize += len(encoded)
		out = append(out, validateScenario(app, item)...)
	}
	if encodedSize > beanscenario.MaxEncodedSize {
		out = append(out, scenarioDiagnostic("", "spec", fmt.Sprintf("encoded Scenario data exceeds %d bytes", beanscenario.MaxEncodedSize)))
	}
	return out
}

func validateScenario(app *appir.App, item appir.Scenario) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if len(item.Nodes) == 0 {
		out = append(out, requiredDiagnostic("Scenario", item.Name, "spec.nodes", "at least one node is required"))
		return out
	}
	if len(item.Nodes) > beanscenario.MaxNodes {
		out = append(out, scenarioDiagnostic(item.Name, "spec.nodes", fmt.Sprintf("exceeds %d nodes", beanscenario.MaxNodes)))
	}
	index := map[string]int{}
	ids := map[string]bool{}
	for i, node := range item.Nodes {
		path := fmt.Sprintf("spec.nodes.%d", i)
		if !scenarioNodeID.MatchString(node.ID) {
			out = append(out, scenarioDiagnostic(item.Name, path+".id", "must match ^[a-z][a-z0-9_]*$"))
		} else if ids[node.ID] {
			out = append(out, duplicateDiagnostic("Scenario", item.Name, path+".id", "duplicate node id"))
		}
		ids[node.ID] = true
		index[node.ID] = i
		out = append(out, validateScenarioNode(app, item.Name, node, path)...)
	}
	if item.Start == "" {
		out = append(out, requiredDiagnostic("Scenario", item.Name, "spec.start", "is required"))
	} else if !ids[item.Start] {
		out = append(out, scenarioDiagnostic(item.Name, "spec.start", "references missing node "+item.Start))
	}
	// A pause node inside a loop body has no resume boundary: the walk
	// parks at the loop node, re-runs the body on resume, and pauses on
	// the same node again — a permanently stuck run. Reject it at
	// compile time instead of letting the run deadlock.
	for i, node := range item.Nodes {
		if node.Type != beanscenario.NodeLoop || node.Body == "" {
			continue
		}
		seen := map[string]bool{}
		for id := node.Body; id != "" && !seen[id]; {
			seen[id] = true
			nodeIndex, exists := index[id]
			if !exists {
				break
			}
			body := item.Nodes[nodeIndex]
			if body.Type == beanscenario.NodePause {
				out = append(out, scenarioDiagnostic(item.Name, fmt.Sprintf("spec.nodes.%d.body", i), "chains a pause node — pause is not supported inside a loop body"))
			}
			id = body.Next
		}
	}
	for i, node := range item.Nodes {
		path := fmt.Sprintf("spec.nodes.%d", i)
		for field, target := range scenarioNodeTargets(node) {
			if !ids[target] {
				out = append(out, scenarioDiagnostic(item.Name, path+"."+field, "references missing node "+target))
			}
		}
	}
	if item.Start != "" && ids[item.Start] {
		out = append(out, validateScenarioReachability(item, index)...)
	}
	return out
}

func scenarioNodeTargets(node appir.ScenarioNode) map[string]string {
	targets := map[string]string{}
	if node.Next != "" {
		targets["next"] = node.Next
	}
	if node.OnFail != "" {
		targets["onFail"] = node.OnFail
	}
	if node.Body != "" {
		targets["body"] = node.Body
	}
	for index, branch := range node.Branches {
		if branch.Next != "" {
			targets[fmt.Sprintf("branches.%d.next", index)] = branch.Next
		}
	}
	return targets
}

func validateScenarioReachability(item appir.Scenario, index map[string]int) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	visited := map[string]bool{}
	queue := []string{item.Start}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		nodeIndex, exists := index[id]
		if visited[id] || !exists {
			continue
		}
		visited[id] = true
		for _, target := range scenarioNodeTargets(item.Nodes[nodeIndex]) {
			if !visited[target] {
				queue = append(queue, target)
			}
		}
	}
	for i, node := range item.Nodes {
		if !visited[node.ID] {
			out = append(out, scenarioDiagnostic(item.Name, fmt.Sprintf("spec.nodes.%d.id", i), "is unreachable from start"))
		}
	}
	return out
}

func validateScenarioNode(app *appir.App, name string, node appir.ScenarioNode, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if utf8.RuneCountInString(node.Label) > beanscenario.MaxLabelRunes {
		out = append(out, scenarioDiagnostic(name, path+".label", fmt.Sprintf("exceeds %d runes", beanscenario.MaxLabelRunes)))
	}
	switch node.Type {
	case beanscenario.NodeWait:
		if !beanscenario.FieldInSet(beanscenario.WaitConditions(), node.Condition) {
			out = append(out, scenarioDiagnostic(name, path+".condition", "has no supported wait condition"))
		} else {
			out = append(out, validateScenarioConditionOperand(name, node.Ref, node.Text, node.Condition, path)...)
		}
		if node.TimeoutSeconds < 1 || node.TimeoutSeconds > beanscenario.MaxTimeoutSeconds {
			out = append(out, scenarioDiagnostic(name, path+".timeoutSeconds", fmt.Sprintf("must be between 1 and %d", beanscenario.MaxTimeoutSeconds)))
		}
	case beanscenario.NodeAssert:
		if !beanscenario.FieldInSet(beanscenario.Assertions(), node.Assertion) {
			out = append(out, scenarioDiagnostic(name, path+".assertion", "has no supported assertion"))
		} else {
			if beanscenario.AssertionNeedsRef(node.Assertion) && node.Ref == "" {
				out = append(out, requiredDiagnostic("Scenario", name, path+".ref", "is required by "+node.Assertion))
			}
			if beanscenario.AssertionNeedsText(node.Assertion) && node.Text == "" {
				out = append(out, requiredDiagnostic("Scenario", name, path+".text", "is required by "+node.Assertion))
			}
		}
	case beanscenario.NodeFill:
		if node.Text != "" && node.Secret != "" {
			out = append(out, scenarioDiagnostic(name, path, "cannot declare both text and secret"))
		}
		if node.Secret != "" && !scenarioBinding.MatchString(node.Secret) {
			out = append(out, scenarioDiagnostic(name, path+".secret", "must match ^[a-z][a-z0-9_]*$"))
		}
	case beanscenario.NodeExtract:
		if !beanscenario.FieldInSet(beanscenario.ExtractAttributes(), node.Attribute) {
			out = append(out, scenarioDiagnostic(name, path+".attribute", "has no supported extract attribute"))
		}
		if node.As != "" && !scenarioBinding.MatchString(node.As) {
			out = append(out, scenarioDiagnostic(name, path+".as", "must match ^[a-z][a-z0-9_]*$"))
		}
		// `name` is the HTML attribute read when the kind is
		// `attribute` — real attribute names (data-testid,
		// aria-label) do not fit the binding charset, so `as`
		// stays purely the result binding.
		if node.Attribute == "attribute" && node.Name == "" {
			out = append(out, requiredDiagnostic("Scenario", name, path+".name", "is required when attribute is attribute"))
		}
		if node.Name != "" && !scenarioAttrName.MatchString(node.Name) {
			out = append(out, scenarioDiagnostic(name, path+".name", "must match ^[a-zA-Z_:][a-zA-Z0-9_:.\\-]*$"))
		}
	case beanscenario.NodeBranch:
		if len(node.Branches) > beanscenario.MaxBranches {
			out = append(out, scenarioDiagnostic(name, path+".branches", fmt.Sprintf("exceeds %d branches", beanscenario.MaxBranches)))
		}
		for index, branch := range node.Branches {
			branchPath := fmt.Sprintf("%s.branches.%d", path, index)
			if !beanscenario.FieldInSet(beanscenario.BranchConditions(), branch.Condition) {
				out = append(out, scenarioDiagnostic(name, branchPath+".condition", "has no supported branch condition"))
			} else {
				out = append(out, validateScenarioConditionOperand(name, branch.Ref, branch.Text, branch.Condition, branchPath)...)
			}
		}
	case beanscenario.NodeLoop:
		if node.Until != "" {
			if !beanscenario.FieldInSet(beanscenario.LoopConditions(), node.Until) {
				out = append(out, scenarioDiagnostic(name, path+".until", "has no supported loop condition"))
			} else {
				out = append(out, validateScenarioConditionOperand(name, node.Ref, node.Text, node.Until, path)...)
			}
		}
		if node.MaxIterations < 1 || node.MaxIterations > beanscenario.MaxIterations {
			out = append(out, scenarioDiagnostic(name, path+".maxIterations", fmt.Sprintf("must be between 1 and %d", beanscenario.MaxIterations)))
		}
	case beanscenario.NodeScript:
		if utf8.RuneCountInString(node.Script) > beanscenario.MaxScriptRunes {
			out = append(out, scenarioDiagnostic(name, path+".script", fmt.Sprintf("exceeds %d runes", beanscenario.MaxScriptRunes)))
		}
		if node.As != "" && !scenarioBinding.MatchString(node.As) {
			out = append(out, scenarioDiagnostic(name, path+".as", "must match ^[a-z][a-z0-9_]*$"))
		}
	case beanscenario.NodeAPICall:
		if _, exists := app.Actions[node.Action]; !exists {
			out = append(out, missingReferenceDiagnostic("Scenario", name, path+".action", "Action", node.Action))
		}
		if node.Input != nil {
			encoded, _ := json.Marshal(node.Input)
			if len(encoded) > beanscenario.MaxInputBytes {
				out = append(out, scenarioDiagnostic(name, path+".input", fmt.Sprintf("exceeds %d bytes", beanscenario.MaxInputBytes)))
			}
		}
		if node.As != "" && !scenarioBinding.MatchString(node.As) {
			out = append(out, scenarioDiagnostic(name, path+".as", "must match ^[a-z][a-z0-9_]*$"))
		}
	}
	return out
}

func validateScenarioConditionOperand(name, ref, text, condition, path string) []definition.Diagnostic {
	out := []definition.Diagnostic{}
	if beanscenario.ConditionNeedsRef(condition) && ref == "" {
		out = append(out, requiredDiagnostic("Scenario", name, path+".ref", "is required by "+condition))
	}
	if beanscenario.ConditionNeedsText(condition) && text == "" {
		out = append(out, requiredDiagnostic("Scenario", name, path+".text", "is required by "+condition))
	}
	return out
}

func scenarioReferences(app *appir.App, name string) []DefinitionReference {
	item := app.Scenarios[name]
	out := []DefinitionReference{}
	for index, node := range item.Nodes {
		if node.Action != "" {
			out = append(out, reference(fmt.Sprintf("nodes.%d.action", index), "Action", node.Action))
		}
	}
	return references(out...)
}
