package content

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
)

type SourceIssue struct {
	Path    string
	Message string
}

var newElementFields = map[string]bool{
	"level": true, "label": true, "target": true, "openIn": true, "caption": true, "columns": true, "rows": true, "rowHeader": true,
	"title": true, "transcript": true, "videoId": true, "playlistId": true, "question": true, "choices": true, "answer": true, "explanation": true,
	"block": true, "panel": true, "tabs": true, "content": true,
}

var elementFields = map[string]map[string]bool{
	"heading":          fields("type", "text", "level"),
	"ordered_list":     fields("type", "items"),
	"link":             fields("type", "label", "target", "openIn"),
	"divider":          fields("type"),
	"table":            fields("type", "caption", "columns", "rows", "rowHeader"),
	"audio":            fields("type", "source", "title", "transcript"),
	"youtube":          fields("type", "videoId", "title", "transcript"),
	"youtube_playlist": fields("type", "playlistId", "title", "transcript"),
	"choices":          fields("type", "question", "choices", "answer", "explanation"),
}

var requiredElementFields = map[string][]string{
	"heading":          {"text"},
	"ordered_list":     {"items"},
	"link":             {"label", "target"},
	"table":            {"caption", "columns", "rows"},
	"audio":            {"source", "title", "transcript"},
	"youtube":          {"videoId", "title", "transcript"},
	"youtube_playlist": {"playlistId", "title", "transcript"},
	"choices":          {"question", "choices", "answer"},
}

func fields(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, name := range names {
		out[name] = true
	}
	return out
}

// ValidateSource checks presence and shape before JSON decoding loses the
// distinction between omitted, null, and zero-valued fields.
func ValidateSource(value any, path string) []SourceIssue {
	elements, ok := value.([]any)
	if !ok {
		return []SourceIssue{{path, "must be a list of semantic content elements"}}
	}
	out := []SourceIssue{}
	for index, raw := range elements {
		elementPath := fmt.Sprintf("%s.%d", path, index)
		element, ok := raw.(map[string]any)
		if !ok {
			out = append(out, SourceIssue{elementPath, "must be an object"})
			continue
		}
		typeValue, exists := element["type"]
		typeName, stringType := typeValue.(string)
		if !exists || typeValue == nil {
			out = append(out, SourceIssue{elementPath + ".type", "is required"})
			continue
		}
		if !stringType {
			out = append(out, SourceIssue{elementPath + ".type", "must be a string"})
			continue
		}
		allowed, newVariant := elementFields[typeName]
		keys := make([]string, 0, len(element))
		for key := range element {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if newVariant && !allowed[key] || !newVariant && newElementFields[key] && !(typeName == "heading" && key == "level") {
				out = append(out, SourceIssue{elementPath + "." + key, "is not supported by content type " + typeName})
			}
		}
		if typeName == "heading" {
			if rawLevel, present := element["level"]; present && rawLevel == nil {
				out = append(out, SourceIssue{elementPath + ".level", "must be an integer"})
			} else if present {
				if !sourceInteger(rawLevel) {
					out = append(out, SourceIssue{elementPath + ".level", "must be an integer"})
				} else if level, _ := strconv.Atoi(fmt.Sprint(rawLevel)); level != 2 && level != 3 && level != 4 {
					out = append(out, SourceIssue{elementPath + ".level", "must be 2, 3, or 4"})
				}
			}
		}
		if !newVariant {
			continue
		}
		for _, field := range requiredElementFields[typeName] {
			if rawValue, present := element[field]; !present || rawValue == nil {
				out = append(out, SourceIssue{elementPath + "." + field, "is required"})
			}
		}
		out = append(out, validateNewElementTypes(element, elementPath, typeName)...)
		if field, values := sourceEnum(typeName); field != "" {
			if rawValue, present := element[field]; present {
				value, stringValue := rawValue.(string)
				if stringValue && !values[value] {
					out = append(out, SourceIssue{elementPath + "." + field, "has no supported value"})
				}
			}
		}
	}
	return out
}

func sourceEnum(typeName string) (string, map[string]bool) {
	switch typeName {
	case "link":
		return "openIn", map[string]bool{"same_tab": true, "new_tab": true}
	case "table":
		return "rowHeader", map[string]bool{"none": true, "first": true}
	default:
		return "", nil
	}
}

func sourceInteger(value any) bool {
	switch number := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return math.Trunc(number) == number
	case json.Number:
		_, err := number.Int64()
		return err == nil
	default:
		return false
	}
}

func validateNewElementTypes(element map[string]any, path, typeName string) []SourceIssue {
	out := []SourceIssue{}
	stringFields := []string{}
	switch typeName {
	case "heading":
		stringFields = []string{"text"}
	case "link":
		stringFields = []string{"label", "target", "openIn"}
	case "table":
		stringFields = []string{"caption", "rowHeader"}
	case "audio":
		stringFields = []string{"source", "title", "transcript"}
	case "youtube":
		stringFields = []string{"videoId", "title", "transcript"}
	case "youtube_playlist":
		stringFields = []string{"playlistId", "title", "transcript"}
	case "choices":
		stringFields = []string{"question", "answer", "explanation"}
	}
	for _, field := range stringFields {
		if value, present := element[field]; present {
			if _, ok := value.(string); !ok {
				out = append(out, SourceIssue{path + "." + field, "must be a string"})
			}
		}
	}
	switch typeName {
	case "ordered_list":
		if value, present := element["items"]; present && value != nil {
			out = append(out, stringListIssues(value, path+".items")...)
		}
	case "table":
		if value, present := element["columns"]; present && value != nil {
			out = append(out, objectListIssues(value, path+".columns", []string{"id", "label"})...)
		}
		if value, present := element["rows"]; present && value != nil {
			rows, ok := value.([]any)
			if !ok {
				out = append(out, SourceIssue{path + ".rows", "must be a list of string rows"})
			} else {
				for index, row := range rows {
					out = append(out, stringListIssues(row, fmt.Sprintf("%s.rows.%d", path, index))...)
				}
			}
		}
	case "choices":
		if value, present := element["choices"]; present && value != nil {
			out = append(out, objectListIssues(value, path+".choices", []string{"id", "text"})...)
		}
	}
	return out
}

func stringListIssues(value any, path string) []SourceIssue {
	items, ok := value.([]any)
	if !ok {
		return []SourceIssue{{path, "must be a list of strings"}}
	}
	out := []SourceIssue{}
	for index, item := range items {
		if _, ok := item.(string); !ok {
			out = append(out, SourceIssue{fmt.Sprintf("%s.%d", path, index), "must be a string"})
		}
	}
	return out
}

func objectListIssues(value any, path string, required []string) []SourceIssue {
	items, ok := value.([]any)
	if !ok {
		return []SourceIssue{{path, "must be a list of objects"}}
	}
	out := []SourceIssue{}
	allowed := fields(required...)
	for index, item := range items {
		itemPath := fmt.Sprintf("%s.%d", path, index)
		object, ok := item.(map[string]any)
		if !ok {
			out = append(out, SourceIssue{itemPath, "must be an object"})
			continue
		}
		keys := make([]string, 0, len(object))
		for key := range object {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if !allowed[key] {
				out = append(out, SourceIssue{itemPath + "." + key, "is not supported"})
			}
		}
		for _, field := range required {
			value, present := object[field]
			if !present || value == nil {
				out = append(out, SourceIssue{itemPath + "." + field, "is required"})
			} else if _, ok := value.(string); !ok {
				out = append(out, SourceIssue{itemPath + "." + field, "must be a string"})
			}
		}
	}
	return out
}
