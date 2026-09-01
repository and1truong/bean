package rule

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	MaxNodes        = 128
	MaxDepth        = 16
	MaxLiteralBytes = 4 << 10
	MaxValueBytes   = 16 << 10
)

const (
	CodeUnknownSource   = "RULE_UNKNOWN_SOURCE"
	CodeUnknownPath     = "RULE_UNKNOWN_PATH"
	CodeUnknownOperator = "RULE_UNKNOWN_OPERATOR"
	CodeArity           = "RULE_INVALID_ARITY"
	CodeType            = "RULE_TYPE_MISMATCH"
	CodeShape           = "RULE_INVALID_SHAPE"
	CodeLimit           = "RULE_RESOURCE_LIMIT"
	CodeMissingValue    = "RULE_MISSING_VALUE"
	CodeDivideByZero    = "RULE_DIVIDE_BY_ZERO"
)

type Type string

const (
	Null     Type = "null"
	Boolean  Type = "boolean"
	Integer  Type = "integer"
	Number   Type = "number"
	String   Type = "string"
	Date     Type = "date"
	DateTime Type = "datetime"
	Strings  Type = "strings"
)

type Expression struct {
	Op      string          `json:"op,omitempty" yaml:"op,omitempty"`
	Args    []Expression    `json:"args,omitempty" yaml:"args,omitempty"`
	Source  string          `json:"source,omitempty" yaml:"source,omitempty"`
	Path    string          `json:"path,omitempty" yaml:"path,omitempty"`
	Literal json.RawMessage `json:"literal,omitempty" yaml:"literal,omitempty"`
}

type TypeEnvironment struct {
	This  map[string]Type
	Input map[string]Type
}

type Environment struct {
	This     map[string]any
	Input    map[string]any
	User     map[string]any
	TenantID string
	Context  map[string]any
}

type Error struct {
	Code    string
	Path    string
	Message string
}

func (e *Error) Error() string {
	if e.Path == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

var operators = []string{"add", "and", "concat", "divide", "eq", "gt", "gte", "if", "is_not_null", "is_null", "lower", "lt", "lte", "multiply", "ne", "not", "or", "subtract", "trim", "upper"}
var sources = []string{"context", "input", "literal", "tenant", "this", "user"}

func Operators() []string { return append([]string{}, operators...) }
func Sources() []string   { return append([]string{}, sources...) }

func validOperator(name string) bool {
	index := sort.SearchStrings(operators, name)
	return index < len(operators) && operators[index] == name
}

func validSource(name string) bool {
	index := sort.SearchStrings(sources, name)
	return index < len(sources) && sources[index] == name
}

func TypeForField(fieldType string) (Type, bool) {
	switch fieldType {
	case "boolean":
		return Boolean, true
	case "integer":
		return Integer, true
	case "money":
		return Number, true
	case "date":
		return Date, true
	case "datetime":
		return DateTime, true
	case "decimal", "email", "enum", "richtext", "slug", "string", "text", "url", "uuid":
		return String, true
	default:
		return "", false
	}
}

func ResultCompatible(expected, actual Type) bool {
	if actual == Null || expected == actual {
		return true
	}
	return expected == Number && actual == Integer
}

func ValueMatches(expected Type, value any) bool {
	if value == nil {
		return true
	}
	switch expected {
	case Boolean:
		_, valid := value.(bool)
		return valid
	case Integer:
		_, valid := integerValue(value)
		return valid
	case Number:
		_, _, valid := numeric(value)
		return valid
	case String:
		_, valid := value.(string)
		return valid
	case Date:
		text, valid := value.(string)
		if !valid {
			return false
		}
		_, err := time.Parse("2006-01-02", text)
		return err == nil
	case DateTime:
		text, valid := value.(string)
		if !valid {
			return false
		}
		_, err := time.Parse(time.RFC3339Nano, text)
		return err == nil
	case Strings:
		switch value.(type) {
		case []string, []any:
			return true
		}
	}
	return false
}

func UsesSource(expression Expression, source string) bool {
	if expression.Source == source {
		return true
	}
	for _, argument := range expression.Args {
		if UsesSource(argument, source) {
			return true
		}
	}
	return false
}

func InputPaths(expression Expression) []string {
	paths := map[string]bool{}
	collectInputPaths(expression, paths)
	out := make([]string, 0, len(paths))
	for path := range paths {
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}

func collectInputPaths(expression Expression, paths map[string]bool) {
	if expression.Source == "input" {
		paths[expression.Path] = true
	}
	for _, argument := range expression.Args {
		collectInputPaths(argument, paths)
	}
}

func encodedSize(value any) (int, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, err
	}
	return len(encoded), nil
}

func decodeLiteral(encoded json.RawMessage) (any, error) {
	if len(encoded) == 0 {
		return nil, nil
	}
	var value any
	decoder := json.NewDecoder(strings.NewReader(string(encoded)))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func ruleError(code, path, message string) error {
	return &Error{Code: code, Path: path, Message: message}
}
