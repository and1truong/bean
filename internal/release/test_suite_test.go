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

func TestTestSuitePublicationIsMetadataOnlyAndSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "test-suite-release.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	ruleDefinition := definition.Definition{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "identity"}, Spec: map[string]any{"result": "integer", "input": map[string]any{"value": map[string]any{"type": "integer", "required": true}}, "expression": map[string]any{"source": "input", "path": "value"}}}
	suiteDefinition := definition.Definition{APIVersion: definition.APIVersion, Kind: "TestSuite", Metadata: definition.Metadata{Name: "identity_contract"}, Spec: map[string]any{"target": map[string]any{"kind": "Rule", "name": "identity"}, "tests": []any{map[string]any{"name": "returns_value", "input": map[string]any{"value": 7}, "expect": map[string]any{"result": 7}}}}}
	bundle := definition.Bundle{Name: "with semantic tests", Definitions: []definition.Definition{ruleDefinition, suiteDefinition}}
	_, plan, err := store.PreviewBundle(ctx, "default", bundle)
	if err != nil || len(plan.Statements) != 0 || len(plan.Descriptions) != 0 {
		t.Fatalf("TestSuite produced physical migration: plan=%+v err=%v", plan, err)
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
	if !exists || active.FormatVersion != appir.CurrentFormat || active.ReleaseID != published.ID || active.TestSuites["identity_contract"].Target.Name != "identity" {
		t.Fatalf("active=%+v", active)
	}
}
