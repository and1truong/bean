package httpapi

import (
	"reflect"
	"testing"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
)

func TestContextualCreatesAreEligibleAuthorizedAndStable(t *testing.T) {
	app := appir.Empty()
	app.Policies["editors"] = appir.Policy{Name: "editors", WriteRoles: []string{"editor"}}
	app.Policies["managers"] = appir.Policy{Name: "managers", WriteRoles: []string{"manager"}}
	app.Entities["page"] = appir.Entity{Name: "page", Navigation: &appir.EntityNavigation{Menus: []string{"contents"}}}
	app.Entities["chapter"] = appir.Entity{Name: "chapter", Navigation: &appir.EntityNavigation{Menus: []string{"contents"}}}
	app.Entities["note"] = appir.Entity{Name: "note"}
	app.Actions["create_page"] = appir.Action{Name: "create_page", Entity: "page", Operation: "create", Policy: "editors"}
	app.Actions["create_chapter"] = appir.Action{Name: "create_chapter", Entity: "chapter", Operation: "create", Policy: "editors"}
	app.Actions["create_note"] = appir.Action{Name: "create_note", Entity: "note", Operation: "create", Policy: "editors"}
	app.Actions["create_hidden"] = appir.Action{Name: "create_hidden", Entity: "page", Operation: "create", Policy: "managers"}
	app.AdminResources["pages"] = appir.AdminResource{Name: "pages", Entity: "page", Label: "Page", CreateAction: "create_page"}
	app.AdminResources["chapters"] = appir.AdminResource{Name: "chapters", Entity: "chapter", Label: "Chapter", CreateAction: "create_chapter"}
	app.AdminResources["notes"] = appir.AdminResource{Name: "notes", Entity: "note", Label: "Note", CreateAction: "create_note"}
	app.AdminResources["hidden"] = appir.AdminResource{Name: "hidden", Entity: "page", Label: "Hidden", CreateAction: "create_hidden"}
	app.AdminResources["missing_action"] = appir.AdminResource{Name: "missing_action", Entity: "page", Label: "Missing"}

	got := contextualCreates(app, "contents", beanctx.Request{User: &beanctx.User{Roles: []string{"editor"}}})
	want := []adminContextCreate{{Resource: "chapters", Entity: "chapter", Label: "Chapter"}, {Resource: "pages", Entity: "page", Label: "Page"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("creates=%+v, want %+v", got, want)
	}
	if got = contextualCreates(app, "other", beanctx.Request{User: &beanctx.User{Roles: []string{"editor"}}}); len(got) != 0 {
		t.Fatalf("invalid Menu produced creates: %+v", got)
	}
}
