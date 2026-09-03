package release_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
)

func TestCurrentAppIRPublicationSurvivesRestart(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "sequence-release.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: kernel.New(), OpenAPI: openapi.Generate}
	if err = store.Initialize(ctx); err != nil {
		t.Fatal(err)
	}
	bundle, err := examples.Load("presentation")
	if err != nil {
		t.Fatal(err)
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
	sequence := active.Sequences["bean_introduction"]
	if !exists || active.FormatVersion != appir.CurrentFormat || active.ReleaseID != published.ID || len(sequence.Frames) != 10 || len(active.Panels["frame_architecture"].Regions[0].Items[0].Content) == 0 {
		t.Fatalf("active=%+v sequence=%+v", active, sequence)
	}

	trackerBundle, err := examples.Load("tracker")
	if err != nil {
		t.Fatal(err)
	}
	trackerRelease, _, diagnostics, err := reloaded.PublishBundle(ctx, "tracker", trackerBundle)
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("tracker publish=%v diagnostics=%v", err, diagnostics)
	}
	finalKernel := kernel.New()
	finalStore := &release.Store{DB: database, Migrations: database, Inspector: database, Kernel: finalKernel, OpenAPI: openapi.Generate}
	if err = finalStore.LoadActive(ctx, "tracker"); err != nil {
		t.Fatal(err)
	}
	active, exists = finalKernel.Active()
	page := active.Pages["tracker_home"]
	if !exists || active.ReleaseID != trackerRelease.ID || len(page.Sections) != 2 || page.Sections[0].Panel != "tracker_intro" || page.Sections[1].Panel != "tracker_operations" {
		t.Fatalf("active=%+v Page=%+v", active, page)
	}
}
