package definition

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

const APIVersion = "bean/v1alpha1"

type manifest struct {
	APIVersion string   `yaml:"apiVersion"`
	Name       string   `yaml:"name"`
	Resources  []string `yaml:"resources,omitempty"`
}

type Diagnostics []Diagnostic

func (d Diagnostics) Error() string {
	lines := make([]string, len(d))
	for i := range d {
		lines[i] = d[i].Error()
	}
	return strings.Join(lines, "\n")
}

func LoadFile(filename string) (Bundle, []Diagnostic) {
	directory := filepath.Dir(filename)
	bundle, diagnostics := loadFS(os.DirFS(directory), filepath.Base(filename))
	if directory != "." {
		prefixSources(&bundle, diagnostics, filepath.ToSlash(directory))
	}
	return bundle, diagnostics
}

func LoadFS(filesystem fs.FS, manifestPath string) (Bundle, []Diagnostic) {
	return loadFS(filesystem, manifestPath)
}

func decodeSource(reader io.Reader, sourcePath string) (Bundle, []Diagnostic) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return Bundle{}, []Diagnostic{{Source: Position{Path: sourcePath, Line: 1, Column: 1}, Message: err.Error()}}
	}
	documents, diagnostics := decodeYAML(data, sourcePath)
	if len(documents) == 0 {
		return Bundle{}, append(diagnostics, Diagnostic{Source: Position{Path: sourcePath, Line: 1, Column: 1}, Message: "application manifest is empty"})
	}
	root, rootDiagnostics := decodeManifest(documents[0], sourcePath)
	diagnostics = append(diagnostics, rootDiagnostics...)
	if len(root.Resources) > 0 {
		diagnostics = append(diagnostics, Diagnostic{Source: mappingKeyPosition(sourcePath, documentRoot(documents[0]), "resources"), Path: "resources", Message: "resources require loading from a file"})
	}
	definitions, definitionDiagnostics := decodeDefinitions(documents[1:], root.APIVersion, sourcePath)
	diagnostics = append(diagnostics, definitionDiagnostics...)
	diagnostics = append(diagnostics, duplicateDefinitionDiagnostics(definitions)...)
	return Bundle{Name: root.Name, Definitions: definitions}, diagnostics
}

func loadFS(filesystem fs.FS, manifestPath string) (Bundle, []Diagnostic) {
	if !fs.ValidPath(manifestPath) || manifestPath == "." {
		return Bundle{}, []Diagnostic{{Source: Position{Path: manifestPath, Line: 1, Column: 1}, Message: "manifest path must be a relative file path"}}
	}
	data, err := fs.ReadFile(filesystem, manifestPath)
	if err != nil {
		return Bundle{}, []Diagnostic{{Source: Position{Path: manifestPath, Line: 1, Column: 1}, Message: err.Error()}}
	}
	documents, diagnostics := decodeYAML(data, manifestPath)
	if len(documents) == 0 {
		return Bundle{}, append(diagnostics, Diagnostic{Source: Position{Path: manifestPath, Line: 1, Column: 1}, Message: "application manifest is empty"})
	}
	root, rootDiagnostics := decodeManifest(documents[0], manifestPath)
	diagnostics = append(diagnostics, rootDiagnostics...)
	bundle := Bundle{Name: root.Name}
	definitions, definitionDiagnostics := decodeDefinitions(documents[1:], root.APIVersion, manifestPath)
	bundle.Definitions = append(bundle.Definitions, definitions...)
	diagnostics = append(diagnostics, definitionDiagnostics...)

	base := path.Dir(manifestPath)
	seenResources := map[string]Position{}
	resourceNodes := manifestResourceNodes(documents[0])
	for i, resource := range root.Resources {
		position := Position{Path: manifestPath, Line: 1, Column: 1}
		if i < len(resourceNodes) {
			position.Line = resourceNodes[i].Line
			position.Column = resourceNodes[i].Column
		}
		if !fs.ValidPath(resource) || resource == "." || strings.HasPrefix(resource, "/") {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Message: fmt.Sprintf("resource %q must be a relative path without '..'", resource)})
			continue
		}
		resourcePath := path.Join(base, resource)
		if first, exists := seenResources[resourcePath]; exists {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Related: &first, Message: fmt.Sprintf("resource %q is listed more than once", resource)})
			continue
		}
		seenResources[resourcePath] = position
		resourceData, readErr := fs.ReadFile(filesystem, resourcePath)
		if readErr != nil {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Message: fmt.Sprintf("resource %q: %v", resource, readErr)})
			continue
		}
		resourceDocuments, resourceDiagnostics := decodeYAML(resourceData, resourcePath)
		diagnostics = append(diagnostics, resourceDiagnostics...)
		resourceDefinitions, resourceDefinitionDiagnostics := decodeDefinitions(resourceDocuments, root.APIVersion, resourcePath)
		bundle.Definitions = append(bundle.Definitions, resourceDefinitions...)
		diagnostics = append(diagnostics, resourceDefinitionDiagnostics...)
	}

	diagnostics = append(diagnostics, duplicateDefinitionDiagnostics(bundle.Definitions)...)
	return bundle, diagnostics
}

