package demoseed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	fieldpkg "github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/view"
)

type Record struct {
	Entity string         `json:"entity"`
	ID     string         `json:"id"`
	Values map[string]any `json:"values"`
}

type lifecycleMove struct {
	Action     string
	State      string
	IDInput    string
	StateInput string
}

type Result struct {
	Name     string `json:"name"`
	Seed     int64  `json:"seed"`
	Records  int    `json:"records"`
	Checksum string `json:"checksum"`
}

func Generate(app *appir.App, seed int64) ([]Record, error) {
	if app.DemoSeed == nil {
		return nil, fmt.Errorf("application has no DemoSeed definition")
	}
	order, err := entityOrder(app)
	if err != nil {
		return nil, err
	}
	referenced := referencedTargetFields(app)
	generated := map[string][]Record{}
	records := []Record{}
	for _, entityName := range order {
		entity := app.Entities[entityName]
		definition := app.DemoSeed.Entities[entityName]
		for index := 0; index < definition.Count; index++ {
			id := stableUUID(seed, entityName, index)
			values := map[string]any{}
			for _, field := range entity.Fields {
				if field.Sensitive || field.Type == "password" || field.Type == "file" {
					continue
				}
				if field.Relation != nil {
					value := relationValue(field, index, generated)
					if value != nil {
						values[field.Name] = value
					}
					continue
				}
				if field.Required || field.Unique || referenced[entityName][field.Name] || includeOptional(index, field.Name) {
					values[field.Name] = scalarValue(field, definition.Profile, index, seed)
				}
			}
			record := Record{Entity: entityName, ID: id, Values: values}
			records = append(records, record)
			generated[entityName] = append(generated[entityName], record)
		}
	}
	if err := validateGeneratedUniqueness(app, records, seed); err != nil {
		return nil, err
	}
	return records, nil
}

func Run(ctx context.Context, database dbal.Database, app *appir.App, seed int64) (Result, error) {
	records, err := Generate(app, seed)
	if err != nil {
		return Result{}, err
	}
	result, err := resultFor(app.DemoSeed.Name, seed, records)
	if err != nil {
		return Result{}, err
	}
	roles := []string{"administrator"}
	for name := range app.Roles {
		if name != "administrator" {
			roles = append(roles, name)
		}
	}
	sort.Strings(roles)
	request := beanctx.Request{User: &beanctx.User{ID: stableUUID(seed, "owner", 0), Email: "demo@bean.local", Roles: roles}, TenantID: stableUUID(seed, "tenant", 0), RequestID: fmt.Sprintf("demo-seed:%d", seed)}
	empty, exact, err := inspectTarget(ctx, database, app, records, request, seed)
	if err != nil {
		return Result{}, err
	}
	if exact {
		return result, nil
	}
	if !empty {
		return Result{}, fmt.Errorf("refusing to seed a non-empty target that does not match the generated dataset")
	}
	lifecyclePlans, err := planLifecycleMoves(app, records)
	if err != nil {
		return Result{}, err
	}
	if err = validateLifecycleTransitionUniqueness(app, records, lifecyclePlans, seed); err != nil {
		return Result{}, err
	}
	ids := map[string][]string{}
	for _, record := range records {
		ids[record.Entity] = append(ids[record.Entity], record.ID)
	}
	nextID := map[string]int{}
	clock := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(seed%8760) * time.Hour)
	service := action.Service{
		DB:  database,
		Now: func() time.Time { return clock },
		CreateID: func(entity appir.Entity, input map[string]any) string {
			index := nextID[entity.Name]
			nextID[entity.Name] = index + 1
			if index < len(ids[entity.Name]) {
				return ids[entity.Name][index]
			}
			return stableUUID(seed, "extra:"+entity.Name, index-len(ids[entity.Name]))
		},
	}
	for index, record := range records {
		if err = seedRecord(ctx, service, app, record, lifecyclePlans[index], request); err != nil {
			return Result{}, fmt.Errorf("seed %s: %w", record.Entity, err)
		}
	}
	_, exact, err = inspectTarget(ctx, database, app, records, request, seed)
	if err != nil {
		return Result{}, err
	}
	if !exact {
		return Result{}, fmt.Errorf("seed Actions did not produce the generated dataset")
	}
	return result, nil
}

