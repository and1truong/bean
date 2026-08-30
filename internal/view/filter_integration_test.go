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

func TestLegacyRichTextIsSanitizedOnRead(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "legacy-richtext.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE article (id TEXT PRIMARY KEY, body TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	source := `<p onclick="alert(1)">Safe</p><img src=x onerror="alert(2)">`
	if _, err = database.Insert(ctx, dbal.Insert{Table: "article", Values: map[string]dbal.Value{"id": "one", "body": source}}); err != nil {
		t.Fatal(err)
	}
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "body", Type: "richtext"}}}
	app.Views["public"] = appir.View{Name: "public", Entity: "article", Fields: []string{"id", "body"}, DefaultLimit: 10, MaxLimit: 10}

	rows, err := (view.Service{DB: database}).Run(ctx, app, "public", view.Params{}, beanctx.Request{})
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	body := rows[0]["body"].(string)
	if body != "<p>Safe</p>" {
		t.Fatalf("legacy rich text was not sanitized: %q", body)
	}
}

func TestCursorPaginationUsesAnUnselectedTieBreakerWithoutExposingIt(t *testing.T) {
	ctx := context.Background()
	database, err := sqlite.Open(filepath.Join(t.TempDir(), "cursor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err = database.ExecuteMigration(ctx, []string{`CREATE TABLE article (id TEXT PRIMARY KEY, title TEXT NOT NULL)`}); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"one", "two", "three"} {
		if _, err = database.Insert(ctx, dbal.Insert{Table: "article", Values: map[string]dbal.Value{"id": id, "title": "Same"}}); err != nil {
			t.Fatal(err)
		}
	}
	app := appir.Empty()
	app.Entities["article"] = appir.Entity{Name: "article", Fields: []appir.Field{{Name: "title", Type: "text"}}}
	app.Views["titles"] = appir.View{Name: "titles", Entity: "article", Fields: []string{"title"}, Sort: []appir.Sort{{Field: "title"}}, DefaultLimit: 2, MaxLimit: 2}
	service := view.Service{DB: database}

	first, err := service.RunPage(ctx, app, "titles", view.Params{}, beanctx.Request{})
	if err != nil || len(first.Rows) != 2 || first.NextCursor == "" {
		t.Fatalf("first page=%+v err=%v", first, err)
	}
	if _, exposed := first.Rows[0]["id"]; exposed {
		t.Fatalf("hidden cursor tie-breaker was exposed: %v", first.Rows)
	}
	second, err := service.RunPage(ctx, app, "titles", view.Params{Cursor: first.NextCursor}, beanctx.Request{})
	if err != nil || len(second.Rows) != 1 || second.Rows[0]["title"] != "Same" {
		t.Fatalf("second page=%+v err=%v", second, err)
	}
}

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