func decodeManifest(document *yaml.Node, sourcePath string) (manifest, []Diagnostic) {
	position := nodePosition(sourcePath, document)
	root := documentRoot(document)
	if root == nil || root.Kind != yaml.MappingNode {
		return manifest{}, []Diagnostic{{Source: position, Message: "application manifest must be a mapping"}}
	}
	diagnostics := duplicateKeyDiagnostics(root, sourcePath, "")
	allowed := map[string]bool{"apiVersion": true, "name": true, "resources": true}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		if !allowed[key.Value] {
			diagnostics = append(diagnostics, Diagnostic{Source: nodePosition(sourcePath, key), Path: key.Value, Message: "unknown manifest field"})
		}
	}
	var result manifest
	if err := root.Decode(&result); err != nil {
		diagnostics = append(diagnostics, yamlDiagnostic(sourcePath, err))
		return result, diagnostics
	}
	if result.APIVersion == "" {
		diagnostics = append(diagnostics, Diagnostic{Source: position, Path: "apiVersion", Message: "is required"})
	} else if result.APIVersion != APIVersion {
		diagnostics = append(diagnostics, Diagnostic{Source: mappingKeyPosition(sourcePath, root, "apiVersion"), Path: "apiVersion", Message: "must be " + APIVersion})
	}
	if strings.TrimSpace(result.Name) == "" {
		diagnostics = append(diagnostics, Diagnostic{Source: position, Path: "name", Message: "is required"})
	}
	return result, diagnostics
}

func decodeDefinitions(documents []*yaml.Node, apiVersion, sourcePath string) ([]Definition, []Diagnostic) {
	definitions := make([]Definition, 0, len(documents))
	diagnostics := []Diagnostic{}
	for _, document := range documents {
		root := documentRoot(document)
		if root == nil {
			continue
		}
		position := nodePosition(sourcePath, root)
		if root.Kind != yaml.MappingNode {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Message: "definition must be a mapping"})
			continue
		}
		diagnostics = append(diagnostics, duplicateKeyDiagnostics(root, sourcePath, "")...)
		definition := Definition{APIVersion: apiVersion, Spec: map[string]any{}, Source: &Source{Position: position, Locations: map[string]Position{}}}
		for i := 0; i+1 < len(root.Content); i += 2 {
			key, value := root.Content[i], root.Content[i+1]
			switch key.Value {
			case "kind":
				definition.Kind = value.Value
				definition.Source.Locations["kind"] = nodePosition(sourcePath, key)
			case "name":
				definition.Metadata.Name = value.Value
				definition.Source.Locations["metadata.name"] = nodePosition(sourcePath, key)
			case "namespace":
				definition.Metadata.Namespace = value.Value
				definition.Source.Locations["metadata.namespace"] = nodePosition(sourcePath, key)
			default:
				var decoded any
				if err := value.Decode(&decoded); err != nil {
					diagnostics = append(diagnostics, Diagnostic{Source: nodePosition(sourcePath, value), Kind: definition.Kind, Name: definition.Metadata.Name, Path: key.Value, Message: err.Error()})
					continue
				}
				definition.Spec[key.Value] = decoded
				indexLocations(definition.Source.Locations, "spec."+key.Value, key, value, sourcePath)
			}
		}
		if definition.Kind == "" {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Name: definition.Metadata.Name, Path: "kind", Message: "is required"})
		}
		if definition.Metadata.Name == "" {
			diagnostics = append(diagnostics, Diagnostic{Source: position, Kind: definition.Kind, Path: "name", Message: "is required"})
		}
		definitions = append(definitions, definition)
	}
	return definitions, diagnostics
}

