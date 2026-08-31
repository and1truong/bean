package definition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

var Kinds = map[string]bool{"Entity": true, "View": true, "Action": true, "Webform": true, "Policy": true, "Filter": true, "Block": true, "Panel": true, "Page": true, "Role": true, "Menu": true, "Job": true, "AdminResource": true, "LocalRegistration": true}
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
	Path   string
	Line   int
	Column int
}
type Source struct {
	Position
	Locations map[string]Position
}
type Diagnostic struct {
	Kind, Name, Path, Message string
	Source                    Position
	Related                   *Position
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
		const unknownPrefix = "json: unknown field \""
		if strings.HasPrefix(diagnostics[i].Message, unknownPrefix) {
			field := strings.TrimSuffix(strings.TrimPrefix(diagnostics[i].Message, unknownPrefix), "\"")
			keys := make([]string, 0, len(source.Locations))
			for key := range source.Locations {
				if strings.HasSuffix(key, "."+field) {
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
		out = append(out, Diagnostic{Kind: d.Kind, Name: d.Metadata.Name, Path: "apiVersion", Message: "must be bean/v1alpha1"})
	}
	if !Kinds[d.Kind] {
		out = append(out, Diagnostic{Kind: d.Kind, Name: d.Metadata.Name, Path: "kind", Message: "unsupported definition kind"})
	}
	if !machineName.MatchString(d.Metadata.Name) {
		out = append(out, Diagnostic{Kind: d.Kind, Name: d.Metadata.Name, Path: "metadata.name", Message: "must match ^[a-z][a-z0-9_]*$"})
	}
	if d.Metadata.Namespace == "" {
		d.Metadata.Namespace = "default"
	}
	if d.Spec == nil {
		out = append(out, Diagnostic{Kind: d.Kind, Name: d.Metadata.Name, Path: "spec", Message: "is required"})
	}
	return out
}
func DecodeSpec(spec map[string]any, target any) error {
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
