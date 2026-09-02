package agentprotocol

import (
	"crypto/sha256"
	"encoding/binary"
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
	for name, suite := range redacted.TestSuites {
		for caseIndex := range suite.Tests {
			test := &suite.Tests[caseIndex]
			test.Input = redactTestMap(test.Input)
			test.This = redactTestMap(test.This)
			for entityName, rows := range test.Fixtures {
				for rowIndex := range rows {
					rows[rowIndex] = redactTestMap(rows[rowIndex])
				}
				test.Fixtures[entityName] = rows
			}
			if test.Context.Actor != nil {
				test.Context.Actor.ID = redactTestString(test.Context.Actor.ID)
				test.Context.Actor.Email = redactTestString(test.Context.Actor.Email)
				test.Context.Actor.DisplayName = redactTestString(test.Context.Actor.DisplayName)
			}
			test.Context.Tenant = redactTestString(test.Context.Tenant)
			test.Context.Time = redactTestString(test.Context.Time)
			test.Context.RequestID = redactTestString(test.Context.RequestID)
			for index := range test.Context.IDs {
				test.Context.IDs[index] = redactTestString(test.Context.IDs[index])
			}
			test.Context.Seed = redactTestSeed(test.Context.Seed)
			for extensionName, results := range test.Providers {
				for index := range results {
					results[index].Output = redactTestMap(results[index].Output)
				}
				test.Providers[extensionName] = results
			}
			if len(test.Expect.Result) > 0 {
				var value any
				if json.Unmarshal(test.Expect.Result, &value) == nil {
					test.Expect.Result, _ = json.Marshal(redactTestValue(value))
				}
			}
			for index := range test.Expect.Changes {
				test.Expect.Changes[index].ID = redactTestString(test.Expect.Changes[index].ID)
				test.Expect.Changes[index].Values = redactTestMap(test.Expect.Changes[index].Values)
			}
			for index := range test.Expect.Events {
				test.Expect.Events[index].Payload = redactTestMap(test.Expect.Events[index].Payload)
			}
			for index := range test.Expect.Audit {
				test.Expect.Audit[index].ActorID = redactTestString(test.Expect.Audit[index].ActorID)
				test.Expect.Audit[index].TenantID = redactTestString(test.Expect.Audit[index].TenantID)
				test.Expect.Audit[index].EntityID = redactTestString(test.Expect.Audit[index].EntityID)
			}
			for index := range test.Expect.ProviderCalls {
				call := &test.Expect.ProviderCalls[index]
				call.InvocationID = redactTestString(call.InvocationID)
				call.IdempotencyKey = redactTestString(call.IdempotencyKey)
				call.Input = redactTestMap(call.Input)
			}
		}
		redacted.TestSuites[name] = suite
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

func redactTestMap(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for name, value := range values {
		out[name] = redactTestValue(value)
	}
	return out
}

func redactTestValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return redactTestMap(typed)
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = redactTestValue(item)
		}
		return out
	default:
		encoded, _ := json.Marshal(value)
		digest := sha256.Sum256(encoded)
		return fmt.Sprintf("[REDACTED sha256:%x]", digest[:8])
	}
}

func redactTestString(value string) string {
	if value == "" {
		return ""
	}
	return redactTestValue(value).(string)
}

func redactTestSeed(value *int64) *int64 {
	if value == nil {
		return nil
	}
	encoded, _ := json.Marshal(*value)
	digest := sha256.Sum256(encoded)
	redacted := int64(binary.BigEndian.Uint64(digest[:8]))
	return &redacted
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