func decodeYAML(data []byte, sourcePath string) ([]*yaml.Node, []Diagnostic) {
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	documents := []*yaml.Node{}
	for {
		var document yaml.Node
		err := decoder.Decode(&document)
		if err == io.EOF {
			break
		}
		if err != nil {
			return documents, []Diagnostic{yamlDiagnostic(sourcePath, err)}
		}
		if documentRoot(&document) != nil {
			documents = append(documents, &document)
		}
	}
	return documents, nil
}

func yamlDiagnostic(sourcePath string, err error) Diagnostic {
	line := 1
	match := regexp.MustCompile(`line ([0-9]+)`).FindStringSubmatch(err.Error())
	if len(match) == 2 {
		line, _ = strconv.Atoi(match[1])
	}
	return Diagnostic{Source: Position{Path: sourcePath, Line: line, Column: 1}, Message: err.Error()}
}

func duplicateKeyDiagnostics(node *yaml.Node, sourcePath, currentPath string) []Diagnostic {
	diagnostics := []Diagnostic{}
	switch node.Kind {
	case yaml.MappingNode:
		seen := map[string]Position{}
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			keyPath := key.Value
			if currentPath != "" {
				keyPath = currentPath + "." + key.Value
			}
			position := nodePosition(sourcePath, key)
			if first, exists := seen[key.Value]; exists {
				diagnostics = append(diagnostics, Diagnostic{Source: position, Related: &first, Path: keyPath, Message: "duplicate field"})
			} else {
				seen[key.Value] = position
			}
			diagnostics = append(diagnostics, duplicateKeyDiagnostics(value, sourcePath, keyPath)...)
		}
	case yaml.SequenceNode:
		for i, child := range node.Content {
			diagnostics = append(diagnostics, duplicateKeyDiagnostics(child, sourcePath, fmt.Sprintf("%s[%d]", currentPath, i))...)
		}
	}
	return diagnostics
}

func duplicateDefinitionDiagnostics(definitions []Definition) []Diagnostic {
	seen := map[string]Position{}
	diagnostics := []Diagnostic{}
	for _, definition := range definitions {
		key := definition.Kind + "/" + definition.Metadata.Name
		if first, exists := seen[key]; exists {
			diagnostics = append(diagnostics, Diagnostic{Kind: definition.Kind, Name: definition.Metadata.Name, Path: "name", Source: definition.Source.Position, Related: &first, Message: "duplicate definition"})
			continue
		}
		seen[key] = definition.Source.Position
	}
	return diagnostics
}

func indexLocations(locations map[string]Position, currentPath string, key, value *yaml.Node, sourcePath string) {
	locations[currentPath] = nodePosition(sourcePath, key)
	switch value.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(value.Content); i += 2 {
			childKey, childValue := value.Content[i], value.Content[i+1]
			indexLocations(locations, currentPath+"."+childKey.Value, childKey, childValue, sourcePath)
		}
	case yaml.SequenceNode:
		for i, child := range value.Content {
			itemPath := fmt.Sprintf("%s[%d]", currentPath, i)
			locations[itemPath] = nodePosition(sourcePath, child)
			if child.Kind == yaml.MappingNode {
				for j := 0; j+1 < len(child.Content); j += 2 {
					childKey, childValue := child.Content[j], child.Content[j+1]
					indexLocations(locations, itemPath+"."+childKey.Value, childKey, childValue, sourcePath)
				}
			}
		}
	}
}

