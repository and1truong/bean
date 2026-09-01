package definition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

var machineName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Metadata struct {
	Namespace string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Name      string `json:"name" yaml:"name"`
}
type Definition struct {
	APIVersion string         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string         `json:"kind" yaml:"kind"`
	Metadata   Metadata       `json:"metadata" yaml:"metadata"`
	Spec       map[string]any `json:"spec" yaml:"spec"`
	Source     *Source        `json:"-" yaml:"-"`
}
type Bundle struct {
	Name        string                      `json:"name" yaml:"name"`
	Definitions []Definition                `json:"definitions" yaml:"definitions"`
	Seed        map[string][]map[string]any `json:"seed,omitempty" yaml:"seed,omitempty"`
}
type Position struct {
	Path   string `json:"path,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
}
type Source struct {
	Position
	Locations map[string]Position
}
type DiagnosticRule string

const (
	RuleSource           DiagnosticRule = "source"
	RuleUnknownField     DiagnosticRule = "unknown-field"
	RuleRequired         DiagnosticRule = "required"
	RuleDuplicate        DiagnosticRule = "duplicate"
	RuleVersion          DiagnosticRule = "version"
	RuleUnsupportedKind  DiagnosticRule = "unsupported-kind"
	RuleMachineName      DiagnosticRule = "machine-name"
	RuleMissingReference DiagnosticRule = "missing-reference"
	RuleInvalidReference DiagnosticRule = "invalid-reference"
	RuleMissingField     DiagnosticRule = "missing-field"
	RuleAction           DiagnosticRule = "action"
	RuleLifecycle        DiagnosticRule = "lifecycle"
	RulePolicy           DiagnosticRule = "policy"
	RulePresentation     DiagnosticRule = "presentation"
	RuleBinding          DiagnosticRule = "binding"
	RuleRoute            DiagnosticRule = "route"
	RuleMigration        DiagnosticRule = "migration"
	RuleFixture          DiagnosticRule = "fixture"
	RuleGeneral          DiagnosticRule = "general"
)

type DiagnosticReference struct {
	Kind string
	Name string
}

type DiagnosticFacts struct {
	Rule             DiagnosticRule
	MissingReference *DiagnosticReference
	UnknownField     string
	MissingField     string
}

type Diagnostic struct {
	Code       string           `json:"code,omitempty"`
	Kind       string           `json:"kind,omitempty"`
	Name       string           `json:"name,omitempty"`
	Path       string           `json:"path,omitempty"`
	Message    string           `json:"message"`
	Value      any              `json:"value,omitempty"`
	Candidates []string         `json:"candidates,omitempty"`
	Source     Position         `json:"source,omitempty"`
	Related    *Position        `json:"related,omitempty"`
	Facts      *DiagnosticFacts `json:"-" yaml:"-"`
}

func NewDiagnostic(rule DiagnosticRule, kind, name, path, message string) Diagnostic {
	return Diagnostic{Code: codeForRule(rule), Kind: kind, Name: name, Path: path, Message: message, Facts: &DiagnosticFacts{Rule: rule}}
}

func MissingReferenceDiagnostic(kind, name, path, targetKind, targetName string) Diagnostic {
	diagnostic := NewDiagnostic(RuleMissingReference, kind, name, path, "references missing "+targetKind+" "+targetName)
	diagnostic.Facts.MissingReference = &DiagnosticReference{Kind: targetKind, Name: targetName}
	return diagnostic
}

func MissingFieldDiagnostic(kind, name, path, fieldName string, target bool) Diagnostic {
	message := "references missing field " + fieldName
	if target {
		message = "references missing target field " + fieldName
	}
	diagnostic := NewDiagnostic(RuleMissingField, kind, name, path, message)
	diagnostic.Facts.MissingField = fieldName
	return diagnostic
}

func UnknownFieldDiagnostic(kind, name, path, fieldName string) Diagnostic {
	diagnostic := NewDiagnostic(RuleUnknownField, kind, name, path, `json: unknown field "`+fieldName+`"`)
	diagnostic.Facts.UnknownField = fieldName
	return diagnostic
}

func codeForRule(rule DiagnosticRule) string {
	switch rule {
	case RuleSource:
		return "BEAN-E1001"
	case RuleUnknownField:
		return "BEAN-E1002"
	case RuleRequired:
		return "BEAN-E1003"
	case RuleDuplicate:
		return "BEAN-E1004"
	case RuleVersion:
		return "BEAN-E1005"
	case RuleUnsupportedKind:
		return "BEAN-E1101"
	case RuleMachineName:
		return "BEAN-E1102"
	case RuleMissingReference:
		return "BEAN-E2001"
	case RuleInvalidReference:
		return "BEAN-E2002"
	case RuleMissingField:
		return "BEAN-E2101"
	case RuleAction:
		return "BEAN-E2201"
	case RuleLifecycle:
		return "BEAN-E2202"
	case RulePolicy:
		return "BEAN-E2301"
	case RulePresentation:
		return "BEAN-E2401"
	case RuleBinding:
		return "BEAN-E2501"
	case RuleRoute:
		return "BEAN-E2601"
	case RuleMigration:
		return "BEAN-E2701"
	case RuleFixture:
		return "BEAN-E2801"
	default:
		return "BEAN-E2900"
	}
}

// ClassifyDiagnostics assigns stable public codes without making human
// wording part of the machine interface. Callers may add more specific codes
// at the point where a diagnostic is created.
func ClassifyDiagnostics(diagnostics []Diagnostic) {
	for index := range diagnostics {
		if diagnostics[index].Code != "" {
			continue
		}
		if diagnostics[index].Facts != nil {
			diagnostics[index].Code = codeForRule(diagnostics[index].Facts.Rule)
			continue
		}
		diagnostics[index].Code = codeForRule(RuleGeneral)
	}
}

func (d Diagnostic) Error() string {
	prefix := ""
	if d.Source.Path != "" {
		prefix = fmt.Sprintf("%s:%d:%d: ", d.Source.Path, d.Source.Line, d.Source.Column)
	}
	displayPath := d.Path
	if d.Source.Path != "" {
		displayPath = strings.TrimPrefix(displayPath, "spec.")
		displayPath = strings.TrimPrefix(displayPath, "metadata.")
		if displayPath == "spec" {
			displayPath = ""
		}
	}
	target := strings.TrimSpace(strings.Join([]string{d.Kind, d.Name, displayPath}, " "))
	message := d.Message
	if d.Related != nil {
		message += fmt.Sprintf("; first declared at %s:%d:%d", d.Related.Path, d.Related.Line, d.Related.Column)
	}
	if target == "" {
		return prefix + message
	}
	return fmt.Sprintf("%s%s: %s", prefix, target, message)
}
func Decode(r io.Reader) (Bundle, error) {
	bundle, diagnostics := decodeSource(r, "app.yaml")
	if len(diagnostics) > 0 {
		return bundle, Diagnostics(diagnostics)
	}
	return bundle, nil
}
func Encode(w io.Writer, b Bundle) error { return encodeSource(w, b) }

func LocateDiagnostics(definitions []Definition, diagnostics []Diagnostic) {
	sources := map[string]*Source{}
	for _, d := range definitions {
		key := d.Kind + "/" + d.Metadata.Name
		if d.Source != nil && sources[key] == nil {
			sources[key] = d.Source
		}
	}
	for i := range diagnostics {
		source := sources[diagnostics[i].Kind+"/"+diagnostics[i].Name]
		if source == nil {
			continue
		}
		diagnostics[i].Source = source.Position
		path := diagnostics[i].Path
		for path != "" {
			if position, ok := source.Locations[path]; ok {
				diagnostics[i].Source = position
				break
			}
			index := strings.LastIndexAny(path, ".[")
			if index < 0 {
				break
			}
			path = path[:index]
		}
		if diagnostics[i].Facts != nil && diagnostics[i].Facts.UnknownField != "" {
			fieldName := diagnostics[i].Facts.UnknownField
			keys := make([]string, 0, len(source.Locations))
			for key := range source.Locations {
				if strings.HasSuffix(key, "."+fieldName) {
					keys = append(keys, key)
				}
			}
			sort.Strings(keys)
			if len(keys) > 0 {
				diagnostics[i].Source = source.Locations[keys[0]]
			}
		}
	}
}
func Checksum(d Definition) (string, error) {
	b, e := json.Marshal(d)
	if e != nil {
		return "", e
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}
func ValidateEnvelope(d Definition) []Diagnostic {
	out := []Diagnostic{}
	if d.APIVersion != "bean/v1alpha1" {
		out = append(out, NewDiagnostic(RuleVersion, d.Kind, d.Metadata.Name, "apiVersion", "must be bean/v1alpha1"))
	}
	if !machineName.MatchString(d.Metadata.Name) {
		out = append(out, NewDiagnostic(RuleMachineName, d.Kind, d.Metadata.Name, "metadata.name", "must match ^[a-z][a-z0-9_]*$"))
	}
	if d.Metadata.Namespace == "" {
		d.Metadata.Namespace = "default"
	}
	if d.Spec == nil {
		out = append(out, NewDiagnostic(RuleRequired, d.Kind, d.Metadata.Name, "spec", "is required"))
	}
	return out
}

type UnknownFieldError struct {
	Field string
}

func (e *UnknownFieldError) Error() string {
	return `json: unknown field "` + e.Field + `"`
}

func DecodeSpec(spec map[string]any, target any) error {
	if fieldName := unknownField(spec, reflect.TypeOf(target)); fieldName != "" {
		return &UnknownFieldError{Field: fieldName}
	}
	b, e := json.Marshal(spec)
	if e != nil {
		return e
	}
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.DisallowUnknownFields()
	if e = decoder.Decode(target); e != nil {
		return e
	}
	if decoder.More() {
		return fmt.Errorf("spec contains trailing data")
	}
	return nil
}

func unknownField(value any, target reflect.Type) string {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	switch target.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		fields := map[string]reflect.Type{}
		for index := 0; index < target.NumField(); index++ {
			definition := target.Field(index)
			name := strings.Split(definition.Tag.Get("json"), ",")[0]
			if name == "" {
				name = strings.ToLower(definition.Name[:1]) + definition.Name[1:]
			}
			if name != "-" {
				fields[name] = definition.Type
			}
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			fieldType, exists := fields[key]
			if !exists {
				return key
			}
			if nested := unknownField(object[key], fieldType); nested != "" {
				return nested
			}
		}
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return ""
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if nested := unknownField(object[key], target.Elem()); nested != "" {
				return nested
			}
		}
	case reflect.Slice, reflect.Array:
		items, ok := value.([]any)
		if !ok {
			return ""
		}
		for _, item := range items {
			if nested := unknownField(item, target.Elem()); nested != "" {
				return nested
			}
		}
	}
	return ""
}
