package rule_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/rule"
)

func TestVocabularyAndFieldTypesAreClosed(t *testing.T) {
	wantOperators := []string{"add", "and", "concat", "divide", "eq", "gt", "gte", "if", "is_not_null", "is_null", "lower", "lt", "lte", "multiply", "ne", "not", "or", "subtract", "trim", "upper"}
	wantSources := []string{"context", "input", "literal", "tenant", "this", "user"}
	if got := rule.Operators(); !reflect.DeepEqual(got, wantOperators) {
		t.Fatalf("operators=%v want=%v", got, wantOperators)
	}
	if got := rule.Sources(); !reflect.DeepEqual(got, wantSources) {
		t.Fatalf("sources=%v want=%v", got, wantSources)
	}
	tests := map[string]rule.Type{
		"boolean":  rule.Boolean,
		"integer":  rule.Integer,
		"money":    rule.Number,
		"decimal":  rule.String,
		"datetime": rule.DateTime,
		"uuid":     rule.String,
	}
	for fieldType, want := range tests {
		if got, valid := rule.TypeForField(fieldType); !valid || got != want {
			t.Fatalf("field type=%s got=%s valid=%v", fieldType, got, valid)
		}
	}
	for _, forbidden := range []string{"file", "json", "password", "relation"} {
		if _, valid := rule.TypeForField(forbidden); valid {
			t.Fatalf("forbidden result field type %s accepted", forbidden)
		}
	}
}

func TestCheckInfersTypedExpressions(t *testing.T) {
	environment := rule.TypeEnvironment{
		This:  map[string]rule.Type{"status": rule.String, "total": rule.Number},
		Input: map[string]rule.Type{"quantity": rule.Integer, "unit_price": rule.Number},
	}
	tests := []struct {
		name       string
		expression rule.Expression
		want       rule.Type
	}{
		{name: "number", expression: op("multiply", source("input", "quantity"), source("input", "unit_price")), want: rule.Number},
		{name: "guard", expression: op("and", op("ne", source("this", "status"), literal("won")), op("lte", source("this", "total"), literal(1000))), want: rule.Boolean},
		{name: "conditional", expression: op("if", literal(true), literal("approved"), literal("rejected")), want: rule.String},
		{name: "context", expression: source("context", "now"), want: rule.DateTime},
		{name: "string", expression: op("concat", op("upper", literal("po")), literal("-"), source("context", "request_id")), want: rule.String},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := rule.Check(test.expression, environment)
			if err != nil || got != test.want {
				t.Fatalf("type=%s err=%v want=%s", got, err, test.want)
			}
		})
	}
}

func TestCheckRejectsUnknownAndIncompatibleExpressions(t *testing.T) {
	environment := rule.TypeEnvironment{This: map[string]rule.Type{"total": rule.Number}, Input: map[string]rule.Type{"quantity": rule.Integer}}
	tests := []struct {
		name       string
		expression rule.Expression
		code       string
	}{
		{name: "source", expression: source("environment", "HOME"), code: rule.CodeUnknownSource},
		{name: "path", expression: source("this", "missing"), code: rule.CodeUnknownPath},
		{name: "operator", expression: op("exec", literal("echo")), code: rule.CodeUnknownOperator},
		{name: "arity", expression: op("not", literal(true), literal(false)), code: rule.CodeArity},
		{name: "type", expression: op("add", literal("one"), literal(2)), code: rule.CodeType},
		{name: "shape", expression: rule.Expression{Op: "upper", Source: "input", Path: "quantity", Args: []rule.Expression{literal("x")}}, code: rule.CodeShape},
		{name: "missing literal", expression: rule.Expression{Source: "literal"}, code: rule.CodeShape},
		{name: "literal on input", expression: rule.Expression{Source: "input", Path: "quantity", Literal: json.RawMessage("1")}, code: rule.CodeShape},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := rule.Check(test.expression, environment)
			assertRuleError(t, err, test.code)
		})
	}
}

