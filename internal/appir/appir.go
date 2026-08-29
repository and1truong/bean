package appir

import (
	"encoding/json"
	"time"

	"github.com/beanruntime/bean/internal/expr"
)

type Field struct {
	Name, Label, Type string
	Required, Unique  bool
	Options           []string
	Relation          *Relation
}
type Relation struct{ Entity, Kind string }
type Entity struct {
	Name, Label, Policy       string
	Fields                    []Field
	Owner, Tenant, SoftDelete bool
	Indexes, Unique           [][]string
}
type Display struct{ Type, Route string }
type View struct {
	Name, Entity           string
	Fields                 []string
	Filter                 *expr.Expr
	Sort                   []Sort
	Policy                 string
	DefaultLimit, MaxLimit int
	Displays               map[string]Display
}
type Sort struct {
	Field string
	Desc  bool
}
type Action struct {
	Name, Entity, Operation, Policy string
	StateField                      string
	Input                           map[string]Field
	Steps                           []Step
	Transitions                     map[string][]string
}
type Step struct {
	Op         string
	Values     map[string]any
	Condition  *expr.Expr
	Event, Job string
}
type Policy struct {
	Name                  string
	ReadRoles, WriteRoles []string
	BypassOwnerRoles      []string
	Owner, Tenant         bool
	OwnerOrPublic         bool
	PublicField           string
	PublicValue           any
	Condition             *expr.Expr
	Redact                []string
}
type FormElement struct {
	Name, Type            string
	Required              bool
	Min, Max              *float64
	MinLength, MaxLength  int
	Pattern               string
	Options               []string
	Visible, RequiredWhen *expr.Expr
}
type Webform struct {
	Name, Action string
	Elements     []FormElement
	Steps        [][]string
	Confirmation string
}
type Block struct {
	Name, Type, View, Entity, Webform, Action, Menu, Text string
	Inputs                                                map[string]Field
}
type Region struct {
	Name   string
	Blocks []string
}
type Panel struct {
	Name, Layout string
	Regions      []Region
}
type Page struct {
	Name, Route, Title, Policy, Panel, Description string
	Context                                        map[string]string
}
type Role struct {
	Name        string
	Permissions []string
}
type Menu struct {
	Name  string
	Items []MenuItem
}
type MenuItem struct{ Label, Route, Policy string }
type Job struct{ Name, Action string }
type App struct {
	ReleaseID, AppID string
	Version          int
	CreatedAt        time.Time
	Entities         map[string]Entity
	Views            map[string]View
	Actions          map[string]Action
	Policies         map[string]Policy
	Webforms         map[string]Webform
	Blocks           map[string]Block
	Panels           map[string]Panel
	Pages            map[string]Page
	Roles            map[string]Role
	Menus            map[string]Menu
	Jobs             map[string]Job
	OpenAPI          json.RawMessage
}

func Empty() *App {
	return &App{Entities: map[string]Entity{}, Views: map[string]View{}, Actions: map[string]Action{}, Policies: map[string]Policy{}, Webforms: map[string]Webform{}, Blocks: map[string]Block{}, Panels: map[string]Panel{}, Pages: map[string]Page{}, Roles: map[string]Role{}, Menus: map[string]Menu{}, Jobs: map[string]Job{}}
}
func (a *App) Clone() (*App, error) {
	b, e := json.Marshal(a)
	if e != nil {
		return nil, e
	}
	var out App
	e = json.Unmarshal(b, &out)
	return &out, e
}
