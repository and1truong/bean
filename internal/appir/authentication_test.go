package appir

import "testing"

func TestAuthenticationFormatCompatibility(t *testing.T) {
	for _, format := range []string{LegacyFormat, DirectionalFormat, CurrentFormat} {
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
		if (err == nil) != (format == CurrentFormat) {
			t.Fatalf("format %s: %v", format, err)
		}
		if app.FormatVersion != format {
			t.Fatal("validation mutated snapshot")
		}
	}
}