func TestEvaluateIsDeterministicAndUsesOnlyInjectedContext(t *testing.T) {
	expression := op("if",
		op("eq", source("this", "tier"), literal("gold")),
		op("multiply", source("input", "quantity"), source("input", "unit_price")),
		literal(0),
	)
	environment := rule.Environment{
		This:     map[string]any{"tier": "gold"},
		Input:    map[string]any{"quantity": int64(3), "unit_price": float64(12.5)},
		User:     map[string]any{"id": "00000000-0000-4000-8000-000000000001", "email": "owner@example.com", "display_name": "Owner", "roles": []string{"manager"}},
		TenantID: "00000000-0000-4000-8000-000000000002",
		Context:  map[string]any{"now": "2026-09-01T07:00:00Z", "request_id": "req-1"},
	}
	first, err := rule.Evaluate(expression, environment)
	if err != nil {
		t.Fatal(err)
	}
	second, err := rule.Evaluate(expression, environment)
	if err != nil || !reflect.DeepEqual(first, second) || first != float64(37.5) {
		t.Fatalf("first=%#v second=%#v err=%v", first, second, err)
	}
	contextExpression := op("concat", source("context", "now"), literal("/"), source("context", "request_id"), literal("/"), source("tenant", "id"), literal("/"), source("user", "email"))
	value, err := rule.Evaluate(contextExpression, environment)
	if err != nil || value != "2026-09-01T07:00:00Z/req-1/00000000-0000-4000-8000-000000000002/owner@example.com" {
		t.Fatalf("context value=%#v err=%v", value, err)
	}
}

func TestEveryOperatorEvaluates(t *testing.T) {
	tests := map[string]struct {
		expression rule.Expression
		want       any
	}{
		"add":         {op("add", literal(2), literal(3)), int64(5)},
		"and":         {op("and", literal(true), literal(true)), true},
		"concat":      {op("concat", literal("a"), literal("b")), "ab"},
		"divide":      {op("divide", literal(5), literal(2)), float64(2.5)},
		"eq":          {op("eq", literal(2), literal(float64(2))), true},
		"gt":          {op("gt", literal(3), literal(2)), true},
		"gte":         {op("gte", literal("2026-09-01"), literal("2026-09-01")), true},
		"if":          {op("if", literal(false), literal("no"), literal("yes")), "yes"},
		"is_not_null": {op("is_not_null", literal("value")), true},
		"is_null":     {op("is_null", literal(nil)), true},
		"lower":       {op("lower", literal("BEAN")), "bean"},
		"lt":          {op("lt", literal(1), literal(2)), true},
		"lte":         {op("lte", literal("2026-09-01T07:00:00Z"), literal("2026-09-01T08:00:00Z")), true},
		"multiply":    {op("multiply", literal(4), literal(3)), int64(12)},
		"ne":          {op("ne", literal("a"), literal("b")), true},
		"not":         {op("not", literal(false)), true},
		"or":          {op("or", literal(false), literal(true)), true},
		"subtract":    {op("subtract", literal(5), literal(3)), int64(2)},
		"trim":        {op("trim", literal(" bean ")), "bean"},
		"upper":       {op("upper", literal("bean")), "BEAN"},
	}
	if len(tests) != len(rule.Operators()) {
		t.Fatalf("operator cases=%d vocabulary=%d", len(tests), len(rule.Operators()))
	}
	for _, operator := range rule.Operators() {
		t.Run(operator, func(t *testing.T) {
			test, exists := tests[operator]
			if !exists {
				t.Fatalf("operator %s has no evaluation contract", operator)
			}
			got, err := rule.Evaluate(test.expression, rule.Environment{})
			if err != nil || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("value=%#v err=%v want=%#v", got, err, test.want)
			}
		})
	}
}

