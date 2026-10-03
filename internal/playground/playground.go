// Package playground implements the browser preview bridge: a versioned,
// bounded request surface that runs the real definition loader, compiler,
// and composition pipeline over an in-memory virtual file system. It is
// platform-neutral — the js/wasm entrypoint and native tests share it.
package playground

import (
	"sort"

	"github.com/beanruntime/bean/internal/appir"
	"github.com/beanruntime/bean/internal/compiler"
	"github.com/beanruntime/bean/internal/definition"
	"github.com/beanruntime/bean/internal/page"
	"github.com/beanruntime/bean/internal/sequence"
)

// ProtocolVersion is the bridge contract version. Bump on breaking changes.
const ProtocolVersion = 1

// Stable bridge failure codes. Definition and compiler diagnostics keep their
// BEAN-Exxxx codes; these cover the bridge itself.
const (
	CodeRequest          = "BEAN-P4100"
	CodeNotFound         = "BEAN-P4201"
	CodeProtected        = "BEAN-P4202"
	CodeBackendRequired  = "BEAN-P4203"
	CodeContextMissing   = "BEAN-P4204"
	CodeComposition      = "BEAN-P4205"
	CodeNoCompiledSource = "BEAN-P4206"
)

type Request struct {
	V        int               `json:"v"`
	Op       string            `json:"op"`
	Files    map[string]string `json:"files,omitempty"`
	Manifest string            `json:"manifest,omitempty"`
	Path     string            `json:"path,omitempty"`
	Query    map[string]string `json:"query,omitempty"`
}

type Response struct {
	V           int                     `json:"v"`
	OK          bool                    `json:"ok"`
	Diagnostics []definition.Diagnostic `json:"diagnostics,omitempty"`
	App         *AppSummary             `json:"app,omitempty"`
	Tree        any                     `json:"tree,omitempty"`
	Unsupported []UnsupportedComponent  `json:"unsupported,omitempty"`
	Error       *Failure                `json:"error,omitempty"`
}

// Failure is a bridge-level (non-diagnostic) error with a stable code.
type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (f *Failure) Error() string { return f.Message }

type Route struct {
	Path        string `json:"path"`
	Kind        string `json:"kind"` // page | sequence | display
	Title       string `json:"title,omitempty"`
	Protected   bool   `json:"protected,omitempty"`
	Unsupported string `json:"unsupported,omitempty"`
}

type ThemeSummary struct {
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	Preset      string `json:"preset,omitempty"`
	Accent      string `json:"accent,omitempty"`
}

// UnsupportedComponent counts render nodes the browser profile replaced with
// UnsupportedBlock placeholders.
type UnsupportedComponent struct {
	Component string `json:"component"`
	Reason    string `json:"reason"`
	Count     int    `json:"count"`
}

type AppSummary struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title,omitempty"`
	Theme       *ThemeSummary          `json:"theme,omitempty"`
	Routes      []Route                `json:"routes"`
	Unsupported []UnsupportedComponent `json:"unsupported,omitempty"`
}

// Session holds the last successfully compiled application. Compile replaces
// it only on success so a failed edit preserves the last valid preview.
type Session struct {
	app   *appir.App
	files map[string]string
}

func NewSession() *Session { return &Session{} }

// Handle dispatches one bridge request. It never panics: malformed input is a
// coded Failure.
func (s *Session) Handle(req Request) (response Response) {
	response.V = ProtocolVersion
	defer func() {
		if recovered := recover(); recovered != nil {
			response = Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeComposition, Message: "internal error"}}
		}
	}()
	if req.V != ProtocolVersion {
		return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeRequest, Message: "unsupported protocol version"}}
	}
	switch req.Op {
	case "compile":
		return s.compile(req)
	case "render":
		return s.render(req.Path, req.Query)
	default:
		return Response{V: ProtocolVersion, OK: false, Error: &Failure{Code: CodeRequest, Message: "unsupported op " + req.Op}}
	}
}

func (s *Session) compile(req Request) Response {
	if failure := validateFiles(req.Files, req.Manifest); failure != nil {
		return Response{V: ProtocolVersion, OK: false, Error: failure}
	}
	bundle, diagnostics := definition.LoadFS(fileMap(req.Files), req.Manifest)
	if len(diagnostics) > 0 {
		return Response{V: ProtocolVersion, OK: false, Diagnostics: diagnostics}
	}
	compiled := compiler.Compile("playground", 1, bundle.Definitions)
	if compiled.App == nil {
		return Response{V: ProtocolVersion, OK: false, Diagnostics: compiled.Diagnostics}
	}
	// The session keeps the last clean compile only, so a failed edit never
	// replaces the last valid preview.
	if len(compiled.Diagnostics) == 0 {
		s.app = compiled.App
		s.files = req.Files
	}
	response := Response{
		V:           ProtocolVersion,
		OK:          len(compiled.Diagnostics) == 0,
		Diagnostics: compiled.Diagnostics,
		App:         summarize(compiled.App, bundle.Name),
	}
	response.App.Unsupported = scanUnsupported(compiled.App)
	return response
}

func summarize(a *appir.App, name string) *AppSummary {
	summary := &AppSummary{Name: name, Title: a.Name}
	if a.Theme != nil {
		summary.Theme = &ThemeSummary{Name: a.Theme.Name, DisplayName: a.Theme.DisplayName, Preset: a.Theme.Preset, Accent: a.Theme.Accent}
	}
	for _, p := range a.Pages {
		title := p.Title
		if title == "" {
			title = p.Name
		}
		summary.Routes = append(summary.Routes, Route{Path: p.Route, Kind: "page", Title: title, Protected: page.Protected(a, p)})
	}
	for _, item := range a.Sequences {
		title := item.Title
		if title == "" {
			title = item.Name
		}
		summary.Routes = append(summary.Routes, Route{Path: item.Route, Kind: "sequence", Title: title, Protected: sequence.Protected(a, item)})
	}
	for viewName, viewDefinition := range a.Views {
		for displayName, display := range viewDefinition.Displays {
			if display.Type != "page" {
				continue
			}
			title := display.Title.Text
			if title == "" {
				title = display.Title.Fallback
			}
			if title == "" {
				title = viewName + "/" + displayName
			}
			summary.Routes = append(summary.Routes, Route{Path: display.Route, Kind: "display", Title: title, Unsupported: "requires a Bean backend"})
		}
	}
	sort.Slice(summary.Routes, func(i, j int) bool {
		if summary.Routes[i].Path != summary.Routes[j].Path {
			return summary.Routes[i].Path < summary.Routes[j].Path
		}
		return summary.Routes[i].Kind < summary.Routes[j].Kind
	})
	return summary
}
