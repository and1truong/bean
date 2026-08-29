package action

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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
	if a.Policy != "" && (a.Operation == "update" || a.Operation == "delete" || a.Operation == "transition") {
		id := fmt.Sprint(input["id"])
		rows, er := s.DB.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
		if er != nil {
			return nil, er
		}
		if len(rows) == 0 || !policy.Can(app.Policies[a.Policy], true, request, recordMap(rows[0])) {
			return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
		}
	}
	var result dbal.Row
	var entityID string
	changed := []string{}
	err := s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		var er error
		switch a.Operation {
		case "create":
			result, er = create(ctx, tx, e, input, request)
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
			result, er = update(ctx, tx, e, input, a, request, false)
		case "transition":
			result, er = update(ctx, tx, e, input, a, request, true)
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
		if result != nil {
			entityID = fmt.Sprint(result["id"])
			for k := range result {
				if !systemField(k) {
					changed = append(changed, k)
				}
			}
			sort.Strings(changed)
		}
		return audit.Write(ctx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: e.Name, EntityID: entityID, Changed: changed, Success: true})
	})
	if err != nil {
		_ = s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
			return audit.Write(ctx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: e.Name, Success: false, Error: safeError(err)})
		})
		return nil, err
	}
	return result, nil
}
func create(ctx context.Context, tx dbal.Transaction, e appir.Entity, input map[string]any, c beanctx.Request) (dbal.Row, error) {
	values := map[string]dbal.Value{}
	for _, f := range e.Fields {
		v := input[f.Name]
		if er := field.Validate(f, v); er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
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
	return dbal.Row(values), nil
}
func update(ctx context.Context, tx dbal.Transaction, e appir.Entity, input map[string]any, a appir.Action, c beanctx.Request, transition bool) (dbal.Row, error) {
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
		values[k] = v
	}
	version := toInt(row["version"])
	values["version"] = version + 1
	values["updated_at"] = time.Now().UTC().Format(time.RFC3339Nano)
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, dbal.Predicate{Op: dbal.OpEQ, Column: "version", Value: version})
	if _, er = tx.Update(ctx, dbal.Update{Table: e.Name, Values: values, Where: where, ExpectedRows: 1}); er != nil {
		return nil, er
	}
	for k, v := range values {
		row[k] = v
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
	for _, step := range a.Steps {
		switch step.Op {
		case "assert":
			if step.Condition == nil {
				return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "assert condition is missing"}
			}
			ok, e := expr.Eval(*step.Condition, c, input)
			if e != nil || !ok {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: message(step, "Action precondition failed")}
			}
		case "assert_no_overlap":
			if e := noOverlap(ctx, tx, a.Entity, step.Values, input); e != nil {
				return nil, e
			}
		case "decrement":
			if e := decrement(ctx, tx, step.Values, input); e != nil {
				return nil, e
			}
		case "create":
			entity := app.Entities[a.Entity]
			if step.Values["entity"] != nil {
				entity = app.Entities[fmt.Sprint(step.Values["entity"])]
			}
			values := resolveValues(step.Values, input)
			delete(values, "entity")
			var e error
			out, e = create(ctx, tx, entity, values, c)
			if e != nil {
				return nil, e
			}
		case "emit":
			if e := event.Emit(ctx, tx, step.Event, input); e != nil {
				return nil, e
			}
		case "schedule":
			if e := job.Schedule(ctx, tx, step.Job, time.Now().UTC(), input); e != nil {
				return nil, e
			}
		case "return":
			if out == nil {
				out = dbal.Row{}
			}
			for k, v := range resolveValues(step.Values, input) {
				out[k] = v
			}
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
func resolveValues(values map[string]any, input map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range values {
		if s, ok := v.(string); ok && strings.HasPrefix(s, "$input.") {
			out[k] = input[strings.TrimPrefix(s, "$input.")]
		} else {
			out[k] = v
		}
	}
	return out
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
func message(s appir.Step, d string) string { return stringCfg(s.Values, "message", d) }
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
