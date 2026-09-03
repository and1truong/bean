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

func TestCompileViewCandidateChecksSequenceRouteConflicts(t *testing.T) {
	definitions := append(validSequenceDefinitions(), definition.Definition{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}})
	active := compiler.Compile("test", 1, definitions)
	if len(active.Diagnostics) != 0 {
		t.Fatalf("active diagnostics=%v", active.Diagnostics)
	}
	result := compiler.CompileViewCandidate(active.App, "candidate_records", map[string]any{
		"entity": "candidate", "fields": []any{"id", "name"},
		"displays": map[string]any{"table": map[string]any{"type": "page", "route": "/presentations/bean", "renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "name"}}}}},
	})
	if !hasDiagnosticMessage(result.Diagnostics, "Sequence", "bean_intro", "spec.route", "overlaps route used by View/candidate_records") {
		t.Fatalf("Sequence route conflict accepted: %v", result.Diagnostics)
	}
}

func TestCompileViewCandidateRebuildsLegacyBlockDisplay(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "article"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "articles"}, Spec: map[string]any{"entity": "article", "fields": []any{"id", "title"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "article_list"}, Spec: map[string]any{"type": "view", "view": "articles", "presentation": map[string]any{"mode": "list", "titleField": "title"}}},
	}
	active := compiler.Compile("test", 1, definitions)
	if len(active.Diagnostics) != 0 {
		t.Fatalf("active diagnostics=%v", active.Diagnostics)
	}
	result := compiler.CompileViewCandidate(active.App, "articles", map[string]any{"entity": "article", "fields": []any{"id", "title"}})
	if len(result.Diagnostics) != 0 {
		t.Fatalf("candidate diagnostics=%v", result.Diagnostics)
	}
	if result.App.Blocks["article_list"].Display != "_block_article_list" || result.App.Views["articles"].Displays["_block_article_list"].Renderer.Type != "list" {
		t.Fatalf("legacy display was not rebuilt: block=%+v view=%+v", result.App.Blocks["article_list"], result.App.Views["articles"])
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
	definitions[3].Spec["regions"] = []any{map[string]any{"name": "main", "blocks": []any{}}}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "Page", "overview", "spec.filters.pipeline_stage.targets.0.block", "must belong to the Page Panel") {
		t.Fatalf("off-page filter target diagnostics=%v", diagnostics)
	}
}

func TestPageFiltersRequireMatchingTargetOperators(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "score", "type": "integer"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "minimum_candidates"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id", "score"}, "exposedFilters": map[string]any{"score": map[string]any{"field": "score", "operator": "gte"}}, "displays": map[string]any{"list": map[string]any{"type": "block", "renderer": map[string]any{"type": "list", "titleField": "id"}}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "maximum_candidates"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id", "score"}, "exposedFilters": map[string]any{"score": map[string]any{"field": "score", "operator": "lte"}}, "displays": map[string]any{"list": map[string]any{"type": "block", "renderer": map[string]any{"type": "list", "titleField": "id"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "minimum_list"}, Spec: map[string]any{"type": "view", "view": "minimum_candidates", "display": "list"}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "maximum_list"}, Spec: map[string]any{"type": "view", "view": "maximum_candidates", "display": "list"}},
		{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"minimum_list", "maximum_list"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"route": "/", "panel": "overview", "filters": map[string]any{"score": map[string]any{"targets": []any{map[string]any{"block": "minimum_list", "filter": "score"}, map[string]any{"block": "maximum_list", "filter": "score"}}}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "Page", "overview", "spec.filters.score.targets.1.filter", "must have the same type, operator, and options as every target") {
		t.Fatalf("operator mismatch diagnostics=%v", diagnostics)
	}
}

func TestPageFiltersValidateDerivedWidgetCompatibility(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "stage", "type": "enum", "options": []any{"applied", "interview"}}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidates"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id", "stage"}, "exposedFilters": map[string]any{"stage": map[string]any{"field": "stage"}}, "displays": map[string]any{"list": map[string]any{"type": "block", "renderer": map[string]any{"type": "list", "titleField": "id"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_list"}, Spec: map[string]any{"type": "view", "view": "candidates", "display": "list"}},
		{APIVersion: definition.APIVersion, Kind: "Panel", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"candidate_list"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Page", Metadata: definition.Metadata{Name: "overview"}, Spec: map[string]any{"route": "/", "panel": "overview", "filters": map[string]any{"stage": map[string]any{"widget": "number", "targets": []any{map[string]any{"block": "candidate_list", "filter": "stage"}}}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "Page", "overview", "spec.filters.stage.widget", "is incompatible with field type enum") {
		t.Fatalf("Page filter widget diagnostics=%v", diagnostics)
	}
	definitions[4].Spec["filters"].(map[string]any)["stage"].(map[string]any)["widget"] = "unknown"
	diagnostics = compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "Page", "overview", "spec.filters.stage.widget", "has no registered control widget") {
		t.Fatalf("unknown Page filter widget diagnostics=%v", diagnostics)
	}
}

func TestGroupedViewRejectsNonGroupedSelectedFields(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}, map[string]any{"name": "stage", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidates_by_stage"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"stage", "name"}, "groupBy": []any{"stage"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "candidates_by_stage", "spec.fields.1", "grouped Views may only select grouped fields") {
		t.Fatalf("grouped projection diagnostics=%v", diagnostics)
	}
}

