package compiler_test

import (
	"encoding/json"
	"testing"

	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
)

func TestDiagnosticsAreActionable(t *testing.T) {
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "broken"}, Spec: map[string]any{"entity": "missing", "fields": []any{"title"}}}}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) != 1 {
		t.Fatalf("diagnostics=%v", r.Diagnostics)
	}
	d := r.Diagnostics[0]
	if d.Kind != "View" || d.Name != "broken" || d.Path != "spec.entity" {
		t.Fatalf("diagnostic=%+v", d)
	}
}

func TestTransitionDiagnosticsValidateStateEdges(t *testing.T) {
	definitions := []definition.Definition{
		{
			APIVersion: definition.APIVersion,
			Kind:       "Entity",
			Metadata:   definition.Metadata{Name: "candidate"},
			Spec: map[string]any{"fields": []any{
				map[string]any{"name": "status", "type": "enum", "options": []any{"applied", "interview", "hired"}},
			}},
		},
		{
			APIVersion: definition.APIVersion,
			Kind:       "Action",
			Metadata:   definition.Metadata{Name: "advance_candidate"},
			Spec:       map[string]any{"entity": "candidate", "operation": "transition", "transitions": map[string]any{"screening": []any{"offer"}}},
		},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if len(diagnostics) != 2 {
		t.Fatalf("diagnostics=%v", diagnostics)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "BEAN-E2201" || len(diagnostic.Candidates) != 3 {
			t.Fatalf("diagnostic=%+v", diagnostic)
		}
	}
}
func TestGeneratedCRUDUsesActionsAndViews(t *testing.T) {
	defs := []definition.Definition{{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string", "required": true}}}}}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) > 0 {
		t.Fatal(r.Diagnostics)
	}
	if _, ok := r.App.Views["book_list"]; !ok {
		t.Fatal("generated View missing")
	}
	if _, ok := r.App.Actions["book_create"]; !ok {
		t.Fatal("generated Action missing")
	}
	admin, ok := r.App.AdminResources["book"]
	if !ok || admin.View != "book_list" || admin.LabelField != "title" || admin.List.PageSize != 25 {
		t.Fatalf("generated AdminResource=%+v", admin)
	}
}

func TestBoardAndTreePresentationsValidateTypedFieldsAndActions(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "task"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "title", "type": "string", "required": true},
			map[string]any{"name": "status", "type": "enum", "options": []any{"todo", "done"}, "required": true},
			map[string]any{"name": "position", "type": "integer", "required": true},
			map[string]any{"name": "parent_id", "type": "relation", "relation": map[string]any{"entity": "task", "kind": "many-to-one", "targetField": "id"}},
		}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "tasks"}, Spec: map[string]any{"entity": "task", "fields": []any{"id", "title", "status", "position", "parent_id"}}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "move_task"}, Spec: map[string]any{"entity": "task", "operation": "transition", "transitions": map[string]any{"todo": []any{"done"}, "done": []any{"todo"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "board"}, Spec: map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "board", "titleField": "title", "groupField": "status", "orderField": "position", "moveAction": "move_task", "columns": []any{"todo", "done"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "tree"}, Spec: map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "tree", "titleField": "title", "parentField": "parent_id", "orderField": "position"}}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) > 0 {
		t.Fatalf("valid presentations diagnostics=%v", result.Diagnostics)
	}
	broken := append([]definition.Definition{}, defs...)
	broken[3].Spec = map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "board", "titleField": "title", "groupField": "title", "moveAction": "task_update", "columns": []any{"unknown"}}}
	if diagnostics := compiler.Compile("test", 1, broken).Diagnostics; len(diagnostics) < 2 {
		t.Fatalf("invalid board accepted: %v", diagnostics)
	}
	redacted := append([]definition.Definition{}, defs...)
	entitySpec := map[string]any{}
	for key, value := range defs[0].Spec {
		entitySpec[key] = value
	}
	entitySpec["policy"] = "task_access"
	redacted[0].Spec = entitySpec
	redacted[1].Spec = map[string]any{"entity": "task", "fields": []any{"id", "title", "status", "position", "parent_id"}, "sort": []any{map[string]any{"field": "status"}}, "exposedFilters": map[string]any{"status": map[string]any{"name": "status", "type": "enum"}}}
	redacted = append(redacted, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "task_access"}, Spec: map[string]any{"redact": []any{"title", "status", "parent_id"}}})
	diagnostics := compiler.Compile("test", 1, redacted).Diagnostics
	paths := map[string]bool{}
	for _, diagnostic := range diagnostics {
		paths[diagnostic.Name+":"+diagnostic.Path] = true
	}
	for _, path := range []string{"tasks:spec.sort.0.field", "tasks:spec.exposedFilters.status", "board:spec.presentation.titleField", "board:spec.presentation.groupField", "tree:spec.presentation.titleField", "tree:spec.presentation.parentField"} {
		if !paths[path] {
			t.Fatalf("redacted presentation field %s accepted: %v", path, diagnostics)
		}
	}
	missingTitle := append([]definition.Definition{}, defs...)
	missingTitle[3].Spec = map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "board", "groupField": "status", "moveAction": "move_task", "columns": []any{"todo", "done"}}}
	if diagnostics = compiler.Compile("test", 1, missingTitle).Diagnostics; !hasDiagnostic(diagnostics, "board", "spec.presentation.titleField") {
		t.Fatalf("board without a title field accepted: %v", diagnostics)
	}
	badOrder := append([]definition.Definition{}, defs...)
	badOrder[3].Spec = map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "board", "titleField": "title", "groupField": "status", "orderField": "title", "moveAction": "move_task", "columns": []any{"todo", "done"}}}
	if diagnostics = compiler.Compile("test", 1, badOrder).Diagnostics; !hasDiagnostic(diagnostics, "board", "spec.presentation.orderField") {
		t.Fatalf("nonnumeric board order accepted: %v", diagnostics)
	}
	extraInput := append([]definition.Definition{}, defs...)
	extraInput[2].Spec = map[string]any{"entity": "task", "operation": "transition", "input": map[string]any{"reason": map[string]any{"type": "string", "required": true}}, "transitions": map[string]any{"todo": []any{"done"}, "done": []any{"todo"}}}
	if diagnostics = compiler.Compile("test", 1, extraInput).Diagnostics; !hasDiagnostic(diagnostics, "board", "spec.presentation.moveAction") {
		t.Fatalf("board Action with extra required input accepted: %v", diagnostics)
	}
	badLink := append([]definition.Definition{}, defs...)
	badLink[4].Spec = map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "tree", "titleField": "title", "parentField": "parent_id", "linkRoute": "/tasks/:slug"}}
	if diagnostics = compiler.Compile("test", 1, badLink).Diagnostics; !hasDiagnostic(diagnostics, "tree", "spec.presentation.linkRoute") {
		t.Fatalf("unselected presentation link field accepted: %v", diagnostics)
	}
	unsafeLink := append([]definition.Definition{}, defs...)
	unsafeLink[4].Spec = map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "tree", "titleField": "title", "parentField": "parent_id", "linkRoute": "/\\evil.example/:id"}}
	if diagnostics = compiler.Compile("test", 1, unsafeLink).Diagnostics; !hasDiagnostic(diagnostics, "tree", "spec.presentation.linkRoute") {
		t.Fatalf("browser-unstable presentation link route accepted: %v", diagnostics)
	}
	aggregateSorted := append([]definition.Definition{}, defs...)
	aggregateSorted[1].Spec = map[string]any{"entity": "task", "fields": []any{"id", "title", "status", "position", "parent_id"}, "groupBy": []any{"id", "title", "status", "position", "parent_id"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "task_count"}}, "sort": []any{map[string]any{"field": "task_count"}}}
	if diagnostics = compiler.Compile("test", 1, aggregateSorted).Diagnostics; !hasDiagnostic(diagnostics, "board", "spec.presentation.mode") || !hasDiagnostic(diagnostics, "tree", "spec.presentation.mode") {
		t.Fatalf("aggregate-sorted structured View accepted: %v", diagnostics)
	}
}

