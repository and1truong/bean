package action

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/actionstep"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/audit"
	"github.com/beanruntime/bean/internal/auth"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/policy"
	"github.com/beanruntime/bean/internal/uid"
	"github.com/beanruntime/bean/internal/valuesource"
)

type Service struct {
	DB                 dbal.Database
	Auth               auth.Service
	CreateID           func(appir.Entity, map[string]any) string
	CreateInvocationID func() string
	Now                func() time.Time
}

func (s Service) now() time.Time {
	if s.Now != nil {
		return s.Now().UTC()
	}
	return time.Now().UTC()
}

func (s Service) createID(entity appir.Entity, input map[string]any) string {
	if s.CreateID != nil {
		return s.CreateID(entity, input)
	}
	return uid.New()
}

func (s Service) createInvocationID() string {
	if s.CreateInvocationID != nil {
		return s.CreateInvocationID()
	}
	return uid.New()
}

func (s Service) Execute(ctx context.Context, app *appir.App, name string, input map[string]any, request beanctx.Request) (dbal.Row, error) {
	a, ok := app.Actions[name]
	if !ok {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "Action not found"}
	}
	e, entityExists := app.Entities[a.Entity]
	if !entityExists && a.Operation != "register_local_user" {
		return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "Action entity is invalid"}
	}
	entityType := e.Name
	if a.Operation == "register_local_user" {
		entityType = "bean_user"
	}
	if a.Operation != "register_local_user" && !authorize(app, a.Policy, true, request, nil) {
		return nil, &dbal.Error{Code: dbal.Conflict, Message: "Action is not permitted"}
	}
	input = copyValues(input)
	if a.Operation == "create" {
		var initialErr error
		input, initialErr = applyLifecycleInitial(app, e, input)
		if initialErr != nil {
			return nil, initialErr
		}
	}
	if err := rejectDerivedInput(a, input); err != nil {
		return nil, err
	}
	if err := validateActionInput(a, input, false); err != nil {
		return nil, err
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
	if a.Operation == "register_local_user" {
		idempotencyKey = ""
	}
	inputHash := ""
	if idempotencyKey != "" {
		idempotencyKey = scopedIdempotencyKey(idempotencyKey, request)
		var fingerprintErr error
		inputHash, fingerprintErr = fingerprint(input)
		if fingerprintErr != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: "Action input cannot be fingerprinted", Cause: fingerprintErr}
		}
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
	createID := ""
	if a.Operation == "create" {
		createID = s.createID(e, input)
	}
	generatedIDs := []string{}
	generatedIDIndex := 0
	generatedInvocationIDs := []string{}
	generatedInvocationIDIndex := 0
	execution := s
	execution.CreateID = func(entity appir.Entity, input map[string]any) string {
		if generatedIDIndex == len(generatedIDs) {
			generatedIDs = append(generatedIDs, s.createID(entity, input))
		}
		id := generatedIDs[generatedIDIndex]
		generatedIDIndex++
		return id
	}
	execution.CreateInvocationID = func() string {
		if generatedInvocationIDIndex == len(generatedInvocationIDs) {
			generatedInvocationIDs = append(generatedInvocationIDs, s.createInvocationID())
		}
		id := generatedInvocationIDs[generatedInvocationIDIndex]
		generatedInvocationIDIndex++
		return id
	}
	baseInput := copyValues(input)
	executionNow := s.now().Format(time.RFC3339Nano)
	err := s.DB.Transaction(ctx, func(tx dbal.Transaction) error {
		generatedIDIndex = 0
		generatedInvocationIDIndex = 0
		input = copyValues(baseInput)
		request.Values = copyValues(request.Values)
		now := executionNow
		request.Values["now"] = now
		result = nil
		entityID = ""
		changed = nil
		if idempotencyKey != "" {
			replay, found, x := lookupReplay(ctx, tx, name, idempotencyKey, inputHash)
			if x != nil {
				return x
			}
			if found {
				result = replay
				return nil
			}
		}
		var current dbal.Row
		needsCurrent := a.Operation == "update" || a.Operation == "delete" || a.Operation == "transition" || a.Operation == "transaction" && actionUsesCurrentRecord(app, a)
		if needsCurrent {
			id := fmt.Sprint(input["id"])
			if id == "" {
				return &dbal.Error{Code: dbal.InvalidQuery, Message: "id is required"}
			}
			rows, x := tx.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
			if x != nil {
				return x
			}
			if len(rows) == 0 {
				return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
			hydrate(rows[0], e)
			current = rows[0]
			if a.Policy != "" && !policy.Can(app.Policies[a.Policy], true, request, recordMap(current)) {
				return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
			}
		}
		var x error
		input, x = applyActionDerivations(app, a, input, current, request)
		if x != nil {
			return x
		}
		if x = validateActionInput(a, input, true); x != nil {
			return x
		}
		guardRecord := current
		if a.Operation == "create" {
			guardRecord = createRuleCandidate(e, input, request, createID, now)
		}
		if x = evaluateActionGuard(app, a, input, guardRecord, request); x != nil {
			return x
		}
		var er error
		switch a.Operation {
		case "create":
			result, er = s.create(ctx, tx, app, e, input, request, createID, now)
		case "update":
			if lifecycle, exists := lifecycleForEntity(app, a.Entity); exists {
				if _, supplied := input[lifecycle.StateField]; supplied {
					return &dbal.Error{Code: dbal.Conflict, Message: "protected state requires a transition Action"}
				}
			}
			for _, candidate := range app.Actions {
				if candidate.Entity != a.Entity || candidate.Operation != "transition" || candidate.Lifecycle != "" {
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
			result, er = update(ctx, tx, app, e, input, a, request, now, false)
		case "transition":
			result, er = update(ctx, tx, app, e, input, a, request, now, true)
		case "delete":
			result, er = remove(ctx, tx, e, input, now)
		case "transaction":
			result, er = execution.steps(ctx, tx, app, a, input, request)
		case "register_local_user":
			result, er = s.Auth.RegisterInTransaction(ctx, tx, fmt.Sprint(input["display_name"]), fmt.Sprint(input["email"]), fmt.Sprint(input["password"]), fmt.Sprint(input["password_confirmation"]), a.DefaultRole)
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
		return audit.Write(ctx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: entityType, EntityID: entityID, Changed: changed, Success: true})
	})
	if err != nil {
		if idempotencyKey != "" && dbal.IsCode(err, dbal.UniqueViolation) {
			if replay, found, replayErr := s.replay(ctx, name, idempotencyKey, inputHash); replayErr != nil {
				return nil, replayErr
			} else if found {
				return replay, nil
			}
		}
		auditCtx, cancelAudit := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelAudit()
		_ = s.DB.Transaction(auditCtx, func(tx dbal.Transaction) error {
			return audit.Write(auditCtx, tx, audit.Entry{RequestID: request.RequestID, UserID: userID(request), TenantID: request.TenantID, Action: name, EntityType: entityType, Success: false, Error: safeError(err)})
		})
		return nil, err
	}
	return result, nil
}
func (s Service) replay(ctx context.Context, actionName, key, inputHash string) (dbal.Row, bool, error) {
	return lookupReplay(ctx, s.DB, actionName, key, inputHash)
}

type rowSelector interface {
	Select(context.Context, dbal.Select) ([]dbal.Row, error)
}

func lookupReplay(ctx context.Context, selector rowSelector, actionName, key, inputHash string) (dbal.Row, bool, error) {
	p := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "action", Value: actionName}, dbal.Predicate{Op: dbal.OpEQ, Column: "key", Value: key})
	rows, e := selector.Select(ctx, dbal.Select{Table: "bean_idempotency", Columns: []string{"input_hash", "result"}, Where: &p, Limit: 1})
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

func scopedIdempotencyKey(key string, request beanctx.Request) string {
	principal := request.TenantID + "\x00" + userID(request)
	scope := sha256.Sum256([]byte(principal))
	return fmt.Sprintf("%x:%s", scope, key)
}
func (s Service) create(ctx context.Context, tx dbal.Transaction, app *appir.App, e appir.Entity, input map[string]any, c beanctx.Request, id, now string) (dbal.Row, error) {
	input, initialErr := applyLifecycleInitial(app, e, input)
	if initialErr != nil {
		return nil, initialErr
	}
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
			if f.Type == "file" {
				storedUpload, uploadErr := storeUpload(ctx, tx, v)
				if uploadErr != nil {
					return nil, uploadErr
				}
				v = storedUpload
			}
			stored, er := field.Encode(f, v)
			if er != nil {
				return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
			}
			values[f.Name] = stored
		}
	}
	if now == "" {
		now = s.now().Format(time.RFC3339Nano)
	}
	if id == "" {
		id = s.createID(e, input)
	}
	values["id"] = id
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
	row := dbal.Row{}
	for name, value := range values {
		row[name] = value
	}
	hydrate(row, e)
	for _, f := range e.Fields {
		if toMany(f) && input[f.Name] != nil {
			row[f.Name] = input[f.Name]
		}
	}
	if er := ValidateEntityRules(app, e, row, c); er != nil {
		return nil, er
	}
	if _, er := tx.Insert(ctx, dbal.Insert{Table: e.Name, Values: values}); er != nil {
		return nil, er
	}
	if er := syncRelations(ctx, tx, app, e, fmt.Sprint(values["id"]), input, c, false); er != nil {
		return nil, er
	}
	return row, nil
}
func update(ctx context.Context, tx dbal.Transaction, app *appir.App, e appir.Entity, input map[string]any, a appir.Action, c beanctx.Request, now string, transition bool) (dbal.Row, error) {
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
	hydrate(row, e)
	values := map[string]dbal.Value{}
	fields := map[string]appir.Field{}
	replacedBlobs := []string{}
	stateField, transitions := transitionContract(app, a)
	stateProtected := len(transitions) > 0
	if lifecycle, exists := lifecycleForEntity(app, e.Name); exists {
		stateField = lifecycle.StateField
		stateProtected = true
		if transition {
			if a.Lifecycle != lifecycle.Name {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: "transition Action is not bound to the Entity Lifecycle"}
			}
			transitions = lifecycle.Transitions
			if a.Transitions != nil {
				transitions = a.Transitions
			}
		}
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
			if !allowedTransition(transitions, from, to) {
				return nil, &dbal.Error{Code: dbal.Conflict, Message: "state transition is not allowed"}
			}
		} else if !transition && k == stateField && stateProtected {
			return nil, &dbal.Error{Code: dbal.Conflict, Message: "protected state requires a transition Action"}
		}
		if er = field.Validate(f, v); er != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: er.Error()}
		}
		if toMany(f) {
			continue
		}
		if f.Type == "richtext" && v != nil {
			v = field.SanitizeRichText(v.(string))
		}
		if f.Type == "file" {
			if row[k] != nil {
				if old := fmt.Sprint(row[k]); old != "" {
					replacedBlobs = append(replacedBlobs, old)
				}
			}
			if v != nil {
				storedUpload, uploadErr := storeUpload(ctx, tx, v)
				if uploadErr != nil {
					return nil, uploadErr
				}
				v = storedUpload
			}
		}
		stored, encodeErr := field.Encode(f, v)
		if encodeErr != nil {
			return nil, &dbal.Error{Code: dbal.InvalidQuery, Message: encodeErr.Error()}
		}
		values[k] = stored
	}
	version := toInt(row["version"])
	values["version"] = version + 1
	values["updated_at"] = now
	candidate := dbal.Row{}
	for name, value := range row {
		candidate[name] = value
	}
	for name, value := range values {
		candidate[name] = value
	}
	hydrate(candidate, e)
	for _, f := range e.Fields {
		if toMany(f) && input[f.Name] != nil {
			candidate[f.Name] = input[f.Name]
		}
	}
	if er = ValidateEntityRules(app, e, candidate, c); er != nil {
		return nil, er
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, dbal.Predicate{Op: dbal.OpEQ, Column: "version", Value: version})
	if _, er = tx.Update(ctx, dbal.Update{Table: e.Name, Values: values, Where: where, ExpectedRows: 1}); er != nil {
		return nil, er
	}
	if er = syncRelations(ctx, tx, app, e, id, input, c, true); er != nil {
		return nil, er
	}
	for _, blobID := range replacedBlobs {
		if _, er = tx.Delete(ctx, dbal.Delete{Table: "bean_blob", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: blobID}, ExpectedRows: 1}); er != nil {
			return nil, er
		}
	}
	for k, v := range values {
		row[k] = v
	}
	hydrate(row, e)
	for _, f := range e.Fields {
		if toMany(f) && input[f.Name] != nil {
			row[f.Name] = input[f.Name]
		}
	}
	return row, nil
}
func remove(ctx context.Context, tx dbal.Transaction, e appir.Entity, input map[string]any, now string) (dbal.Row, error) {
	id := fmt.Sprint(input["id"])
	if e.SoftDelete {
		_, er := tx.Update(ctx, dbal.Update{Table: e.Name, Values: map[string]dbal.Value{"deleted_at": now}, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1})
		return dbal.Row{"id": id}, er
	}
	rows, er := tx.Select(ctx, dbal.Select{Table: e.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, Limit: 1})
	if er != nil {
		return nil, er
	}
	if len(rows) == 0 {
		return nil, &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	if _, er = tx.Delete(ctx, dbal.Delete{Table: e.Name, Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: id}, ExpectedRows: 1}); er != nil {
		return nil, er
	}
	for _, definition := range e.Fields {
		if definition.Type != "file" || rows[0][definition.Name] == nil {
			continue
		}
		if _, er = tx.Delete(ctx, dbal.Delete{Table: "bean_blob", Where: dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0][definition.Name]}, ExpectedRows: 1}); er != nil {
			return nil, er
		}
	}
	return dbal.Row{"id": id}, nil
}

