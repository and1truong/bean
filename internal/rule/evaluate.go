package rule

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func Evaluate(expression Expression, environment Environment) (any, error) {
	if err := checkStructure(expression); err != nil {
		return nil, err
	}
	evaluated := 0
	return evaluate(expression, environment, "expression", &evaluated)
}

func EvaluateTyped(expression Expression, environment Environment, expected Type) (any, error) {
	value, err := Evaluate(expression, environment)
	if err != nil {
		return nil, err
	}
	if !ValueMatches(expected, value) {
		return nil, typeError("expression", fmt.Sprintf("rule result does not match %s", expected))
	}
	return value, nil
}

func evaluate(expression Expression, environment Environment, path string, evaluated *int) (any, error) {
	*evaluated++
	if *evaluated > MaxNodes {
		return nil, ruleError(CodeLimit, path, fmt.Sprintf("rule evaluates more than %d nodes", MaxNodes))
	}
	if expression.Op == "" {
		value, err := resolve(expression, environment, path)
		if err != nil {
			return nil, err
		}
		return bounded(value, path)
	}
	switch expression.Op {
	case "and", "or":
		for index, argument := range expression.Args {
			value, err := evaluate(argument, environment, fmt.Sprintf("%s.args.%d", path, index), evaluated)
			if err != nil {
				return nil, err
			}
			boolean, ok := value.(bool)
			if !ok {
				return nil, typeError(path, expression.Op+" requires boolean arguments")
			}
			if expression.Op == "and" && !boolean {
				return false, nil
			}
			if expression.Op == "or" && boolean {
				return true, nil
			}
		}
		return expression.Op == "and", nil
	case "not":
		value, err := evaluate(expression.Args[0], environment, path+".args.0", evaluated)
		if err != nil {
			return nil, err
		}
		boolean, ok := value.(bool)
		if !ok {
			return nil, typeError(path, "not requires a boolean argument")
		}
		return !boolean, nil
	case "if":
		condition, err := evaluate(expression.Args[0], environment, path+".args.0", evaluated)
		if err != nil {
			return nil, err
		}
		boolean, ok := condition.(bool)
		if !ok {
			return nil, typeError(path+".args.0", "if condition must be boolean")
		}
		branch := 2
		if boolean {
			branch = 1
		}
		return evaluate(expression.Args[branch], environment, fmt.Sprintf("%s.args.%d", path, branch), evaluated)
	}
	arguments := make([]any, len(expression.Args))
	for index, argument := range expression.Args {
		value, err := evaluate(argument, environment, fmt.Sprintf("%s.args.%d", path, index), evaluated)
		if err != nil {
			return nil, err
		}
		arguments[index] = value
	}
	var result any
	var err error
	switch expression.Op {
	case "eq", "ne":
		equal := valuesEqual(arguments[0], arguments[1])
		if expression.Op == "ne" {
			equal = !equal
		}
		result = equal
	case "gt", "gte", "lt", "lte":
		result, err = ordered(arguments[0], arguments[1], expression.Op, path)
	case "is_null":
		result = arguments[0] == nil
	case "is_not_null":
		result = arguments[0] != nil
	case "add", "subtract", "multiply", "divide":
		result, err = arithmetic(expression.Op, arguments, path)
	case "concat":
		var builder strings.Builder
		for _, argument := range arguments {
			text, ok := argument.(string)
			if !ok {
				return nil, typeError(path, "concat requires string arguments")
			}
			builder.WriteString(text)
			if builder.Len() > MaxValueBytes {
				return nil, ruleError(CodeLimit, path, fmt.Sprintf("rule value exceeds %d bytes", MaxValueBytes))
			}
		}
		result = builder.String()
	case "lower", "upper", "trim":
		text, ok := arguments[0].(string)
		if !ok {
			return nil, typeError(path, expression.Op+" requires a string argument")
		}
		switch expression.Op {
		case "lower":
			result = strings.ToLower(text)
		case "upper":
			result = strings.ToUpper(text)
		default:
			result = strings.TrimSpace(text)
		}
	default:
		return nil, ruleError(CodeUnknownOperator, path+".op", "unknown rule operator "+expression.Op)
	}
	if err != nil {
		return nil, err
	}
	return bounded(result, path)
}