func TestNumericComparisonsPreserveIntegerPrecision(t *testing.T) {
	left := int64(1 << 53)
	right := left + 1
	environment := rule.Environment{Input: map[string]any{"left": left, "right": right}}
	for _, test := range []struct {
		expression rule.Expression
		want       bool
	}{
		{expression: op("eq", source("input", "left"), source("input", "right")), want: false},
		{expression: op("ne", source("input", "left"), source("input", "right")), want: true},
		{expression: op("lt", source("input", "left"), source("input", "right")), want: true},
		{expression: op("gte", source("input", "left"), source("input", "right")), want: false},
	} {
		got, err := rule.Evaluate(test.expression, environment)
		if err != nil || got != test.want {
			t.Fatalf("expression=%s value=%v err=%v want=%v", test.expression.Op, got, err, test.want)
		}
	}
	for _, decimal := range []json.Number{"9007199254740992.0", "9.007199254740992e15", "9007199254740992.5"} {
		environment := rule.Environment{Input: map[string]any{"left": decimal, "right": right}}
		for _, test := range []struct {
			expression rule.Expression
			want       bool
		}{
			{expression: op("eq", source("input", "left"), source("input", "right")), want: false},
			{expression: op("lt", source("input", "left"), source("input", "right")), want: true},
		} {
			got, err := rule.Evaluate(test.expression, environment)
			if err != nil || got != test.want {
				t.Fatalf("decimal=%s expression=%s value=%v err=%v want=%v", decimal, test.expression.Op, got, err, test.want)
			}
		}
	}
}

func TestTypedEvaluationRejectsRuntimeResultMismatch(t *testing.T) {
	if _, err := rule.EvaluateTyped(literal("true"), rule.Environment{}, rule.Boolean); err == nil {
		t.Fatal("string result accepted as boolean")
	}
	if !rule.ResultCompatible(rule.Number, rule.Integer) || rule.ResultCompatible(rule.Integer, rule.Number) {
		t.Fatal("numeric result compatibility is incorrect")
	}
}

func TestEvaluateShortCircuitsAndFailsClosed(t *testing.T) {
	unsafe := op("gt", op("divide", literal(1), literal(0)), literal(0))
	for _, expression := range []rule.Expression{
		op("and", literal(false), unsafe),
		op("or", literal(true), unsafe),
	} {
		if _, err := rule.Evaluate(expression, rule.Environment{}); err != nil {
			t.Fatalf("short-circuit expression failed: %v", err)
		}
	}
	for _, test := range []struct {
		expression rule.Expression
		code       string
	}{
		{expression: op("not"), code: rule.CodeArity},
		{expression: op("divide", literal(1), literal(0)), code: rule.CodeDivideByZero},
		{expression: source("input", "missing"), code: rule.CodeMissingValue},
		{expression: source("context", "now"), code: rule.CodeMissingValue},
		{expression: op("upper", literal(12)), code: rule.CodeType},
	} {
		_, err := rule.Evaluate(test.expression, rule.Environment{})
		assertRuleError(t, err, test.code)
	}
}

func TestCompileAndRuntimeBounds(t *testing.T) {
	tooDeep := literal(true)
	for range rule.MaxDepth {
		tooDeep = op("not", tooDeep)
	}
	tooMany := make([]rule.Expression, rule.MaxNodes+1)
	for index := range tooMany {
		tooMany[index] = literal(true)
	}
	tests := []struct {
		name       string
		expression rule.Expression
	}{
		{name: "depth", expression: tooDeep},
		{name: "nodes", expression: op("and", tooMany...)},
		{name: "literal", expression: literal(strings.Repeat("x", rule.MaxLiteralBytes+1))},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := rule.Check(test.expression, rule.TypeEnvironment{})
			assertRuleError(t, err, rule.CodeLimit)
		})
	}
	pieces := make([]rule.Expression, 5)
	for index := range pieces {
		pieces[index] = literal(strings.Repeat("x", rule.MaxLiteralBytes-2))
	}
	resultExpression := op("concat", pieces...)
	_, err := rule.Evaluate(resultExpression, rule.Environment{})
	assertRuleError(t, err, rule.CodeLimit)
}

func source(name, path string) rule.Expression {
	return rule.Expression{Source: name, Path: path}
}

func literal(value any) rule.Expression {
	encoded, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return rule.Expression{Source: "literal", Literal: encoded}
}

func op(name string, args ...rule.Expression) rule.Expression {
	return rule.Expression{Op: name, Args: args}
}

func assertRuleError(t *testing.T, err error, code string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error", code)
	}
	ruleError, ok := err.(*rule.Error)
	if !ok || ruleError.Code != code {
		t.Fatalf("error=%T %v want code=%s", err, err, code)
	}
}
