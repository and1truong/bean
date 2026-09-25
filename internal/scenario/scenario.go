// Package scenario owns the closed vocabulary and limits for Scenario
// definitions: executable workflow graphs orchestrating browser, HTTP, and
// Action steps outside the Action transaction boundary.
package scenario

const (
	// MaxScenarios bounds Scenario definitions per application.
	MaxScenarios = 64
	// MaxNodes bounds nodes in a single Scenario graph.
	MaxNodes = 128
	// MaxBranches bounds branch edges on a single branch node.
	MaxBranches = 16
	// MaxIterations bounds a loop node iteration count.
	MaxIterations = 64
	// DefaultIterations applies when a loop omits maxIterations.
	DefaultIterations = 8
	// MaxEncodedSize bounds total encoded Scenario data per application.
	MaxEncodedSize = 1 << 20
	// MaxLabelRunes bounds a node label.
	MaxLabelRunes = 200
	// MaxScriptRunes bounds a script node body.
	MaxScriptRunes = 4096
	// MaxInputBytes bounds encoded api_call input per node.
	MaxInputBytes = 1 << 16
	// DefaultTimeoutSeconds applies when a wait omits timeoutSeconds.
	DefaultTimeoutSeconds = 30
	// MaxTimeoutSeconds bounds wait timeouts.
	MaxTimeoutSeconds = 300
	// DefaultAttribute is the extracted element property when omitted.
	DefaultAttribute = "text"
)

// Node types are the orchestration primitives of a Scenario graph. They are
// deliberately session-oriented: a node executes inside a browser/session
// lifecycle, never inside an Action transaction.
const (
	NodeNavigate = "navigate"
	NodeClick    = "click"
	NodeFill     = "fill"
	NodeSelect   = "select"
	NodePress    = "press"
	NodeWait     = "wait"
	NodeAssert   = "assert"
	NodeExtract  = "extract"
	NodeBranch   = "branch"
	NodeLoop     = "loop"
	NodeScript   = "script"
	NodeAPICall  = "api_call"
	NodePause    = "pause"
)

// Wait conditions drive wait nodes.
const (
	WaitNavigation  = "navigation"
	WaitNetworkIdle = "network_idle"
	WaitRefVisible  = "ref_visible"
	WaitRefHidden   = "ref_hidden"
	WaitTextPresent = "text_present"
)

// Assertions drive assert nodes. Assertions that need an expected value read
// the node text field.
const (
	AssertRefVisible  = "ref_visible"
	AssertRefHidden   = "ref_hidden"
	AssertRefText     = "ref_text"
	AssertURLEquals   = "url_equals"
	AssertURLContains = "url_contains"
	AssertTextPresent = "text_present"
)

// Branch conditions route branch nodes to the first matching edge.
const (
	BranchLastStepPassed = "last_step_passed"
	BranchLastStepFailed = "last_step_failed"
	BranchRefVisible     = "ref_visible"
	BranchRefHidden      = "ref_hidden"
	BranchURLEquals      = "url_equals"
	BranchURLContains    = "url_contains"
	BranchTextPresent    = "text_present"
)

// Loop conditions decide whether a loop keeps iterating; the node body
// re-enters the loop until the condition holds or maxIterations is reached.
const (
	LoopRefVisible  = "ref_visible"
	LoopRefHidden   = "ref_hidden"
	LoopURLEquals   = "url_equals"
	LoopURLContains = "url_contains"
	LoopTextPresent = "text_present"
)

// Extract attributes name the element property bound by an extract node.
const (
	ExtractText      = "text"
	ExtractValue     = "value"
	ExtractAttribute = "attribute"
)

var nodeTypes = []string{
	NodeNavigate, NodeClick, NodeFill, NodeSelect, NodePress, NodeWait,
	NodeAssert, NodeExtract, NodeBranch, NodeLoop, NodeScript, NodeAPICall,
	NodePause,
}

// NodeTypes returns the supported Scenario node type identifiers.
func NodeTypes() []string { return append([]string{}, nodeTypes...) }

// ValidNodeType reports whether the identifier is a supported node type.
func ValidNodeType(name string) bool {
	for _, nodeType := range nodeTypes {
		if nodeType == name {
			return true
		}
	}
	return false
}

