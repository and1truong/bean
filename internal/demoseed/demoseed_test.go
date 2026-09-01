package demoseed

import (
	"context"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/expr"
	fieldpkg "github.com/beanruntime/bean/internal/field"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestGenerateIsDeterministicAndOrdersRelations(t *testing.T) {
	app := appir.Empty()
	app.Entities["company"] = appir.Entity{Name: "company", Fields: []appir.Field{{Name: "name", Label: "Name", Type: "string", Required: true}}}
	app.Entities["candidate"] = appir.Entity{Name: "candidate", Fields: []appir.Field{
		{Name: "name", Label: "Name", Type: "string", Required: true},
		{Name: "company_id", Type: "relation", Required: true, Relation: &appir.Relation{Entity: "company", Kind: "many-to-one", TargetField: "id"}},
		{Name: "stage", Type: "enum", Required: true, Options: []string{"applied", "interview"}},
	}}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"candidate": {Count: 3, Profile: "people"}, "company": {Count: 2, Profile: "companies"}}}

	first, err := Generate(app, 42)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Generate(app, 42)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("same seed produced different records")
	}
	if len(first) != 5 || first[0].Entity != "company" || first[2].Entity != "candidate" {
		t.Fatalf("records=%+v", first)
	}
	if first[2].Values["company_id"] != first[0].ID {
		t.Fatalf("relation=%v want %s", first[2].Values["company_id"], first[0].ID)
	}
}

func TestRunReachesGeneratedLifecycleStatesThroughActions(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "lifecycle-seed.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runtime := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Kernel: runtime, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "order"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"pending", "paid", "fulfilled"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "order_fulfillment"}, Spec: map[string]any{"entity": "order", "initial": "pending", "transitions": map[string]any{"pending": []any{"paid"}, "paid": []any{"fulfilled"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "advance_order"}, Spec: map[string]any{"entity": "order", "operation": "transition", "lifecycle": "order_fulfillment"}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"order": map[string]any{"count": 3}}}},
	}
	if err = store.SaveBundle(ctx, "default", definition.Bundle{Name: "lifecycle seed", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	if _, diagnostics, publishErr := store.Publish(ctx, "default"); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Active()
	result, err := Run(ctx, database, app, 42)
	if err != nil || result.Records != 3 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "order", Columns: []string{"status", "updated_at", "version"}, OrderBy: []dbal.Order{{Column: "status"}}, Limit: 10})
	if err != nil || len(rows) != 3 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	states := map[string]int{}
	wantUpdatedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC).Add(42 * time.Hour).Format(time.RFC3339Nano)
	for _, row := range rows {
		states[row["status"].(string)] = int(row["version"].(int64))
		if row["updated_at"] != wantUpdatedAt {
			t.Fatalf("nondeterministic updated_at row=%v want=%s", row, wantUpdatedAt)
		}
	}
	for state, version := range map[string]int{"pending": 1, "paid": 2, "fulfilled": 3} {
		if states[state] != version {
			t.Fatalf("state %s version=%d want=%d rows=%v", state, states[state], version, rows)
		}
	}
	if replay, replayErr := Run(ctx, database, app, 42); replayErr != nil || replay != result {
		t.Fatalf("replay=%+v want=%+v err=%v", replay, result, replayErr)
	}
}

func TestLifecyclePlanningUsesOnlyExecutableActionEdges(t *testing.T) {
	app := appir.Empty()
	app.Lifecycles["flow"] = appir.Lifecycle{
		Name: "flow", Entity: "item", StateField: "status", Initial: "initial",
		Transitions: map[string][]string{"initial": {"dead_end", "route"}, "dead_end": {"done"}, "route": {"done"}},
	}
	app.Actions["enter_dead_end"] = appir.Action{Name: "enter_dead_end", Entity: "item", Operation: "transition", Lifecycle: "flow", Transitions: map[string][]string{"initial": {"dead_end"}}}
	app.Actions["take_route"] = appir.Action{Name: "take_route", Entity: "item", Operation: "transition", Lifecycle: "flow", Transitions: map[string][]string{"initial": {"route"}, "route": {"done"}}}
	plans, err := planLifecycleMoves(app, []Record{{Entity: "item", ID: "00000000-0000-4000-8000-000000000001", Values: map[string]any{"status": "done"}}})
	if err != nil || !reflect.DeepEqual(plans, [][]lifecycleMove{{{Action: "take_route", State: "route"}, {Action: "take_route", State: "done"}}}) {
		t.Fatalf("plans=%+v err=%v", plans, err)
	}
}

