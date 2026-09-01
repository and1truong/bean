package action

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/registry"
)

type stepExecution struct {
	service  Service
	ctx      context.Context
	tx       dbal.Transaction
	app      *appir.App
	action   appir.Action
	step     appir.Step
	input    map[string]any
	request  beanctx.Request
	results  map[string]any
	bindings map[string]any
	output   dbal.Row
}

type stepOutcome struct {
	result        any
	output        dbal.Row
	replaceOutput bool
}

type stepHandler func(stepExecution) (stepOutcome, error)

var stepHandlers = registry.Must(
	registry.Identity[stepHandler],
	registry.Entry[stepHandler]{Name: "assert", Value: executeAssertStep},
	registry.Entry[stepHandler]{Name: "assert_no_overlap", Value: executeNoOverlapStep},
	registry.Entry[stepHandler]{Name: "conditional_update", Value: executeUpdateStep},
	registry.Entry[stepHandler]{Name: "create", Value: executeCreateStep},
	registry.Entry[stepHandler]{Name: "decrement", Value: executeDecrementStep},
	registry.Entry[stepHandler]{Name: "delete", Value: executeDeleteStep},
	registry.Entry[stepHandler]{Name: "emit", Value: executeEmitStep},
	registry.Entry[stepHandler]{Name: "load", Value: executeLoadStep},
	registry.Entry[stepHandler]{Name: "query", Value: executeQueryStep},
	registry.Entry[stepHandler]{Name: "return", Value: executeReturnStep},
	registry.Entry[stepHandler]{Name: "schedule", Value: executeScheduleStep},
	registry.Entry[stepHandler]{Name: "transition", Value: executeUpdateStep},
	registry.Entry[stepHandler]{Name: "update", Value: executeUpdateStep},
)

func (s Service) steps(ctx context.Context, tx dbal.Transaction, app *appir.App, actionDefinition appir.Action, input map[string]any, request beanctx.Request) (dbal.Row, error) {
	var output dbal.Row
	results := map[string]any{}
	for _, step := range actionDefinition.Steps {
		handler, exists := stepHandlers.Lookup(step.Op)
		if !exists {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "unsupported Action step " + step.Op}
		}
		outcome, err := handler(stepExecution{
			service: s, ctx: ctx, tx: tx, app: app, action: actionDefinition, step: step,
			input: input, request: request, results: results, bindings: bindingInput(input, results), output: output,
		})
		if err != nil {
			return nil, err
		}
		if outcome.replaceOutput {
			output = outcome.output
		}
		if step.Result != "" {
			results[step.Result] = outcome.result
		}
	}
	return output, nil
}

func executeLoadStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	values := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	id := fmt.Sprint(values["id"])
	rows, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(rows) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(rows[0], entity)
	if !canReadRecord(execution.app, entity, execution.request, rows[0]) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	return stepOutcome{result: rows[0], output: rows[0], replaceOutput: true}, nil
}

