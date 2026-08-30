package appir_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
)

func TestCompatibilityV1Fixture(t *testing.T) {
	b, e := os.ReadFile("testdata/v1.json")
	if e != nil {
		t.Fatal(e)
	}
	var app appir.App
	if e = json.Unmarshal(b, &app); e != nil {
		t.Fatal(e)
	}
	if e = app.ValidateFormat(); e != nil {
		t.Fatal(e)
	}
	encoded, e := json.Marshal(&app)
	if e != nil || !json.Valid(encoded) {
		t.Fatalf("round trip failed: %v", e)
	}
}

func TestRejectsUnsupportedFormat(t *testing.T) {
	app := appir.Empty()
	app.FormatVersion = "bean/appir/v99"
	if e := app.ValidateFormat(); e == nil {
		t.Fatal("unsupported format accepted")
	}
}
