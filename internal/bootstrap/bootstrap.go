package bootstrap

import (
	"context"
	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/auth"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/httpapi"
	"github.com/beanruntime/bean/internal/job"
	"github.com/beanruntime/bean/internal/kernel"
	"github.com/beanruntime/bean/internal/openapi"
	"github.com/beanruntime/bean/internal/release"
	"github.com/beanruntime/bean/internal/view"
)

type Runtime struct {
	DB     *sqlite.DB
	Kernel *kernel.Kernel
	Store  *release.Store
	HTTP   *httpapi.Server
	Jobs   job.Runner
}

func Open(ctx context.Context, path string, secure bool) (*Runtime, error) {
	db, e := sqlite.Open(path)
	if e != nil {
		return nil, e
	}
	k := kernel.New()
	store := &release.Store{DB: db, Migrations: db, Kernel: k, OpenAPI: openapi.Generate}
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
			return nil
		}
		definition, ok := app.Jobs[name]
		if !ok {
			return nil
		}
		_, e := actions.Execute(ctx, app, definition.Action, payload, beanctx.Request{User: &beanctx.User{ID: "system", Roles: []string{"administrator"}}, RequestID: "job:" + name})
		return e
	}}
	return &Runtime{DB: db, Kernel: k, Store: store, HTTP: server, Jobs: runner}, nil
}