func TestLifecyclePlanningUsesTransactionActionEdges(t *testing.T) {
	app := appir.Empty()
	app.Lifecycles["flow"] = appir.Lifecycle{
		Name: "flow", Entity: "item", StateField: "status", Initial: "initial",
		Transitions: map[string][]string{"initial": {"done"}},
	}
	app.Actions["finish_item"] = appir.Action{
		Name: "finish_item", Entity: "item", Operation: "transaction", Lifecycle: "flow",
		Input: map[string]appir.Field{
			"item_id":    {Name: "item_id", Type: "uuid", Required: true},
			"next_state": {Name: "next_state", Type: "enum", Required: true, Options: []string{"initial", "done"}},
		},
		Steps: []appir.Step{{Op: "transition", Values: []appir.Assignment{
			{Field: "id", Value: appir.ValueBinding{Source: "input", Path: "item_id"}},
			{Field: "status", Value: appir.ValueBinding{Source: "input", Path: "next_state"}},
		}}},
	}
	plans, err := planLifecycleMoves(app, []Record{{Entity: "item", ID: "00000000-0000-4000-8000-000000000001", Values: map[string]any{"status": "done"}}})
	want := [][]lifecycleMove{{{Action: "finish_item", State: "done", IDInput: "item_id", StateInput: "next_state"}}}
	if err != nil || !reflect.DeepEqual(plans, want) {
		t.Fatalf("plans=%+v want=%+v err=%v", plans, want, err)
	}
}

func TestLifecyclePlanningRejectsUnsafeTransactionActions(t *testing.T) {
	lifecycle := appir.Lifecycle{
		Name: "flow", Entity: "item", StateField: "status", Initial: "initial",
		Transitions: map[string][]string{"initial": {"done"}},
	}
	base := appir.Action{
		Name: "finish_item", Entity: "item", Operation: "transaction", Lifecycle: "flow",
		Input: map[string]appir.Field{
			"item_id":    {Name: "item_id", Type: "uuid", Required: true},
			"next_state": {Name: "next_state", Type: "enum", Required: true, Options: []string{"initial", "done"}},
		},
		Steps: []appir.Step{{Op: "transition", Values: []appir.Assignment{
			{Field: "id", Value: appir.ValueBinding{Source: "input", Path: "item_id"}},
			{Field: "status", Value: appir.ValueBinding{Source: "input", Path: "next_state"}},
		}}},
	}
	tests := map[string]appir.Action{
		"extra mutation": func() appir.Action {
			action := base
			action.Steps = append(append([]appir.Step{}, base.Steps...), appir.Step{Op: "create"})
			return action
		}(),
		"invalid id input": func() appir.Action {
			action := base
			action.Input = map[string]appir.Field{
				"item_id":    {Name: "item_id", Type: "integer", Required: true},
				"next_state": base.Input["next_state"],
			}
			return action
		}(),
		"invalid state input": func() appir.Action {
			action := base
			action.Input = map[string]appir.Field{
				"item_id":    base.Input["item_id"],
				"next_state": {Name: "next_state", Type: "enum", Required: true, Options: []string{"initial"}},
			}
			return action
		}(),
		"legacy entity override": func() appir.Action {
			action := base
			action.Steps = append([]appir.Step{}, base.Steps...)
			action.Steps[0].Values = append(action.Steps[0].Values, appir.Assignment{Field: "entity", Value: appir.ValueBinding{Source: "literal", Literal: []byte(`"other"`)}})
			return action
		}(),
	}
	for name, actionDefinition := range tests {
		t.Run(name, func(t *testing.T) {
			app := appir.Empty()
			app.Lifecycles["flow"] = lifecycle
			app.Actions["finish_item"] = actionDefinition
			_, err := planLifecycleMoves(app, []Record{{Entity: "item", ID: "00000000-0000-4000-8000-000000000001", Values: map[string]any{"status": "done"}}})
			if err == nil || !strings.Contains(err.Error(), "no executable Action path") {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestRunUsesTransactionActionForLifecycleTransition(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "lifecycle-transaction.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runtime := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Kernel: runtime, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"initial", "done"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "flow"}, Spec: map[string]any{"entity": "item", "initial": "initial", "transitions": map[string]any{"initial": []any{"done"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "finish_item"}, Spec: map[string]any{
			"entity": "item", "operation": "transaction", "lifecycle": "flow",
			"input": map[string]any{"item_id": map[string]any{"type": "uuid", "required": true}},
			"output": map[string]any{
				"id": map[string]any{"type": "uuid", "required": true}, "status": map[string]any{"type": "enum", "required": true, "options": []any{"initial", "done"}},
				"created_at": map[string]any{"type": "datetime", "required": true}, "updated_at": map[string]any{"type": "datetime", "required": true}, "version": map[string]any{"type": "integer", "required": true},
			},
			"steps": []any{map[string]any{"op": "transition", "values": map[string]any{"id": "$input.item_id", "status": "done"}}},
		}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"item": map[string]any{"count": 2}}}},
	}
	if _, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", definition.Bundle{Name: "transaction transition", Definitions: definitions}); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Active()
	if _, err = Run(ctx, database, app, 42); err != nil {
		t.Fatal(err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"status"}, OrderBy: []dbal.Order{{Column: "status"}}, Limit: 10})
	if err != nil || len(rows) != 2 || rows[0]["status"] != "done" || rows[1]["status"] != "initial" {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
}