func resolve(expression Expression, environment Environment, path string) (any, error) {
	switch expression.Source {
	case "literal":
		value, err := decodeLiteral(expression.Literal)
		if err != nil {
			return nil, typeError(path+".literal", "rule literal is invalid JSON")
		}
		return value, nil
	case "this":
		return requiredLookup(environment.This, expression.Path, path)
	case "input":
		return requiredLookup(environment.Input, expression.Path, path)
	case "user":
		if !map[string]bool{"id": true, "email": true, "display_name": true, "roles": true}[expression.Path] {
			return nil, ruleError(CodeUnknownPath, path+".path", "unknown rule path "+expression.Path)
		}
		if environment.User == nil {
			return nil, nil
		}
		return environment.User[expression.Path], nil
	case "tenant":
		if expression.Path != "id" {
			return nil, ruleError(CodeUnknownPath, path+".path", "unknown rule path "+expression.Path)
		}
		return environment.TenantID, nil
	case "context":
		if !map[string]bool{"now": true, "request_id": true}[expression.Path] {
			return nil, ruleError(CodeUnknownPath, path+".path", "unknown rule path "+expression.Path)
		}
		return requiredLookup(environment.Context, expression.Path, path)
	default:
		return nil, ruleError(CodeUnknownSource, path+".source", "unknown rule source "+expression.Source)
	}
}

func requiredLookup(values map[string]any, name, path string) (any, error) {
	value, exists := values[name]
	if !exists {
		return nil, ruleError(CodeMissingValue, path+".path", "rule value is unavailable for "+name)
	}
	return value, nil
}

func valuesEqual(left, right any) bool {
	leftInteger, leftIsInteger := integerValue(left)
	rightInteger, rightIsInteger := integerValue(right)
	if leftIsInteger && rightIsInteger {
		return leftInteger == rightInteger
	}
	leftNumber, _, leftNumeric := numeric(left)
	rightNumber, _, rightNumeric := numeric(right)
	if leftNumeric && rightNumeric {
		return leftNumber == rightNumber
	}
	return reflect.DeepEqual(left, right)
}

func ordered(left, right any, operator, path string) (bool, error) {
	if leftString, ok := left.(string); ok {
		rightString, valid := right.(string)
		if !valid {
			return false, typeError(path, operator+" requires compatible arguments")
		}
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02"} {
			leftTime, leftErr := time.Parse(layout, leftString)
			rightTime, rightErr := time.Parse(layout, rightString)
			if leftErr == nil && rightErr == nil {
				comparison := leftTime.Compare(rightTime)
				return compareResult(comparison, operator), nil
			}
		}
		return false, typeError(path, operator+" requires numbers, dates, or datetimes")
	}
	leftInteger, leftIsInteger := integerValue(left)
	rightInteger, rightIsInteger := integerValue(right)
	if leftIsInteger && rightIsInteger {
		comparison := 0
		if leftInteger < rightInteger {
			comparison = -1
		} else if leftInteger > rightInteger {
			comparison = 1
		}
		return compareResult(comparison, operator), nil
	}
	leftNumber, _, leftOK := numeric(left)
	rightNumber, _, rightOK := numeric(right)
	if !leftOK || !rightOK {
		return false, typeError(path, operator+" requires numbers, dates, or datetimes")
	}
	comparison := 0
	if leftNumber < rightNumber {
		comparison = -1
	} else if leftNumber > rightNumber {
		comparison = 1
	}
	return compareResult(comparison, operator), nil
}

func compareResult(comparison int, operator string) bool {
	switch operator {
	case "gt":
		return comparison > 0
	case "gte":
		return comparison >= 0
	case "lt":
		return comparison < 0
	default:
		return comparison <= 0
	}
}

