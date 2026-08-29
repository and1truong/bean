package audit

import (
	"context"
	"encoding/json"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/uid"
	"time"
)

type Entry struct {
	RequestID, UserID, TenantID, Action, EntityType, EntityID string
	Changed                                                   []string
	Success                                                   bool
	Error                                                     string
}

func Write(ctx context.Context, tx dbal.Transaction, e Entry) error {
	changed, _ := json.Marshal(e.Changed)
	success := 0
	if e.Success {
		success = 1
	}
	_, err := tx.Insert(ctx, dbal.Insert{Table: "bean_audit", Values: map[string]dbal.Value{"id": uid.New(), "at": time.Now().UTC().Format(time.RFC3339Nano), "request_id": e.RequestID, "user_id": e.UserID, "tenant_id": e.TenantID, "action": e.Action, "entity_type": e.EntityType, "entity_id": e.EntityID, "changed_fields": string(changed), "success": success, "error": e.Error}})
	return err
}