func planLifecycleMoves(app *appir.App, records []Record) ([][]lifecycleMove, error) {
	plans := make([][]lifecycleMove, len(records))
	for index, record := range records {
		lifecycle, exists := demoLifecycleForEntity(app, record.Entity)
		if !exists {
			continue
		}
		desiredState := lifecycleDesiredState(lifecycle, record.Values)
		if desiredState == lifecycle.Initial {
			continue
		}
		path, executable := lifecycleExecutablePath(app, lifecycle, record.ID, desiredState)
		if !executable {
			return nil, fmt.Errorf("Lifecycle %s has no executable Action path from %s to generated state %s", lifecycle.Name, lifecycle.Initial, desiredState)
		}
		plans[index] = path
	}
	return plans, nil
}

func seedRecord(ctx context.Context, service action.Service, app *appir.App, record Record, moves []lifecycleMove, request beanctx.Request) error {
	values := make(map[string]any, len(record.Values))
	for name, value := range record.Values {
		values[name] = value
	}
	lifecycle, hasLifecycle := demoLifecycleForEntity(app, record.Entity)
	if hasLifecycle {
		values[lifecycle.StateField] = lifecycle.Initial
	}
	created, err := service.Execute(ctx, app, record.Entity+"_create", values, request)
	if err != nil || !hasLifecycle || len(moves) == 0 {
		return err
	}
	for _, move := range moves {
		input := map[string]any{"id": created["id"], lifecycle.StateField: move.State}
		if move.IDInput != "" {
			input = map[string]any{move.IDInput: created["id"]}
			if move.StateInput != "" {
				input[move.StateInput] = move.State
			}
		}
		if _, err = service.Execute(ctx, app, move.Action, input, request); err != nil {
			return err
		}
	}
	return nil
}

func lifecycleDesiredState(lifecycle appir.Lifecycle, values map[string]any) string {
	if desired := values[lifecycle.StateField]; desired != nil {
		return fmt.Sprint(desired)
	}
	return lifecycle.Initial
}

func demoLifecycleForEntity(app *appir.App, entity string) (appir.Lifecycle, bool) {
	names := make([]string, 0, len(app.Lifecycles))
	for name := range app.Lifecycles {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if app.Lifecycles[name].Entity == entity {
			return app.Lifecycles[name], true
		}
	}
	return appir.Lifecycle{}, false
}

func lifecycleExecutablePath(app *appir.App, lifecycle appir.Lifecycle, recordID, target string) ([]lifecycleMove, bool) {
	if target == lifecycle.Initial {
		return []lifecycleMove{}, true
	}
	type predecessor struct {
		state string
		move  lifecycleMove
	}
	queue := []string{lifecycle.Initial}
	previous := map[string]predecessor{lifecycle.Initial: {}}
	for len(queue) > 0 {
		from := queue[0]
		queue = queue[1:]
		next := append([]string{}, lifecycle.Transitions[from]...)
		sort.Strings(next)
		for _, state := range next {
			if _, visited := previous[state]; visited {
				continue
			}
			move, executable := lifecycleTransitionMove(app, lifecycle, recordID, from, state)
			if !executable {
				continue
			}
			previous[state] = predecessor{state: from, move: move}
			if state == target {
				path := []lifecycleMove{}
				for current := state; current != lifecycle.Initial; {
					edge := previous[current]
					path = append(path, edge.move)
					current = edge.state
				}
				for left, right := 0, len(path)-1; left < right; left, right = left+1, right-1 {
					path[left], path[right] = path[right], path[left]
				}
				return path, true
			}
			queue = append(queue, state)
		}
	}
	return nil, false
}