func TestDemoThemeMetricTimelineAndSearchContracts(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Theme", Metadata: definition.Metadata{Name: "default"}, Spec: map[string]any{"displayName": "Acme Recruiting", "preset": "professional", "accent": "indigo"}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"candidate": map[string]any{"count": 12, "profile": "people"}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "name", "type": "string", "required": true},
			map[string]any{"name": "stage", "type": "enum", "options": []any{"applied", "interview"}, "required": true},
			map[string]any{"name": "applied_at", "type": "datetime", "required": true},
			map[string]any{"name": "note", "type": "text"},
		}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidate_metrics"}, Spec: map[string]any{"entity": "candidate", "fields": []any{}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "recent_candidates"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id", "name", "stage", "applied_at", "note"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_count"}, Spec: map[string]any{"type": "view", "view": "candidate_metrics", "presentation": map[string]any{"mode": "metric", "metricField": "candidate_count", "metricLabel": "Candidates"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_timeline"}, Spec: map[string]any{"type": "view", "view": "recent_candidates", "presentation": map[string]any{"mode": "timeline", "titleField": "name", "bodyField": "note", "timeField": "applied_at", "metaFields": []any{"stage"}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_search"}, Spec: map[string]any{"type": "view", "view": "recent_candidates", "presentation": map[string]any{"mode": "list", "titleField": "name", "searchFields": []any{"name", "note"}}}},
	}
	result := compiler.Compile("test", 1, definitions)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("valid demo presentation diagnostics=%v", result.Diagnostics)
	}
	if result.App.Theme == nil || result.App.Theme.DisplayName != "Acme Recruiting" || result.App.Theme.Accent != "indigo" {
		t.Fatalf("theme=%+v", result.App.Theme)
	}
	if result.App.DemoSeed == nil || result.App.DemoSeed.Entities["candidate"].Count != 12 || result.App.DemoSeed.Entities["candidate"].Profile != "people" {
		t.Fatalf("demo seed=%+v", result.App.DemoSeed)
	}

	invalidTheme := append([]definition.Definition{}, definitions...)
	invalidTheme[0].Spec = map[string]any{"preset": "javascript", "accent": "#ff00ff"}
	invalidTheme = append(invalidTheme, definition.Definition{APIVersion: definition.APIVersion, Kind: "Theme", Metadata: definition.Metadata{Name: "second"}, Spec: map[string]any{"preset": "professional", "accent": "indigo"}})
	diagnostics := compiler.Compile("test", 1, invalidTheme).Diagnostics
	for _, path := range []string{"spec.preset", "spec.accent", "metadata.name"} {
		if !hasDiagnostic(diagnostics, "default", path) && !hasDiagnostic(diagnostics, "second", path) {
			t.Fatalf("missing Theme diagnostic %s: %v", path, diagnostics)
		}
	}

	invalidPresentations := append([]definition.Definition{}, definitions...)
	invalidPresentations[5].Spec = map[string]any{"type": "view", "view": "candidate_metrics", "presentation": map[string]any{"mode": "metric", "metricField": "missing"}}
	invalidPresentations[6].Spec = map[string]any{"type": "view", "view": "recent_candidates", "presentation": map[string]any{"mode": "timeline", "titleField": "name", "timeField": "stage"}}
	invalidPresentations[7].Spec = map[string]any{"type": "view", "view": "recent_candidates", "presentation": map[string]any{"mode": "list", "searchFields": []any{"stage", "missing"}}}
	diagnostics = compiler.Compile("test", 1, invalidPresentations).Diagnostics
	for _, target := range [][2]string{{"candidate_count", "spec.presentation.metricField"}, {"candidate_timeline", "spec.presentation.timeField"}, {"candidate_search", "spec.presentation.searchFields.0"}, {"candidate_search", "spec.presentation.searchFields.1"}} {
		if !hasDiagnostic(diagnostics, target[0], target[1]) {
			t.Fatalf("missing presentation diagnostic %v: %v", target, diagnostics)
		}
	}
}