func storeUpload(ctx context.Context, tx dbal.Transaction, value any) (string, error) {
	upload, ok := value.(field.Upload)
	if !ok {
		return "", &dbal.Error{Code: dbal.InvalidQuery, Message: "file must be uploaded as multipart form data"}
	}
	id := uid.New()
	contentType := strings.TrimSpace(upload.ContentType)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	_, err := tx.Insert(ctx, dbal.Insert{Table: "bean_blob", Values: map[string]dbal.Value{
		"id": id, "file_name": upload.Name, "content_type": contentType, "size": len(upload.Data),
		"content": base64.StdEncoding.EncodeToString(upload.Data), "created_at": time.Now().UTC().Format(time.RFC3339Nano),
	}})
	if err != nil {
		return "", err
	}
	return id, nil
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
func decrement(ctx context.Context, tx dbal.Transaction, entity appir.Entity, cfg, input map[string]any, authorized func(dbal.Row) bool, validate func(dbal.Row) error, now string) error {
	fieldName := stringCfg(cfg, "field", "inventory")
	idInput := stringCfg(cfg, "id_input", "id")
	amountInput := stringCfg(cfg, "amount_input", "amount")
	rows, e := tx.Select(ctx, dbal.Select{Table: entity.Name, Where: &dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: input[idInput]}, Limit: 1})
	if e != nil {
		return e
	}
	if len(rows) == 0 {
		return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	hydrate(rows[0], entity)
	if !authorized(rows[0]) {
		return &dbal.Error{Code: dbal.NotFound, Message: "record not found"}
	}
	amount := toInt(input[amountInput])
	current := toInt(rows[0][fieldName])
	if amount < 1 || current < amount {
		return &dbal.Error{Code: dbal.Conflict, Message: stringCfg(cfg, "message", "Insufficient quantity.")}
	}
	version := toInt(rows[0]["version"])
	candidate := dbal.Row{}
	for name, value := range rows[0] {
		candidate[name] = value
	}
	candidate[fieldName] = current - amount
	candidate["version"] = version + 1
	candidate["updated_at"] = now
	if err := validate(candidate); err != nil {
		return err
	}
	where := dbal.And(dbal.Predicate{Op: dbal.OpEQ, Column: "id", Value: rows[0]["id"]}, dbal.Predicate{Op: dbal.OpEQ, Column: "version", Value: version})
	_, e = tx.Update(ctx, dbal.Update{Table: entity.Name, Values: map[string]dbal.Value{fieldName: current - amount, "version": version + 1, "updated_at": now}, Where: where, ExpectedRows: 1})
	return e
}
func resolveValues(values []appir.Assignment, input map[string]any, results map[string]any, c beanctx.Request) (map[string]any, error) {
	out := map[string]any{}
	for _, assignment := range values {
		value, err := resolveValue(assignment.Value, input, results, c)
		if err != nil {
			return nil, fmt.Errorf("resolve Action value %s: %w", assignment.Field, err)
		}
		out[assignment.Field] = value
	}
	return out, nil
}

func hydrate(row dbal.Row, entity appir.Entity) {
	for _, definition := range entity.Fields {
		if value, ok := row[definition.Name]; ok {
			row[definition.Name] = field.Decode(definition, value)
		}
	}
}
func resolveValue(binding appir.ValueBinding, input map[string]any, results map[string]any, c beanctx.Request) (any, error) {
	var literal any
	if valuesource.IsLiteral(binding.Source) {
		decoder := json.NewDecoder(strings.NewReader(string(binding.Literal)))
		decoder.UseNumber()
		if err := decoder.Decode(&literal); err != nil {
			return nil, fmt.Errorf("invalid literal: %w", err)
		}
	}
	return valuesource.Resolve(valuesource.Action, binding.Source, binding.Path, valuesource.Environment{
		Request: c,
		Literal: literal,
		Input:   input,
		Results: results,
	})
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
	name := actionstep.EntityName(a, s)
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
		if f.Type != "relation" || f.Relation == nil || input[f.Name] == nil {
			continue
		}
		values := []any{input[f.Name]}
		if toMany(f) {
			var ok bool
			values, ok = input[f.Name].([]any)
			if !ok {
				return &dbal.Error{Code: dbal.InvalidQuery, Message: f.Name + " must be a list of UUIDs"}
			}
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
			if len(rows) > 0 {
				hydrate(rows[0], target)
			}
			trusted, _ := c.Values[trustedRelationKey(target.Name, value)].(bool)
			if len(rows) == 0 || !trusted && !canReadRecord(app, target, c, rows[0]) {
				return &dbal.Error{Code: dbal.NotFound, Message: "related record not found"}
			}
		}
		if !toMany(f) {
			continue
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

func trustedRelationKey(entity string, id any) string {
	return "trusted_relation:" + entity + ":" + fmt.Sprint(id)
}

func canReadRecord(app *appir.App, target appir.Entity, c beanctx.Request, row dbal.Row) bool {
	return canAccessRecord(app, target, false, c, row)
}

func canAccessRecord(app *appir.App, target appir.Entity, write bool, c beanctx.Request, row dbal.Row) bool {
	if target.Tenant && (c.TenantID == "" || fmt.Sprint(row["tenant_id"]) != c.TenantID) {
		return false
	}
	if target.Owner && target.Policy == "" && (c.User == nil || fmt.Sprint(row["owner_id"]) != c.User.ID) {
		return false
	}
	return target.Policy == "" || authorize(app, target.Policy, write, c, recordMap(row))
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
func lifecycleForEntity(app *appir.App, entity string) (appir.Lifecycle, bool) {
	names := make([]string, 0, len(app.Lifecycles))
	for name := range app.Lifecycles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		lifecycle := app.Lifecycles[name]
		if lifecycle.Entity == entity {
			return lifecycle, true
		}
	}
	return appir.Lifecycle{}, false
}
func applyLifecycleInitial(app *appir.App, entity appir.Entity, input map[string]any) (map[string]any, error) {
	lifecycle, exists := lifecycleForEntity(app, entity.Name)
	if !exists || lifecycle.Initial == "" {
		return input, nil
	}
	if value, supplied := input[lifecycle.StateField]; supplied && value != nil && fmt.Sprint(value) != lifecycle.Initial {
		return nil, &dbal.Error{Code: dbal.Conflict, Message: "record must start in Lifecycle initial state"}
	}
	initialized := make(map[string]any, len(input)+1)
	for name, value := range input {
		initialized[name] = value
	}
	initialized[lifecycle.StateField] = lifecycle.Initial
	return initialized, nil
}
func transitionContract(app *appir.App, action appir.Action) (string, map[string][]string) {
	if lifecycle, exists := app.Lifecycles[action.Lifecycle]; exists {
		if action.Transitions != nil {
			return lifecycle.StateField, action.Transitions
		}
		return lifecycle.StateField, lifecycle.Transitions
	}
	stateField := action.StateField
	if stateField == "" {
		stateField = "status"
	}
	return stateField, action.Transitions
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
