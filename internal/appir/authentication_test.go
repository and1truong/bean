package appir

import "testing"

func TestAuthenticationFormatCompatibility(t *testing.T) {
	for _, format := range []string{LegacyFormat, DirectionalFormat, AuthenticationFormat, CurrentFormat} {
		app := Empty()
		app.FormatVersion = format
		app.LocalRegistration = &LocalRegistration{Action: "signup"}
		if !app.RegistrationActionEnabled("signup") || app.RegistrationActionEnabled("other") {
			t.Fatal("legacy registration contract changed")
		}
		if err := app.ValidateFormat(); err != nil {
			t.Fatal(err)
		}
		app.Authentication = &Authentication{Preset: "internal"}
		if app.RegistrationEnabled() {
			t.Fatal("disabled registration enabled")
		}
		err := app.ValidateFormat()
		if (err == nil) != (format == CurrentFormat || format == AuthenticationFormat) {
			t.Fatalf("format %s: %v", format, err)
		}
		app.Authentication.PasswordRecovery = true
		if (app.ValidateFormat() == nil) != (format == CurrentFormat) {
			t.Fatal("recovery format boundary", format)
		}
		if app.FormatVersion != format {
			t.Fatal("validation mutated snapshot")
		}
	}
}