func TestLifecyclePlanningFailsBeforeCreatingAnyRecord(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "lifecycle-no-path.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runtime := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Kernel: runtime, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"initial", "done"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "flow"}, Spec: map[string]any{"entity": "item", "initial": "initial", "transitions": map[string]any{"initial": []any{"done"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_item"}, Spec: map[string]any{"entity": "item", "operation": "transition", "lifecycle": "flow", "transitions": map[string]any{}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"item": map[string]any{"count": 2}}}},
	}
	if _, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", definition.Bundle{Name: "no path", Definitions: definitions}); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Active()
	if _, err = Run(ctx, database, app, 42); err == nil || !strings.Contains(err.Error(), "no executable Action path") {
		t.Fatalf("error=%v", err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id"}, Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("partial seed rows=%v err=%v", rows, err)
	}
}

func TestLifecycleIntermediateUniquenessFailsBeforeCreatingAnyRecord(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "lifecycle-unique-initial.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runtime := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Kernel: runtime, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "unique": true, "options": []any{"initial", "done"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "flow"}, Spec: map[string]any{"entity": "item", "initial": "initial", "transitions": map[string]any{"initial": []any{"done"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_item"}, Spec: map[string]any{"entity": "item", "operation": "transition", "lifecycle": "flow"}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"item": map[string]any{"count": 2}}}},
	}
	if _, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", definition.Bundle{Name: "unique initial", Definitions: definitions}); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Active()
	if _, err = Run(ctx, database, app, 42); err == nil || !strings.Contains(err.Error(), "intermediate initial state") {
		t.Fatalf("error=%v", err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id"}, Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("partial seed rows=%v err=%v", rows, err)
	}
}

