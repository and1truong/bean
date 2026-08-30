package action

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/audit"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/uid"
)

type Service struct{ DB dbal.Database }

func (s Service) Execute(ctx context.Context, app *appir.App, name string, input map[string]any, request beanctx.Request) (dbal.Row, error) {
	a, ok := app.Actions[name]
	if !ok {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "Action not found"}
	}
	e, ok := app.Entities[a.Entity]
	if !ok {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "Action entity is invalid"}
	}
	if !authorize(app, a.Policy, true, request, nil) {
		return nil, &dbal.Error{Code: dbal.Conflict, Message: "Action is not permitted"}
	}
	for inputName, definition := range a.Input {
		if er := field.Validate(definition, input[inputName]); er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
	}
	for inputName := range input {
		if strings.HasPrefix(inputName, "_") {
			continue
		}
		if _, declared := a.Input[inputName]; !declared {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "undeclared Action input " + inputName}
		}
	}
	idempotencyKey, _ := input["_idempotencyKey"].(string)
	inputHash, fingerprintErr := fingerprint(input)
	if fingerprintErr != nil {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "Action input cannot be fingerprinted", Cause: fingerprintErr}
	}
	if idempotencyKey != "" {
		if replay, found, er := s.replay(ctx, name, idempotencyKey, inputHash); er != nil {
			return nil, er
		} else if found {
			return replay, nil
		}
	}
	var result dbal.Row
	var entityID string
	changed := []string{}
	err := s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		if a.Policy != "" && (a.Operation == "update" || a.Operation == "delete" || a.Operation == "transition") {
			id := fmt.Sprint(input["id"])
			rows, x := tx.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
			if x != nil {
				return x
			}
			if len(rows) == 0 || !policy.Can(app.Policies[a.Policy], true, request, recordMap(rows[0])) {
				return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
		}
		var er error
		switch a.Operation {
		case "create":
			result, er = create(ctx, tx, app, e, input, request)
		case "update":
			for _, candidate := range app.Actions {
				if candidate.Entity != a.Entity || candidate.Operation != "transition" {
					continue
				}
				fieldName := candidate.StateField
				if fieldName == "" {
					fieldName = "status"
				}
				if _, exists := input[fieldName]; exists {
					return &dbal.Error{Code: dbal.Conflict, Message: "protected state requires a transition Action"}
				}
			}
			result, er = update(ctx, tx, app, e, input, a, request, false)
		case "transition":
			result, er = update(ctx, tx, app, e, input, a, request, true)
		case "delete":
			result, er = remove(ctx, tx, e, input)
		case "transaction":
			result, er = steps(ctx, tx, app, a, input, request)
		default:
			er = &dbal.Error{Code: dbal.InvalidQuery, Message: "unsupported Action operation"}
		}
		if er != nil {
			return er
		}
		for outputName, value := range result {
			definition, declared := a.Output[outputName]
			if !declared {
				return &dbal.Error{Code: dbal.InvalidQuery, Message: "undeclared Action output " + outputName}
			}
			if x := field.Validate(definition, value); x != nil {
				return &dbal.Error{Code: dbal.InvalidQuery, Message: x.Error()}
			}
		}
		if result != nil {
			entityID = fmt.Sprint(result["id"])
			for k := range result {
				if !systemField(k) {
					changed = append(changed, k)
				}
			}
			sort.Strings(changed)
		}
		if idempotencyKey != "" {
			encoded, x := json.Marshal(result)
			if x != nil {
				return x
			}
			if _, x = tx.Insert(ctx, dbal.Insert{Table: "bean_idempotency", Values: map[string]dbal.Value{"action": name, "key": idempotencyKey, "input_hash": inputHash, "result": string(encoded), "created_at": time.Now().UTC().Format(time.RFC3339Nano)}}); x != nil {
				return x
			}
		}
		return audit.Write(ctx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: e.Name, EntityID: entityID, Changed: changed, Success: true})
	})
	if err != nil {
		if idempotencyKey != "" && dbal.IsCode(err, dbal.UniqueViolation) {
			if replay, found, replayErr := s.replay(ctx, name, idempotencyKey, inputHash); replayErr != nil {
				return nil, replayErr
			} else if found {
				return replay, nil
			}
		}
		_ = s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
			return audit.Write(ctx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: e.Name, Success: false, Error: safeError(err)})
		})
		return nil, err
	}
	return result, nil
}
func (s Service) replay(ctx context.Context, actionName, key, inputHash string) (dbal.Row, bool, error) {
	p := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "action", Value: actionName}, dbal.Predicate{Op: dbal.OpEQ, Column: "key", Value: key})
	rows, e := s.DB.Select(ctx, dbal.Select{Table: "bean_idempotency", Columns: []string{"input_hash", "result"}, Where: &p, Limit: 1})
	if e != nil {
		return nil, false, e
	}
	if len(rows) == 0 {
		return nil, false, nil
	}
	if fmt.Sprint(rows[0]["input_hash"]) != inputHash {
		return nil, false, &dbal.Error{Code: dbal.Conflict, Message: "idempotency key was already used with different input"}
	}
	var out dbal.Row
	if e = json.Unmarshal([]byte(fmt.Sprint(rows[0]["result"])), &out); e != nil {
		return nil, false, e
	}
	return out, true, nil
}

