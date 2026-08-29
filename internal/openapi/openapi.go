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
			props[f] = map[string]any{"type": "string"}
		}
		parameters := []any{map[string]any{"name": "limit", "in": "query", "schema": map[string]any{"type": "integer", "maximum": v.MaxLimit}}, map[string]any{"name": "offset", "in": "query", "schema": map[string]any{"type": "integer", "minimum": 0}}, map[string]any{"name": "cursor", "in": "query", "schema": map[string]any{"type": "string"}}}
		for filterName, filter := range v.ExposedFilters {
			parameters = append(parameters, map[string]any{"name": filterName, "in": "query", "schema": schema(filter.Type)})
		}
		paths["/api/views/"+name] = map[string]any{"get": map[string]any{"operationId": "view_" + name, "parameters": parameters, "responses": map[string]any{"200": map[string]any{"description": "View result", "headers": map[string]any{"Bean-Next-Cursor": map[string]any{"schema": map[string]any{"type": "string"}}}, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": props}}}}}}}, "401": errorResponse(), "403": errorResponse()}}}
	}
	for name, a := range a.Actions {
		props := map[string]any{}
		required := []string{}
		for n, f := range a.Input {
			props[n] = schema(f.Type)
			if f.Required {
				required = append(required, n)
			}
		}
		output := map[string]any{}
		for n, f := range a.Output {
			output[n] = schema(f.Type)
		}
		paths["/api/actions/"+name] = map[string]any{"post": map[string]any{"operationId": "action_" + name, "security": []any{map[string]any{"cookieAuth": []any{}}}, "requestBody": map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "additionalProperties": false, "properties": props, "required": required}}}}, "responses": map[string]any{"200": map[string]any{"description": "Action result", "content": map[string]any{"application/json": map[string]any{"schema": map[string]any{"type": "object", "properties": map[string]any{"data": map[string]any{"type": "object", "properties": output}}}}}}, "400": errorResponse(), "401": errorResponse(), "403": errorResponse(), "409": errorResponse()}}}
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