func TestDemoSeedValidation(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{
			"candidate": map[string]any{"count": 201, "profile": "fiction"},
			"missing":   map[string]any{"count": 1},
		}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	for _, path := range []string{"spec.entities.candidate.count", "spec.entities.candidate.profile", "spec.entities.missing"} {
		if !hasDiagnostic(diagnostics, "demo", path) {
			t.Fatalf("missing DemoSeed diagnostic %s: %v", path, diagnostics)
		}
	}
}

func TestDemoSeedRejectsRequiredRelationCycles(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "left"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "right_id", "type": "relation", "required": true, "relation": map[string]any{"entity": "right"}}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "right"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "left_id", "type": "relation", "required": true, "relation": map[string]any{"entity": "left"}}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"left": map[string]any{"count": 1}, "right": map[string]any{"count": 1}}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "demo", "spec.entities") {
		t.Fatalf("required relation cycle accepted: %v", diagnostics)
	}
}

func TestDemoSeedRejectsImpossibleUniqueValues(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "sample"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "enabled", "type": "boolean", "unique": true},
			map[string]any{"name": "state", "type": "enum", "unique": true, "options": []any{"one", "two"}},
		}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"sample": map[string]any{"count": 3}}}},
	}
	diagnostics := compiler.Compile("test", 1, definitions).Diagnostics
	if !hasDiagnostic(diagnostics, "demo", "spec.entities.sample.count") {
		t.Fatalf("impossible seed accepted: %v", diagnostics)
	}
}

