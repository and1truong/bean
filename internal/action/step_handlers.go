package action

import (
	"context"
	"fmt"
	"time"

	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/expr"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/registry"
	"github.com/beanruntime/bean/internal/view"
)

type stepExecution struct {
	service       Service
	ctx           context.Context
	tx            dbal.Transaction
	app           *appir.App
	action        appir.Action
	step          appir.Step
	specification actionstep.Specification
	input         map[string]any
	request       beanctx.Request
	results       map[string]any
	bindings      map[string]any
	output        dbal.Row
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
		specification, declared := actionstep.Lookup(step.Op)
		if !exists || !declared {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "unsupported Action step " + step.Op}
		}
		outcome, err := handler(stepExecution{
			service: s, ctx: ctx, tx: tx, app: app, action: actionDefinition, step: step, specification: specification,
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

func authorizeStepEntity(execution stepExecution, entity appir.Entity, row dbal.Row) bool {
	specification := execution.specification
	if !specification.Effects.ReadsEntity && !specification.Effects.MutatesEntity {
		return false
	}
	write := specification.Effects.MutatesEntity
	if row == nil {
		return entity.Policy == "" || authorize(execution.app, entity.Policy, write, execution.request, nil)
	}
	return canAccessRecord(execution.app, entity, write, execution.request, row)
}

func executeLoadStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	id := fmt.Sprint(values["id"])
	rows, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(rows) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(rows[0], entity)
	if !authorizeStepEntity(execution, entity, rows[0]) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	return stepOutcome{result: rows[0], output: rows[0], replaceOutput: true}, nil
}

func executeQueryStep(execution stepExecution) (stepOutcome, error) {
	if !execution.specification.Effects.ReadsEntity {
		return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "query step has no declared read effect"}
	}
	viewDefinition, exists := execution.app.Views[execution.step.View]
	if !exists {
		return stepOutcome{}, &dbal.Error{Code: dbal.InvalidQuery, Message: "query step references an invalid View"}
	}
	limit := viewDefinition.MaxLimit
	if limit <= 0 || limit > 200 {
		limit = 200
	}
	result, err := view.ReadPage(execution.ctx, execution.tx, execution.app, execution.step.View, view.ReadOptions{
		Params:           view.Params{Limit: limit},
		ExpressionValues: execution.bindings,
		ExtraPredicate:   execution.step.Where,
	}, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	entity := execution.app.Entities[viewDefinition.Entity]
	for _, row := range result.Rows {
		if execution.request.Values != nil && row["id"] != nil {
			execution.request.Values[trustedRelationKey(entity.Name, row["id"])] = true
		}
	}
	return stepOutcome{result: result.Rows}, nil
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
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	if !authorizeStepEntity(execution, entity, nil) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "records not found"}
	}
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	err = noOverlap(execution.ctx, execution.tx, entity.Name, values, execution.input)
	return stepOutcome{}, err
}

func executeDecrementStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	err = decrement(execution.ctx, execution.tx, entity, values, execution.input, func(row dbal.Row) bool {
		return authorizeStepEntity(execution, entity, row)
	}, func(row dbal.Row) error {
		return ValidateEntityRules(execution.app, entity, row, execution.request)
	}, fmt.Sprint(execution.request.Values["now"]))
	return stepOutcome{}, err
}

func executeCreateStep(execution stepExecution) (stepOutcome, error) {
	entity, err := stepEntity(execution.app, execution.action, execution.step)
	if err != nil {
		return stepOutcome{}, err
	}
	if !authorizeStepEntity(execution, entity, nil) {
		return stepOutcome{}, &dbal.Error{Code: dbal.Conflict, Message: "Action is not permitted"}
	}
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	delete(values, "entity")
	row, err := execution.service.create(execution.ctx, execution.tx, execution.app, entity, values, execution.request, "", fmt.Sprint(execution.request.Values["now"]))
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
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	id := fmt.Sprint(values["id"])
	loaded, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(loaded) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(loaded[0], entity)
	if !authorizeStepEntity(execution, entity, loaded[0]) {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	specification := execution.specification
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
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	id := fmt.Sprint(values["id"])
	loaded, err := execution.tx.Select(execution.ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if err != nil {
		return stepOutcome{}, err
	}
	if len(loaded) == 0 {
		return stepOutcome{}, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(loaded[0], entity)
	if !authorizeStepEntity(execution, entity, loaded[0]) {
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
		resolved, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
		if err != nil {
			return stepOutcome{}, err
		}
		payload = resolved
	}
	return stepOutcome{}, event.Emit(execution.ctx, execution.tx, execution.step.Event, payload)
}

func executeScheduleStep(execution stepExecution) (stepOutcome, error) {
	payload := execution.bindings
	if len(execution.step.Values) > 0 {
		resolved, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
		if err != nil {
			return stepOutcome{}, err
		}
		payload = resolved
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
	values, err := resolveValues(execution.step.Values, execution.input, execution.results, execution.request)
	if err != nil {
		return stepOutcome{}, err
	}
	for name, value := range values {
		output[name] = value
	}
	return stepOutcome{result: output, output: output, replaceOutput: true}, nil
}
