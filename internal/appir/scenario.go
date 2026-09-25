package appir

// ScenarioBranch is one conditional edge of a branch node. The run engine
// evaluates branches in order and follows the first matching Next; the node
// Next edge is the fallback.
type ScenarioBranch struct {
	Condition string `json:"condition"`
	Ref       string `json:"ref,omitempty"`
	Text      string `json:"text,omitempty"`
	Next      string `json:"next"`
}

// ScenarioNode is one step of a compiled Scenario graph. Nodes are
// session-oriented orchestration primitives; which fields apply depends on
// Type and is enforced at compile time.
type ScenarioNode struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Label string `json:"label,omitempty"`

	// Control flow edges target node IDs within the same Scenario.
	Next     string           `json:"next,omitempty"`
	OnFail   string           `json:"onFail,omitempty"`
	Branches []ScenarioBranch `json:"branches,omitempty"`

	// navigate
	URL string `json:"url,omitempty"`

	// Element-targeted steps operate on semantic snapshot refs.
	Ref   string `json:"ref,omitempty"`
	Text  string `json:"text,omitempty"`
	Value string `json:"value,omitempty"`
	Key   string `json:"key,omitempty"`

	// fill may reference a named secret instead of literal text; the
	// plaintext never enters the IR or the run event stream.
	Secret string `json:"secret,omitempty"`

	// wait
	Condition      string `json:"condition,omitempty"`
	TimeoutSeconds int    `json:"timeoutSeconds,omitempty"`

	// assert
	Assertion string `json:"assertion,omitempty"`

	// extract / script / api_call bind their result under As.
	As        string `json:"as,omitempty"`
	Attribute string `json:"attribute,omitempty"`
	Name      string `json:"name,omitempty"`

	// loop
	Until         string `json:"until,omitempty"`
	Body          string `json:"body,omitempty"`
	MaxIterations int    `json:"maxIterations,omitempty"`

	// script
	Script string `json:"script,omitempty"`

	// api_call invokes an existing Action through the application plane.
	Action string         `json:"action,omitempty"`
	Input  map[string]any `json:"input,omitempty"`
}

// Scenario is a compiled executable workflow graph: the orchestration
// boundary for browser, HTTP, and Action steps.
type Scenario struct {
	Name        string         `json:"name"`
	Title       string         `json:"title,omitempty"`
	Description string         `json:"description,omitempty"`
	Start       string         `json:"start"`
	Nodes       []ScenarioNode `json:"nodes"`
}
