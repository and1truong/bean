package user

type User struct {
	ID, Email, TenantID string
	Roles               []string
}
