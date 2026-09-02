package compiler_test

import (
	"testing"

	"github.com/beanruntime/bean/examples"
	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestCompileViewCandidateUsesCanonicalViewContract(t *testing.T) {
	app := appir.Empty()
	app.Entities["candidate"] = appir.Entity{Name: "candidate", Fields: []appir.Field{{Name: "name", Type: "string"}, {Name: "stage", Type: "enum", Options: []string{"applied", "interview"}}}}

	result := compiler.CompileViewCandidate(app, "candidate_records", map[string]any{
		"entity":         "candidate",
		"fields":         []any{"id", "name", "stage", "created_at", "updated_at", "version"},
		"search":         map[string]any{"fields": []any{"name"}},
		"exposedFilters": map[string]any{"stage": map[string]any{"field": "stage", "operator": "eq"}},
		"sort":           []any{map[string]any{"field": "id"}},
		"defaultLimit":   25,
		"maxLimit":       200,
		"displays": map[string]any{"table": map[string]any{
			"type": "block", "title": map[string]any{"text": "Candidate"},
			"renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "id", "label": "ID"}, map[string]any{"field": "name", "label": "Name"}, map[string]any{"field": "stage", "label": "Stage"}, map[string]any{"field": "created_at", "label": "Created at"}, map[string]any{"field": "updated_at", "label": "Updated at"}, map[string]any{"field": "version", "label": "Version"}}},
			"pager":    map[string]any{"type": "cursor", "pageSize": 25},
		}},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	view := result.App.Views["candidate_records"]
	if view.Name != "candidate_records" || len(view.Search.Fields) != 1 || view.Search.Fields[0] != "name" || view.ExposedFilters["stage"].Type != "enum" {
		t.Fatalf("view=%+v", view)
	}
}

func TestPageFiltersCompileTypedExplicitTargets(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "stage", "type": "enum", "options": []any{"applied", "interview"}}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidates"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id", "stage"}, "exposedFilters": map[string]any{"stage": map[string]any{"field": "stage", "operator": "eq"}}, "displays": map[string]any{"list": map[string]any{"type": "block", "renderer": map[string]any{"type": "list", "titleField": "id"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_list"}, Spec: map[string]any{"type": "view", "view": "candidates", "display": "list"}},
		{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"candidate_list"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"route": "/", "panel": "overview", "filters": map[string]any{"pipeline_stage": map[string]any{"label": "Stage", "widget": "select", "targets": []any{map[string]any{"block": "candidate_list", "filter": "stage"}}}}}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	filter := result.App.Pages["overview"].Filters["pipeline_stage"]
	if filter.Type != "enum" || len(filter.Options) != 2 || len(filter.Targets) != 1 {
		t.Fatalf("filter=%+v", filter)
	}
}

func TestATSExploreCandidateCompilesAgainstActiveApplication(t *testing.T) {
	bundle, err := examples.Load("ats")
	if err != nil {
		t.Fatal(err)
	}
	active := compiler.Compile("default", 1, bundle.Definitions)
	if len(active.Diagnostics) != 0 {
		t.Fatalf("active diagnostics=%+v", active.Diagnostics)
	}
	fields := []any{"id", "job_id", "name", "email", "stage", "applied_at", "summary", "created_at", "updated_at", "version"}
	columns := make([]any, 0, len(fields))
	for _, field := range fields {
		columns = append(columns, map[string]any{"field": field, "label": field})
	}
	result := compiler.CompileViewCandidate(active.App, "candidate_follow_up", map[string]any{
		"entity": "candidate", "fields": fields,
		"search":         map[string]any{"fields": []any{"name", "email", "summary"}},
		"exposedFilters": map[string]any{"stage": map[string]any{"field": "stage", "operator": "eq"}},
		"sort":           []any{map[string]any{"field": "id", "desc": false}},
		"defaultLimit":   25, "maxLimit": 200,
		"displays": map[string]any{"table": map[string]any{"type": "block", "title": map[string]any{"text": "Candidate"}, "renderer": map[string]any{"type": "table", "fields": columns}, "pager": map[string]any{"type": "cursor", "pageSize": 25}}},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
}

func TestCompileViewCandidateRejectsUnsafeSearch(t *testing.T) {
	app := appir.Empty()
	app.Entities["candidate"] = appir.Entity{Name: "candidate", Fields: []appir.Field{{Name: "name", Type: "string"}, {Name: "score", Type: "integer"}}}

	for _, test := range []struct {
		name   string
		fields []any
		search string
	}{
		{name: "unselected", fields: []any{"id"}, search: "name"},
		{name: "non_text", fields: []any{"id", "score"}, search: "score"},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.CompileViewCandidate(app, "candidate_records", map[string]any{
				"entity": "candidate", "fields": test.fields, "search": map[string]any{"fields": []any{test.search}},
			})
			if len(result.Diagnostics) == 0 {
				t.Fatal("invalid search accepted")
			}
		})
	}
}

func TestCompileViewCandidateDerivesTypedGroupShape(t *testing.T) {
	app := appir.Empty()
	app.Entities["candidate"] = appir.Entity{Name: "candidate", Fields: []appir.Field{{Name: "stage", Type: "enum", Options: []string{"applied", "interview"}}, {Name: "applied_at", Type: "datetime"}}}

	result := compiler.CompileViewCandidate(app, "candidates_by_month", map[string]any{
		"entity": "candidate", "fields": []any{"applied_at"},
		"groupBy":    []any{map[string]any{"field": "applied_at", "as": "applied_month", "bucket": "month"}},
		"aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}},
	})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("diagnostics=%+v", result.Diagnostics)
	}
	view := result.App.Views["candidates_by_month"]
	if view.ResultShape != "groups" || len(view.GroupBy) != 1 || view.GroupBy[0].Output() != "applied_month" || len(view.Sort) != 1 || view.Sort[0].Field != "applied_month" {
		t.Fatalf("view=%+v", view)
	}
}

func TestCompileViewCandidateRejectsInvalidAggregates(t *testing.T) {
	app := appir.Empty()
	app.Entities["deal"] = appir.Entity{Name: "deal", Fields: []appir.Field{{Name: "title", Type: "string"}, {Name: "amount", Type: "money"}}}

	tests := []struct {
		name, function, field string
	}{
		{name: "sum text", function: "sum", field: "title"},
		{name: "average money", function: "avg", field: "amount"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := compiler.CompileViewCandidate(app, "invalid_metric", map[string]any{
				"entity": "deal", "aggregates": []any{map[string]any{"function": test.function, "field": test.field, "alias": "value"}},
			})
			if len(result.Diagnostics) == 0 {
				t.Fatal("invalid aggregate accepted")
			}
		})
	}
}
