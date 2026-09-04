package openapi

import (
	"encoding/json"
	"fmt"
	"github.com/beanruntime/bean/internal/appir"
)

func Generate(a *appir.App) (json.RawMessage, error) {
	paths := map[string]any{}
	for name, v := range a.Views {
		props := map[string]any{}
		for _, f := range v.Fields {
			property := map[string]any{"type": "string"}
			if _, formatted := v.FieldFilters[f]; formatted {
				property["contentMediaType"] = "text/html"
			}
			props[f] = property
		}
		parameters := []any{map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "maximum": v.MaxLimit}}, map[string]any{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0}}, map[string]any{"name": "cursor", "in": "query", "schema": map[string]any{"type": "string"}}}
		for filterName, filter := range v.ExposedFilters {
			parameters = append(parameters, map[string]any{"name": filterName, "in": "query", "schema": schema(filter.Type)})
		}
		paths["/api/views/"+name] = map[string]any{"get": map[string]any{"operationId": "view_" + name, "parameters": parameters, "responses": map[string]any{"200": map[string]any{"description": "View result", "headers": map[string]any{"Bean-Next-Cursor": map[string]any{"schema": map[string]any{"type": "string"}}}, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": props}}}}}}}, "401": errorResponse(), "403": errorResponse()}}}
	}
	for name, actionDefinition := range a.Actions {
		if actionDefinition.Operation == "register_local_user" && !a.RegistrationActionEnabled(actionDefinition.Name) {
			continue
		}
		props := map[string]any{}
		required := []string{}
		hasFile := false
		for n, f := range actionDefinition.Input {
			if _, derived := actionDefinition.Derive[n]; derived {
				continue
			}
			definition := schema(f.Type)
			if f.Type == "file" {
				definition = map[string]any{"type": "string", "format": "binary"}
				hasFile = true
			}
			if f.Sensitive {
				definition["writeOnly"] = true
				definition["format"] = "password"
			}
			props[n] = definition
			if f.Required {
				required = append(required, n)
			}
		}
		if entity := a.Entities[actionDefinition.Entity]; entity.Navigation != nil && (actionDefinition.Operation == "create" || actionDefinition.Operation == "update") {
			placementProperties := map[string]any{
				"menu":          map[string]any{"type": "string"},
				"ownerId":       map[string]any{"type": "string", "format": "uuid"},
				"parentId":      map[string]any{"type": "string", "format": "uuid"},
				"weight":        map[string]any{"type": "integer", "minimum": -1000, "maximum": 1000},
				"labelOverride": map[string]any{"type": "string", "maxLength": 120},
			}
			placement := map[string]any{"type": "object", "additionalProperties": false, "required": []string{"menu", "ownerId", "weight"}, "properties": placementProperties}
			placements := map[string]any{"type": "array", "maxItems": 32, "items": placement}
			props["_navigation"] = map[string]any{"type": "object", "additionalProperties": false, "required": []string{"placements"}, "properties": map[string]any{"placements": placements}}
		}
		output := map[string]any{}
		for n, f := range actionDefinition.Output {
			output[n] = schema(f.Type)
		}
		security := []any{map[string]any{"cookieAuth": []any{}}}
		if a.RegistrationActionEnabled(actionDefinition.Name) {
			security = []any{}
		}
		mediaType := "application/json"
		if hasFile {
			mediaType = "multipart/form-data"
		}
		paths["/api/actions/"+name] = map[string]any{"post": map[string]any{"operationId": "action_" + name, "security": security, "requestBody": map[string]any{"required": true, "content": map[string]any{mediaType: map[string]any{"schema": map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": required}}}}, "responses": map[string]any{"200": map[string]any{"description": "Action result", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "object", "properties": output}}}}}}, "400": errorResponse(), "401": errorResponse(), "403": errorResponse(), "409": errorResponse()}}}
	}
	doc := map[string]any{"openapi": "3.1.0", "info": map[string]any{"title": "Bean " + a.AppID, "version": fmt.Sprint(a.Version)}, "paths": paths, "components": map[string]any{"securitySchemes": map[string]any{"cookieAuth": map[string]any{"type": "apiKey", "in": "cookie", "name": "bean_session"}}, "schemas": map[string]any{"Error": map[string]any{"type": "object", "properties": map[string]any{"error": map[string]any{"type": "object"}}}}}}
	b, e := json.MarshalIndent(doc, "", "  ")
	return b, e
}
func schema(t string) map[string]any {
	switch t {
	case "integer", "money":
		return map[string]any{"type": "integer"}
	case "boolean":
		return map[string]any{"type": "boolean"}
	case "json":
		return map[string]any{}
	default:
		return map[string]any{"type": "string"}
	}
}
func errorResponse() map[string]any {
	return map[string]any{"description": "Error", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"$ref": "#/components/schemas/Error"}}}}
}
func Validate(doc []byte) error {
	var v map[string]any
	if e := json.Unmarshal(doc, &v); e != nil {
		return e
	}
	if v["openapi"] == nil || v["paths"] == nil {
		return fmt.Errorf("OpenAPI document is missing required keys")
	}
	return nil
}
