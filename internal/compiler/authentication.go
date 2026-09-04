package compiler

import (
	"reflect"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/definition"
)

func authenticationDefinitionKind() definitionKind {
	return definitionKind{
		Specification: reflect.TypeOf(appir.Authentication{}),
		Storage:       reflect.TypeOf(appir.Authentication{}),
		Normalize:     noDefinitionNormalization,
		Compile: func(app *appir.App, source definition.Definition) []definition.Diagnostic {
			var value appir.Authentication
			if err := definition.DecodeSpec(source.Spec, &value); err != nil {
				return []definition.Diagnostic{diagError(source, "spec", err)}
			}
			if source.Metadata.Name != "auth" {
				return []definition.Diagnostic{diagWithRule(source, definition.RuleGeneral, "metadata.name", "Authentication must be named auth")}
			}
			if app.Authentication != nil {
				return []definition.Diagnostic{diagWithRule(source, definition.RuleDuplicate, "metadata.name", "only one Authentication definition is allowed")}
			}
			app.Authentication = &value
			return nil
		},
		Lookup: func(app *appir.App, _ string) (any, bool) {
			if app.Authentication == nil {
				return nil, false
			}
			return *app.Authentication, true
		},
		Names: func(app *appir.App) []string {
			if app.Authentication == nil {
				return nil
			}
			return []string{"auth"}
		},
		Validate: func(app *appir.App, _ *validationState) []definition.Diagnostic {
			if app.Authentication == nil {
				return nil
			}
			var out []definition.Diagnostic
			switch app.Authentication.Preset {
			case "local", "internal", "public":
			default:
				out = append(out, diagnostic("Authentication", "auth", "spec.preset", "must be local, internal, or public"))
			}
			if app.Authentication.Registration && app.LocalRegistration == nil {
				out = append(out, diagnostic("Authentication", "auth", "spec.registration", "requires a LocalRegistration definition with a fixed-role registration Action"))
			}
			return out
		},
	}
}