func executeQueryStep(execution stepExecution) (stepOutcome, error) {
	viewDefinition, exists := execution.app.Views[execution.step.View]
	if !exists {
		return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "query step references an invalid View"}
	}
	entity := execution.app.Entities[viewDefinition.Entity]
	joined := len(viewDefinition.Relationships) > 0
	var predicates []dbal.Predicate
	for _, expression := range []*expr.Expr{viewDefinition.Filter, viewDefinition.ContextFilter, execution.step.Where} {
		if expression == nil {
			continue
		}
		predicate, err := expr.PredicateContext(*expression, execution.bindings, execution.request)
		if err != nil {
			return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: err.Error()}
		}
		predicates = append(predicates, qualifyActionPredicate(predicate, entity.Name, joined))
	}
	policyName := viewDefinition.Policy
	if policyName == "" {
		policyName = entity.Policy
	}
	var redact []string
	if policyName != "" {
		definition, valid := execution.app.Policies[policyName]
		if !valid {
			return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "query View policy is invalid"}
		}
		predicate, allowed := policy.Predicate(definition, execution.request)
		if !allowed {
			return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
		}
		if predicate != nil {
			predicates = append(predicates, qualifyActionPredicate(*predicate, entity.Name, joined))
		}
		redact = definition.Redact
	}
	if entity.Owner && policyName == "" {
		if execution.request.User == nil {
			return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "owner_id", Value: execution.request.User.ID})
	}
	if entity.Tenant && policyName == "" {
		if execution.request.TenantID == "" {
			return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
		}
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpEQ, Column: "tenant_id", Value: execution.request.TenantID})
	}
	if entity.SoftDelete {
		predicates = append(predicates, dbal.Predicate{Op: dbal.OpIsNull, Column: "deleted_at"})
	}
	var where *dbal.Predicate
	if len(predicates) == 1 {
		where = &predicates[0]
	} else if len(predicates) > 1 {
		combined := dbal.And(predicates...)
		where = &combined
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
	aggregateAliases := map[string]bool{}
	aggregates := []dbal.Aggregate{}
	for _, aggregate := range viewDefinition.Aggregates {
		function := aggregate.Function
		if function == "average" {
			function = "avg"
		}
		aggregateAliases[aggregate.Alias] = true
		aggregates = append(aggregates, dbal.Aggregate{Function: function, Column: qualifyViewField(aggregate.Field, entity.Name, joined), Alias: aggregate.Alias})
	}
	orders := []dbal.Order{}
	aggregateSort := false
	for _, order := range viewDefinition.Sort {
		column := order.Field
		if aggregateAliases[column] {
			aggregateSort = true
		} else {
			column = qualifyViewField(column, entity.Name, joined)
		}
		orders = append(orders, dbal.Order{Column: column, Desc: order.Desc})
	}
	if aggregateSort {
		for _, group := range viewDefinition.GroupBy {
			groupOrdered := false
			for _, order := range orders {
				if order.Column == group || strings.HasSuffix(order.Column, "."+group) {
					groupOrdered = true
					break
				}
			}
			if !groupOrdered {
				orders = append(orders, dbal.Order{Column: qualifyViewField(group, entity.Name, joined)})
			}
		}
	}
	limit := viewDefinition.MaxLimit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	rows, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Columns: qualifyViewFields(viewDefinition.Fields, entity.Name, joined), Joins: joins, Where: where, GroupBy: qualifyViewFields(viewDefinition.GroupBy, entity.Name, joined), Aggregates: aggregates, OrderBy: orders, Limit: limit})
	if err != nil {
		return stepOutcome{}, err
	}
	for _, row := range rows {
		for _, selected := range viewDefinition.Fields {
			compiled := qualifyViewField(selected, entity.Name, joined)
			encoded := strings.ReplaceAll(compiled, ".", "__")
			if encoded == selected {
				continue
			}
			value, found := row[encoded]
			if !found {
				continue
			}
			row[selected] = value
			delete(row, encoded)
			parts := strings.Split(selected, ".")
			if len(parts) == 2 {
				if _, found = row[parts[1]]; !found {
					row[parts[1]] = value
				}
			}
		}
		hydrate(row, entity)
	}
	for _, row := range rows {
		policy.Redact(row, redact)
		if execution.request.Values != nil && row["id"] != nil {
			execution.request.Values[trustedRelationKey(entity.Name, row["id"])] = true
		}
	}
	return stepOutcome{result: rows}, nil
}

func executeAssertStep(execution stepExecution) (stepOutcome, error) {
	if execution.step.Condition == nil {
		return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "assert condition is missing"}
	}
	allowed, err := expr.Eval(*execution.step.Condition, execution.request, execution.bindings)
	if err != nil || !allowed {
		return stepOutcome{}, &dbal.Error{Code: dbal.Conflict, Message: message(execution.step, "Action precondition failed")}
	}
	return stepOutcome{}, nil
}

