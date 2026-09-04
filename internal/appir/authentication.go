package appir

// Authentication contains validated application choices, never host credentials.
// Omitted configuration preserves the pre-configuration registration contract.
type Authentication struct {
	Preset           string
	Registration     bool
	PasswordRecovery bool
}

func (a *App) PasswordRecoveryEnabled() bool {
	return a.Authentication != nil && a.Authentication.PasswordRecovery
}

// RegistrationEnabled is shared by HTTP presentation and all Action callers.
func (a *App) RegistrationEnabled() bool {
	return a.LocalRegistration != nil && (a.Authentication == nil || a.Authentication.Registration)
}

func (a *App) RegistrationActionEnabled(name string) bool {
	return a.RegistrationEnabled() && a.LocalRegistration.Action == name
}