func TestLifecycleIntermediateTransitionUniquenessFailsBeforeCreatingAnyRecord(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "lifecycle-unique-transition.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	runtime := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Kernel: runtime, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "status", "type": "enum", "required": true, "unique": true, "options": []any{"mid", "done", "initial"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Lifecycle", Metadata: definition.Metadata{Name: "flow"}, Spec: map[string]any{"entity": "item", "initial": "initial", "transitions": map[string]any{"initial": []any{"mid"}, "mid": []any{"done"}}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "move_item"}, Spec: map[string]any{"entity": "item", "operation": "transition", "lifecycle": "flow"}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"item": map[string]any{"count": 2}}}},
	}
	if _, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", definition.Bundle{Name: "unique transition", Definitions: definitions}); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	app, _ := runtime.Active()
	if _, err = Run(ctx, database, app, 42); err == nil || !strings.Contains(err.Error(), "intermediate transition state mid") {
		t.Fatalf("error=%v", err)
	}
	rows, err := database.Select(ctx, dbal.Select{Table: "item", Columns: []string{"id"}, Limit: 10})
	if err != nil || len(rows) != 0 {
		t.Fatalf("partial seed rows=%v err=%v", rows, err)
	}
}

func TestGenerateUsesDeclaredRelationTargetField(t *testing.T) {
	app := appir.Empty()
	app.Entities["account"] = appir.Entity{Name: "account", Fields: []appir.Field{{Name: "external_id", Type: "uuid", Required: true, Unique: true}}}
	app.Entities["contact"] = appir.Entity{Name: "contact", Fields: []appir.Field{{Name: "account_key", Type: "relation", Required: true, Relation: &appir.Relation{Entity: "account", Kind: "many-to-one", TargetField: "external_id"}}}}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"account": {Count: 2}, "contact": {Count: 2}}}
	records, err := Generate(app, 42)
	if err != nil {
		t.Fatal(err)
	}
	target := records[0].Values["external_id"]
	if records[2].Values["account_key"] != target || target == records[0].ID {
		t.Fatalf("relation=%v target=%v record ID=%s", records[2].Values["account_key"], target, records[0].ID)
	}
}

func TestGenerateRejectsRepeatedUniqueValuesBeforeSeeding(t *testing.T) {
	t.Run("relation", func(t *testing.T) {
		app := appir.Empty()
		app.Entities["account"] = appir.Entity{Name: "account", Fields: []appir.Field{{Name: "name", Type: "string", Required: true}}}
		app.Entities["contact"] = appir.Entity{Name: "contact", Fields: []appir.Field{{Name: "account_id", Type: "relation", Required: true, Unique: true, Relation: &appir.Relation{Entity: "account", Kind: "many-to-one", TargetField: "id"}}}}
		app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"account": {Count: 2}, "contact": {Count: 3}}}
		if _, err := Generate(app, 42); err == nil || !strings.Contains(err.Error(), "unique constraint contact(account_id)") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("tuple", func(t *testing.T) {
		app := appir.Empty()
		app.Entities["sample"] = appir.Entity{Name: "sample", Fields: []appir.Field{{Name: "enabled", Type: "boolean", Required: true}, {Name: "state", Type: "enum", Required: true, Options: []string{"one", "two"}}}, Unique: [][]string{{"enabled", "state"}}}
		app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"sample": {Count: 3}}}
		if _, err := Generate(app, 42); err == nil || !strings.Contains(err.Error(), "unique constraint sample(enabled,state)") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("system field", func(t *testing.T) {
		app := appir.Empty()
		app.Entities["sample"] = appir.Entity{Name: "sample", Fields: []appir.Field{{Name: "name", Type: "string", Required: true}}, Unique: [][]string{{"version"}}}
		app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"sample": {Count: 2}}}
		if _, err := Generate(app, 42); err == nil || !strings.Contains(err.Error(), "unique constraint sample(version)") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestVerificationAppBuildsUnrestrictedCompleteViews(t *testing.T) {
	app := appir.Empty()
	app.Entities["sample"] = appir.Entity{Name: "sample", Owner: true, Tenant: true, SoftDelete: true, Fields: []appir.Field{{Name: "name", Type: "string"}}}
	filter := &expr.Expr{}
	app.Views["sample_list"] = appir.View{Name: "sample_list", Entity: "other", Fields: []string{"name"}, Relationships: []appir.ViewRelationship{{Name: "related"}}, Filter: filter, ContextFilter: filter, GroupBy: []string{"name"}, Aggregates: []appir.Aggregate{{Function: "count", Field: "id", Alias: "total"}}, Policy: "scoped", FieldFilters: map[string]string{"name": "markdown"}, MaxLimit: 2}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"sample": {Count: 1}}}
	verification, err := verificationApp(app)
	if err != nil {
		t.Fatal(err)
	}
	view := verification.Views["sample_list"]
	if view.Entity != "sample" || !reflect.DeepEqual(view.Fields, []string{"id", "name", "owner_id", "tenant_id", "deleted_at", "created_at", "updated_at", "version"}) || view.Filter != nil || view.ContextFilter != nil || len(view.Relationships) != 0 || len(view.GroupBy) != 0 || len(view.Aggregates) != 0 || len(view.FieldFilters) != 0 {
		t.Fatalf("verification View=%+v", view)
	}
	if view.Policy == "" || verification.Policies[view.Policy].Name != view.Policy || verification.Entities["sample"].SoftDelete {
		t.Fatalf("verification scope was not removed: view=%+v entity=%+v", view, verification.Entities["sample"])
	}
}