func lifecycleTransitionMove(app *appir.App, lifecycle appir.Lifecycle, recordID, from, target string) (lifecycleMove, bool) {
	names := make([]string, 0, len(app.Actions))
	for name := range app.Actions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		actionDefinition := app.Actions[name]
		if actionDefinition.Entity != lifecycle.Entity || actionDefinition.Lifecycle != lifecycle.Name || actionDefinition.Operation != "transition" && actionDefinition.Operation != "transaction" {
			continue
		}
		transitions := lifecycle.Transitions
		if actionDefinition.Transitions != nil {
			transitions = actionDefinition.Transitions
		}
		allowed := false
		for _, candidate := range transitions[from] {
			allowed = allowed || candidate == target
		}
		if !allowed {
			continue
		}
		if actionDefinition.Operation == "transaction" {
			idInput, stateInput, compatible := transactionLifecycleInputs(app, actionDefinition, lifecycle, recordID, target)
			if compatible {
				return lifecycleMove{Action: name, State: target, IDInput: idInput, StateInput: stateInput}, true
			}
			continue
		}
		compatible := true
		for inputName, input := range actionDefinition.Input {
			if input.Required && inputName != "id" && inputName != lifecycle.StateField {
				compatible = false
			}
		}
		if compatible {
			return lifecycleMove{Action: name, State: target}, true
		}
	}
	return lifecycleMove{}, false
}

func transactionLifecycleInputs(app *appir.App, actionDefinition appir.Action, lifecycle appir.Lifecycle, recordID, target string) (string, string, bool) {
	if len(actionDefinition.Steps) != 1 {
		return "", "", false
	}
	var transition *appir.Step
	for index := range actionDefinition.Steps {
		step := &actionDefinition.Steps[index]
		entity := transactionStepEntity(actionDefinition, *step)
		if step.Op != "transition" || entity != lifecycle.Entity {
			continue
		}
		if transition != nil {
			return "", "", false
		}
		transition = step
	}
	if transition == nil {
		return "", "", false
	}
	entityField := false
	for _, field := range app.Entities[lifecycle.Entity].Fields {
		entityField = entityField || field.Name == "entity"
	}
	for _, assignment := range transition.Values {
		if assignment.Field == "id" {
			continue
		}
		if assignment.Field == lifecycle.StateField {
			if assignment.Field == "entity" && assignment.Value.Source == "literal" {
				return "", "", false
			}
			continue
		}
		if assignment.Field == "entity" && assignment.Value.Source == "literal" && !entityField {
			continue
		}
		return "", "", false
	}
	bindings := map[string]appir.ValueBinding{}
	for _, assignment := range transition.Values {
		bindings[assignment.Field] = assignment.Value
	}
	id := bindings["id"]
	if id.Source != "input" || id.Path == "" {
		return "", "", false
	}
	state := bindings[lifecycle.StateField]
	stateInput := ""
	switch state.Source {
	case "input":
		stateInput = state.Path
		if stateInput == "" || stateInput == id.Path {
			return "", "", false
		}
	case "literal":
		var literal string
		if json.Unmarshal(state.Literal, &literal) != nil || literal != target {
			return "", "", false
		}
	default:
		return "", "", false
	}
	for inputName, input := range actionDefinition.Input {
		if input.Required && inputName != id.Path && inputName != stateInput {
			return "", "", false
		}
	}
	idDefinition, exists := actionDefinition.Input[id.Path]
	if !exists || fieldpkg.Validate(idDefinition, recordID) != nil {
		return "", "", false
	}
	if stateInput != "" {
		stateDefinition, exists := actionDefinition.Input[stateInput]
		if !exists || fieldpkg.Validate(stateDefinition, target) != nil {
			return "", "", false
		}
	}
	return id.Path, stateInput, true
}

func transactionStepEntity(actionDefinition appir.Action, step appir.Step) string {
	entity := step.Entity
	if entity == "" {
		entity = actionDefinition.Entity
	}
	if entity == actionDefinition.Entity {
		for _, assignment := range step.Values {
			if assignment.Field == "entity" && assignment.Value.Source == "literal" {
				_ = json.Unmarshal(assignment.Value.Literal, &entity)
			}
		}
	}
	return entity
}

