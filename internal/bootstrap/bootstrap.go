package bootstrap

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/auth"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/postgres"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/event"
	"github.com/beanruntime/bean/internal/httpapi"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/migration"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
)

type Runtime struct {
	DB     Database
	Kernel *kernel.Kernel
	Store  *release.Store
	HTTP   *httpapi.Server
	Jobs   job.Runner
	Outbox event.Runner
}

type Database interface {
	dbal.Database
	migration.Inspector
	migration.Executor
}

func Open(ctx context.Context, path string, secure bool) (*Runtime, error) {
	return OpenURL(ctx, path, secure)
}

func OpenURL(ctx context.Context, databaseURL string, secure bool) (*Runtime, error) {
	var db Database
	var e error
	switch {
	case strings.HasPrefix(databaseURL, "postgres://"), strings.HasPrefix(databaseURL, "postgresql://"):
		db, e = postgres.Open(databaseURL)
	case strings.HasPrefix(databaseURL, "sqlite://"):
		path := strings.TrimPrefix(databaseURL, "sqlite://")
		if path == "" {
			return nil, fmt.Errorf("SQLite database URL requires a path")
		}
		db, e = sqlite.Open(path)
	default:
		db, e = sqlite.Open(databaseURL)
	}
	if e != nil {
		return nil, e
	}
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Inspector: db, Kernel: k, OpenAPI: openapi.Generate}
	if e = store.Initialize(ctx); e != nil {
		db.Close()
		return nil, e
	}
	if e = store.LoadActive(ctx, "default"); e != nil {
		db.Close()
		return nil, e
	}
	authService := auth.Service{DB: db}
	actions := action.Service{DB: db}
	views := view.Service{DB: db}
	server := &httpapi.Server{Kernel: k, Store: store, Auth: authService, Actions: actions, Views: views, SecureCookies: secure}
	runner := job.Runner{DB: db, Handle: func(ctx context.Context, name string, payload map[string]any) error {
		app, ok := k.Active()
		if !ok {
			return fmt.Errorf("Job %s cannot run without an active release", name)
		}
		definition, ok := app.Jobs[name]
		if !ok {
			return fmt.Errorf("Job %s is not defined in the active release", name)
		}
		tenantID, _ := payload[job.TenantIDPayloadKey].(string)
		delete(payload, job.TenantIDPayloadKey)
		_, e := actions.Execute(ctx, app, definition.Action, payload, beanctx.Request{User: &beanctx.User{ID: "system", Roles: []string{"administrator"}}, TenantID: tenantID, RequestID: "job:" + name})
		return e
	}}
	outbox := event.Runner{DB: db, Deliver: func(ctx context.Context, topic string, _ map[string]any) error {
		slog.InfoContext(ctx, "Bean event delivered", "topic", topic)
		return nil
	}}
	return &Runtime{DB: db, Kernel: k, Store: store, HTTP: server, Jobs: runner, Outbox: outbox}, nil
}
