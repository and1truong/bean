package openapi_test

import (
	"encoding/json"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/openapi"
)

func TestFilteredViewFieldsDeclareHTMLMediaType(t *testing.T) {
	app := appir.Empty()
	app.AppID = "test"
	app.Views["articles"] = appir.View{Name: "articles", Entity: "article", Fields: []string{"id", "body"}, FieldFilters: map[string]string{"body": "markdown"}, MaxLimit: 10}
	document, err := openapi.Generate(app)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(document, &decoded); err != nil {
		t.Fatal(err)
	}
	properties := decoded["paths"].(map[string]any)["/api/views/articles"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["data"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	if properties["body"].(map[string]any)["contentMediaType"] != "text/html" {
		t.Fatalf("filtered field schema=%v", properties["body"])
	}
}