func executeNoOverlapStep(execution stepExecution) (stepOutcome, error) {
	err := noOverlap(execution.ctx, execution.tx, execution.action.Entity, resolveValues(execution.step.Values, execution.input, execution.results, execution.request), execution.input)
	return stepOutcome{}, err
}

func executeDecrementStep(execution stepExecution) (stepOutcome, error) {
	err := decrement(execution.ctx, execution.tx, resolveValues(execution.step.Values, execution.input, execution.results, execution.request), execution.input)
	return stepOutcome{}, err
}

func executeCreateStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	if entity.Policy != "" && !authorize(execution.app, entity.Policy, true, execution.request, nil) {
		return stepOutcome{}, &dbal.Error{Code: dbal.Conflict, Message: "Action is not permitted"}
	}
	values := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	delete(values, "entity")
	row, err := execution.service.create(execution.ctx, execution.tx, execution.app, entity, values, execution.request, "")
	if err != nil {
		return stepOutcome{}, err
	}
	return stepOutcome{result: row, output: row, replaceOutput: true}, nil
}

func executeUpdateStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	values := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	id := fmt.Sprint(values["id"])
	loaded, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(loaded) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(loaded[0], entity)
	if entity.Policy != "" && !authorize(execution.app, entity.Policy, true, execution.request, recordMap(loaded[0])) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	specification, _ := actionstep.Lookup(execution.step.Op)
	if specification.RequiresCondition {
		if execution.step.Condition == nil {
			return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "conditional update condition is missing"}
		}
		request := execution.request
		request.Entity = recordMap(loaded[0])
		allowed, evaluateErr := expr.Eval(*execution.step.Condition, request, execution.bindings)
		if evaluateErr != nil || !allowed {
			return stepOutcome{}, &dbal.Error{Code: dbal.Conflict, Message: message(execution.step, "conditional update failed")}
		}
	}
	operation := appir.Action{Entity: entity.Name, Lifecycle: execution.action.Lifecycle, StateField: execution.step.StateField, Transitions: execution.action.Transitions}
	row, err := update(execution.ctx, execution.tx, execution.app, entity, values, operation, execution.request, fmt.Sprint(execution.request.Values["now"]), specification.Transition)
	if err != nil {
		return stepOutcome{}, err
	}
	return stepOutcome{result: row, output: row, replaceOutput: true}, nil
}

func executeDeleteStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	values := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	id := fmt.Sprint(values["id"])
	loaded, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(loaded) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(loaded[0], entity)
	if entity.Policy != "" && !authorize(execution.app, entity.Policy, true, execution.request, recordMap(loaded[0])) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	row, err := remove(execution.ctx, execution.tx, entity, values)
	if err != nil {
		return stepOutcome{}, err
	}
	return stepOutcome{result: row, output: row, replaceOutput: true}, nil
}

func executeEmitStep(execution stepExecution) (stepOutcome, error) {
	payload := any(execution.bindings)
	if len(execution.step.Values) > 0 {
		payload = resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	}
	return stepOutcome{}, event.Emit(execution.ctx, execution.tx, execution.step.Event, payload)
}

func executeScheduleStep(execution stepExecution) (stepOutcome, error) {
	payload := execution.bindings
	if len(execution.step.Values) > 0 {
		payload = resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	}
	delete(payload, job.TenantIDPayloadKey)
	if execution.request.TenantID != "" {
		payload[job.TenantIDPayloadKey] = execution.request.TenantID
	}
	return stepOutcome{}, job.Schedule(execution.ctx, execution.tx, execution.step.Job, time.Now().UTC(), payload)
}

func executeReturnStep(execution stepExecution) (stepOutcome, error) {
	output := execution.output
	if output == nil {
		output = dbal.Row{}
	}
	for name, value := range resolveValues(execution.step.Values, execution.input, execution.results, execution.request) {
		output[name] = value
	}
	return stepOutcome{result: output, output: output, replaceOutput: true}, nil
}
