package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/action"
	"github.com/beanruntime/bean/internal/bootstrap"
	"github.com/beanruntime/bean/internal/compiler"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/definition"
)

const version = "0.6.0-alpha"

func main() {
	os.Exit(execute(os.Args[1:], os.Stdout, os.Stderr))
}

func execute(args []string, stdout, stderr io.Writer) int {
	if exit, handled := runAgentCommand(args, stdout, stderr); handled {
		return exit
	}
	if e := run(args); e != nil {
		fmt.Fprintln(stderr, "command failed:", e)
		return exitRuntime
	}
	return exitOK
}
func run(args []string) error {
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "init":
		return initCommand(args[1:])
	case "serve":
		return serveCommand(args[1:])
	case "validate":
		return validateCommand(args[1:])
	case "publish":
		return publishCommand(args[1:])
	case "migrate":
		return migrateCommand(args[1:])
	case "app":
		return appCommand(args[1:])
	case "demo":
		return demoCommand(args[1:])
	case "user":
		return userCommand(args[1:])
	case "version":
		fmt.Println("bean " + version)
		return nil
	default:
		return usage()
	}
}
func usage() error {
	return fmt.Errorf("usage: bean {init|serve|validate|publish|migrate|capabilities|schema|app init|app validate|app inspect|app plan|app diff|app publish|app test|app import|app export|user create|demo|version}")
}
func userCommand(args []string) error {
	if len(args) == 0 || args[0] != "create" {
		return fmt.Errorf("usage: bean user create")
	}
	f := flag.NewFlagSet("user create", flag.ContinueOnError)
	db := f.String("db", "bean.db", "SQLite database")
	databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	email := f.String("email", "", "user email")
	password := f.String("password", "test-password", "user password")
	roles := f.String("roles", "authenticated", "comma-separated roles")
	tenant := f.String("tenant", "", "tenant UUID")
	if e := f.Parse(args[1:]); e != nil {
		return e
	}
	if *email == "" {
		return fmt.Errorf("--email is required")
	}
	r, e := bootstrap.OpenURL(context.Background(), databaseTarget(*db, *databaseURL), false)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	return r.HTTP.Auth.Create(context.Background(), *email, *password, strings.Split(*roles, ","), *tenant)
}
func initCommand(args []string) error {
	f := flag.NewFlagSet("init", flag.ContinueOnError)
	db := f.String("db", "bean.db", "SQLite database")
	databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	email := f.String("admin-email", "admin@example.test", "administrator email")
	password := f.String("admin-password", "test-password", "administrator password")
	if e := f.Parse(args); e != nil {
		return e
	}
	r, e := bootstrap.OpenURL(context.Background(), databaseTarget(*db, *databaseURL), false)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	if e = r.Store.EnsureApp(context.Background(), "default", "Bean"); e != nil {
		return e
	}
	return r.HTTP.Auth.Bootstrap(context.Background(), *email, *password)
}
func serveCommand(args []string) error {
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	db := f.String("db", "bean.db", "SQLite database")
	databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	addr := f.String("addr", "127.0.0.1:8080", "listen address")
	secure := f.Bool("secure-cookie", false, "set Secure session cookies")
	trustedProxy := f.String("trusted-proxy", "", "comma-separated trusted reverse-proxy IPs or CIDRs")
	if e := f.Parse(args); e != nil {
		return e
	}
	return serve(databaseTarget(*db, *databaseURL), *addr, *secure, *trustedProxy)
}
func serve(db, addr string, secure bool, trustedProxy string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	r, e := bootstrap.Open(ctx, db, secure)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	for _, value := range strings.Split(trustedProxy, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, parseErr := netip.ParsePrefix(value)
		if parseErr != nil {
			address, addressErr := netip.ParseAddr(value)
			if addressErr != nil {
				return fmt.Errorf("invalid trusted proxy %q", value)
			}
			prefix = netip.PrefixFrom(address, address.BitLen())
		}
		r.HTTP.TrustedProxies = append(r.HTTP.TrustedProxies, prefix)
	}
	server := &http.Server{Addr: addr, Handler: r.HTTP.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	errors := make(chan error, 1)
	workerDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		defer close(workerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = r.Jobs.RunOnce(ctx)
				_ = r.Outbox.RunOnce(ctx)
			}
		}
	}()
	go func() { slog.Info("Bean listening", "addr", addr); errors <- server.ListenAndServe() }()
	select {
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		e := server.Shutdown(shutdown)
		<-workerDone
		return e
	case e := <-errors:
		if e == http.ErrServerClosed {
			return nil
		}
		return e
	}
}
func validateCommand(args []string) error {
	db, e := dbFlag("validate", args)
	if e != nil {
		return e
	}
	r, e := bootstrap.Open(context.Background(), db, false)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	result, e := r.Store.Validate(context.Background(), "default")
	if e != nil {
		return e
	}
	for _, d := range result.Diagnostics {
		fmt.Println(d.Error())
	}
	if len(result.Diagnostics) > 0 {
		return fmt.Errorf("draft is invalid")
	}
	fmt.Println("draft is valid")
	return nil
}
func publishCommand(args []string) error {
	db, e := dbFlag("publish", args)
	if e != nil {
		return e
	}
	r, e := bootstrap.Open(context.Background(), db, false)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	p, ds, e := r.Store.Publish(context.Background(), "default")
	if e != nil {
		return e
	}
	if len(ds) > 0 {
		for _, d := range ds {
			fmt.Println(d.Error())
		}
		return fmt.Errorf("release validation failed")
	}
	fmt.Printf("published release %s version %d\n", p.ID, p.Version)
	return nil
}
func migrateCommand(args []string) error {
	db, e := dbFlag("migrate", args)
	if e != nil {
		return e
	}
	r, e := bootstrap.Open(context.Background(), db, false)
	if e != nil {
		return e
	}
	return r.DB.Close()
}
func appCommand(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: bean app {init|validate|inspect|plan|diff|publish|test|import|export}")
	}
	switch args[0] {
	case "validate":
		f := flag.NewFlagSet("app validate", flag.ContinueOnError)
		file := f.String("file", "", "application manifest")
		if e := f.Parse(args[1:]); e != nil {
			return e
		}
		if *file == "" {
			return fmt.Errorf("--file is required")
		}
		bundle, e := loadValidSource(*file, os.Stderr)
		if e != nil {
			return e
		}
		fmt.Printf("valid: %s — %d definitions\n", bundle.Name, len(bundle.Definitions))
		return nil
	case "import":
		f := flag.NewFlagSet("app import", flag.ContinueOnError)
		db := f.String("db", "bean.db", "SQLite database")
		databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
		file := f.String("file", "", "application manifest")
		if e := f.Parse(args[1:]); e != nil {
			return e
		}
		if *file == "" {
			return fmt.Errorf("--file is required")
		}
		bundle, e := loadValidSource(*file, os.Stderr)
		if e != nil {
			return e
		}
		return importBundle(databaseTarget(*db, *databaseURL), bundle)
	case "export":
		f := flag.NewFlagSet("app export", flag.ContinueOnError)
		db := f.String("db", "bean.db", "SQLite database")
		databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
		file := f.String("file", "", "output YAML bundle")
		if e := f.Parse(args[1:]); e != nil {
			return e
		}
		if *file == "" {
			return fmt.Errorf("--file is required")
		}
		r, e := bootstrap.OpenURL(context.Background(), databaseTarget(*db, *databaseURL), false)
		if e != nil {
			return e
		}
		defer r.DB.Close()
		defs, e := r.Store.Draft(context.Background(), "default")
		if e != nil {
			return e
		}
		out, e := os.Create(*file)
		if e != nil {
			return e
		}
		defer out.Close()
		return definition.Encode(out, definition.Bundle{Name: "Bean", Definitions: defs})
	default:
		return fmt.Errorf("unknown app command %q", args[0])
	}
}
func importBundle(db string, bundle definition.Bundle) error {
	r, e := bootstrap.Open(context.Background(), db, false)
	if e != nil {
		return e
	}
	defer r.DB.Close()
	return r.Store.SaveBundle(context.Background(), "default", bundle)
}

