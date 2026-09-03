package sqlite

import (
	"database/sql/driver"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/beanruntime/bean/internal/dbal"
	modernsqlite "modernc.org/sqlite"
)

const decimalValueBytes = 4096

var decimalText = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

type decimalAggregate struct {
	function string
	count    int64
	value    *big.Rat
}

func init() {
	modernsqlite.MustRegisterCollationUtf8("bean_decimal", compareDecimalText)
	for _, function := range []string{"sum", "avg", "min", "max"} {
		function := function
		modernsqlite.MustRegisterFunction("bean_decimal_"+function, &modernsqlite.FunctionImpl{
			NArgs:         1,
			Deterministic: true,
			MakeAggregate: func(modernsqlite.FunctionContext) (modernsqlite.AggregateFunction, error) {
				return &decimalAggregate{function: function}, nil
			},
		})
	}
}

func (a *decimalAggregate) Step(_ *modernsqlite.FunctionContext, arguments []driver.Value) error {
	if len(arguments) != 1 || arguments[0] == nil {
		return nil
	}
	value, err := parseDecimal(arguments[0])
	if err != nil {
		return err
	}
	a.count++
	if a.value == nil {
		a.value = new(big.Rat).Set(value)
		return nil
	}
	switch a.function {
	case "sum", "avg":
		a.value.Add(a.value, value)
	case "min":
		if value.Cmp(a.value) < 0 {
			a.value.Set(value)
		}
	case "max":
		if value.Cmp(a.value) > 0 {
			a.value.Set(value)
		}
	}
	return nil
}

func (*decimalAggregate) WindowInverse(*modernsqlite.FunctionContext, []driver.Value) error {
	return fmt.Errorf("decimal aggregates do not support window evaluation")
}

func (a *decimalAggregate) WindowValue(*modernsqlite.FunctionContext) (driver.Value, error) {
	if a.value == nil {
		return nil, nil
	}
	value := new(big.Rat).Set(a.value)
	if a.function == "avg" {
		value.Quo(value, new(big.Rat).SetInt64(a.count))
		return canonicalDecimalText(value.FloatString(dbal.DecimalAverageScale)), nil
	}
	if text, ok := finiteDecimal(value); ok {
		return text, nil
	}
	return nil, fmt.Errorf("decimal aggregate result is not finite")
}

func (*decimalAggregate) Final(*modernsqlite.FunctionContext) {}

func parseDecimal(value driver.Value) (*big.Rat, error) {
	var text string
	switch typed := value.(type) {
	case string:
		text = typed
	case []byte:
		text = string(typed)
	case int64:
		text = strconv.FormatInt(typed, 10)
	case float64:
		text = strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return nil, fmt.Errorf("invalid decimal value %T", value)
	}
	if len(text) > decimalValueBytes {
		return nil, fmt.Errorf("decimal value exceeds %d characters", decimalValueBytes)
	}
	if !decimalText.MatchString(text) {
		return nil, fmt.Errorf("invalid decimal value %q", text)
	}
	if !boundedDecimalExponent(text) {
		return nil, fmt.Errorf("decimal exponent exceeds normalized limit")
	}
	rational, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, fmt.Errorf("invalid decimal value %q", text)
	}
	_, ok = finiteDecimalScale(rational)
	if !ok {
		return nil, fmt.Errorf("decimal value is not finite")
	}
	return rational, nil
}

func boundedDecimalExponent(text string) bool {
	unsigned := strings.TrimPrefix(strings.TrimPrefix(text, "-"), "+")
	exponent := 0
	if index := strings.IndexAny(unsigned, "eE"); index >= 0 {
		parsed, err := strconv.Atoi(unsigned[index+1:])
		if err != nil || parsed < -decimalValueBytes || parsed > decimalValueBytes {
			return false
		}
		exponent = parsed
		unsigned = unsigned[:index]
	}
	point := strings.IndexByte(unsigned, '.')
	if point < 0 {
		point = len(unsigned)
		unsigned += "."
	}
	digits := unsigned[:point] + unsigned[point+1:]
	first := strings.IndexFunc(digits, func(character rune) bool { return character != '0' })
	if first < 0 {
		return true
	}
	normalized := exponent + point - first - 1
	return normalized >= -decimalValueBytes && normalized <= decimalValueBytes
}

func finiteDecimal(value *big.Rat) (string, bool) {
	scale, ok := finiteDecimalScale(value)
	if !ok {
		return "", false
	}
	return canonicalDecimalText(value.FloatString(scale)), true
}

func finiteDecimalScale(value *big.Rat) (int, bool) {
	denominator := new(big.Int).Set(value.Denom())
	twos := 0
	for denominator.Bit(0) == 0 {
		denominator.Rsh(denominator, 1)
		twos++
	}
	fives := 0
	five := big.NewInt(5)
	for new(big.Int).Mod(denominator, five).Sign() == 0 {
		denominator.Quo(denominator, five)
		fives++
	}
	if denominator.Cmp(big.NewInt(1)) != 0 {
		return 0, false
	}
	if fives > twos {
		twos = fives
	}
	return twos, true
}

func compareDecimalText(left, right string) int {
	leftValue, leftOK := new(big.Rat).SetString(left)
	rightValue, rightOK := new(big.Rat).SetString(right)
	if leftOK && rightOK {
		return leftValue.Cmp(rightValue)
	}
	return strings.Compare(left, right)
}

func canonicalDecimalText(text string) string {
	if strings.Contains(text, ".") {
		text = strings.TrimRight(strings.TrimRight(text, "0"), ".")
	}
	if text == "-0" || text == "+0" {
		return "0"
	}
	return text
}
