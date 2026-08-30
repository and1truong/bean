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

func TestSensitiveInputsAreWriteOnlyAndRegistrationIsAnonymous(t *testing.T) {
	a := appir.Empty()
	a.AppID = "test"
	a.Actions["signup"] = appir.Action{Name: "signup", Operation: "register_local_user", Input: map[string]appir.Field{"password": {Name: "password", Type: "password", Sensitive: true}}}
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
}