func loadValidSource(filename string, diagnosticsOut io.Writer) (definition.Bundle, error) {
	bundle, diagnostics := validateSource(filename)
	if len(diagnostics) == 0 {
		return bundle, nil
	}
	for _, diagnostic := range diagnostics {
		fmt.Fprintln(diagnosticsOut, diagnostic.Error())
	}
	label := "errors"
	if len(diagnostics) == 1 {
		label = "error"
	}
	return bundle, fmt.Errorf("application source is invalid (%d %s)", len(diagnostics), label)
}

func validateSource(filename string) (definition.Bundle, []definition.Diagnostic) {
	bundle, diagnostics, complete := definition.LoadFileForValidation(filename)
	if complete {
		diagnostics = mergeSourceDiagnostics(diagnostics, compiler.Compile("default", 1, bundle.Definitions).Diagnostics)
	} else {
		diagnostics = mergeSourceDiagnostics(diagnostics, compiler.CompileRecovered("default", 1, bundle.Definitions).Diagnostics)
	}
	definition.ClassifyDiagnostics(diagnostics)
	sort.SliceStable(diagnostics, func(left, right int) bool {
		a, b := diagnostics[left], diagnostics[right]
		if a.Source.Path != b.Source.Path {
			return a.Source.Path < b.Source.Path
		}
		if a.Source.Line != b.Source.Line {
			return a.Source.Line < b.Source.Line
		}
		if a.Source.Column != b.Source.Column {
			return a.Source.Column < b.Source.Column
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		return a.Path < b.Path
	})
	return bundle, diagnostics
}

func mergeSourceDiagnostics(loader, compiled []definition.Diagnostic) []definition.Diagnostic {
	diagnostics := append([]definition.Diagnostic{}, loader...)
	invalidAPIVersion := false
	duplicates := map[string]bool{}
	missingRequiredField := map[string]bool{}
	for _, diagnostic := range loader {
		if diagnostic.Path == "apiVersion" {
			invalidAPIVersion = true
		}
		if diagnostic.Message == "is required" && (diagnostic.Path == "kind" || diagnostic.Path == "name") {
			missingRequiredField[diagnosticSourceKey(diagnostic)+"/"+diagnostic.Path] = true
		}
		if diagnostic.Message == "duplicate definition" {
			duplicates[diagnostic.Kind+"/"+diagnostic.Name] = true
		}
	}
	for _, diagnostic := range compiled {
		if invalidAPIVersion && diagnostic.Path == "apiVersion" ||
			diagnostic.Path == "kind" && missingRequiredField[diagnosticSourceKey(diagnostic)+"/kind"] ||
			diagnostic.Path == "metadata.name" && missingRequiredField[diagnosticSourceKey(diagnostic)+"/name"] ||
			diagnostic.Message == "duplicate machine name" && duplicates[diagnostic.Kind+"/"+diagnostic.Name] {
			continue
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return diagnostics
}

func diagnosticSourceKey(diagnostic definition.Diagnostic) string {
	return fmt.Sprintf("%s:%d:%d", diagnostic.Source.Path, diagnostic.Source.Line, diagnostic.Source.Column)
}

func demoCommand(args []string) error {
	f := flag.NewFlagSet("demo", flag.ContinueOnError)
	name := f.String("app", "cms", "reference application")
	db := f.String("db", filepath.Join("tmp", "demo.db"), "SQLite database")
	databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	addr := f.String("addr", "127.0.0.1:8080", "listen address")
	if e := f.Parse(args); e != nil {
		return e
	}
	target := databaseTarget(*db, *databaseURL)
	if *databaseURL == "" {
		if e := os.MkdirAll(filepath.Dir(*db), 0o755); e != nil {
			return e
		}
	}
	bundle, e := examples.Load(*name)
	if e != nil {
		return e
	}
	r, e := bootstrap.OpenURL(context.Background(), target, false)
	if e != nil {
		return e
	}
	if e = r.Store.EnsureApp(context.Background(), "default", bundle.Name); e == nil {
		e = r.HTTP.Auth.Bootstrap(context.Background(), "admin@example.test", "test-password")
	}
	if e == nil {
		e = r.Store.SaveBundle(context.Background(), "default", bundle)
	}
	if e == nil {
		_, ds, publishErr := r.Store.Publish(context.Background(), "default")
		e = publishErr
		if len(ds) > 0 {
			e = ds[0]
		}
	}
	if e == nil {
		admin := beanctx.Request{User: &beanctx.User{ID: "demo-admin", Email: "admin@example.test", Roles: []string{"administrator"}}, RequestID: "demo-seed"}
		engine := action.Service{DB: r.DB}
		app, _ := r.Kernel.Active()
		for entity, rows := range bundle.Seed {
			for _, row := range rows {
				_, e = engine.Execute(context.Background(), app, entity+"_create", row, admin)
				if e != nil {
					break
				}
			}
		}
	}
	r.DB.Close()
	if e != nil {
		return e
	}
	return serve(target, *addr, false, "")
}
func dbFlag(name string, args []string) (string, error) {
	f := flag.NewFlagSet(name, flag.ContinueOnError)
	db := f.String("db", "bean.db", "SQLite database")
	databaseURL := f.String("database-url", os.Getenv("BEAN_DATABASE_URL"), "database URL (PostgreSQL or SQLite)")
	if e := f.Parse(args); e != nil {
		return "", e
	}
	return databaseTarget(*db, *databaseURL), nil
}

func databaseTarget(path, databaseURL string) string {
	if databaseURL != "" {
		return databaseURL
	}
	return path
}
