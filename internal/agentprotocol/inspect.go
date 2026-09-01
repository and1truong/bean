package agentprotocol

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/rule"
	"github.com/beanruntime/bean/internal/valuesource"
)

type SemanticChange struct {
	Operation string `json:"operation"`
	Path      string `json:"path"`
	Before    any    `json:"before,omitempty"`
	After     any    `json:"after,omitempty"`
}

func SemanticDiff(current, candidate *appir.App) []SemanticChange {
	if current == nil {
		current = appir.Empty()
	}
	left := RedactedApp(current)
	right := RedactedApp(candidate)
	for _, app := range []*appir.App{left, right} {
		app.ReleaseID = ""
		app.AppID = ""
		app.Version = 0
		app.OpenAPI = nil
	}
	changes := []SemanticChange{}
	diffValue("", normalizedJSON(left), normalizedJSON(right), &changes)
	return changes
}

func RedactedApp(source *appir.App) *appir.App {
	redacted, _ := source.Clone()
	for name, item := range redacted.Views {
		redactExpression(item.Filter)
		redactExpression(item.ContextFilter)
		redacted.Views[name] = item
	}
	for name, item := range redacted.Actions {
		for stepIndex := range item.Steps {
			redactExpression(item.Steps[stepIndex].Where)
			redactExpression(item.Steps[stepIndex].Condition)
			for valueIndex := range item.Steps[stepIndex].Values {
				value := &item.Steps[stepIndex].Values[valueIndex].Value
				if valuesource.IsLiteral(value.Source) {
					value.Literal = json.RawMessage(`"[REDACTED]"`)
				}
			}
		}
		redacted.Actions[name] = item
	}
	for name, item := range redacted.Rules {
		redactRuleExpression(&item.Expression)
		redacted.Rules[name] = item
	}
	for name, item := range redacted.Policies {
		redactExpression(item.Condition)
		redacted.Policies[name] = item
	}
	for name, item := range redacted.Webforms {
		redactFormExpressions(item.Elements)
		redacted.Webforms[name] = item
	}
	return redacted
}

func redactRuleExpression(expression *rule.Expression) {
	if expression.Source == "literal" {
		digest := sha256.Sum256(expression.Literal)
		expression.Literal = json.RawMessage(fmt.Sprintf(`"[REDACTED sha256:%x]"`, digest[:8]))
	}
	for index := range expression.Args {
		redactRuleExpression(&expression.Args[index])
	}
}

func redactExpression(expression *expr.Expr) {
	if expression == nil {
		return
	}
	for _, value := range []*expr.Value{expression.Left, expression.Right} {
		if value != nil && valuesource.IsLiteral(value.Source) {
			value.Literal = "[REDACTED]"
		}
	}
	for index := range expression.Args {
		redactExpression(&expression.Args[index])
	}
}

func redactFormExpressions(elements []appir.FormElement) {
	for index := range elements {
		redactExpression(elements[index].Visible)
		redactExpression(elements[index].RequiredWhen)
		redactFormExpressions(elements[index].Children)
	}
}

func normalizedJSON(value any) any {
	encoded, _ := json.Marshal(value)
	var decoded any
	_ = json.Unmarshal(encoded, &decoded)
	return normalizeJSONKeys(decoded)
}

func normalizeJSONKeys(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			runes := []rune(key)
			if len(runes) > 0 {
				runes[0] = []rune(strings.ToLower(string(runes[0])))[0]
			}
			out[string(runes)] = normalizeJSONKeys(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index := range typed {
			out[index] = normalizeJSONKeys(typed[index])
		}
		return out
	default:
		return value
	}
}

func diffValue(path string, before, after any, changes *[]SemanticChange) {
	leftMap, leftIsMap := before.(map[string]any)
	rightMap, rightIsMap := after.(map[string]any)
	if leftIsMap && rightIsMap {
		keys := map[string]bool{}
		for key := range leftMap {
			keys[key] = true
		}
		for key := range rightMap {
			keys[key] = true
		}
		ordered := make([]string, 0, len(keys))
		for key := range keys {
			ordered = append(ordered, key)
		}
		sort.Strings(ordered)
		for _, key := range ordered {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			left, leftExists := leftMap[key]
			right, rightExists := rightMap[key]
			switch {
			case !leftExists:
				*changes = append(*changes, SemanticChange{Operation: "add", Path: childPath, After: right})
			case !rightExists:
				*changes = append(*changes, SemanticChange{Operation: "remove", Path: childPath, Before: left})
			default:
				diffValue(childPath, left, right, changes)
			}
		}
		return
	}
	if !reflect.DeepEqual(before, after) {
		*changes = append(*changes, SemanticChange{Operation: "change", Path: path, Before: before, After: after})
	}
}
