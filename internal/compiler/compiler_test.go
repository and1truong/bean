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
	redacted[1].Spec = map[string]any{"entity": "task", "policy": "task_access", "fields": []any{"id", "title", "status", "position", "parent_id"}}
	redacted = append(redacted, definition.Definition{APIVersion: "bean/v1alpha1", Kind: "Policy", Metadata: definition.Metadata{Name: "task_access"}, Spec: map[string]any{"redact": []any{"title", "status", "parent_id"}}})
	diagnostics := compiler.Compile("test", 1, redacted).Diagnostics
	paths := map[string]bool{}
	for _, diagnostic := range diagnostics {
		paths[diagnostic.Name+":"+diagnostic.Path] = true
	}
	for _, path := range []string{"board:spec.presentation.titleField", "board:spec.presentation.groupField", "tree:spec.presentation.titleField", "tree:spec.presentation.parentField"} {
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
	aggregateSorted := append([]definition.Definition{}, defs...)
	aggregateSorted[1].Spec = map[string]any{"entity": "task", "fields": []any{"id", "title", "status", "position", "parent_id"}, "groupBy": []any{"id", "title", "status", "position", "parent_id"}, "aggregates": []any{map[string]any{"function": "count", "field": "id", "alias": "task_count"}}, "sort": []any{map[string]any{"field": "task_count"}}}
	if diagnostics = compiler.Compile("test", 1, aggregateSorted).Diagnostics; !hasDiagnostic(diagnostics, "board", "spec.presentation.mode") || !hasDiagnostic(diagnostics, "tree", "spec.presentation.mode") {
		t.Fatalf("aggregate-sorted structured View accepted: %v", diagnostics)
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
