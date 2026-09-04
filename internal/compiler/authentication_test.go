package compiler_test

import (
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"testing"
)

func TestAuthenticationPresetsAndRegistration(t *testing.T) {
	for _, preset := range []string{"local", "internal", "public"} {
		for _, enabled := range []bool{false, true} {
			defs := []definition.Definition{
				{APIVersion: "bean/v1alpha1", Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": preset, "registration": enabled}},
				{APIVersion: "bean/v1alpha1", Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
				{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
				{APIVersion: "bean/v1alpha1", Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup"}},
			}
			result := compiler.Compile("test", 1, defs)
			if len(result.Diagnostics) > 0 {
				t.Fatal(result.Diagnostics)
			}
			if result.App.RegistrationEnabled() != enabled {
				t.Fatalf("preset %s enabled %v", preset, enabled)
			}
			if err := result.App.ValidateFormat(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestAuthenticationRejectsInvalidOrUnsupportedConfiguration(t *testing.T) {
	for _, spec := range []map[string]any{
		{"preset": "unknown"}, {}, {"preset": "internal", "registration": true},
		{"preset": "public", "emailVerification": true}, {"preset": "public", "passwordRecovery": true},
		{"preset": "local", "csrf": false}, {"preset": "internal", "mfa": true},
	} {
		result := compiler.Compile("test", 1, []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: spec}})
		if len(result.Diagnostics) == 0 {
			t.Fatalf("accepted %v", spec)
		}
	}
	for _, preset := range []string{"local", "internal", "public"} {
		result := compiler.Compile("test", 1, []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Authentication", Metadata: definition.Metadata{Name: "auth"}, Spec: map[string]any{"preset": preset}}})
		if len(result.Diagnostics) > 0 || result.App.RegistrationEnabled() {
			t.Fatalf("preset default %s: %v", preset, result.Diagnostics)
		}
	}
}