func inspectTarget(ctx context.Context, database dbal.Database, app *appir.App, records []Record, request beanctx.Request, seed int64) (bool, bool, error) {
	verification, err := verificationApp(app)
	if err != nil {
		return false, false, fmt.Errorf("prepare seed verification Views: %w", err)
	}
	expected := map[string]map[string]Record{}
	for _, record := range records {
		if expected[record.Entity] == nil {
			expected[record.Entity] = map[string]Record{}
		}
		expected[record.Entity][record.ID] = record
	}
	empty := true
	exact := true
	views := view.Service{DB: database}
	entityNames := make([]string, 0, len(app.DemoSeed.Entities))
	for entityName := range app.DemoSeed.Entities {
		entityNames = append(entityNames, entityName)
	}
	sort.Strings(entityNames)
	for _, entityName := range entityNames {
		rows := []dbal.Row{}
		cursor := ""
		for {
			page, err := views.RunPage(ctx, verification, entityName+"_list", view.Params{Limit: 200, Cursor: cursor}, request)
			if err != nil {
				return false, false, fmt.Errorf("verify seeded View %s_list: %w", entityName, err)
			}
			rows = append(rows, page.Rows...)
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		if len(rows) > 0 {
			empty = false
		}
		if len(rows) != len(expected[entityName]) {
			exact = false
			continue
		}
		entity := app.Entities[entityName]
		for _, row := range rows {
			record, exists := expected[entityName][fmt.Sprint(row["id"])]
			if !exists {
				exact = false
				break
			}
			for _, fieldName := range verificationFields(entity)[1:] {
				value, generated := generatedConstraintValue(app, entityName, record, fieldName, seed)
				if !generated && omittedValueMatches(entity, fieldName, row[fieldName]) {
					continue
				}
				if canonical(row[fieldName]) != canonical(value) {
					exact = false
					break
				}
			}
		}
	}
	return empty, exact && !empty, nil
}

func verificationApp(app *appir.App) (*appir.App, error) {
	verification, err := app.Clone()
	if err != nil {
		return nil, err
	}
	const policyName = "__bean_demo_seed_verification"
	verification.Policies[policyName] = appir.Policy{Name: policyName}
	for entityName := range app.DemoSeed.Entities {
		entity := verification.Entities[entityName]
		fields := verificationFields(entity)
		if entity.SoftDelete {
			entity.SoftDelete = false
			verification.Entities[entityName] = entity
		}
		viewName := entityName + "_list"
		viewDefinition := verification.Views[viewName]
		viewDefinition.Name = viewName
		viewDefinition.Entity = entityName
		viewDefinition.Fields = fields
		viewDefinition.Relationships = nil
		viewDefinition.Filter = nil
		viewDefinition.ContextFilter = nil
		viewDefinition.FieldFilters = nil
		viewDefinition.Sort = nil
		viewDefinition.GroupBy = nil
		viewDefinition.Aggregates = nil
		viewDefinition.Policy = policyName
		verification.Views[viewName] = viewDefinition
	}
	return verification, nil
}

func verificationFields(entity appir.Entity) []string {
	fields := []string{"id"}
	for _, field := range entity.Fields {
		fields = append(fields, field.Name)
	}
	if entity.Owner {
		fields = append(fields, "owner_id")
	}
	if entity.Tenant {
		fields = append(fields, "tenant_id")
	}
	if entity.SoftDelete {
		fields = append(fields, "deleted_at")
	}
	return append(fields, "created_at", "updated_at", "version")
}

func validateGeneratedUniqueness(app *appir.App, records []Record, seed int64) error {
	return validateRecordUniqueness(app, records, seed)
}

func validateLifecycleTransitionUniqueness(app *appir.App, records []Record, plans [][]lifecycleMove, seed int64) error {
	type plannedRecord struct {
		record Record
		moves  []lifecycleMove
	}
	byEntity := map[string][]plannedRecord{}
	for index, record := range records {
		byEntity[record.Entity] = append(byEntity[record.Entity], plannedRecord{record: record, moves: plans[index]})
	}
	entityNames := make([]string, 0, len(byEntity))
	for entityName := range byEntity {
		entityNames = append(entityNames, entityName)
	}
	sort.Strings(entityNames)
	for _, entityName := range entityNames {
		lifecycle, exists := demoLifecycleForEntity(app, entityName)
		if !exists {
			continue
		}
		created := []Record{}
		for _, planned := range byEntity[entityName] {
			transient := planned.record
			transient.Values = make(map[string]any, len(planned.record.Values))
			for name, value := range planned.record.Values {
				transient.Values[name] = value
			}
			transient.Values[lifecycle.StateField] = lifecycle.Initial
			candidate := append(append([]Record{}, created...), transient)
			if err := validateRecordUniqueness(app, candidate, seed); err != nil {
				return fmt.Errorf("Lifecycle %s intermediate initial state: %w", lifecycle.Name, err)
			}
			for _, move := range planned.moves {
				transient.Values[lifecycle.StateField] = move.State
				candidate[len(candidate)-1] = transient
				if err := validateRecordUniqueness(app, candidate, seed); err != nil {
					return fmt.Errorf("Lifecycle %s intermediate transition state %s: %w", lifecycle.Name, move.State, err)
				}
			}
			created = append(created, planned.record)
		}
	}
	return nil
}

func validateRecordUniqueness(app *appir.App, records []Record, seed int64) error {
	byEntity := map[string][]Record{}
	for _, record := range records {
		byEntity[record.Entity] = append(byEntity[record.Entity], record)
	}
	entityNames := make([]string, 0, len(byEntity))
	for entityName := range byEntity {
		entityNames = append(entityNames, entityName)
	}
	sort.Strings(entityNames)
	for _, entityName := range entityNames {
		entityRecords := byEntity[entityName]
		entity := app.Entities[entityName]
		constraints := append([][]string{}, entity.Unique...)
		for _, field := range entity.Fields {
			toMany := field.Relation != nil && (field.Relation.Kind == "one-to-many" || field.Relation.Kind == "many-to-many")
			if field.Unique && !toMany || field.Relation != nil && field.Relation.Kind == "one-to-one" {
				constraints = append(constraints, []string{field.Name})
			}
		}
		for _, fields := range constraints {
			if len(fields) == 0 {
				continue
			}
			seen := map[string]bool{}
			for _, record := range entityRecords {
				values := make([]any, 0, len(fields))
				complete := true
				for _, fieldName := range fields {
					value, exists := generatedConstraintValue(app, entityName, record, fieldName, seed)
					if !exists || value == nil {
						complete = false
						break
					}
					values = append(values, value)
				}
				if !complete {
					continue
				}
				key := canonical(values)
				if seen[key] {
					return fmt.Errorf("generated values violate unique constraint %s(%s)", entityName, strings.Join(fields, ","))
				}
				seen[key] = true
			}
		}
	}
	return nil
}

func generatedConstraintValue(app *appir.App, entityName string, record Record, fieldName string, seed int64) (any, bool) {
	entity := app.Entities[entityName]
	switch fieldName {
	case "id":
		return record.ID, true
	case "created_at", "updated_at":
		value := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(time.Duration(seed%8760) * time.Hour).Format(time.RFC3339Nano)
		return value, true
	case "version":
		version := 1
		if lifecycle, exists := demoLifecycleForEntity(app, entityName); exists {
			if path, executable := lifecycleExecutablePath(app, lifecycle, record.ID, lifecycleDesiredState(lifecycle, record.Values)); executable {
				version += len(path)
			}
		}
		return version, true
	case "owner_id":
		if entity.Owner {
			return stableUUID(seed, "owner", 0), true
		}
	case "tenant_id":
		if entity.Tenant {
			return stableUUID(seed, "tenant", 0), true
		}
	case "deleted_at":
		if entity.SoftDelete {
			return nil, true
		}
	}
	value, exists := record.Values[fieldName]
	return value, exists
}

func omittedValueMatches(entity appir.Entity, fieldName string, value any) bool {
	if value == nil {
		return true
	}
	for _, field := range entity.Fields {
		if field.Name != fieldName || field.Relation == nil || field.Relation.Kind != "one-to-many" && field.Relation.Kind != "many-to-many" {
			continue
		}
		values, ok := value.([]any)
		return ok && len(values) == 0
	}
	return false
}

func resultFor(name string, seed int64, records []Record) (Result, error) {
	encoded, err := json.Marshal(records)
	if err != nil {
		return Result{}, err
	}
	sum := sha256.Sum256(encoded)
	return Result{Name: name, Seed: seed, Records: len(records), Checksum: hex.EncodeToString(sum[:])}, nil
}

func entityOrder(app *appir.App) ([]string, error) {
	remaining := map[string]bool{}
	for entity := range app.DemoSeed.Entities {
		remaining[entity] = true
	}
	order := []string{}
	for len(remaining) > 0 {
		ready := []string{}
		fallback := []string{}
		for name := range remaining {
			blocked, requiredBlocked := false, false
			for _, field := range app.Entities[name].Fields {
				if field.Relation != nil && field.Relation.Entity != name && remaining[field.Relation.Entity] {
					blocked = true
					requiredBlocked = requiredBlocked || field.Required
				}
				if field.Required && field.Relation != nil && field.Relation.Entity == name {
					return nil, fmt.Errorf("required self relation %s.%s cannot be demo seeded", name, field.Name)
				}
			}
			if !blocked {
				ready = append(ready, name)
			}
			if !requiredBlocked {
				fallback = append(fallback, name)
			}
		}
		if len(ready) == 0 {
			sort.Strings(fallback)
			if len(fallback) == 0 {
				return nil, fmt.Errorf("demo seed required relations contain a cycle")
			}
			ready = fallback[:1]
		}
		sort.Strings(ready)
		for _, name := range ready {
			order = append(order, name)
			delete(remaining, name)
		}
	}
	return order, nil
}

func relationValue(field appir.Field, index int, records map[string][]Record) any {
	targets := records[field.Relation.Entity]
	if len(targets) == 0 {
		return nil
	}
	target := targets[index%len(targets)]
	value := any(target.ID)
	if field.Relation.TargetField != "id" {
		value = target.Values[field.Relation.TargetField]
	}
	if field.Relation.Kind == "one-to-many" || field.Relation.Kind == "many-to-many" {
		return []any{value}
	}
	return value
}

func referencedTargetFields(app *appir.App) map[string]map[string]bool {
	out := map[string]map[string]bool{}
	for entityName := range app.DemoSeed.Entities {
		out[entityName] = map[string]bool{}
	}
	for entityName := range app.DemoSeed.Entities {
		for _, field := range app.Entities[entityName].Fields {
			if field.Relation != nil && field.Relation.TargetField != "id" {
				if target, seeded := out[field.Relation.Entity]; seeded {
					target[field.Relation.TargetField] = true
				}
			}
		}
	}
	return out
}

func scalarValue(field appir.Field, profile string, index int, seed int64) any {
	n := index + 1
	label := field.Label
	if label == "" {
		label = strings.ReplaceAll(field.Name, "_", " ")
	}
	switch field.Type {
	case "string", "text", "richtext":
		if field.Type == "text" || field.Type == "richtext" {
			return fmt.Sprintf("Demo %s %d for a credible %s workflow.", label, n, profile)
		}
		return namedValue(field.Name, profile, n)
	case "slug":
		return fmt.Sprintf("%s-%d", strings.ReplaceAll(field.Name, "_", "-"), n)
	case "integer", "money":
		return n * 10
	case "decimal":
		return fmt.Sprintf("%d.50", n)
	case "boolean":
		return index%2 == 0
	case "enum":
		if len(field.Options) > 0 {
			return field.Options[index%len(field.Options)]
		}
	case "date":
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, index).Format("2006-01-02")
	case "datetime":
		return time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC).Add(time.Duration(index) * 24 * time.Hour).Format(time.RFC3339)
	case "email":
		return fmt.Sprintf("demo%d@example.test", n)
	case "url":
		return fmt.Sprintf("https://example.test/%s/%d", field.Name, n)
	case "uuid":
		return stableUUID(seed, field.Name, index)
	case "json":
		return map[string]any{"demo": true, "index": n}
	}
	return nil
}

func namedValue(name, profile string, n int) string {
	if profile == "people" || strings.Contains(name, "name") {
		first := []string{"Avery", "Jordan", "Morgan", "Riley", "Taylor", "Casey"}
		last := []string{"Nguyen", "Smith", "Patel", "Garcia", "Brown", "Wilson"}
		return fmt.Sprintf("%s %s %d", first[(n-1)%len(first)], last[(n-1)%len(last)], n)
	}
	prefix := map[string]string{"activities": "Follow-up", "companies": "Northstar", "jobs": "Product role", "notes": "Interview note"}[profile]
	if prefix == "" {
		prefix = strings.ReplaceAll(name, "_", " ")
	}
	return fmt.Sprintf("%s %d", prefix, n)
}

func includeOptional(index int, name string) bool {
	return (index+len(name))%3 != 0
}

func stableUUID(seed int64, namespace string, index int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", seed, namespace, index)))
	sum[6] = (sum[6] & 0x0f) | 0x40
	sum[8] = (sum[8] & 0x3f) | 0x80
	h := hex.EncodeToString(sum[:16])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

func canonical(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}
