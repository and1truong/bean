package release_test

import (
	"context"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"path/filepath"
	"testing"
)

type failingMigrations struct {
	base *sqlite.DB
	fail bool
}

func (f *failingMigrations) ExecuteMigration(c context.Context, s []string) error {
	if f.fail {
		return context.Canceled
	}
	return f.base.ExecuteMigration(c, s)
}
func TestInvalidAndFailedReleaseCannotReplaceActive(t *testing.T) {
	ctx := context.Background()
	db, e := sqlite.Open(filepath.Join(t.TempDir(), "release.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	k := kernel.New()
	m := &failingMigrations{base: db}
	s := &release.Store{DB: db, Migrations: m, Kernel: k, OpenAPI: openapi.Generate}
	if e = s.Initialize(ctx); e != nil {
		t.Fatal(e)
	}
	good := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}}
	if e = s.SaveBundle(ctx, "default", definition.Bundle{Name: "test", Definitions: []definition.Definition{good}}); e != nil {
		t.Fatal(e)
	}
	first, ds, e := s.Publish(ctx, "default")
	if e != nil || len(ds) > 0 {
		t.Fatalf("publish=%v diagnostics=%v", e, ds)
	}
	bad := good
	bad.Spec = map[string]any{"fields": []any{map[string]any{"name": "title", "type": "integer"}}}
	if e = s.SaveDefinition(ctx, "default", bad); e != nil {
		t.Fatal(e)
	}
	_, ds, e = s.Publish(ctx, "default")
	if e != nil || len(ds) == 0 {
		t.Fatalf("unsafe publish err=%v diagnostics=%v", e, ds)
	}
	active, _ := k.Active()
	if active.ReleaseID != first.ID {
		t.Fatal("invalid release replaced active")
	}
	add := good
	add.Spec = map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "author", "type": "string"}}}
	if e = s.SaveDefinition(ctx, "default", add); e != nil {
		t.Fatal(e)
	}
	m.fail = true
	if _, _, e = s.Publish(ctx, "default"); e == nil {
		t.Fatal("migration failure expected")
	}
	active, _ = k.Active()
	if active.ReleaseID != first.ID {
		t.Fatal("failed migration replaced active")
	}
	_ = migration.Plan{}
}
