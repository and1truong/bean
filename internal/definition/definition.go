package definition

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"gopkg.in/yaml.v3"
)

var Kinds = map[string]bool{"Entity": true, "View": true, "Action": true, "Webform": true, "Policy": true, "Block": true, "Panel": true, "Page": true, "Role": true, "Menu": true, "Job": true}
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
}
type Bundle struct {
	Name        string                      `json:"name" yaml:"name"`
	Definitions []Definition                `json:"definitions" yaml:"definitions"`
	Seed        map[string][]map[string]any `json:"seed,omitempty" yaml:"seed,omitempty"`
}
type Diagnostic struct{ Kind, Name, Path, Message string }

func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s %s %s: %s", d.Kind, d.Name, d.Path, d.Message)
}
func Decode(r io.Reader) (Bundle, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var b Bundle
	if e := dec.Decode(&b); e != nil {
		return b, e
	}
	if b.Name == "" {
		return b, fmt.Errorf("bundle name is required")
	}
	return b, nil
}
func Encode(w io.Writer, b Bundle) error { return yaml.NewEncoder(w).Encode(b) }
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
		out = append(out, Diagnostic{d.Kind, d.Metadata.Name, "apiVersion", "must be bean/v1alpha1"})
	}
	if !Kinds[d.Kind] {
		out = append(out, Diagnostic{d.Kind, d.Metadata.Name, "kind", "unsupported definition kind"})
	}
	if !machineName.MatchString(d.Metadata.Name) {
		out = append(out, Diagnostic{d.Kind, d.Metadata.Name, "metadata.name", "must match ^[a-z][a-z0-9_]*$"})
	}
	if d.Metadata.Namespace == "" {
		d.Metadata.Namespace = "default"
	}
	if d.Spec == nil {
		out = append(out, Diagnostic{d.Kind, d.Metadata.Name, "spec", "is required"})
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
