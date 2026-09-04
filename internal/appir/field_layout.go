package appir

// FieldLayout is presentation-only. Consumers retain authority over membership,
// projection, and writable inputs; layout never grants access to a field.
type FieldLayout struct {
	Groups []FieldGroup
}
type FieldGroup struct {
	Name, Label string
	Columns     int
	Fields      []LayoutField
}
type LayoutField struct {
	Field, Span string
}