func arithmetic(operator string, arguments []any, path string) (any, error) {
	numbers := make([]float64, len(arguments))
	allIntegers := true
	for index, argument := range arguments {
		value, integer, ok := numeric(argument)
		if !ok {
			return nil, typeError(path, operator+" requires numeric arguments")
		}
		numbers[index] = value
		allIntegers = allIntegers && integer
	}
	if allIntegers && operator != "divide" {
		result := big.NewInt(0)
		first, _ := integerValue(arguments[0])
		result.SetInt64(first)
		for _, argument := range arguments[1:] {
			value, _ := integerValue(argument)
			next := big.NewInt(value)
			switch operator {
			case "add":
				result.Add(result, next)
			case "subtract":
				result.Sub(result, next)
			case "multiply":
				result.Mul(result, next)
			}
		}
		if !result.IsInt64() {
			return nil, ruleError(CodeLimit, path, "rule integer result is out of range")
		}
		return result.Int64(), nil
	}
	result := numbers[0]
	switch operator {
	case "add":
		for _, value := range numbers[1:] {
			result += value
		}
	case "subtract":
		result -= numbers[1]
	case "multiply":
		for _, value := range numbers[1:] {
			result *= value
		}
	case "divide":
		if numbers[1] == 0 {
			return nil, ruleError(CodeDivideByZero, path, "rule division by zero")
		}
		result /= numbers[1]
		allIntegers = false
	}
	if math.IsInf(result, 0) || math.IsNaN(result) {
		return nil, ruleError(CodeLimit, path, "rule numeric result is not finite")
	}
	return result, nil
}

func integerValue(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int8:
		return int64(typed), true
	case int16:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case uint:
		if uint64(typed) <= math.MaxInt64 {
			return int64(typed), true
		}
	case uint8:
		return int64(typed), true
	case uint16:
		return int64(typed), true
	case uint32:
		return int64(typed), true
	case uint64:
		if typed <= math.MaxInt64 {
			return int64(typed), true
		}
	case float32:
		value := float64(typed)
		if math.Trunc(value) == value && value >= math.MinInt64 && value <= math.MaxInt64 {
			return int64(value), true
		}
	case float64:
		if math.Trunc(typed) == typed && typed >= math.MinInt64 && typed <= math.MaxInt64 {
			return int64(typed), true
		}
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return integer, true
		}
		text := typed.String()
		if index := strings.IndexAny(text, "eE"); index >= 0 {
			exponent, err := strconv.Atoi(text[index+1:])
			if err != nil || exponent < -64 || exponent > 64 {
				return 0, false
			}
		}
		rational, ok := new(big.Rat).SetString(text)
		if ok && rational.IsInt() && rational.Num().IsInt64() {
			return rational.Num().Int64(), true
		}
	}
	return 0, false
}

func numeric(value any) (float64, bool, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true, true
	case int8:
		return float64(typed), true, true
	case int16:
		return float64(typed), true, true
	case int32:
		return float64(typed), true, true
	case int64:
		return float64(typed), true, true
	case uint:
		return float64(typed), true, true
	case uint8:
		return float64(typed), true, true
	case uint16:
		return float64(typed), true, true
	case uint32:
		return float64(typed), true, true
	case uint64:
		return float64(typed), true, true
	case float32:
		return float64(typed), false, true
	case float64:
		return typed, math.Trunc(typed) == typed, true
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return float64(integer), true, true
		}
		if number, err := typed.Float64(); err == nil {
			return number, false, true
		}
	}
	return 0, false, false
}

func bounded(value any, path string) (any, error) {
	size, err := encodedSize(value)
	if err != nil {
		return nil, typeError(path, "rule value is not JSON encodable")
	}
	if size > MaxValueBytes {
		return nil, ruleError(CodeLimit, path, fmt.Sprintf("rule value exceeds %d bytes", MaxValueBytes))
	}
	return value, nil
}
