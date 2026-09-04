package action_test

import (
	"context"
	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"testing"
)

func TestRegistrationCannotBypassOptInAtActionBoundary(t *testing.T) {
	app := appir.Empty()
	app.Actions["signup"] = appir.Action{Name: "signup", Operation: "register_local_user"}
	for _, registration := range []*appir.LocalRegistration{nil, {Action: "other"}, {Action: "signup"}} {
		app.LocalRegistration = registration
		app.Authentication = &appir.Authentication{Preset: "internal", Registration: registration == nil || registration.Action != "signup"}
		// No DB is needed: reject before validation, idempotency, or transaction work.
		_, err := (action.Service{}).Execute(context.Background(), app, "signup", map[string]any{"_idempotencyKey": "replay"}, beanctx.Request{})
		if !dbal.IsCode(err, dbal.NotFound) {
			t.Fatalf("registration %+v: %v", registration, err)
		}
	}
	app.Authentication = nil
	app.LocalRegistration = nil
	if _, err := (action.Service{}).Execute(context.Background(), app, "signup", nil, beanctx.Request{}); !dbal.IsCode(err, dbal.NotFound) {
		t.Fatal(err)
	}
}
