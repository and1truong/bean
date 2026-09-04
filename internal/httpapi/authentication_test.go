package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/bootstrap"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/definition"
)

func TestAuthenticationRegistrationActivationAndEnforcement(t *testing.T) {
	ctx := context.Background()
	runtime, err := bootstrap.OpenURL(ctx, filepath.Join(t.TempDir(), "auth.db"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.DB.Close()
	bundle, err := examples.Load("blog")
	if err != nil {
		t.Fatal(err)
	}
	config := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": "internal", "registration": false}}
	bundle.Definitions = append(bundle.Definitions, config)
	handler := runtime.HTTP.Handler()
	for index, enabled := range []bool{false, true, false} {
		config.Spec["registration"] = enabled
		if err := runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
			t.Fatal(err)
		}
		if _, ds, err := runtime.Store.Publish(ctx, "default"); err != nil || len(ds) > 0 {
			t.Fatalf("publish: %v %v", err, ds)
		}
		// Re-load the persisted AppIR, not just the compiler's in-memory result.
		if err := runtime.Store.LoadActive(ctx, "default"); err != nil {
			t.Fatal(err)
		}
		app, err := runtime.Store.ActiveApp(ctx, "default")
		if err != nil {
			t.Fatal(err)
		}
		if app.RegistrationEnabled() != enabled {
			t.Fatal("capability not activated")
		}
		manifest := serve(t, handler, http.MethodGet, "/api/system/manifest", nil, nil, "")
		var body map[string]any
		if err := json.Unmarshal(manifest.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if (body["localRegistration"] != nil) != enabled {
			t.Fatalf("manifest: %s", manifest.Body.String())
		}
		if strings.Contains(string(app.OpenAPI), "/api/actions/register_member") != enabled {
			t.Fatal("OpenAPI capability mismatch")
		}
		page := serve(t, handler, http.MethodGet, "/api/system/page?path=/signup", nil, nil, "")
		if page.Code != 200 {
			t.Fatalf("page: %d %s", page.Code, page.Body.String())
		}
		if strings.Contains(page.Body.String(), "WebformBlock") != enabled {
			t.Fatalf("signup form capability mismatch: %s", page.Body.String())
		}
		for channel, path := range []string{"/api/actions/register_member", "/api/webforms/signup/submit?_page=/signup&_block=signup_form", "direct"} {
			input := map[string]any{"display_name": "Member", "email": fmt.Sprintf("member-%d-%d@example.test", index, channel), "password": "test-password", "password_confirmation": "test-password"}
			if path == "direct" {
				_, err := runtime.HTTP.Actions.Execute(ctx, app, "register_member", input, beanctx.Request{})
				if enabled && err != nil || !enabled && !dbal.IsCode(err, dbal.NotFound) {
					t.Fatalf("direct: %v", err)
				}
			} else {
				result := serve(t, handler, http.MethodPost, path, input, nil, "")
				if enabled && result.Code != 200 || !enabled && result.Code == 200 {
					t.Fatalf("%s enabled=%v: %d %s", path, enabled, result.Code, result.Body.String())
				}
			}
		}
	}
	// An invalid replacement cannot silently reopen registration.
	config.Spec["preset"] = "unsupported"
	if err := runtime.Store.SaveBundle(ctx, "default", bundle); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, err := runtime.Store.Publish(ctx, "default"); err == nil && len(diagnostics) == 0 {
		t.Fatal("invalid auth configuration published")
	}
	active, err := runtime.Store.ActiveApp(ctx, "default")
	if err != nil || active.RegistrationEnabled() {
		t.Fatalf("invalid publish replaced active auth: %v", err)
	}
	rows, err := runtime.DB.Select(ctx, dbal.Select{Table: "bean_user", Limit: 100})
	if err != nil || len(rows) != 3 {
		t.Fatalf("disabled paths created users: %d %v", len(rows), err)
	}
}
