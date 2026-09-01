package release_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestRulePublicationIsMetadataOnlyAndSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "rule-release.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	entity := definition.Definition{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "invoice"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "amount", "type": "money", "required": true}, map[string]any{"name": "total", "type": "money", "required": true}}}}
	if _, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", definition.Bundle{Name: "before rules", Definitions: []definition.Definition{entity}}); publishErr != nil || len(diagnostics) != 0 {
		t.Fatalf("initial publish=%v diagnostics=%v", publishErr, diagnostics)
	}
	entity.Spec["validations"] = map[string]any{"non_negative": "non_negative"}
	copyTotal := definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "copy_total"}, Spec: map[string]any{"entity": "invoice", "result": "number", "input": map[string]any{"amount": map[string]any{"type": "money"}}, "expression": map[string]any{"source": "input", "path": "amount"}}}
	nonNegative := definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "non_negative"}, Spec: map[string]any{"entity": "invoice", "result": "boolean", "expression": map[string]any{"op": "gte", "args": []any{map[string]any{"source": "this", "path": "total"}, map[string]any{"source": "literal", "literal": 0}}}}}
	create := definition.Definition{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "invoice_create"}, Spec: map[string]any{"entity": "invoice", "operation": "create", "derive": map[string]any{"total": "copy_total"}}}
	bundle := definition.Bundle{Name: "with rules", Definitions: []definition.Definition{entity, copyTotal, nonNegative, create}}
	_, plan, err := store.PreviewBundle(ctx, "default", bundle)
	if err != nil || len(plan.Statements) != 0 || len(plan.Descriptions) != 0 {
		t.Fatalf("Rules produced physical migration: plan=%+v err=%v", plan, err)
	}
	published, _, diagnostics, err := store.PublishBundle(ctx, "default", bundle)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("publish=%v diagnostics=%v", err, diagnostics)
	}
	reloadedKernel := kernel.New()
	reloaded := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: reloadedKernel, OpenAPI: openapi.Generate}
	if err = reloaded.LoadActive(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	active, exists := reloadedKernel.Active()
	if !exists || active.FormatVersion != appir.CurrentFormat || active.ReleaseID != published.ID || active.Rules["copy_total"].Result != "number" || active.Actions["invoice_create"].Derive["total"] != "copy_total" {
		t.Fatalf("active=%+v", active)
	}
	request := beanctx.Request{User: &beanctx.User{ID: "00000000-0000-4000-8000-000000000001", Roles: []string{"administrator"}}, RequestID: "restart"}
	created, err := (action.Service{DB: database}).Execute(ctx, active, "invoice_create", map[string]any{"amount": 42}, request)
	if err != nil || created["total"] != int64(42) && created["total"] != 42 {
		t.Fatalf("created=%v err=%v", created, err)
	}
}