func manifestResourceNodes(document *yaml.Node) []*yaml.Node {
	root := documentRoot(document)
	if root == nil || root.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value == "resources" && root.Content[i+1].Kind == yaml.SequenceNode {
			return root.Content[i+1].Content
		}
	}
	return nil
}

func mappingKeyPosition(sourcePath string, node *yaml.Node, name string) Position {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == name {
			return nodePosition(sourcePath, node.Content[i])
		}
	}
	return nodePosition(sourcePath, node)
}

func documentRoot(document *yaml.Node) *yaml.Node {
	if document == nil {
		return nil
	}
	if document.Kind == yaml.DocumentNode {
		if len(document.Content) == 0 || document.Content[0].Kind == 0 {
			return nil
		}
		return document.Content[0]
	}
	return document
}

func nodePosition(sourcePath string, node *yaml.Node) Position {
	if node == nil {
		return Position{Path: sourcePath, Line: 1, Column: 1}
	}
	return Position{Path: sourcePath, Line: node.Line, Column: node.Column}
}

func prefixSources(bundle *Bundle, diagnostics []Diagnostic, prefix string) {
	for i := range bundle.Definitions {
		if bundle.Definitions[i].Source == nil {
			continue
		}
		bundle.Definitions[i].Source.Path = path.Join(prefix, bundle.Definitions[i].Source.Path)
		for key, position := range bundle.Definitions[i].Source.Locations {
			position.Path = path.Join(prefix, position.Path)
			bundle.Definitions[i].Source.Locations[key] = position
		}
	}
	for i := range diagnostics {
		diagnostics[i].Source.Path = path.Join(prefix, diagnostics[i].Source.Path)
		if diagnostics[i].Related != nil {
			diagnostics[i].Related.Path = path.Join(prefix, diagnostics[i].Related.Path)
		}
	}
}

func encodeSource(w io.Writer, bundle Bundle) error {
	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	if err := encoder.Encode(manifest{APIVersion: APIVersion, Name: bundle.Name}); err != nil {
		return err
	}
	for _, definition := range bundle.Definitions {
		root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		appendScalarPair(root, "kind", definition.Kind)
		appendScalarPair(root, "name", definition.Metadata.Name)
		if definition.Metadata.Namespace != "" && definition.Metadata.Namespace != "default" {
			appendScalarPair(root, "namespace", definition.Metadata.Namespace)
		}
		keys := make([]string, 0, len(definition.Spec))
		for key := range definition.Spec {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			valueNode := &yaml.Node{}
			if err := valueNode.Encode(definition.Spec[key]); err != nil {
				return err
			}
			compactYAML(valueNode)
			root.Content = append(root.Content, keyNode, valueNode)
		}
		if err := encoder.Encode(root); err != nil {
			return err
		}
	}
	return nil
}

func compactYAML(node *yaml.Node) {
	for _, child := range node.Content {
		compactYAML(child)
	}
	switch node.Kind {
	case yaml.SequenceNode:
		allScalar := len(node.Content) > 0
		for _, child := range node.Content {
			allScalar = allScalar && child.Kind == yaml.ScalarNode
		}
		if allScalar {
			node.Style = yaml.FlowStyle
		}
	case yaml.MappingNode:
		allScalar := len(node.Content) > 0
		for i := 1; i < len(node.Content); i += 2 {
			allScalar = allScalar && node.Content[i].Kind == yaml.ScalarNode
		}
		if allScalar {
			node.Style = yaml.FlowStyle
		}
	}
}

func appendScalarPair(node *yaml.Node, key, value string) {
	node.Content = append(node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}
