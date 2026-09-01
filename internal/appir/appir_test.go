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

func TestLifecycleRequiresV2Format(t *testing.T) {
	app := appir.Empty()
	app.Lifecycles["order_fulfillment"] = appir.Lifecycle{
		Name: "order_fulfillment", Entity: "order", StateField: "status", Initial: "pending",
		Transitions: map[string][]string{"pending": {"paid"}},
	}
	encoded, err := json.Marshal(app)
	if err != nil {
		t.Fatal(err)
	}
	var decoded appir.App
	if err = json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if err = decoded.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	if decoded.Lifecycles["order_fulfillment"].Initial != "pending" {
		t.Fatalf("lifecycle=%+v", decoded.Lifecycles["order_fulfillment"])
	}
	decoded.FormatVersion = appir.LifecycleFormat
	if err = decoded.ValidateFormat(); err != nil {
		t.Fatalf("v2 AppIR rejected Lifecycle semantics: %v", err)
	}
	decoded.FormatVersion = appir.LegacyFormat
	if err = decoded.ValidateFormat(); err == nil {
		t.Fatal("v1 AppIR accepted Lifecycle semantics that a v0.8 runtime would discard")
	}
}

func TestRuleRequiresV3Format(t *testing.T) {
	app := appir.Empty()
	app.Rules["always"] = appir.Rule{Name: "always", Result: "boolean"}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.LifecycleFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v2 AppIR accepted Rule semantics that a v0.9 runtime would discard")
	}
	app.FormatVersion = appir.RuleFormat
	if err := app.ValidateFormat(); err != nil {
		t.Fatalf("v3 AppIR rejected Rule semantics: %v", err)
	}
}

func TestTestSuiteRequiresV4Format(t *testing.T) {
	app := appir.Empty()
	app.TestSuites["always"] = appir.TestSuite{Name: "always", Target: appir.TestTarget{Kind: "Rule", Name: "always"}}
	if err := app.ValidateFormat(); err != nil {
		t.Fatal(err)
	}
	app.FormatVersion = appir.RuleFormat
	if err := app.ValidateFormat(); err == nil {
		t.Fatal("v3 AppIR accepted TestSuite semantics that a v0.10 runtime would discard")
	}
}