// WaitConditions returns the supported wait node conditions.
func WaitConditions() []string {
	return []string{WaitNavigation, WaitNetworkIdle, WaitRefVisible, WaitRefHidden, WaitTextPresent}
}

// Assertions returns the supported assert node assertions.
func Assertions() []string {
	return []string{AssertRefVisible, AssertRefHidden, AssertRefText, AssertURLEquals, AssertURLContains, AssertTextPresent}
}

// BranchConditions returns the supported branch edge conditions.
func BranchConditions() []string {
	return []string{BranchLastStepPassed, BranchLastStepFailed, BranchRefVisible, BranchRefHidden, BranchURLEquals, BranchURLContains, BranchTextPresent}
}

// LoopConditions returns the supported loop until conditions.
func LoopConditions() []string {
	return []string{LoopRefVisible, LoopRefHidden, LoopURLEquals, LoopURLContains, LoopTextPresent}
}

// ExtractAttributes returns the supported extract node attributes.
func ExtractAttributes() []string {
	return []string{ExtractText, ExtractValue, ExtractAttribute}
}

// FieldInSet reports whether name is a supported value in the given condition
// family, mirroring the compiler's nameSet helper without a dependency.
func FieldInSet(set []string, name string) bool {
	for _, value := range set {
		if value == name {
			return true
		}
	}
	return false
}

// commonFields are accepted on every node type.
var commonFields = []string{"id", "type", "label", "next", "onFail"}

// nodeFields declares the additional fields each node type accepts. The
// compiler rejects fields outside commonFields plus this set, and enforces
// the required markers from RequiredFields.
var nodeFields = map[string][]string{
	NodeNavigate: {"url"},
	NodeClick:    {"ref"},
	NodeFill:     {"ref", "text", "secret"},
	NodeSelect:   {"ref", "value"},
	NodePress:    {"ref", "key"},
	NodeWait:     {"condition", "ref", "text", "timeoutSeconds"},
	NodeAssert:   {"assertion", "ref", "text"},
	NodeExtract:  {"ref", "as", "attribute", "name"},
	NodeBranch:   {"branches"},
	NodeLoop:     {"until", "ref", "text", "body", "maxIterations"},
	NodeScript:   {"script", "as"},
	NodeAPICall:  {"action", "input", "as"},
	NodePause:    {},
}

// nodeRequired declares fields that must be present per node type.
var nodeRequired = map[string][]string{
	NodeNavigate: {"url"},
	NodeClick:    {"ref"},
	NodeFill:     {"ref"},
	NodeSelect:   {"ref", "value"},
	NodePress:    {"key"},
	NodeWait:     {"condition"},
	NodeAssert:   {"assertion"},
	NodeExtract:  {"ref", "as"},
	NodeBranch:   {"branches"},
	NodeLoop:     {"body"},
	NodeScript:   {"script"},
	NodeAPICall:  {"action"},
}

// Fields returns the node-type-specific fields accepted on the given node
// type (excluding common fields), in declaration order.
func Fields(nodeType string) []string {
	return append([]string{}, nodeFields[nodeType]...)
}

// AllowedFields reports whether field is accepted on the given node type.
func AllowedFields(nodeType, field string) bool {
	if FieldInSet(commonFields, field) {
		return true
	}
	return FieldInSet(nodeFields[nodeType], field)
}

// RequiredFields returns the fields a node type must declare.
func RequiredFields(nodeType string) []string {
	return append([]string{}, nodeRequired[nodeType]...)
}

// ConditionNeedsRef reports whether a wait/branch/loop condition targets an
// element ref.
func ConditionNeedsRef(condition string) bool {
	return condition == "ref_visible" || condition == "ref_hidden"
}

// ConditionNeedsText reports whether a wait/branch/loop condition reads an
// expected text value.
func ConditionNeedsText(condition string) bool {
	return condition == "text_present" || condition == "url_equals" || condition == "url_contains"
}

// AssertionNeedsRef reports whether an assertion targets an element ref.
func AssertionNeedsRef(assertion string) bool {
	return assertion == AssertRefVisible || assertion == AssertRefHidden || assertion == AssertRefText
}

// AssertionNeedsText reports whether an assertion reads an expected text.
func AssertionNeedsText(assertion string) bool {
	return assertion == AssertRefText || assertion == AssertURLEquals || assertion == AssertURLContains || assertion == AssertTextPresent
}
