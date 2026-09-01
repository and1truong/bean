package agentprotocol

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/registry"
)

const (
	APIVersion    = "bean.agent/v1alpha1"
	CLIAPIVersion = "bean.cli/v1alpha1"
)

type Plane string

const (
	DefinitionPlane  Plane = "definition"
	ReleasePlane     Plane = "release"
	ApplicationPlane Plane = "application"
)

type Principal struct {
	Planes  map[Plane]bool
	Request beanctx.Request
}

func (p Principal) Allows(plane Plane) bool { return p.Planes[plane] }

func ParsePlanes(value string) (map[Plane]bool, error) {
	planes := map[Plane]bool{}
	for _, raw := range strings.Split(value, ",") {
		name := Plane(strings.TrimSpace(raw))
		if name == "" {
			continue
		}
		if name != DefinitionPlane && name != ReleasePlane && name != ApplicationPlane {
			return nil, fmt.Errorf("unknown Agent Protocol plane %q", name)
		}
		planes[name] = true
	}
	return planes, nil
}

func AllPlanes() map[Plane]bool {
	return map[Plane]bool{DefinitionPlane: true, ReleasePlane: true, ApplicationPlane: true}
}

type Operation struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Plane       Plane          `json:"plane"`
	InputSchema map[string]any `json:"inputSchema"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Outcome struct {
	APIVersion  string                  `json:"apiVersion"`
	Operation   string                  `json:"operation"`
	OK          bool                    `json:"ok"`
	Result      any                     `json:"result,omitempty"`
	Diagnostics []definition.Diagnostic `json:"diagnostics"`
	Error       *Error                  `json:"error,omitempty"`
}

type Handler func(context.Context, json.RawMessage, Principal) Outcome

type operationEntry struct {
	Operation Operation
	Handler   Handler
}

type Service struct {
	operations registry.Registry[operationEntry]
}

func New() *Service {
	service, err := NewWithHandlers(nil)
	if err != nil {
		panic(err)
	}
	return service
}

// NewWithHandlers creates a sealed dispatcher with optional handler overrides.
// It is the explicit constructor seam used by transport contract tests.
func NewWithHandlers(overrides map[string]Handler) (*Service, error) {
	service := &Service{}
	definitions := service.operationDefinitions()
	known := map[string]bool{}
	for _, item := range definitions {
		known[item.Operation.Name] = true
	}
	for name, handler := range overrides {
		if !known[name] {
			return nil, fmt.Errorf("unknown Agent Protocol operation override %q", name)
		}
		if handler == nil {
			return nil, fmt.Errorf("Agent Protocol operation %q has no handler", name)
		}
	}
	entries := make([]registry.Entry[operationEntry], 0, len(definitions))
	for _, item := range definitions {
		if handler := overrides[item.Operation.Name]; handler != nil {
			item.Handler = handler
		}
		if item.Operation.Name == "" || item.Operation.Plane == "" || item.Handler == nil {
			return nil, fmt.Errorf("Agent Protocol operation is incomplete")
		}
		entries = append(entries, registry.Entry[operationEntry]{Name: item.Operation.Name, Value: item})
	}
	registered, err := registry.New(cloneOperationEntry, entries...)
	if err != nil {
		return nil, err
	}
	service.operations = registered
	return service, nil
}

func (s *Service) Operations(principal Principal) []Operation {
	out := []Operation{}
	for _, name := range s.operations.Names() {
		entry, _ := s.operations.Lookup(name)
		if principal.Allows(entry.Operation.Plane) {
			out = append(out, entry.Operation)
		}
	}
	return out
}

func (s *Service) Call(ctx context.Context, name string, arguments json.RawMessage, principal Principal) Outcome {
	entry, exists := s.operations.Lookup(name)
	if !exists {
		return failure(name, "BEAN-P1001", "unknown Agent Protocol operation")
	}
	if !principal.Allows(entry.Operation.Plane) {
		return failure(name, "BEAN-P1002", "Agent Protocol plane "+string(entry.Operation.Plane)+" is not allowed")
	}
	outcome := entry.Handler(ctx, arguments, principal)
	outcome.APIVersion = APIVersion
	outcome.Operation = name
	if outcome.Diagnostics == nil {
		outcome.Diagnostics = []definition.Diagnostic{}
	}
	return outcome
}

func cloneOperationEntry(entry operationEntry) operationEntry {
	entry.Operation.InputSchema = cloneSchema(entry.Operation.InputSchema)
	return entry
}

func cloneSchema(source map[string]any) map[string]any {
	if source == nil {
		return nil
	}
	encoded, _ := json.Marshal(source)
	var cloned map[string]any
	_ = json.Unmarshal(encoded, &cloned)
	return cloned
}

func success(result any) Outcome {
	return Outcome{OK: true, Result: result, Diagnostics: []definition.Diagnostic{}}
}

func failure(operation, code, message string) Outcome {
	return Outcome{APIVersion: APIVersion, Operation: operation, OK: false, Diagnostics: []definition.Diagnostic{}, Error: &Error{Code: code, Message: message}}
}

func objectSchema(properties map[string]any, required ...string) map[string]any {
	schema := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"type":                 "object",
		"additionalProperties": false,
		"properties":           properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func operation(metadata Operation, handler Handler) operationEntry {
	return operationEntry{Operation: metadata, Handler: handler}
}

func (s *Service) operationDefinitions() []operationEntry {
	file := map[string]any{"type": "string", "minLength": 1, "description": "Path to a Bean application manifest"}
	target := map[string]any{"type": "string", "minLength": 1, "description": "SQLite path or PostgreSQL/SQLite database URL"}
	stringValue := func(description string) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "description": description}
	}
	stringMap := map[string]any{"type": "object", "additionalProperties": true}
	viewParams := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"filter": stringMap, "exactFilters": stringMap,
			"search": map[string]any{"type": "string"}, "recordID": map[string]any{"type": "string"},
			"searchFields": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"sort": map[string]any{"type": "array", "items": map[string]any{
				"type": "object", "additionalProperties": false, "required": []string{"field"},
				"properties": map[string]any{"field": map[string]any{"type": "string"}, "desc": map[string]any{"type": "boolean"}},
			}},
			"limit": map[string]any{"type": "integer", "minimum": 0}, "offset": map[string]any{"type": "integer", "minimum": 0},
			"cursor": map[string]any{"type": "string"},
		},
	}
	return []operationEntry{
		operation(Operation{Name: "bean.definition.capabilities", Title: "Bean capabilities", Description: "Inspect Bean's compiler-owned vocabulary and protocol capabilities.", Plane: DefinitionPlane, InputSchema: objectSchema(map[string]any{})}, s.capabilities),
		operation(Operation{Name: "bean.definition.schema", Title: "Bean schema", Description: "Get the canonical manifest schema or one definition-kind schema.", Plane: DefinitionPlane, InputSchema: objectSchema(map[string]any{"kind": map[string]any{"type": "string"}})}, s.schema),
		operation(Operation{Name: "bean.definition.validate", Title: "Validate Bean application", Description: "Load and compile Bean application source without database mutation.", Plane: DefinitionPlane, InputSchema: objectSchema(map[string]any{"file": file}, "file")}, s.validate),
		operation(Operation{Name: "bean.definition.inspect", Title: "Inspect Bean application", Description: "Inspect redacted AppIR or one named definition and its references.", Plane: DefinitionPlane, InputSchema: objectSchema(map[string]any{"file": file, "kind": map[string]any{"type": "string"}, "name": map[string]any{"type": "string"}}, "file")}, s.inspect),
		operation(Operation{Name: "bean.release.plan", Title: "Plan Bean release", Description: "Preview deterministic additive migrations without mutating the target.", Plane: ReleasePlane, InputSchema: objectSchema(map[string]any{"file": file, "target": target}, "file")}, s.plan),
		operation(Operation{Name: "bean.release.diff", Title: "Diff Bean release", Description: "Compare candidate semantics with the active target release.", Plane: ReleasePlane, InputSchema: objectSchema(map[string]any{"file": file, "target": target}, "file")}, s.diff),
		operation(Operation{Name: "bean.release.publish", Title: "Publish Bean release", Description: "Compile, migrate, persist, and atomically activate a candidate release.", Plane: ReleasePlane, InputSchema: objectSchema(map[string]any{"file": file, "target": target}, "file", "target")}, s.publish),
		operation(Operation{Name: "bean.release.test", Title: "Test Bean release", Description: "Run isolated compile, migration, publication, and restart smoke checks.", Plane: ReleasePlane, InputSchema: objectSchema(map[string]any{"file": file}, "file")}, s.test),
		operation(Operation{Name: "bean.application.query", Title: "Query Bean View", Description: "Read active application data through one compiled View.", Plane: ApplicationPlane, InputSchema: objectSchema(map[string]any{"target": target, "view": stringValue("Compiled View name"), "params": viewParams}, "target", "view")}, s.query),
		operation(Operation{Name: "bean.application.execute", Title: "Execute Bean Action", Description: "Mutate active application data through one compiled Action.", Plane: ApplicationPlane, InputSchema: objectSchema(map[string]any{"target": target, "action": stringValue("Compiled Action name"), "input": map[string]any{"type": "object"}}, "target", "action", "input")}, s.execute),
	}
}
