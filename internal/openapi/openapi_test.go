package openapi_test

import (
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
