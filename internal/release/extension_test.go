package release_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestExtensionPublicationIsMetadataOnlyAndSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "extension-release.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	item := definition.Definition{APIVersion: definition.APIVersion, Kind: "Extension", Metadata: definition.Metadata{Name: "notify"}, Spec: map[string]any{
		"transport": "http", "endpoint": "https://provider.example/notify",
		"input":       map[string]any{"message": map[string]any{"type": "string", "required": true}},
		"output":      map[string]any{"accepted": map[string]any{"type": "boolean", "required": true}},
		"permissions": []any{"network"}, "sideEffects": []any{"external_write"}, "authentication": "none",
		"timeoutSeconds": 5, "retry": map[string]any{"maxAttempts": 3, "delaySeconds": 60},
		"idempotency": "required", "transaction": "after_commit", "failure": "retry_then_fail",
	}}
	bundle := definition.Bundle{Name: "with extension", Definitions: []definition.Definition{item}}
	_, plan, err := store.PreviewBundle(ctx, "default", bundle)
	if err != nil || len(plan.Statements) != 0 || len(plan.Descriptions) != 0 {
		t.Fatalf("Extension produced physical migration: plan=%+v err=%v", plan, err)
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
	if !exists || active.FormatVersion != appir.CurrentFormat || active.ReleaseID != published.ID || active.Extensions["notify"].Endpoint != "https://provider.example/notify" {
		t.Fatalf("active=%+v", active)
	}
}
