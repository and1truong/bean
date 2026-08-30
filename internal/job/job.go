package job

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/fault"
	"github.com/beanruntime/bean/internal/uid"
)

const TenantIDPayloadKey = "_beanTenantId"

func Schedule(ctx context.Context, tx dbal.Transaction, name string, runAt time.Time, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Insert(ctx, dbal.Insert{Table: "bean_job", Values: map[string]dbal.Value{"id": uid.New(), "name": name, "run_at": runAt.UTC().Format(time.RFC3339Nano), "status": "pending", "payload": string(b), "attempts": 0, "retry_delay": 60, "max_attempts": 5}})
	return err
}

type Runner struct {
	DB        dbal.Database
	Handle    func(context.Context, string, map[string]any) error
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
		dbal.Predicate{Op: dbal.OpLTE, Column: "run_at", Value: timestamp(now)},
		dbal.Or(
			dbal.Predicate{Op: dbal.OpIsNull, Column: "next_attempt_at"},
			dbal.Predicate{Op: dbal.OpLTE, Column: "next_attempt_at", Value: timestamp(now)},
		),
	)
	limit := r.BatchSize
	if limit <= 0 {
		limit = 20
	}
	rows, err := r.DB.Select(ctx, dbal.Select{Table: "bean_job", Where: &due, OrderBy: []dbal.Order{{Column: "run_at"}}, Limit: limit})
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
	result, err := r.DB.Update(ctx, dbal.Update{Table: "bean_job", Values: map[string]dbal.Value{"status": "running", "claim_token": token, "claimed_at": timestamp(now)}, Where: claim})
	if err != nil {
		return err
	}
	if result.Affected == 0 {
		return nil
	}
	fault.Point("job.after_claim")
	payload := map[string]any{}
	if err = json.Unmarshal([]byte(fmt.Sprint(row["payload"])), &payload); err == nil {
		err = r.Handle(ctx, fmt.Sprint(row["name"]), payload)
	}
	fault.Point("job.after_handle")
	attempts := integer(row["attempts"]) + 1
	values := map[string]dbal.Value{"attempts": attempts, "claim_token": nil, "claimed_at": nil}
	if err == nil {
		values["status"] = "complete"
		values["completed_at"] = timestamp(r.now())
		values["last_error"] = nil
	} else if attempts >= integer(row["max_attempts"]) {
		values["status"] = "failed"
		values["last_error"] = err.Error()
	} else {
		values["status"] = "pending"
		values["last_error"] = err.Error()
		values["next_attempt_at"] = timestamp(r.now().Add(time.Duration(integer(row["retry_delay"])) * time.Second))
	}
	owned := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: row["id"]},
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "running"},
		dbal.Predicate{Op: dbal.OpEQ, Column: "claim_token", Value: token},
	)
	_, updateErr := r.DB.Update(ctx, dbal.Update{Table: "bean_job", Values: values, Where: owned, ExpectedRows: 1})
	return updateErr
}

func (r Runner) recoverStale(ctx context.Context, now time.Time) error {
	stale := dbal.And(
		dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "running"},
		dbal.Predicate{Op: dbal.OpLTE, Column: "claimed_at", Value: timestamp(now.Add(-r.lease()))},
	)
	_, err := r.DB.Update(ctx, dbal.Update{Table: "bean_job", Values: map[string]dbal.Value{"status": "pending", "claim_token": nil, "claimed_at": nil, "next_attempt_at": timestamp(now)}, Where: stale})
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

func timestamp(value time.Time) string { return value.UTC().Format(time.RFC3339Nano) }

func integer(value any) int64 {
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
