package release_test

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestStorageValidationAllowsLegacySQLiteBoolean(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "legacy-boolean.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err = db.ExecuteMigration(ctx, []string{`CREATE TABLE feature (id TEXT PRIMARY KEY,enabled INTEGER,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,version INTEGER NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	app := appir.Empty()
	app.ReleaseID = "legacy-release"
	app.Entities["feature"] = appir.Entity{Name: "feature", Fields: []appir.Field{{Name: "enabled", Type: "boolean"}}}
	if err = (&release.Store{Inspector: db}).ValidateStorage(ctx, app); err != nil {
		t.Fatal(err)
	}
}

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

func TestSaveBundleExactIsNotCappedAtTwoHundredDefinitions(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "definitions.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := &release.Store{DB: db, Migrations: db, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	definitions := make([]definition.Definition, 250)
	for index := range definitions {
		definitions[index] = definition.Definition{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: fmt.Sprintf("role_%03d", index)}, Spec: map[string]any{"permissions": []any{}}}
	}
	if err = store.SaveBundleExact(ctx, "default", definition.Bundle{Name: "large", Definitions: definitions}); err != nil {
		t.Fatal(err)
	}
	draft, err := store.Draft(ctx, "default")
	if err != nil || len(draft) != len(definitions) {
		t.Fatalf("draft definitions=%d err=%v", len(draft), err)
	}
	if err = store.SaveBundleExact(ctx, "default", definition.Bundle{Name: "small", Definitions: definitions[:1]}); err != nil {
		t.Fatal(err)
	}
	draft, err = store.Draft(ctx, "default")
	if err != nil || len(draft) != 1 || draft[0].Metadata.Name != "role_000" {
		t.Fatalf("replacement=%#v err=%v", draft, err)
	}
}

type publishBarrier struct {
	base  *sqlite.DB
	ready chan struct{}
	start chan struct{}
}

func (b *publishBarrier) ExecuteMigration(ctx context.Context, statements []string) error {
	b.ready <- struct{}{}
	<-b.start
	return b.base.ExecuteMigration(ctx, statements)
}

func TestConcurrentBundlePublishNeverActivatesAnotherCandidate(t *testing.T) {
	ctx := context.Background()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "concurrent.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initializer := &release.Store{DB: db, Migrations: db, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = initializer.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	barrier := &publishBarrier{base: db, ready: make(chan struct{}, 2), start: make(chan struct{})}
	store := &release.Store{DB: db, Migrations: barrier, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	type outcome struct {
		role      string
		published release.Published
		err       error
	}
	outcomes := make(chan outcome, 2)
	for _, role := range []string{"alpha", "beta"} {
		go func(role string) {
			bundle := definition.Bundle{Name: role, Definitions: []definition.Definition{{APIVersion: definition.APIVersion, Kind: "Role", Metadata: definition.Metadata{Name: role}, Spec: map[string]any{"permissions": []any{}}}}}
			published, _, diagnostics, publishErr := store.PublishBundle(ctx, "default", bundle)
			if len(diagnostics) > 0 {
				publishErr = diagnostics[0]
			}
			outcomes <- outcome{role: role, published: published, err: publishErr}
		}(role)
	}
	<-barrier.ready
	<-barrier.ready
	close(barrier.start)
	results := []outcome{<-outcomes, <-outcomes}
	var winner *outcome
	for index := range results {
		if results[index].err == nil {
			if winner != nil {
				t.Fatalf("both stale candidates published: %#v", results)
			}
			winner = &results[index]
		}
	}
	if winner == nil {
		t.Fatalf("no candidate published: %#v", results)
	}
	active, err := store.ActiveApp(ctx, "default")
	if err != nil || active == nil || active.ReleaseID != winner.published.ID {
		t.Fatalf("active=%#v winner=%#v err=%v", active, winner, err)
	}
	if _, exists := active.Roles[winner.role]; !exists || len(active.Roles) != 1 {
		t.Fatalf("winner %q does not match active roles %#v", winner.role, active.Roles)
	}
	draft, err := store.Draft(ctx, "default")
	if err != nil || len(draft) != 1 || draft[0].Metadata.Name != winner.role {
		t.Fatalf("draft=%#v winner=%#v err=%v", draft, winner, err)
	}
}
