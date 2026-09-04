package release_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestFieldLayoutsSurvivePublicationAndDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "layout.db")
	database, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	firstKernel := kernel.New()
	store := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: firstKernel, OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	bundle, err := examples.Load("blog")
	if err != nil {
		t.Fatal(err)
	}
	published, _, diagnostics, err := store.PublishBundle(ctx, "default", bundle)
	if err != nil || len(diagnostics) > 0 {
		t.Fatalf("publish: %v %v", err, diagnostics)
	}
	original, _ := firstKernel.Active()
	if err = database.Close(); err != nil {
		t.Fatal(err)
	}
	database, err = sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	nextKernel := kernel.New()
	store = &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: nextKernel, OpenAPI: openapi.Generate}
	// No definition bundle or source loader is supplied on restart.
	if err = store.LoadActive(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	restored, ok := nextKernel.Active()
	if !ok || restored.ReleaseID != published.ID {
		t.Fatal("release not restored")
	}
	form := restored.AdminResources["post"].Form.Layout
	detail := restored.Views["published_post"].Displays["record"].Renderer.Layout
	if form == nil || detail == nil || form.Groups[0].Columns != 2 || form.Groups[0].Fields[2].Span != "full" {
		t.Fatalf("form=%+v detail=%+v", form, detail)
	}
	if !reflect.DeepEqual(original.AdminResources["post"].Form.Layout, form) || !reflect.DeepEqual(original.Views["published_post"].Displays["record"].Renderer.Layout, detail) {
		t.Fatal("restart changed layouts")
	}
}