func TestDemoSeedRejectsUnsupportedRequiredRelationTarget(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "secret", "type": "string", "unique": true, "sensitive": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "contact"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "account_key", "type": "relation", "required": true, "relation": map[string]any{"entity": "account", "kind": "many-to-one", "targetField": "secret"}}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"account": map[string]any{"count": 1}, "contact": map[string]any{"count": 1}}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "demo", "spec.entities.contact") {
		t.Fatalf("unsupported target accepted: %v", diagnostics)
	}
}

func TestDemoSeedRejectsRequiredRelationTargetingSystemField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "contact"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "account_version", "type": "relation", "required": true, "relation": map[string]any{"entity": "account", "kind": "many-to-one", "targetField": "version"}}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"account": map[string]any{"count": 1}, "contact": map[string]any{"count": 1}}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "demo", "spec.entities.contact") {
		t.Fatalf("system target accepted: %v", diagnostics)
	}
}

func TestDemoSeedRejectsRequiredRelationTargetingNonUUIDField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "slug", "type": "slug", "unique": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "contact"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "account_slug", "type": "relation", "required": true, "relation": map[string]any{"entity": "account", "kind": "many-to-one", "targetField": "slug"}}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"account": map[string]any{"count": 1}, "contact": map[string]any{"count": 1}}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "demo", "spec.entities.contact") {
		t.Fatalf("non-UUID relation target accepted: %v", diagnostics)
	}
}

func TestDemoSeedRejectsOptionalRelationTargetingNonUUIDField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "account"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "slug", "type": "slug", "unique": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "contact"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "account_slug", "type": "relation", "relation": map[string]any{"entity": "account", "kind": "many-to-one", "targetField": "slug"}}}}},
		{APIVersion: definition.APIVersion, Kind: "DemoSeed", Metadata: definition.Metadata{Name: "demo"}, Spec: map[string]any{"entities": map[string]any{"account": map[string]any{"count": 1}, "contact": map[string]any{"count": 1}}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "demo", "spec.entities.contact") {
		t.Fatalf("optional non-UUID relation target accepted: %v", diagnostics)
	}
}

func TestMetricRejectsGroupedView(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "stage", "type": "enum", "options": []any{"applied", "hired"}}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidate_metrics"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"stage"}, "groupBy": []any{"stage"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_count"}, Spec: map[string]any{"type": "view", "view": "candidate_metrics", "presentation": map[string]any{"mode": "metric", "metricField": "candidate_count"}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "candidate_count", "spec.presentation.metricField") {
		t.Fatalf("grouped metric accepted: %v", diagnostics)
	}
}

func TestMetricRejectsSelectedRecordFields(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "candidate"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "name", "type": "string"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "candidate_metrics"}, Spec: map[string]any{"entity": "candidate", "fields": []any{"id"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "candidate_count"}}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "candidate_count"}, Spec: map[string]any{"type": "view", "view": "candidate_metrics", "presentation": map[string]any{"mode": "metric", "metricField": "candidate_count"}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "candidate_count", "spec.presentation.metricField") {
		t.Fatalf("metric View with selected fields accepted: %v", diagnostics)
	}
}

func TestTimelineRequiresSelectedTimeField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "activity"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "occurred_at", "type": "datetime"}}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "activities"}, Spec: map[string]any{"entity": "activity", "fields": []any{"id", "title"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "timeline"}, Spec: map[string]any{"type": "view", "view": "activities", "presentation": map[string]any{"mode": "timeline", "titleField": "title", "timeField": "occurred_at"}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "timeline", "spec.presentation.timeField") {
		t.Fatalf("unselected timeline time field accepted: %v", diagnostics)
	}
}