func TestGroupedTableUsesEmittedGroupAliases(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "stage", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidates_by_stage"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"stage"}, "groupBy": []any{map[string]any{"field": "stage", "as": "stage_name"}}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}, "displays": map[string]any{"table": map[string]any{"type": "block", "renderer": map[string]any{"type": "table", "fields": []any{map[string]any{"field": "stage"}, map[string]any{"field": "candidate_count"}}}}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "candidates_by_stage", "spec.displays.table.renderer.fields.0.field", "must be selected by View candidates_by_stage") {
		t.Fatalf("source group field accepted as emitted output: %v", diagnostics)
	}
	definitions[1].Spec["displays"].(map[string]any)["table"].(map[string]any)["renderer"].(map[string]any)["fields"].([]any)[0].(map[string]any)["field"] = "stage_name"
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("group alias rejected: %v", diagnostics)
	}
}

func TestGroupedOutputsCannotCollideWithRedactedFields(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "candidate_policy"}, Spec: map[string]any{"redact": []any{"secret"}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "stage", "type": "string"}, map[string]any{"name": "secret", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidates_by_stage"}, Spec: map[string]any{"entity": "candidate", "policy": "candidate_policy", "fields": []any{"stage"}, "groupBy": []any{map[string]any{"field": "stage", "as": "secret"}}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "candidates_by_stage", "spec.groupBy.0.as", "must not collide with a redacted field") {
		t.Fatalf("redacted group output collision accepted: %v", diagnostics)
	}
	view := definitions[2].Spec
	view["groupBy"].([]any)[0].(map[string]any)["as"] = "stage_name"
	view["aggregates"].([]any)[0].(map[string]any)["alias"] = "secret"
	diagnostics = compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "candidates_by_stage", "spec.aggregates.0.alias", "must not collide with a redacted field") {
		t.Fatalf("redacted aggregate output collision accepted: %v", diagnostics)
	}
}

func TestSensitiveFieldsCannotDriveGroupedSemantics(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "secret", "type": "string", "sensitive": true}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "accounts_by_secret"}, Spec: map[string]any{"entity": "account", "fields": []any{"secret"}, "groupBy": []any{"secret"}, "aggregates": []any{map[string]any{"function": "count", "field": "secret", "alias": "account_count"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "accounts_by_secret", "spec.groupBy.0.field", "sensitive fields cannot control grouping") {
		t.Fatalf("sensitive group source accepted: %v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "View", "accounts_by_secret", "spec.aggregates.0.field", "sensitive fields cannot be aggregated") {
		t.Fatalf("sensitive aggregate source accepted: %v", diagnostics)
	}
}

func TestSensitiveFieldsCannotBeSelectedOrSearched(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "secret", "type": "string", "sensitive": true}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "accounts"}, Spec: map[string]any{"entity": "account", "fields": []any{"id", "secret"}, "search": map[string]any{"fields": []any{"secret"}}}},
	}
	result := compiler.Compile("test", 1, definitions)
	diagnostics := result.Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "accounts", "spec.fields.1", "sensitive fields cannot be selected") {
		t.Fatalf("sensitive projection accepted: %v", diagnostics)
	}
	if !hasDiagnosticMessage(diagnostics, "View", "accounts", "spec.search.fields.0", "sensitive fields cannot be searched") {
		t.Fatalf("sensitive search accepted: %v", diagnostics)
	}
	if contains(result.App.Views["account_list"].Fields, "secret") || contains(result.App.AdminResources["account"].List.Columns, "secret") || contains(result.App.AdminResources["account"].List.Search, "secret") {
		t.Fatalf("generated read surfaces expose sensitive field: view=%+v admin=%+v", result.App.Views["account_list"], result.App.AdminResources["account"])
	}
}

func TestSensitiveFieldsCannotBeExposedAsFilters(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "secret", "type": "string", "sensitive": true}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "accounts"}, Spec: map[string]any{"entity": "account", "fields": []any{"id"}, "exposedFilters": map[string]any{"secret": map[string]any{"field": "secret", "operator": "contains"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "accounts", "spec.exposedFilters.secret.field", "sensitive fields cannot be exposed as filters") {
		t.Fatalf("sensitive exposed filter accepted: %v", diagnostics)
	}
}

func TestGroupedViewSortRequiresEmittedOutput(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "event"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "occurred_at", "type": "datetime"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "events_by_month"}, Spec: map[string]any{"entity": "event", "fields": []any{"occurred_at"}, "groupBy": []any{map[string]any{"field": "occurred_at", "as": "month", "bucket": "month"}}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "event_count"}}, "sort": []any{map[string]any{"field": "occurred_at"}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnosticMessage(diagnostics, "View", "events_by_month", "spec.sort.0.field", "grouped Views must sort by an emitted group or aggregate field") {
		t.Fatalf("group source sort accepted: %v", diagnostics)
	}
	definitions[1].Spec["sort"].([]any)[0].(map[string]any)["field"] = "month"
	if diagnostics = compiler.Compile("test", 1, definitions).Diagnostics; len(diagnostics) != 0 {
		t.Fatalf("emitted group sort rejected: %v", diagnostics)
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