func TestOmittedValueOnlyAcceptsEmptySlicesForToManyRelations(t *testing.T) {
	entity := appir.Entity{Fields: []appir.Field{
		{Name: "metadata", Type: "json"},
		{Name: "tags", Type: "relation", Relation: &appir.Relation{Kind: "many-to-many"}},
	}}
	if omittedValueMatches(entity, "metadata", []any{}) {
		t.Fatal("empty JSON array matched an omitted value")
	}
	if !omittedValueMatches(entity, "metadata", nil) {
		t.Fatal("null did not match an omitted optional value")
	}
	if !omittedValueMatches(entity, "tags", []any{}) {
		t.Fatal("empty to-many relation did not match an omitted value")
	}
}

func TestGenerateCoversSupportedScalarTypesAndSkipsUnsafeFields(t *testing.T) {
	app := appir.Empty()
	fields := []appir.Field{
		{Name: "name", Type: "string", Required: true, Unique: true},
		{Name: "text", Type: "text", Required: true}, {Name: "rich", Type: "richtext", Required: true},
		{Name: "slug", Type: "slug", Required: true}, {Name: "integer", Type: "integer", Required: true},
		{Name: "money", Type: "money", Required: true}, {Name: "decimal", Type: "decimal", Required: true},
		{Name: "boolean", Type: "boolean", Required: true}, {Name: "enum", Type: "enum", Required: true, Options: []string{"one", "two"}},
		{Name: "date", Type: "date", Required: true}, {Name: "datetime", Type: "datetime", Required: true},
		{Name: "email", Type: "email", Required: true, Unique: true}, {Name: "url", Type: "url", Required: true},
		{Name: "uuid", Type: "uuid", Required: true}, {Name: "json", Type: "json", Required: true},
		{Name: "password", Type: "password"}, {Name: "file", Type: "file"}, {Name: "secret", Type: "string", Sensitive: true},
	}
	app.Entities["sample"] = appir.Entity{Name: "sample", Fields: fields}
	app.DemoSeed = &appir.DemoSeed{Name: "demo", Entities: map[string]appir.DemoSeedEntity{"sample": {Count: 3, Profile: "auto"}}}
	records, err := Generate(app, 7)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range records {
		for _, definition := range fields[:15] {
			if err := fieldpkg.Validate(definition, record.Values[definition.Name]); err != nil {
				t.Fatalf("%s=%v: %v", definition.Name, record.Values[definition.Name], err)
			}
		}
		for _, name := range []string{"password", "file", "secret"} {
			if _, exists := record.Values[name]; exists {
				t.Fatalf("unsafe field %s was generated", name)
			}
		}
	}
	if records[0].Values["name"] == records[1].Values["name"] || records[0].Values["email"] == records[1].Values["email"] {
		t.Fatal("unique values were repeated")
	}
}
