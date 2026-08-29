package job

import (
	"context"
	"encoding/json"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
	"time"
)

func Schedule(ctx context.Context, tx dbal.Transaction, name string, runAt time.Time, payload any) error {
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_job", Values: map[string]dbal.Value{"id": uid.New(), "name": name, "run_at": runAt.UTC().Format(time.RFC3339Nano), "status": "pending", "payload": string(b), "attempts": 0, "retry_delay": 60}})
	return e
}

type Runner struct {
	DB     dbal.Database
	Handle func(context.Context, string, map[string]any) error
}

func (r Runner) RunOnce(ctx context.Context) error {
	p := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "status", Value: "pending"}, dbal.Predicate{Op: dbal.OpLTE, Column: "run_at", Value: time.Now().UTC().Format(time.RFC3339Nano)})
	rows, e := r.DB.Select(ctx, dbal.Select{Table: "bean_job", Where: &p, OrderBy: []dbal.Order{{Column: "run_at"}}, Limit: 20})
	if e != nil {
		return e
	}
	for _, row := range rows {
		var payload map[string]any
		_ = json.Unmarshal([]byte(row["payload"].(string)), &payload)
		e = r.Handle(ctx, row["name"].(string), payload)
		values := map[string]dbal.Value{"attempts": row["attempts"].(int64) + 1}
		if e == nil {
			values["status"] = "complete"
			values["completed_at"] = time.Now().UTC().Format(time.RFC3339Nano)
		} else {
			values["last_error"] = e.Error()
		}
		_, _ = r.DB.Update(ctx, dbal.Update{Table: "bean_job", Values: values, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: row["id"]}, ExpectedRows: 1})
	}
	return nil
}
