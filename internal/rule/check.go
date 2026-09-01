package rule

import (
	"encoding/json"
	"fmt"
)

func Check(expression Expression, environment TypeEnvironment) (Type, error) {
	if err := checkStructure(expression); err != nil {
		return "", err
	}
	return infer(expression, environment, "expression")
}

func checkStructure(expression Expression) error {
	nodes := 0
	var walk func(Expression, int, string) error
	walk = func(current Expression, depth int, path string) error {
		nodes++
		if nodes > MaxNodes {
			return ruleError(CodeLimit, path, fmt.Sprintf("rule exceeds %d nodes", MaxNodes))
		}
		if depth > MaxDepth {
			return ruleError(CodeLimit, path, fmt.Sprintf("rule exceeds depth %d", MaxDepth))
		}
		if current.Op != "" {
			if current.Source != "" || current.Path != "" || len(current.Literal) > 0 {
				return ruleError(CodeShape, path, "operator node cannot also contain a value source")
			}
			if !validOperator(current.Op) {
				return ruleError(CodeUnknownOperator, path+".op", "unknown rule operator "+current.Op)
			}
			minimum, maximum := arity(current.Op)
			if err := requireArity(current.Op, len(current.Args), minimum, maximum, path); err != nil {
				return err
			}
		} else {
			if current.Source == "" {
				return ruleError(CodeShape, path, "value node requires a source")
			}
			if current.Args != nil {
				return ruleError(CodeShape, path, "value node cannot contain arguments")
			}
			if !validSource(current.Source) {
				return ruleError(CodeUnknownSource, path+".source", "unknown rule source "+current.Source)
			}
			if current.Source == "literal" {
				if current.Path != "" {
					return ruleError(CodeShape, path+".path", "literal source cannot contain a path")
				}
				if len(current.Literal) == 0 {
					return ruleError(CodeShape, path+".literal", "literal source requires a value")
				}
				literal, err := decodeLiteral(current.Literal)
				if err != nil || !literalScalar(literal) {
					return ruleError(CodeType, path+".literal", "rule literals must be JSON scalars")
				}
				if len(current.Literal) > MaxLiteralBytes {
					return ruleError(CodeLimit, path+".literal", fmt.Sprintf("rule literal exceeds %d bytes", MaxLiteralBytes))
				}
			} else {
				if len(current.Literal) > 0 {
					return ruleError(CodeShape, path+".literal", "non-literal source cannot contain a literal value")
				}
				if current.Path == "" {
					return ruleError(CodeShape, path+".path", "rule source path is required")
				}
			}
		}
		for index, argument := range current.Args {
			if err := walk(argument, depth+1, fmt.Sprintf("%s.args.%d", path, index)); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(expression, 1, "expression")
}

func arity(operator string) (int, int) {
	switch operator {
	case "and", "or", "add", "multiply":
		return 2, -1
	case "concat":
		return 1, -1
	case "not", "is_null", "is_not_null", "lower", "upper", "trim":
		return 1, 1
	case "if":
		return 3, 3
	default:
		return 2, 2
	}
}

func literalScalar(value any) bool {
	switch value.(type) {
	case nil, bool, string, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func infer(expression Expression, environment TypeEnvironment, path string) (Type, error) {
	if expression.Op == "" {
		return inferValue(expression, environment, path)
	}
	types := make([]Type, len(expression.Args))
	for index, argument := range expression.Args {
		inferred, err := infer(argument, environment, fmt.Sprintf("%s.args.%d", path, index))
		if err != nil {
			return "", err
		}
		types[index] = inferred
	}
	switch expression.Op {
	case "and", "or":
		if err := requireArity(expression.Op, len(types), 2, -1, path); err != nil {
			return "", err
		}
		if !all(types, Boolean) {
			return "", typeError(path, expression.Op+" requires boolean arguments")
		}
		return Boolean, nil
	case "not":
		if err := requireArity(expression.Op, len(types), 1, 1, path); err != nil {
			return "", err
		}
		if types[0] != Boolean {
			return "", typeError(path, "not requires a boolean argument")
		}
		return Boolean, nil
	case "eq", "ne":
		if err := requireArity(expression.Op, len(types), 2, 2, path); err != nil {
			return "", err
		}
		if _, ok := unify(types[0], types[1]); !ok {
			return "", typeError(path, expression.Op+" requires compatible arguments")
		}
		return Boolean, nil
	case "gt", "gte", "lt", "lte":
		if err := requireArity(expression.Op, len(types), 2, 2, path); err != nil {
			return "", err
		}
		if types[0] == Null || types[1] == Null {
			return "", typeError(path, expression.Op+" does not accept null arguments")
		}
		unified, ok := unify(types[0], types[1])
		if !ok || unified != Integer && unified != Number && unified != Date && unified != DateTime {
			return "", typeError(path, expression.Op+" requires compatible number, date, or datetime arguments")
		}
		return Boolean, nil
	case "is_null", "is_not_null":
		if err := requireArity(expression.Op, len(types), 1, 1, path); err != nil {
			return "", err
		}
		return Boolean, nil
	case "add", "multiply":
		if err := requireArity(expression.Op, len(types), 2, -1, path); err != nil {
			return "", err
		}
		return numericResult(types, path, expression.Op)
	case "subtract", "divide":
		if err := requireArity(expression.Op, len(types), 2, 2, path); err != nil {
			return "", err
		}
		result, err := numericResult(types, path, expression.Op)
		if err != nil {
			return "", err
		}
		if expression.Op == "divide" {
			return Number, nil
		}
		return result, nil
	case "if":
		if err := requireArity(expression.Op, len(types), 3, 3, path); err != nil {
			return "", err
		}
		if types[0] != Boolean {
			return "", typeError(path+".args.0", "if condition must be boolean")
		}
		result, ok := unify(types[1], types[2])
		if !ok {
			return "", typeError(path, "if branches must have compatible types")
		}
		return result, nil
	case "concat":
		if err := requireArity(expression.Op, len(types), 1, -1, path); err != nil {
			return "", err
		}
		if !all(types, String) {
			return "", typeError(path, "concat requires string arguments")
		}
		return String, nil
	case "lower", "upper", "trim":
		if err := requireArity(expression.Op, len(types), 1, 1, path); err != nil {
			return "", err
		}
		if types[0] != String {
			return "", typeError(path, expression.Op+" requires a string argument")
		}
		return String, nil
	default:
		return "", ruleError(CodeUnknownOperator, path+".op", "unknown rule operator "+expression.Op)
	}
}

func inferValue(expression Expression, environment TypeEnvironment, path string) (Type, error) {
	switch expression.Source {
	case "literal":
		value, err := decodeLiteral(expression.Literal)
		if err != nil {
			return "", typeError(path+".literal", "rule literal is invalid JSON")
		}
		return literalType(value, path)
	case "this":
		return pathType(environment.This, expression.Path, path)
	case "input":
		return pathType(environment.Input, expression.Path, path)
	case "user":
		return fixedPathType(map[string]Type{"id": String, "email": String, "display_name": String, "roles": Strings}, expression.Path, path)
	case "tenant":
		return fixedPathType(map[string]Type{"id": String}, expression.Path, path)
	case "context":
		return fixedPathType(map[string]Type{"now": DateTime, "request_id": String}, expression.Path, path)
	default:
		return "", ruleError(CodeUnknownSource, path+".source", "unknown rule source "+expression.Source)
	}
}

func pathType(types map[string]Type, name, path string) (Type, error) {
	valueType, exists := types[name]
	if !exists {
		return "", ruleError(CodeUnknownPath, path+".path", "unknown rule path "+name)
	}
	return valueType, nil
}

func fixedPathType(types map[string]Type, name, path string) (Type, error) {
	return pathType(types, name, path)
}

func literalType(value any, path string) (Type, error) {
	switch typed := value.(type) {
	case nil:
		return Null, nil
	case bool:
		return Boolean, nil
	case string:
		return String, nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return Integer, nil
	case float32, float64:
		return Number, nil
	case json.Number:
		if _, err := typed.Int64(); err == nil {
			return Integer, nil
		}
		if _, err := typed.Float64(); err == nil {
			return Number, nil
		}
	}
	return "", typeError(path+".literal", "rule literal has unsupported type")
}

func requireArity(operator string, got, minimum, maximum int, path string) error {
	if got < minimum || maximum >= 0 && got > maximum {
		expected := fmt.Sprintf("at least %d", minimum)
		if minimum == maximum {
			expected = fmt.Sprint(minimum)
		}
		return ruleError(CodeArity, path+".args", fmt.Sprintf("%s requires %s arguments", operator, expected))
	}
	return nil
}

func all(types []Type, expected Type) bool {
	for _, valueType := range types {
		if valueType != expected {
			return false
		}
	}
	return true
}

func numericResult(types []Type, path, operator string) (Type, error) {
	result := Integer
	for _, valueType := range types {
		if valueType != Integer && valueType != Number {
			return "", typeError(path, operator+" requires numeric arguments")
		}
		if valueType == Number {
			result = Number
		}
	}
	return result, nil
}

func unify(left, right Type) (Type, bool) {
	if left == right {
		return left, true
	}
	if left == Null {
		return right, true
	}
	if right == Null {
		return left, true
	}
	if left == Integer && right == Number || left == Number && right == Integer {
		return Number, true
	}
	return "", false
}

func typeError(path, message string) error {
	return ruleError(CodeType, path, message)
}
