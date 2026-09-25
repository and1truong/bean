// Package scenariogen turns a natural-language description into a Scenario
// definition spec. Generation never writes a definition: the caller receives a
// draft spec that compiles against the active application, and the draft is
// persisted only when a user saves it through the normal definition flow.
package scenariogen

import (
	"context"
	"fmt"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	beanscenario "github.com/beanruntime/bean/internal/scenario"
)

// Request is one generation attempt. Feedback carries the diagnostics of the
// previous attempt so the model can correct itself.
type Request struct {
	Prompt   string `json:"prompt"`
	Feedback string `json:"feedback,omitempty"`
}

// Generator produces one scenario spec per call. The returned map is the spec
// body of a Scenario definition (title, start, nodes); callers own validation.
type Generator interface {
	Generate(ctx context.Context, request Request) (map[string]any, error)
}

// MaxAttempts bounds the generate -> compile -> feedback loop.
const MaxAttempts = 3

// Draft generates a scenario spec for prompt and returns it once it compiles
// cleanly against active. When generation cannot produce a compilable draft
// within MaxAttempts, the error carries the final diagnostics; DraftError
// wraps them for the HTTP layer. On success the spec's description is stamped
// with the prompt so provenance survives save.
func Draft(ctx context.Context, gen Generator, active *appir.App, name, prompt string) (map[string]any, error) {
	feedback := ""
	var diagnostics []definition.Diagnostic
	for attempt := 0; attempt < MaxAttempts; attempt++ {
		spec, err := gen.Generate(ctx, Request{Prompt: prompt, Feedback: feedback})
		if err != nil {
			return nil, err
		}
		compiled := compiler.CompileScenarioCandidate(active, name, spec)
		if len(compiled.Diagnostics) == 0 {
			spec["description"] = provenance(prompt, spec)
			return spec, nil
		}
		diagnostics = compiled.Diagnostics
		feedback = diagnosticSummary(diagnostics)
	}
	return nil, &DraftError{Diagnostics: diagnostics}
}

func provenance(prompt string, spec map[string]any) string {
	provenance := fmt.Sprintf("Agent-generated from prompt %q.", prompt)
	if existing, _ := spec["description"].(string); existing != "" && !strings.HasPrefix(existing, "Agent-generated") {
		provenance += "\n\n" + existing
	}
	return provenance
}

// DraftError reports a generation loop that never produced a compilable spec.
type DraftError struct {
	Diagnostics []definition.Diagnostic
}

func (e *DraftError) Error() string {
	return fmt.Sprintf("generated scenario does not compile: %s", diagnosticSummary(e.Diagnostics))
}

func diagnosticSummary(diagnostics []definition.Diagnostic) string {
	parts := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		parts = append(parts, fmt.Sprintf("%s %s", diagnostic.Path, diagnostic.Message))
	}
	return strings.Join(parts, "; ")
}

// Slug derives a definition name from the prompt when the caller does not
// name the scenario.
func Slug(prompt string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(prompt) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case b.Len() > 0 && !strings.HasSuffix(b.String(), "_"):
			b.WriteByte('_')
		}
		if b.Len() >= 48 {
			break
		}
	}
	name := strings.Trim(b.String(), "_")
	if name == "" || name[0] < 'a' || name[0] > 'z' {
		name = "generated_" + name
	}
	return name
}

// Vocabulary documents the scenario node contract for the model, generated
// from the same tables the compiler enforces so the prompt cannot drift.
func Vocabulary() string {
	var b strings.Builder
	b.WriteString("Every node requires `id` and `type`. Optional common fields: `label`, `next`, `onFail` (node id of the next step or failure edge). `start` must be the id of an existing node; every `next`, `onFail`, branch `next`, and loop `body` reference must resolve.\n")
	for _, nodeType := range beanscenario.NodeTypes() {
		required := beanscenario.RequiredFields(nodeType)
		optional := []string{}
		for _, field := range beanscenario.Fields(nodeType) {
			if !beanscenario.FieldInSet(required, field) {
				optional = append(optional, field)
			}
		}
		fmt.Fprintf(&b, "- %s: required %v; optional %v\n", nodeType, required, optional)
	}
	fmt.Fprintf(&b, "wait conditions: %v\n", beanscenario.WaitConditions())
	fmt.Fprintf(&b, "assert assertions: %v\n", beanscenario.Assertions())
	fmt.Fprintf(&b, "branch edge conditions: %v\n", beanscenario.BranchConditions())
	fmt.Fprintf(&b, "loop until conditions: %v\n", beanscenario.LoopConditions())
	fmt.Fprintf(&b, "extract attributes: %v\n", beanscenario.ExtractAttributes())
	return b.String()
}