func TestTimelineRejectsRedactedTimeField(t *testing.T) {
	definitions := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "activity"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "occurred_at", "type": "datetime"}}}},
		{APIVersion: definition.APIVersion, Kind: "Policy", Metadata: definition.Metadata{Name: "activity_policy"}, Spec: map[string]any{"redact": []any{"occurred_at"}}},
		{APIVersion: definition.APIVersion, Kind: "View", Metadata: definition.Metadata{Name: "activities"}, Spec: map[string]any{"entity": "activity", "policy": "activity_policy", "fields": []any{"id", "title", "occurred_at"}}},
		{APIVersion: definition.APIVersion, Kind: "Block", Metadata: definition.Metadata{Name: "timeline"}, Spec: map[string]any{"type": "view", "view": "activities", "presentation": map[string]any{"mode": "timeline", "titleField": "title", "timeField": "occurred_at"}}},
	}
	if diagnostics := compiler.Compile("test", 1, definitions).Diagnostics; !hasDiagnostic(diagnostics, "timeline", "spec.presentation.timeField") {
		t.Fatalf("redacted timeline time field accepted: %v", diagnostics)
	}
}

func TestPresentationRejectsRelatedFileDownloads(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "attachment"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "file", "type": "file"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "task"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}, map[string]any{"name": "attachment_id", "type": "relation", "relation": map[string]any{"entity": "attachment", "kind": "many-to-one", "targetField": "id"}}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "tasks"}, Spec: map[string]any{"entity": "task", "fields": []any{"id", "title", "attachment.file"}, "relationships": []any{map[string]any{"name": "attachment", "entity": "attachment", "localField": "attachment_id", "targetField": "id", "type": "left"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "tasks"}, Spec: map[string]any{"type": "view", "view": "tasks", "presentation": map[string]any{"mode": "list", "titleField": "title", "bodyField": "attachment.file"}}},
	}
	diagnostics := compiler.Compile("test", 1, defs).Diagnostics
	if !hasDiagnostic(diagnostics, "tasks", "spec.presentation.bodyField") {
		t.Fatalf("related file presentation accepted: %v", diagnostics)
	}
}

func hasDiagnostic(diagnostics []definition.Diagnostic, name, path string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Name == name && diagnostic.Path == path {
			return true
		}
	}
	return false
}

func TestAdminResourceReferencesAreValidated(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "title", "type": "string"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "library"}, Spec: map[string]any{"entity": "book", "labelField": "missing", "list": map[string]any{"columns": []any{"id", "missing"}, "pageSize": 500}}},
	}
	r := compiler.Compile("test", 1, defs)
	if len(r.Diagnostics) < 2 {
		t.Fatalf("expected AdminResource diagnostics, got %v", r.Diagnostics)
	}
}
func TestRejectsUnknownFieldsAndUnimplementedSteps(t *testing.T) {
	entity := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{}, "mystery": true}}
	r := compiler.Compile("test", 1, []definition.Definition{entity})
	if len(r.Diagnostics) == 0 {
		t.Fatal("unknown field accepted")
	}
	entity.Spec = map[string]any{"fields": []any{}}
	action := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "bad"}, Spec: map[string]any{"entity": "book", "operation": "transaction", "steps": []any{map[string]any{"op": "load"}}}}
	r = compiler.Compile("test", 1, []definition.Definition{entity, action})
	if len(r.Diagnostics) == 0 {
		t.Fatal("step without executor accepted")
	}
}
func TestCompilationIsDeterministic(t *testing.T) {
	d := definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "book"}, Spec: map[string]any{"fields": []any{}}}
	a := compiler.Compile("test", 1, []definition.Definition{d})
	b := compiler.Compile("test", 1, []definition.Definition{d})
	aj, _ := json.Marshal(a.App)
	bj, _ := json.Marshal(b.App)
	if string(aj) != string(bj) {
		t.Fatalf("non-deterministic AppIR\n%s\n%s", aj, bj)
	}
}

