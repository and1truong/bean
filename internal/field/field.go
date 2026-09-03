package field

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"math/big"
	"net/mail"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var slug = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
var decimal = regexp.MustCompile(`^[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?$`)

const (
	MaxFileBytes   = 5 << 20
	maxDecimalSize = 4096
)

func Types() []string {
	return []string{"boolean", "date", "datetime", "decimal", "email", "enum", "file", "integer", "json", "money", "password", "relation", "richtext", "slug", "string", "text", "url", "uuid"}
}

type Upload struct {
	Name        string
	ContentType string
	Data        []byte
}

func Validate(f appir.Field, v any) error {
	if v == nil {
		if f.Required {
			return fmt.Errorf("%s is required", f.Name)
		}
		return nil
	}
	switch f.Type {
	case "string", "text", "richtext", "password":
		if _, ok := v.(string); !ok {
			return typeError(f)
		}
	case "slug":
		s, ok := v.(string)
		if !ok || !slug.MatchString(s) {
			return fmt.Errorf("%s must be a lowercase slug", f.Name)
		}
	case "integer", "money":
		switch v.(type) {
		case int, int64, float64, json.Number:
		default:
			return typeError(f)
		}
	case "decimal":
		value, ok := v.(string)
		if !ok || !validDecimal(value) {
			return fmt.Errorf("%s must be a bounded decimal string", f.Name)
		}
	case "boolean":
		if _, ok := v.(bool); !ok {
			return typeError(f)
		}
	case "enum":
		s, ok := v.(string)
		if !ok {
			return typeError(f)
		}
		found := false
		for _, o := range f.Options {
			found = found || o == s
		}
		if !found {
			return fmt.Errorf("%s is not an allowed option", f.Name)
		}
	case "date":
		s, ok := v.(string)
		if !ok {
			return typeError(f)
		}
		if _, e := time.Parse("2006-01-02", s); e != nil {
			return fmt.Errorf("%s must be a date", f.Name)
		}
	case "datetime":
		s, ok := v.(string)
		if !ok {
			return typeError(f)
		}
		if _, e := time.Parse(time.RFC3339, s); e != nil {
			return fmt.Errorf("%s must be RFC3339", f.Name)
		}
	case "file":
		switch upload := v.(type) {
		case Upload:
			if upload.Name == "" || len(upload.Data) == 0 || len(upload.Data) > MaxFileBytes {
				return fmt.Errorf("%s must be a file no larger than %d bytes", f.Name, MaxFileBytes)
			}
		case string:
			if !uuid.MatchString(upload) {
				return typeError(f)
			}
		default:
			return typeError(f)
		}
	case "uuid", "relation":
		if f.Type == "relation" && f.Relation != nil && (f.Relation.Kind == "one-to-many" || f.Relation.Kind == "many-to-many") {
			values, ok := v.([]any)
			if !ok {
				return fmt.Errorf("%s must be a list of UUIDs", f.Name)
			}
			for _, value := range values {
				s, valid := value.(string)
				if !valid || !uuid.MatchString(s) {
					return fmt.Errorf("%s must be a list of UUIDs", f.Name)
				}
			}
			return nil
		}
		s, ok := v.(string)
		if !ok || !uuid.MatchString(s) {
			return fmt.Errorf("%s must be a UUID", f.Name)
		}
	case "email":
		s, ok := v.(string)
		if !ok {
			return typeError(f)
		}
		if _, e := mail.ParseAddress(s); e != nil {
			return fmt.Errorf("%s must be an email", f.Name)
		}
	case "url":
		s, ok := v.(string)
		if !ok {
			return typeError(f)
		}
		u, e := url.ParseRequestURI(s)
		if e != nil || u.Scheme == "" {
			return fmt.Errorf("%s must be a URL", f.Name)
		}
	case "json":
		if _, e := json.Marshal(v); e != nil {
			return fmt.Errorf("%s must be JSON", f.Name)
		}
	default:
		return fmt.Errorf("unknown field type %s", f.Type)
	}
	return nil
}

func validDecimal(value string) bool {
	if len(value) == 0 || len(value) > maxDecimalSize || !decimal.MatchString(value) || !boundedDecimalExponent(value) {
		return false
	}
	_, ok := new(big.Rat).SetString(value)
	return ok
}

func boundedDecimalExponent(value string) bool {
	unsigned := strings.TrimPrefix(strings.TrimPrefix(value, "-"), "+")
	exponent := 0
	if index := strings.IndexAny(unsigned, "eE"); index >= 0 {
		parsed, err := strconv.Atoi(unsigned[index+1:])
		if err != nil || parsed < -maxDecimalSize || parsed > maxDecimalSize {
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
	return normalized >= -maxDecimalSize && normalized <= maxDecimalSize
}

func Encode(f appir.Field, value any) (any, error) {
	if value == nil || f.Type != "json" {
		return value, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return string(encoded), nil
}

func Decode(f appir.Field, value any) any {
	if value == nil {
		return nil
	}
	switch f.Type {
	case "boolean":
		switch stored := value.(type) {
		case int64:
			return stored != 0
		case int:
			return stored != 0
		}
	case "json":
		var encoded []byte
		switch stored := value.(type) {
		case string:
			encoded = []byte(stored)
		case []byte:
			encoded = stored
		default:
			return value
		}
		var decoded any
		if err := json.Unmarshal(encoded, &decoded); err == nil {
			return decoded
		}
	}
	return value
}

func SanitizeRichText(s string) string {
	for _, name := range []string{"script", "style", "iframe", "object", "svg", "math", "template"} {
		closed := regexp.MustCompile(`(?is)<` + name + `\b[^>]*>.*?</` + name + `\s*>`)
		for closed.MatchString(s) {
			s = closed.ReplaceAllString(s, "")
		}
		unclosed := regexp.MustCompile(`(?is)<` + name + `\b[^>]*>.*$`)
		s = unclosed.ReplaceAllString(s, "")
	}
	escaped := stdhtml.EscapeString(s)
	escaped = regexp.MustCompile(`(?i)(javascript|vbscript|data):`).ReplaceAllString(escaped, "")
	formatting := regexp.MustCompile(`(?i)&lt;(/?)(p|strong|em|ul|ol|li|blockquote|code|pre|h2|h3|h4)&gt;|&lt;br\s*/?&gt;`)
	return formatting.ReplaceAllStringFunc(escaped, func(tag string) string {
		decoded := stdhtml.UnescapeString(tag)
		return strings.ToLower(decoded)
	})
}
func typeError(f appir.Field) error { return fmt.Errorf("%s has invalid %s value", f.Name, f.Type) }
