package event

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/fault"
	"github.com/beanruntime/bean/internal/uid"
)

func Emit(ctx context.Context, tx dbal.Transaction, topic string, payload any) error {
	_, err := Enqueue(ctx, tx, topic, payload, Options{})
	return err
}

type Options struct {
	ID          string
	RetryDelay  time.Duration
	MaxAttempts int
	CreatedAt   time.Time
}

func Enqueue(ctx context.Context, tx dbal.Transaction, topic string, payload any, options Options) (string, error) {
	b, e := json.Marshal(payload)
	if e != nil {
		return "", e
	}
	id := options.ID
	if id == "" {
		id = uid.New()
	}
	createdAt := options.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	retryDelay := int(options.RetryDelay / time.Second)
	if retryDelay <= 0 {
		retryDelay = 60
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 10
	}
	now := createdAt.Format(time.RFC3339Nano)
	_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_outbox", Values: map[string]dbal.Value{"id": id, "topic": topic, "payload": string(b), "created_at": now, "status": "pending", "attempts": 0, "retry_delay": retryDelay, "max_attempts": maxAttempts, "next_attempt_at": now}})
	return id, e
}

type Runner struct {
	DB        dbal.Database
	Deliver   func(context.Context, string, map[string]any) error
	Now       func() time.Time
	Lease     time.Duration
	BatchSize int
}

func (r Runner) RunOnce(ctx context.Context) error {
	now := r.now()
	if err := r.recoverStale(ctx, now); err != nil {
		return err
	}
	due := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "pending"},
		dbal.Predicate{Op: dbal.OpLTE, Column: "next_attempt_at", Value: stamp(now)},
	)
	limit := r.BatchSize
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.DB.Select(ctx, dbal.Select{Table: "bean_outbox", Where: &due, OrderBy: []dbal.Order{{Column: "created_at"}, {Column: "id"}}, Limit: limit})
	if err != nil {
		return err
	}
	for _, row := range rows {
		if err = r.run(ctx, row, now); err != nil {
			return err
		}
	}
	return nil
}

func (r Runner) run(ctx context.Context, row dbal.Row, now time.Time) error {
	token := uid.New()
	claim := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: row["id"]},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "pending"},
	)
	result, err := r.DB.Update(ctx, dbal.Update{Table: "bean_outbox", Values: map[string]dbal.Value{"status": "delivering", "claim_token": token, "claimed_at": stamp(now)}, Where: claim})
	if err != nil {
		return err
	}
	if result.Affected == 0 {
		return nil
	}
	fault.Point("outbox.after_claim")
	payload := map[string]any{}
	decoder := json.NewDecoder(strings.NewReader(fmt.Sprint(row["payload"])))
	decoder.UseNumber()
	if err = decoder.Decode(&payload); err == nil {
		err = r.Deliver(ctx, fmt.Sprint(row["topic"]), payload)
	}
	fault.Point("outbox.after_delivery")
	attempts := number(row["attempts"]) + 1
	values := map[string]dbal.Value{"attempts": attempts, "claim_token": nil, "claimed_at": nil}
	if err == nil {
		values["status"] = "delivered"
		values["delivered_at"] = stamp(r.now())
		values["last_error"] = nil
	} else if !retryable(err) || attempts >= number(row["max_attempts"]) {
		values["status"] = "failed"
		values["last_error"] = err.Error()
	} else {
		values["status"] = "pending"
		values["last_error"] = err.Error()
		values["next_attempt_at"] = stamp(r.now().Add(time.Duration(number(row["retry_delay"])) * time.Second))
	}
	owned := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: row["id"]},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "delivering"},
		dbal.Predicate{Op: dbal.OpEQ, Column: "claim_token", Value: token},
	)
	_, updateErr := r.DB.Update(ctx, dbal.Update{Table: "bean_outbox", Values: values, Where: owned, ExpectedRows: 1})
	return updateErr
}

type retryableFailure interface {
	Retryable() bool
}

func retryable(err error) bool {
	var classified retryableFailure
	return !errors.As(err, &classified) || classified.Retryable()
}

func (r Runner) recoverStale(ctx context.Context, now time.Time) error {
	stale := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "delivering"},
		dbal.Predicate{Op: dbal.OpLTE, Column: "claimed_at", Value: stamp(now.Add(-r.lease()))},
	)
	_, err := r.DB.Update(ctx, dbal.Update{Table: "bean_outbox", Values: map[string]dbal.Value{"status": "pending", "claim_token": nil, "claimed_at": nil, "next_attempt_at": stamp(now)}, Where: stale})
	return err
}

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r Runner) lease() time.Duration {
	if r.Lease > 0 {
		return r.Lease
	}
	return 5 * time.Minute
}

func stamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func number(value any) int64 {
	switch n := value.(type) {
	case int:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	default:
		return 0
	}
}