func TestLocalRegistrationCompilesFixedSensitiveBoundary(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
		{APIVersion: "bean/v1alpha1", Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup"}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) != 0 {
		t.Fatal(result.Diagnostics)
	}
	action := result.App.Actions["signup"]
	if action.DefaultRole != "member" || !action.Input["password"].Sensitive || !action.Input["password_confirmation"].Sensitive {
		t.Fatalf("registration boundary=%+v", action)
	}
	if _, exposed := action.Output["password"]; exposed {
		t.Fatal("password exposed in registration output")
	}
	defs[1].Spec["defaultRole"] = "administrator"
	if invalid := compiler.Compile("test", 1, defs); len(invalid.Diagnostics) == 0 {
		t.Fatal("undefined registration role accepted")
	}
	defs = append(defs, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Role", Metadata: definition.Metadata{Name: "administrator"}, Spec: map[string]any{}})
	if invalid := compiler.Compile("test", 1, defs); len(invalid.Diagnostics) == 0 {
		t.Fatal("privileged self-registration role accepted")
	}
}

func TestLocalRegistrationRouteMustRenderItsSelectedAction(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Role", Metadata: definition.Metadata{Name: "member"}, Spec: map[string]any{}},
		{APIVersion: "bean/v1alpha1", Kind: "Action", Metadata: definition.Metadata{Name: "signup"}, Spec: map[string]any{"operation": "register_local_user", "defaultRole": "member"}},
		{APIVersion: "bean/v1alpha1", Kind: "Webform", Metadata: definition.Metadata{Name: "signup_form"}, Spec: map[string]any{"action": "signup", "elements": []any{map[string]any{"name": "email", "type": "email", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "signup_block"}, Spec: map[string]any{"type": "webform", "webform": "signup_form"}},
		{APIVersion: "bean/v1alpha1", Kind: "Panel", Metadata: definition.Metadata{Name: "signup_panel"}, Spec: map[string]any{"layout": "single-column", "regions": []any{map[string]any{"name": "main", "blocks": []any{"signup_block"}}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Page", Metadata: definition.Metadata{Name: "signup_page"}, Spec: map[string]any{"route": "/register", "panel": "signup_panel"}},
		{APIVersion: "bean/v1alpha1", Kind: "LocalRegistration", Metadata: definition.Metadata{Name: "local"}, Spec: map[string]any{"action": "signup", "route": "/register"}},
	}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration Webform missing required Action inputs was accepted")
	}
	defs[2].Spec["elements"] = []any{
		map[string]any{"name": "display_name", "type": "text", "required": true},
		map[string]any{"name": "email", "type": "email", "required": true},
		map[string]any{"name": "password", "type": "password", "required": true},
		map[string]any{"name": "password_confirmation", "type": "password", "required": true},
	}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) != 0 {
		t.Fatalf("valid registration route diagnostics=%v", result.Diagnostics)
	}
	delete(defs[5].Spec, "panel")
	defs[5].Spec["sections"] = []any{map[string]any{"panel": "signup_panel"}, map[string]any{"panel": "signup_panel"}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) != 0 {
		t.Fatalf("registration Webform in Page sections diagnostics=%v", result.Diagnostics)
	}
	delete(defs[5].Spec, "sections")
	defs[5].Spec["panel"] = "signup_panel"
	displayName := defs[2].Spec["elements"].([]any)[0].(map[string]any)
	displayName["visible"] = map[string]any{"op": "eq", "left": map[string]any{"source": "literal", "literal": true}, "right": map[string]any{"source": "literal", "literal": false}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("conditionally hidden required registration input was accepted")
	}
	delete(displayName, "visible")
	defs[len(defs)-1].Spec["route"] = "/missing"
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration route without its Webform was accepted")
	}
	defs[len(defs)-1].Spec["route"] = "/register"
	defs = append(defs, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "members_only"}, Spec: map[string]any{"authenticated": true}})
	defs[5].Spec["policy"] = "members_only"
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration Page inaccessible to anonymous users was accepted")
	}
	delete(defs[5].Spec, "policy")
	defs = append(defs, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "context_sensitive"}, Spec: map[string]any{"condition": map[string]any{"op": "ne", "left": map[string]any{"source": "record", "name": "x"}, "right": map[string]any{"source": "context", "name": "invite"}}}})
	defs[5].Spec["policy"] = "context_sensitive"
	defs[5].Spec["context"] = map[string]any{"invite": map[string]any{"source": "query", "name": "invite"}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration Page policy evaluated only after resolving optional context")
	}
	delete(defs[5].Spec, "policy")
	defs[5].Spec["context"] = map[string]any{"invite": map[string]any{"source": "query", "name": "invite", "required": true}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration Page with unresolved required context was accepted")
	}
	delete(defs[5].Spec, "context")
	defs[3].Spec["inputs"] = map[string]any{"email": map[string]any{"type": "email"}}
	defs[3].Spec["bindings"] = map[string]any{"email": map[string]any{"source": "context", "name": "invite"}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration field duplicated by an immutable Block binding was accepted")
	}
	delete(defs[3].Spec, "inputs")
	delete(defs[3].Spec, "bindings")
	defs = append(defs, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "broken_sibling"}, Spec: map[string]any{"type": "text", "text": "Broken", "inputs": map[string]any{"invite": map[string]any{"type": "string", "required": true}}, "bindings": map[string]any{"invite": map[string]any{"source": "context", "name": "invite"}}}})
	defs[4].Spec["regions"] = []any{map[string]any{"name": "main", "blocks": []any{"signup_block", "broken_sibling"}}}
	if result := compiler.Compile("test", 1, defs); len(result.Diagnostics) == 0 {
		t.Fatal("registration Page with an unrenderable sibling Block was accepted")
	}
}

