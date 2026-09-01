package action

import (
	"fmt"
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/rule"
)

func rejectDerivedInput(action appir.Action, input map[string]any) error {
	for name := range action.Derive {
		if _, supplied := input[name]; supplied {
			return &dbal.Error{Code: dbal.InvalidQuery, Message: "derived Action input " + name + " is server-owned"}
		}
	}
	return nil
}

func validateActionInput(action appir.Action, input map[string]any, includeDerived bool) error {
	for name, definition := range action.Input {
		if !includeDerived {
			if _, derived := action.Derive[name]; derived {
				continue
			}
		}
		value := input[name]
		if definition.Type == "file" && value != nil {
			if _, upload := value.(field.Upload); !upload {
				return &dbal.Error{Code: dbal.InvalidQuery, Message: name + " must be uploaded as multipart form data"}
			}
		}
		if err := field.Validate(definition, value); err != nil {
			return &dbal.Error{Code: dbal.InvalidQuery, Message: err.Error()}
		}
	}
	return nil
}

func applyActionDerivations(app *appir.App, action appir.Action, input map[string]any, current dbal.Row, request beanctx.Request) (map[string]any, error) {
	base := copyValues(input)
	derived := map[string]any{}
	names := make([]string, 0, len(action.Derive))
	for name := range action.Derive {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value, err := evaluateRule(app.Rules[action.Derive[name]], base, current, request)
		if err != nil {
			return nil, ruleRuntimeError(action.Derive[name], err)
		}
		derived[name] = value
	}
	for name, value := range derived {
		base[name] = value
	}
	return base, nil
}

func evaluateActionGuard(app *appir.App, action appir.Action, input map[string]any, current dbal.Row, request beanctx.Request) error {
	if action.When == "" {
		return nil
	}
	value, err := evaluateRule(app.Rules[action.When], input, current, request)
	if err != nil {
		return ruleRuntimeError(action.When, err)
	}
	allowed, ok := value.(bool)
	if !ok {
		return ruleRuntimeError(action.When, &rule.Error{Code: rule.CodeType, Path: "expression", Message: "guard result is not boolean"})
	}
	if !allowed {
		return &dbal.Error{Code: dbal.Conflict, Message: "Action guard " + action.When + " denied the mutation"}
	}
	return nil
}

func actionUsesCurrentRecord(app *appir.App, action appir.Action) bool {
	if action.When != "" && rule.UsesSource(app.Rules[action.When].Expression, "this") {
		return true
	}
	for _, ruleName := range action.Derive {
		if rule.UsesSource(app.Rules[ruleName].Expression, "this") {
			return true
		}
	}
	return false
}

func createRuleCandidate(entity appir.Entity, input map[string]any, request beanctx.Request, id, now string) dbal.Row {
	candidate := dbal.Row{"id": id, "created_at": now, "updated_at": now, "version": int64(1)}
	for _, definition := range entity.Fields {
		if value, exists := input[definition.Name]; exists {
			candidate[definition.Name] = value
		}
	}
	if entity.Owner && request.User != nil {
		candidate["owner_id"] = request.User.ID
	}
	if entity.Tenant {
		candidate["tenant_id"] = request.TenantID
	}
	return candidate
}

func validateEntityRules(app *appir.App, entity appir.Entity, candidate dbal.Row, request beanctx.Request) error {
	names := make([]string, 0, len(entity.Validations))
	for name := range entity.Validations {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ruleName := entity.Validations[name]
		value, err := evaluateRule(app.Rules[ruleName], nil, candidate, request)
		if err != nil {
			return ruleRuntimeError(ruleName, err)
		}
		valid, ok := value.(bool)
		if !ok {
			return ruleRuntimeError(ruleName, &rule.Error{Code: rule.CodeType, Path: "expression", Message: "validation result is not boolean"})
		}
		if !valid {
			return &dbal.Error{Code: dbal.Conflict, Message: "Entity validation " + name + " failed"}
		}
	}
	return nil
}

func evaluateRule(definition appir.Rule, input map[string]any, current dbal.Row, request beanctx.Request) (any, error) {
	user := map[string]any(nil)
	if request.User != nil {
		user = map[string]any{
			"id": request.User.ID, "email": request.User.Email, "display_name": request.User.DisplayName, "roles": request.User.Roles,
		}
	}
	contextValues := map[string]any{}
	if request.Values != nil {
		if now, exists := request.Values["now"]; exists {
			contextValues["now"] = now
		}
	}
	if request.RequestID != "" {
		contextValues["request_id"] = request.RequestID
	}
	return rule.EvaluateTyped(definition.Expression, rule.Environment{
		This: recordMap(current), Input: input, User: user, TenantID: request.TenantID, Context: contextValues,
	}, definition.Result)
}

func ruleRuntimeError(name string, cause error) error {
	return &dbal.Error{Code: dbal.InvalidQuery, Message: fmt.Sprintf("Rule %s could not be evaluated", name), Cause: cause}
}

func copyValues(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for name, value := range input {
		out[name] = value
	}
	return out
}
