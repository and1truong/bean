package block

import (
	"fmt"

	"github.com/beanruntime/bean/internal/appir"
	beanctx "github.com/beanruntime/bean/internal/context"
	beanmenu "github.com/beanruntime/bean/internal/menu"
	"github.com/beanruntime/bean/internal/registry"
	beanview "github.com/beanruntime/bean/internal/view"
)

type InputTarget uint8

const (
	NoInputTarget InputTarget = iota
	ViewInputTarget
	WebformInputTarget
	ResourceInputTarget
)

type Specification struct {
	Component                string
	InputTarget              InputTarget
	RequiresResource         bool
	RequiresEditorReadPolicy bool
	RequiresContent          bool
	SupportsPresentation     bool
	DerivesViewFromResource  bool
}

type propertyBuilder func(*appir.App, appir.Block, beanctx.Request, map[string]any) error

type capability struct {
	Specification
	BuildProperties propertyBuilder
}

var capabilities = registry.Must(
	registry.Identity[capability],
	entry("action", Specification{Component: "ActionBlock"}, actionProperties),
	entry("content", Specification{Component: "ContentBlock", RequiresContent: true}, contentProperties),
	entry("entity", Specification{Component: "EntityBlock"}, entityProperties),
	entry("menu", Specification{Component: "MenuBlock"}, menuProperties),
	entry("resource-list", Specification{Component: "ResourceListBlock", InputTarget: ResourceInputTarget, RequiresResource: true, RequiresEditorReadPolicy: true, DerivesViewFromResource: true}, resourceListProperties),
	entry("text", Specification{Component: "TextBlock"}, textProperties),
	entry("view", Specification{Component: "ViewBlock", InputTarget: ViewInputTarget, SupportsPresentation: true}, viewProperties),
	entry("webform", Specification{Component: "WebformBlock", InputTarget: WebformInputTarget}, webformProperties),
)

func entry(name string, specification Specification, builder propertyBuilder) registry.Entry[capability] {
	return registry.Entry[capability]{Name: name, Value: capability{Specification: specification, BuildProperties: builder}}
}

func Lookup(name string) (Specification, bool) {
	registered, exists := capabilities.Lookup(name)
	return registered.Specification, exists
}

func Names() []string {
	return capabilities.Names()
}

func textProperties(_ *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["text"] = block.Text
	return nil
}

func contentProperties(_ *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["content"] = block.Content
	return nil
}

func viewProperties(app *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["view"] = block.View
	presentation := block.Presentation
	if display, exists := app.Views[block.View].Displays[block.Display]; exists {
		if len(display.Renderer.SearchFields) == 0 {
			display.Renderer.SearchFields = append([]string{}, app.Views[block.View].Search.Fields...)
		}
		props["display"] = display
		props["displayName"] = block.Display
		displays := map[string]appir.Display{}
		for name, candidate := range app.Views[block.View].Displays {
			if candidate.Type == "block" {
				if len(candidate.Renderer.SearchFields) == 0 {
					candidate.Renderer.SearchFields = append([]string{}, app.Views[block.View].Search.Fields...)
				}
				displays[name] = candidate
			}
		}
		props["displays"] = displays
		props["filters"] = app.Views[block.View].ExposedFilters
		presentation = display.Renderer.Presentation()
	}
	props["presentation"] = presentation
	props["maxRows"] = app.Views[block.View].MaxLimit
	props["searchFields"] = append([]string{}, app.Views[block.View].Search.Fields...)
	props["fieldTypes"] = beanview.FieldTypes(app, app.Views[block.View])
	props["formattedFields"] = formattedFields(app, block)
	props["fileFields"] = fileFields(app, block)
	return nil
}

func entityProperties(_ *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["entity"] = block.Entity
	return nil
}

func webformProperties(app *appir.App, block appir.Block, request beanctx.Request, props map[string]any) error {
	props["webform"] = block.Webform
	form := app.Webforms[block.Webform]
	boundElements, err := bindFormElements(form.Elements, request)
	if err != nil {
		return fmt.Errorf("render Webform conditions: %w", err)
	}
	form.Elements = boundElements
	props["form"] = form
	return nil
}

func actionProperties(_ *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["action"] = block.Action
	return nil
}

func menuProperties(app *appir.App, block appir.Block, request beanctx.Request, props map[string]any) error {
	definition := app.Menus[block.Menu]
	props["menu"] = block.Menu
	props["profile"] = definition.Profile
	props["ownerEntity"] = ""
	if definition.Owner != nil {
		props["ownerEntity"] = definition.Owner.Entity
	}
	props["items"] = beanmenu.StaticTree(app, definition, request)
	return nil
}

func resourceListProperties(_ *appir.App, block appir.Block, _ beanctx.Request, props map[string]any) error {
	props["resource"] = block.Resource
	props["view"] = block.View
	props["filters"] = block.Filters
	props["defaultFilters"] = block.DefaultFilters
	return nil
}
