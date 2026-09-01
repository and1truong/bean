package openapi_test

import (
	"encoding/json"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/openapi"
	"testing"
)

func TestGeneratedDocumentValidates(t *testing.T) {
	a := appir.Empty()
	a.AppID = "test"
	a.Version = 1
	a.Entities["book"] = appir.Entity{Name: "book"}
	a.Views["books"] = appir.View{Name: "books", Entity: "book", Fields: []string{"id"}, MaxLimit: 200}
	a.Actions["book_create"] = appir.Action{Name: "book_create", Entity: "book", Operation: "create"}
	doc, e := openapi.Generate(a)
	if e != nil {
		t.Fatal(e)
	}
	if e = openapi.Validate(doc); e != nil {
		t.Fatal(e)
	}
}

func TestFileInputsUseMultipartBinarySchemas(t *testing.T) {
	a := appir.Empty()
	a.AppID = "test"
	a.Actions["upload"] = appir.Action{Name: "upload", Operation: "create", Input: map[string]appir.Field{"label": {Name: "label", Type: "string"}, "file": {Name: "file", Type: "file", Required: true}}}
	doc, err := openapi.Generate(a)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(doc, &decoded); err != nil {
		t.Fatal(err)
	}
	content := decoded["paths"].(map[string]any)["/api/actions/upload"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)
	if _, advertisedJSON := content["application/json"]; advertisedJSON {
		t.Fatalf("file Action advertised JSON: %v", content)
	}
	file := content["multipart/form-data"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["file"].(map[string]any)
	if file["type"] != "string" || file["format"] != "binary" {
		t.Fatalf("file schema=%v", file)
	}
}

func TestSensitiveInputsAreWriteOnlyAndRegistrationIsAnonymous(t *testing.T) {
	a := appir.Empty()
	a.AppID = "test"
	a.Actions["signup"] = appir.Action{Name: "signup", Operation: "register_local_user", Input: map[string]appir.Field{"password": {Name: "password", Type: "password", Sensitive: true}}}
	a.Actions["disabled_signup"] = appir.Action{Name: "disabled_signup", Operation: "register_local_user"}
	a.LocalRegistration = &appir.LocalRegistration{Action: "signup"}
	doc, err := openapi.Generate(a)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(doc, &decoded); err != nil {
		t.Fatal(err)
	}
	post := decoded["paths"].(map[string]any)["/api/actions/signup"].(map[string]any)["post"].(map[string]any)
	password := post["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)["password"].(map[string]any)
	if password["writeOnly"] != true || password["format"] != "password" || len(post["security"].([]any)) != 0 {
		t.Fatalf("registration OpenAPI=%v", post)
	}
	if _, exposed := decoded["paths"].(map[string]any)["/api/actions/disabled_signup"]; exposed {
		t.Fatal("disabled registration action was exposed in OpenAPI")
	}
}

func TestDerivedInputsAreNotAdvertisedToClients(t *testing.T) {
	a := appir.Empty()
	a.AppID = "test"
	a.Actions["invoice_create"] = appir.Action{
		Name: "invoice_create", Operation: "create",
		Input: map[string]appir.Field{
			"quantity": {Name: "quantity", Type: "integer", Required: true},
			"total":    {Name: "total", Type: "money", Required: true},
		},
		Derive: map[string]string{"total": "subtotal"},
	}
	doc, err := openapi.Generate(a)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err = json.Unmarshal(doc, &decoded); err != nil {
		t.Fatal(err)
	}
	schema := decoded["paths"].(map[string]any)["/api/actions/invoice_create"].(map[string]any)["post"].(map[string]any)["requestBody"].(map[string]any)["content"].(map[string]any)["application/json"].(map[string]any)["schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if properties["quantity"] == nil || properties["total"] != nil {
		t.Fatalf("request schema=%v", schema)
	}
}