func fingerprint(input map[string]any) (string, error) {
	canonical := make(map[string]any, len(input))
	for name, value := range input {
		if name != "_idempotencyKey" {
			canonical[name] = value
		}
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(encoded)), nil
}
func create(ctx context.Context, tx dbal.Transaction, app *appir.App, e appir.Entity, input map[string]any, c beanctx.Request) (dbal.Row, error) {
	values := map[string]dbal.Value{}
	for _, f := range e.Fields {
		v := input[f.Name]
		if er := field.Validate(f, v); er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		if toMany(f) {
			continue
		}
		if v != nil {
			if f.Type == "richtext" {
				v = field.SanitizeRichText(v.(string))
			}
			values[f.Name] = v
		}
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	values["id"] = uid.New()
	values["created_at"] = now
	values["updated_at"] = now
	values["version"] = 1
	if e.Owner && c.User != nil {
		values["owner_id"] = c.User.ID
	}
	if e.Tenant {
		if c.TenantID == "" {
			return nil, &dbal.Error{Code: dbal.Conflict, Message: "tenant context is required"}
		}
		values["tenant_id"] = c.TenantID
	}
	if _, er := tx.Insert(ctx, dbal.Insert{Table: e.Name, Values: values}); er != nil {
		return nil, er
	}
	if er := syncRelations(ctx, tx, app, e, fmt.Sprint(values["id"]), input, c, false); er != nil {
		return nil, er
	}
	for _, f := range e.Fields {
		if toMany(f) && input[f.Name] != nil {
			values[f.Name] = input[f.Name]
		}
	}
	return dbal.Row(values), nil
}
func update(ctx context.Context, tx dbal.Transaction, app *appir.App, e appir.Entity, input map[string]any, a appir.Action, c beanctx.Request, transition bool) (dbal.Row, error) {
	id := fmt.Sprint(input["id"])
	if id == "" {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "id is required"}
	}
	rows, er := tx.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if er != nil {
		return nil, er
	}
	if len(rows) == 0 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	row := rows[0]
	values := map[string]dbal.Value{}
	fields := map[string]appir.Field{}
	stateField := a.StateField
	if stateField == "" {
		stateField = "status"
	}
	for _, f := range e.Fields {
		fields[f.Name] = f
	}
	for k, v := range input {
		f, ok := fields[k]
		if !ok {
			continue
		}
		if transition && k == stateField {
			from := fmt.Sprint(row[k])
			to := fmt.Sprint(v)
			if !allowedTransition(a.Transitions, from, to) {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: "state transition is not allowed"}
			}
		} else if !transition && k == stateField && len(a.Transitions) > 0 {
			return nil, &dbal.Error{Code: dbal.Conflict, Message: "protected state requires a transition Action"}
		}
		if er = field.Validate(f, v); er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		if toMany(f) {
			continue
		}
		values[k] = v
	}
	version := toInt(row["version"])
	values["version"] = version + 1
	values["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, dbal.Predicate{Op: dbal.OpEQ, Column: "version", Value: version})
	if _, er = tx.Update(ctx, dbal.Update{Table: e.Name, Values: values, Where: where, ExpectedRows: 1}); er != nil {
		return nil, er
	}
	if er = syncRelations(ctx, tx, app, e, id, input, c, true); er != nil {
		return nil, er
	}
	for k, v := range values {
		row[k] = v
	}
	for _, f := range e.Fields {
		if toMany(f) && input[f.Name] != nil {
			row[f.Name] = input[f.Name]
		}
	}
	return row, nil
}
func remove(ctx context.Context, tx dbal.Transaction, e appir.Entity, input map[string]any) (dbal.Row, error) {
	id := fmt.Sprint(input["id"])
	if e.SoftDelete {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, er := tx.Update(ctx, dbal.Update{Table: e.Name, Values: map[string]dbal.Value{"deleted_at": now}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1})
		return dbal.Row{"id": id}, er
	}
	_, er := tx.Delete(ctx, dbal.Delete{Table: e.Name, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1})
	return dbal.Row{"id": id}, er
}
func steps(ctx context.Context, tx dbal.Transaction, app *appir.App, a appir.Action, input map[string]any, c beanctx.Request) (dbal.Row, error) {
	var out dbal.Row
	results := map[string]any{}
	for _, step := range a.Steps {
		bindings := bindingInput(input, results)
		var stepResult any
		switch step.Op {
		case "load":
			entity, e := stepEntity(app, a, step)
			if e != nil {
				return nil, e
			}
			values := resolveValues(step.Values, input, results, c)
			id := fmt.Sprint(values["id"])
			rows, e := tx.Select(ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
			if e != nil {
				return nil, e
			}
			if len(rows) == 0 {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			if entity.Policy != "" && !authorize(app, entity.Policy, false, c, recordMap(rows[0])) {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			stepResult = rows[0]
			out = rows[0]
		case "query":
			viewDefinition, exists := app.Views[step.View]
			if !exists {
				return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "query step references an invalid View"}
			}
			entity := app.Entities[viewDefinition.Entity]
			var predicates []dbal.Predicate
			for _, expression := range []*expr.Expr{viewDefinition.Filter, viewDefinition.ContextFilter, step.Where} {
				if expression == nil {
					continue
				}
				p, x := expr.PredicateContext(*expression, bindings, c)
				if x != nil {
					return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: x.Error()}
				}
				predicates = append(predicates, p)
			}
			policyName := viewDefinition.Policy
			if policyName == "" {
				policyName = entity.Policy
			}
			var redact []string
			if policyName != "" {
				definition, valid := app.Policies[policyName]
				if !valid {
					return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "query View policy is invalid"}
				}
				p, allowed := policy.Predicate(definition, c)
				if !allowed {
					return nil, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
				}
				if p != nil {
					predicates = append(predicates, *p)
				}
				redact = definition.Redact
			}
			if entity.Tenant && policyName == "" {
				if c.TenantID == "" {
					return nil, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
				}
				predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "tenant_id", Value: c.TenantID})
			}
			if entity.SoftDelete {
				predicates = append(predicates, dbal.Predicate{Op: dbal.OpIsNull, Column: "deleted_at"})
			}
			var where *dbal.Predicate
			if len(predicates) == 1 {
				where = &predicates[0]
			} else if len(predicates) > 1 {
				x := dbal.And(predicates...)
				where = &x
			}
			joins := []dbal.Join{}
			for _, relationship := range viewDefinition.Relationships {
				var relationDefinition *appir.Field
				for _, field := range entity.Fields {
					if field.Name == relationship.RelationField {
						copy := field
						relationDefinition = &copy
					}
				}
				if relationDefinition != nil && toMany(*relationDefinition) {
					linkAlias := relationship.Name + "_links"
					joins = append(joins,
						dbal.Join{Table: entity.Name + "_" + relationDefinition.Name, Alias: linkAlias, Type: relationship.Type, Left: entity.Name + ".id", Right: linkAlias + "." + entity.Name + "_id"},
						dbal.Join{Table: relationship.Entity, Alias: relationship.Name, Type: relationship.Type, Left: linkAlias + "." + relationship.Entity + "_id", Right: relationship.Name + "." + relationship.TargetField},
					)
				} else {
					joins = append(joins, dbal.Join{Table: relationship.Entity, Alias: relationship.Name, Type: relationship.Type, Left: entity.Name + "." + relationship.LocalField, Right: relationship.Name + "." + relationship.TargetField})
				}
			}
			aggregates := []dbal.Aggregate{}
			for _, aggregate := range viewDefinition.Aggregates {
				fn := aggregate.Function
				if fn == "average" {
					fn = "avg"
				}
				aggregates = append(aggregates, dbal.Aggregate{Function: fn, Column: aggregate.Field, Alias: aggregate.Alias})
			}
			orders := []dbal.Order{}
			for _, order := range viewDefinition.Sort {
				orders = append(orders, dbal.Order{Column: order.Field, Desc: order.Desc})
			}
			limit := viewDefinition.MaxLimit
			if limit <= 0 || limit > 200 {
				limit = 200
			}
			rows, x := tx.Select(ctx, dbal.Select{Table: entity.Name, Columns: viewDefinition.Fields, Joins: joins, Where: where, GroupBy: viewDefinition.GroupBy, Aggregates: aggregates, OrderBy: orders, Limit: limit})
			if x != nil {
				return nil, x
			}
			for _, row := range rows {
				policy.Redact(row, redact)
			}
			stepResult = rows
		case "assert":
			if step.Condition == nil {
				return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "assert condition is missing"}
			}
			ok, e := expr.Eval(*step.Condition, c, bindings)
			if e != nil || !ok {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: message(step, "Action precondition failed")}
			}
		case "assert_no_overlap":
			if e := noOverlap(ctx, tx, a.Entity, resolveValues(step.Values, input, results, c), input); e != nil {
				return nil, e
			}
		case "decrement":
			if e := decrement(ctx, tx, resolveValues(step.Values, input, results, c), input); e != nil {
				return nil, e
			}
		case "create":
			entity, x := stepEntity(app, a, step)
			if x != nil {
				return nil, x
			}
			if entity.Policy != "" && !authorize(app, entity.Policy, true, c, nil) {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: "Action is not permitted"}
			}
			values := resolveValues(step.Values, input, results, c)
			delete(values, "entity")
			row, x := create(ctx, tx, app, entity, values, c)
			if x != nil {
				return nil, x
			}
			out = row
			stepResult = row
		case "update", "conditional_update", "transition":
			entity, x := stepEntity(app, a, step)
			if x != nil {
				return nil, x
			}
			values := resolveValues(step.Values, input, results, c)
			id := fmt.Sprint(values["id"])
			loaded, x := tx.Select(ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
			if x != nil {
				return nil, x
			}
			if len(loaded) == 0 {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			if entity.Policy != "" && !authorize(app, entity.Policy, true, c, recordMap(loaded[0])) {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			if step.Op == "conditional_update" {
				if step.Condition == nil {
					return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "conditional update condition is missing"}
				}
				cc := c
				cc.Entity = recordMap(loaded[0])
				ok, er := expr.Eval(*step.Condition, cc, bindings)
				if er != nil || !ok {
					return nil, &dbal.Error{Code: dbal.Conflict, Message: message(step, "conditional update failed")}
				}
			}
			operation := appir.Action{Entity: entity.Name, StateField: step.StateField, Transitions: a.Transitions}
			row, x := update(ctx, tx, app, entity, values, operation, c, step.Op == "transition")
			if x != nil {
				return nil, x
			}
			out = row
			stepResult = row
		case "delete":
			entity, x := stepEntity(app, a, step)
			if x != nil {
				return nil, x
			}
			values := resolveValues(step.Values, input, results, c)
			id := fmt.Sprint(values["id"])
			loaded, x := tx.Select(ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
			if x != nil {
				return nil, x
			}
			if len(loaded) == 0 {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			if entity.Policy != "" && !authorize(app, entity.Policy, true, c, recordMap(loaded[0])) {
				return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			row, x := remove(ctx, tx, entity, values)
			if x != nil {
				return nil, x
			}
			out = row
			stepResult = row
		case "emit":
			payload := any(bindings)
			if len(step.Values) > 0 {
				payload = resolveValues(step.Values, input, results, c)
			}
			if e := event.Emit(ctx, tx, step.Event, payload); e != nil {
				return nil, e
			}
		case "schedule":
			payload := any(bindings)
			if len(step.Values) > 0 {
				payload = resolveValues(step.Values, input, results, c)
			}
			if e := job.Schedule(ctx, tx, step.Job, time.Now().UTC(), payload); e != nil {
				return nil, e
			}
		case "return":
			if out == nil {
				out = dbal.Row{}
			}
			for k, v := range resolveValues(step.Values, input, results, c) {
				out[k] = v
			}
			stepResult = out
		default:
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "unsupported Action step " + step.Op}
		}
		if step.Result != "" {
			results[step.Result] = stepResult
		}
	}
	return out, nil
}
func noOverlap(ctx context.Context, tx dbal.Transaction, entity string, cfg, input map[string]any) error {
	startField := stringCfg(cfg, "start", "start_at")
	endField := stringCfg(cfg, "end", "end_at")
	match := stringCfg(cfg, "match", "resource_id")
	p := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: match, Value: input[match]}, dbal.Predicate{Op: dbal.OpLT, Column: startField, Value: input[endField]}, dbal.Predicate{Op: dbal.OpGT, Column: endField, Value: input[startField]})
	rows, e := tx.Select(ctx, dbal.Select{Table: entity, Columns: []string{"id"}, Where: &p, Limit: 1})
	if e != nil {
		return e
	}
	if len(rows) > 0 {
		return &dbal.Error{Code: dbal.Conflict, Message: stringCfg(cfg, "message", "The requested booking overlaps an existing booking.")}
	}
	return nil
}
func decrement(ctx context.Context, tx dbal.Transaction, cfg, input map[string]any) error {
	entity := fmt.Sprint(cfg["entity"])
	fieldName := stringCfg(cfg, "field", "inventory")
	idInput := stringCfg(cfg, "id_input", "id")
	amountInput := stringCfg(cfg, "amount_input", "amount")
	rows, e := tx.Select(ctx, dbal.Select{Table: entity, Columns: []string{"id", fieldName, "version"}, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: input[idInput]}, Limit: 1})
	if e != nil {
		return e
	}
	if len(rows) == 0 {
		return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	amount := toInt(input[amountInput])
	current := toInt(rows[0][fieldName])
	if amount < 1 || current < amount {
		return &dbal.Error{Code: dbal.Conflict, Message: stringCfg(cfg, "message", "Insufficient quantity.")}
	}
	version := toInt(rows[0]["version"])
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["id"]}, dbal.Predicate{Op: dbal.OpEQ, Column: "version", Value: version})
	_, e = tx.Update(ctx, dbal.Update{Table: entity, Values: map[string]dbal.Value{fieldName: current - amount, "version": version + 1, "updated_at": time.Now().UTC().Format(time.RFC3339Nano)}, Where: where, ExpectedRows: 1})
	return e
}
func resolveValues(values []appir.Assignment, input map[string]any, results map[string]any, c beanctx.Request) map[string]any {
	out := map[string]any{}
	for _, assignment := range values {
		out[assignment.Field] = resolveValue(assignment.Value, input, results, c)
	}
	return out
}
func resolveValue(binding appir.ValueBinding, input map[string]any, results map[string]any, c beanctx.Request) any {
	switch binding.Source {
	case "literal":
		var value any
		_ = json.Unmarshal(binding.Literal, &value)
		return value
	case "input":
		return input[binding.Path]
	case "record":
		return c.Entity[binding.Path]
	case "result":
		parts := strings.Split(binding.Path, ".")
		if len(parts) > 0 {
			v := results[parts[0]]
			for _, p := range parts[1:] {
				switch x := v.(type) {
				case dbal.Row:
					v = x[p]
				case map[string]any:
					v = x[p]
				case []dbal.Row:
					i, e := strconv.Atoi(p)
					if e != nil || i < 0 || i >= len(x) {
						return nil
					}
					v = x[i]
				default:
					return nil
				}
			}
			return v
		}
	case "context":
		return c.Values[binding.Path]
	case "tenant":
		return c.TenantID
	case "user":
		if c.User != nil {
			if binding.Path == "id" {
				return c.User.ID
			}
			if binding.Path == "email" {
				return c.User.Email
			}
		}
	}
	return nil
}
func bindingInput(input map[string]any, results map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range input {
		out[k] = v
	}
	for name, value := range results {
		switch x := value.(type) {
		case dbal.Row:
			for k, v := range x {
				out["result."+name+"."+k] = v
			}
		case map[string]any:
			for k, v := range x {
				out["result."+name+"."+k] = v
			}
		case []dbal.Row:
			out["result."+name+".count"] = len(x)
			for i, row := range x {
				for k, v := range row {
					out[fmt.Sprintf("result.%s.%d.%s", name, i, k)] = v
				}
			}
		}
	}
	return out
}
func stepEntity(app *appir.App, a appir.Action, s appir.Step) (appir.Entity, error) {
	name := s.Entity
	if name == "" {
		name = a.Entity
	}
	if name == a.Entity {
		for _, assignment := range s.Values {
			if assignment.Field == "entity" && assignment.Value.Source == "literal" {
				_ = json.Unmarshal(assignment.Value.Literal, &name)
			}
		}
	}
	entity, ok := app.Entities[name]
	if !ok {
		return entity, &dbal.Error{Code: dbal.InvalidQuery, Message: "Action step references unknown Entity " + name}
	}
	return entity, nil
}
func toMany(f appir.Field) bool {
	return f.Type == "relation" && f.Relation != nil && (f.Relation.Kind == "one-to-many" || f.Relation.Kind == "many-to-many")
}
func syncRelations(ctx context.Context, tx dbal.Transaction, app *appir.App, entity appir.Entity, entityID string, input map[string]any, c beanctx.Request, replace bool) error {
	for _, f := range entity.Fields {
		if !toMany(f) || input[f.Name] == nil {
			continue
		}
		values, ok := input[f.Name].([]any)
		if !ok {
			return &dbal.Error{Code: dbal.InvalidQuery, Message: f.Name + " must be a list of UUIDs"}
		}
		target := app.Entities[f.Relation.Entity]
		targetField := f.Relation.TargetField
		if targetField == "" {
			targetField = "id"
		}
		for _, value := range values {
			rows, e := tx.Select(ctx, dbal.Select{Table: target.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: targetField, Value: value}, Limit: 1})
			if e != nil {
				return e
			}
			if len(rows) == 0 || target.Policy != "" && !authorize(app, target.Policy, false, c, recordMap(rows[0])) {
				return &dbal.Error{Code: dbal.NotFound, Message: "related record not found"}
			}
		}
		table := entity.Name + "_" + f.Name
		entityColumn := entity.Name + "_id"
		targetColumn := target.Name + "_id"
		if replace {
			if _, e := tx.Delete(ctx, dbal.Delete{Table: table, Where: dbal.Predicate{Op: dbal.OpEQ, Column: entityColumn, Value: entityID}}); e != nil {
				return e
			}
		}
		for _, value := range values {
			if _, e := tx.Insert(ctx, dbal.Insert{Table: table, Values: map[string]dbal.Value{entityColumn: entityID, targetColumn: value}}); e != nil {
				return e
			}
		}
	}
	return nil
}
func authorize(app *appir.App, name string, write bool, c beanctx.Request, row map[string]any) bool {
	if c.User != nil {
		for _, r := range c.User.Roles {
			if r == "administrator" {
				return true
			}
		}
	}
	if name == "" {
		return !write
	}
	p, ok := app.Policies[name]
	return ok && policy.Can(p, write, c, row)
}
func allowedTransition(m map[string][]string, from, to string) bool {
	for _, v := range m[from] {
		if v == to {
			return true
		}
	}
	return false
}
func stringCfg(m map[string]any, k, d string) string {
	if s, ok := m[k].(string); ok && s != "" {
		return s
	}
	return d
}
func message(s appir.Step, d string) string {
	for _, assignment := range s.Values {
		if assignment.Field == "message" && assignment.Value.Source == "literal" {
			var value string
			if json.Unmarshal(assignment.Value.Literal, &value) == nil && value != "" {
				return value
			}
		}
	}
	return d
}
func toInt(v any) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}
func userID(c beanctx.Request) string {
	if c.User == nil {
		return ""
	}
	return c.User.ID
}
func systemField(k string) bool { return k == "created_at" || k == "updated_at" || k == "version" }
func safeError(e error) string {
	if x, ok := e.(*dbal.Error); ok {
		return x.Message
	}
	return "Action failed"
}
func recordMap(row dbal.Row) map[string]any {
	out := map[string]any{}
	for k, v := range row {
		out[k] = v
	}
	return out
}