func TestResourceListBlockRequiresEditorOnlyPolicy(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "body", "type": "text", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "members"}, Spec: map[string]any{"readRoles": []any{"member"}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "notes"}, Spec: map[string]any{"type": "resource-list", "resource": "note", "policy": "members"}},
	}
	result := compiler.Compile("test", 1, defs)
	found := false
	for _, diagnostic := range result.Diagnostics {
		found = found || diagnostic.Kind == "Block" && diagnostic.Name == "notes" && diagnostic.Path == "spec.policy"
	}
	if !found {
		t.Fatalf("non-editor resource-list diagnostics=%v", result.Diagnostics)
	}
}

func TestWebformBlockRejectsRenderedAndBoundFieldOverlap(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "note"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "body", "type": "text", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Webform", Metadata: definition.Metadata{Name: "note_form"}, Spec: map[string]any{"action": "note_create", "elements": []any{map[string]any{"name": "body", "type": "text", "required": true}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "note_block"}, Spec: map[string]any{"type": "webform", "webform": "note_form", "inputs": map[string]any{"body": map[string]any{"type": "text"}}, "bindings": map[string]any{"body": map[string]any{"source": "context", "name": "body"}}}},
	}
	result := compiler.Compile("test", 1, defs)
	found := false
	for _, diagnostic := range result.Diagnostics {
		found = found || diagnostic.Kind == "Block" && diagnostic.Name == "note_block" && diagnostic.Path == "spec.bindings.body"
	}
	if !found {
		t.Fatalf("Webform Block field/binding collision diagnostics=%v", result.Diagnostics)
	}
}

func TestWebformRejectsFileElementsInsideRepeatingGroups(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "submission"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "attachments", "type": "json"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Webform", Metadata: definition.Metadata{Name: "submission_form"}, Spec: map[string]any{"action": "submission_create", "elements": []any{map[string]any{"name": "attachments", "type": "group", "children": []any{map[string]any{"name": "file", "type": "file"}}}}}},
	}
	result := compiler.Compile("test", 1, defs)
	found := false
	for _, diagnostic := range result.Diagnostics {
		found = found || diagnostic.Kind == "Webform" && diagnostic.Name == "submission_form" && diagnostic.Path == "spec.elements.0.children.0.type"
	}
	if !found {
		t.Fatalf("nested file diagnostics=%v", result.Diagnostics)
	}
}

func TestWebformRejectsDerivedActionInput(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: definition.APIVersion, Kind: "Entity", Metadata: definition.Metadata{Name: "invoice"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "total", "type": "money", "required": true}}}},
		{APIVersion: definition.APIVersion, Kind: "Rule", Metadata: definition.Metadata{Name: "one"}, Spec: map[string]any{"result": "number", "expression": map[string]any{"source": "literal", "literal": 1}}},
		{APIVersion: definition.APIVersion, Kind: "Action", Metadata: definition.Metadata{Name: "invoice_create"}, Spec: map[string]any{"entity": "invoice", "operation": "create", "derive": map[string]any{"total": "one"}}},
		{APIVersion: definition.APIVersion, Kind: "Webform", Metadata: definition.Metadata{Name: "invoice_form"}, Spec: map[string]any{"action": "invoice_create", "elements": []any{map[string]any{"name": "total", "type": "number", "required": true}}}},
	}
	result := compiler.Compile("test", 1, defs)
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Kind == "Webform" && diagnostic.Name == "invoice_form" && diagnostic.Path == "spec.elements.0.name" {
			return
		}
	}
	t.Fatalf("derived Webform input diagnostics=%v", result.Diagnostics)
}

