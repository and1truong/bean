package view_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	"github.com/beanruntime/bean/internal/dbal"
	"github.com/beanruntime/bean/internal/dbal/sqlite"
	"github.com/beanruntime/bean/internal/view"
)

func TestViewFieldFilterFormatsOutputWithoutChangingSource(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "filters.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE article (id TEXT PRIMARY KEY, body TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	source := "Hello **Bean** <script>alert(1)</script>"
	if _, err = database.Insert(ctx, dbal.Insert{Table: "article", Values: map[string]dbal.Value{"id": "one", "body": source}}); err != nil {
		t.Fatal(err)
	}
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "body", Type: "text"}}}
	app.Filters["markdown"] = appir.Filter{Name: "markdown", Steps: []appir.FilterStep{{Type: "markdown"}}}
	app.Views["public"] = appir.View{Name: "public", Entity: "article", Fields: []string{"id", "body"}, FieldFilters: map[string]string{"body": "markdown"}, DefaultLimit: 10, MaxLimit: 10}
	app.Views["admin"] = appir.View{Name: "admin", Entity: "article", Fields: []string{"id", "body"}, DefaultLimit: 10, MaxLimit: 10}
	service := view.Service{DB: database}

	publicRows, err := service.Run(ctx, app, "public", view.Params{}, beanctx.Request{})
	if err != nil || len(publicRows) != 1 {
		t.Fatalf("public rows=%v err=%v", publicRows, err)
	}
	formatted := publicRows[0]["body"].(string)
	if !strings.Contains(formatted, "<strong>Bean</strong>") || strings.Contains(strings.ToLower(formatted), "<script") {
		t.Fatalf("formatted body=%q", formatted)
	}
	adminRows, err := service.Run(ctx, app, "admin", view.Params{}, beanctx.Request{})
	if err != nil || len(adminRows) != 1 || adminRows[0]["body"] != source {
		t.Fatalf("source changed: rows=%v err=%v", adminRows, err)
	}
}
