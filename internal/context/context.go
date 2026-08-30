package context

type User struct {
	ID, Email, DisplayName string
	Roles                  []string
}
type Request struct {
	User                               *User
	TenantID, Locale, Route, RequestID string
	RouteParams                        map[string]string
	Entity                             map[string]any
	Values                             map[string]any
}
