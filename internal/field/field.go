package field

import (
	"encoding/json"
	"fmt"
	"net/mail"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
)

var uuid = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)

func Validate(f appir.Field, v any) error {
	if v == nil {
		if f.Required {
			return fmt.Errorf("%s is required", f.Name)
		}
		return nil
	}
	switch f.Type {
	case "string", "text", "richtext":
		if _, ok := v.(string); !ok {
			return typeError(f)
		}
	case "integer", "money":
		switch v.(type) {
		case int, int64, float64, json.Number:
		default:
			return typeError(f)
		}
	case "decimal":
		if _, ok := v.(string); !ok {
			return fmt.Errorf("%s decimal must be a string", f.Name)
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
	case "uuid", "relation":
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
func SanitizeRichText(s string) string {
	for {
		start := strings.Index(strings.ToLower(s), "<script")
		if start < 0 {
			break
		}
		end := strings.Index(strings.ToLower(s[start:]), "</script>")
		if end < 0 {
			return s[:start]
		}
		s = s[:start] + s[start+end+9:]
	}
	s = strings.ReplaceAll(s, "javascript:", "")
	return s
}
func typeError(f appir.Field) error { return fmt.Errorf("%s has invalid %s value", f.Name, f.Type) }