func TestExpressionsRejectUnimplementedNowSource(t *testing.T) {
	defs := []definition.Definition{{
		APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "future"},
		Spec: map[string]any{"condition": map[string]any{
			"op": "lt", "left": map[string]any{"source": "record", "name": "published_at"}, "right": map[string]any{"source": "now"},
		}},
	}}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) != 1 || result.Diagnostics[0].Path != "spec.condition" {
		t.Fatalf("unimplemented now source diagnostics=%v", result.Diagnostics)
	}
}

func TestBlockBindingsMustMatchTargetTypes(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "post"}, Spec: map[string]any{"fields": []any{map[string]any{"name": "slug", "type": "slug"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "post_by_slug"}, Spec: map[string]any{"entity": "post", "fields": []any{"id", "slug"}, "exposedFilters": map[string]any{"slug": map[string]any{"name": "slug", "type": "slug"}}}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "detail"}, Spec: map[string]any{"type": "view", "view": "post_by_slug", "inputs": map[string]any{"slug": map[string]any{"type": "uuid"}}, "bindings": map[string]any{"slug": map[string]any{"source": "context", "name": "slug"}}}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) == 0 {
		t.Fatal("mismatched bound input type accepted")
	}
}

func TestResourceListBlockValidatesScopeAndFilters(t *testing.T) {
	defs := []definition.Definition{
		{APIVersion: "bean/v1alpha1", Kind: "Entity", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{"fields": []any{
			map[string]any{"name": "parent_id", "type": "uuid", "required": true},
			map[string]any{"name": "status", "type": "enum", "required": true, "options": []any{"pending", "approved"}},
		}}},
		{APIVersion: "bean/v1alpha1", Kind: "View", Metadata: definition.Metadata{Name: "item_admin"}, Spec: map[string]any{
			"entity": "item", "fields": []any{"id", "parent_id", "status", "created_at", "updated_at", "version"}, "exposedFilters": map[string]any{
				"parent_id": map[string]any{"name": "parent_id", "type": "uuid", "required": true},
				"status":    map[string]any{"name": "status", "type": "enum", "options": []any{"pending", "approved"}},
			},
		}},
		{APIVersion: "bean/v1alpha1", Kind: "AdminResource", Metadata: definition.Metadata{Name: "item"}, Spec: map[string]any{
			"entity": "item", "view": "item_admin", "list": map[string]any{"columns": []any{"id", "status"}, "filters": []any{"parent_id", "status"}},
		}},
		{APIVersion: "bean/v1alpha1", Kind: "Block", Metadata: definition.Metadata{Name: "scoped_items"}, Spec: map[string]any{
			"type": "resource-list", "resource": "item", "policy": "editor_only",
			"inputs":   map[string]any{"parent_id": map[string]any{"type": "uuid", "required": true}},
			"bindings": map[string]any{"parent_id": map[string]any{"source": "context", "name": "id", "required": true}},
			"filters":  []any{"status"}, "defaultFilters": map[string]any{"status": "pending"},
		}},
		{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "editor_only"}, Spec: map[string]any{"readRoles": []any{"editor", "administrator"}}},
	}
	result := compiler.Compile("test", 1, defs)
	if len(result.Diagnostics) != 0 {
		t.Fatalf("valid resource-list diagnostics=%v", result.Diagnostics)
	}
	block := result.App.Blocks["scoped_items"]
	if block.View != "item_admin" || block.Resource != "item" || block.DefaultFilters["status"] != "pending" {
		t.Fatalf("resource-list normalization=%+v", block)
	}

	defs[3].Spec["filters"] = []any{"parent_id"}
	if invalid := compiler.Compile("test", 1, defs); len(invalid.Diagnostics) == 0 {
		t.Fatal("bound input was accepted as an interactive filter")
	}
	defs[3].Spec["filters"] = []any{"status"}
	defs[3].Spec["resource"] = ""
	if invalid := compiler.Compile("test", 1, defs); len(invalid.Diagnostics) == 0 {
		t.Fatal("resource-list without a resource was accepted")
	}
}
