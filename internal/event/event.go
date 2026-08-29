package event

import (
	"context"
	"encoding/json"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
	"time"
)

func Emit(ctx context.Context, tx dbal.Transaction, topic string, payload any) error {
	b, e := json.Marshal(payload)
	if e != nil {
		return e
	}
	_, e = tx.Insert(ctx, dbal.Insert{Table: "bean_outbox", Values: map[string]dbal.Value{"id": uid.New(), "topic": topic, "payload": string(b), "created_at": time.Now().UTC().Format(time.RFC3339Nano)}})
	return e
}
